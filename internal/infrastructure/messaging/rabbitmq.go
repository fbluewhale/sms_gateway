package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	app "sms_gateway/internal/application/sms"
	"sms_gateway/internal/domain/wallet"
	"sms_gateway/internal/infrastructure/persistence"
)

const Exchange = "sms.delivery"

func Declare(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	for _, line := range []string{"express", "normal"} {
		q, err := ch.QueueDeclare("sms."+line, true, false, false, false, amqp.Table{"x-queue-type": "quorum"})
		if err != nil {
			return err
		}
		if err := ch.QueueBind(q.Name, line, Exchange, false, nil); err != nil {
			return err
		}
	}
	return nil
}

type Dispatcher struct {
	db   *gorm.DB
	conn *amqp.Connection
}

func NewDispatcher(db *gorm.DB, brokerURL string) (*Dispatcher, error) {
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	return &Dispatcher{db: db, conn: conn}, nil
}

func (d *Dispatcher) Close() error { return d.conn.Close() }

func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		published, err := d.publishFairBatch(ctx)
		if err != nil {
			return err
		}
		if published > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) publishFairBatch(ctx context.Context) (int, error) {
	total := 0
	// A bounded batch per routing key prevents a hot line from hiding the other
	// line behind an unbounded prefix of older outbox rows.
	for _, line := range []string{"express", "normal"} {
		published, err := d.publishLineBatch(ctx, line, 100)
		if err != nil {
			return total, err
		}
		total += published
	}
	return total, nil
}

func (d *Dispatcher) publishLineBatch(ctx context.Context, line string, limit int) (int, error) {
	var rows []persistence.SMSOutboxModel
	if err := d.db.WithContext(ctx).Where("published_at IS NULL AND routing_key = ?", line).Order("id").Limit(limit).Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	ch, err := d.conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()
	if err := Declare(ch); err != nil {
		return 0, err
	}
	if err := ch.Confirm(false); err != nil {
		return 0, err
	}
	confirms := make([]*amqp.DeferredConfirmation, 0, len(rows))
	for _, row := range rows {
		dc, err := ch.PublishWithDeferredConfirmWithContext(ctx, Exchange, row.RoutingKey, true, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: row.MessageID, Timestamp: row.CreatedAt, Body: row.Payload,
		})
		if err != nil {
			return 0, err
		}
		confirms = append(confirms, dc)
	}
	// Publish the complete line batch before waiting. RabbitMQ can pipeline the
	// confirms, avoiding one network round trip per SMS at high request rates.
	for i, row := range rows {
		if confirms[i] == nil || !confirms[i].Wait() {
			return 0, fmt.Errorf("publish not confirmed: %s", row.MessageID)
		}
		now := time.Now().UTC()
		if err := d.db.WithContext(ctx).Model(&persistence.SMSOutboxModel{}).Where("id = ? AND published_at IS NULL", row.ID).Update("published_at", now).Error; err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

type Sender interface {
	Send(context.Context, app.DeliveryEvent) error
}

type Worker struct {
	db           *gorm.DB
	conn         *amqp.Connection
	line         string
	prefetch     int
	concurrency  int
	sender       Sender
	reservations app.ReservationStore
}

func NewWorker(db *gorm.DB, brokerURL, line string, prefetch, concurrency int, sender Sender) (*Worker, error) {
	return newWorker(db, brokerURL, line, prefetch, concurrency, sender, nil)
}

func NewWorkerWithReservation(db *gorm.DB, brokerURL, line string, prefetch, concurrency int, sender Sender, reservations app.ReservationStore) (*Worker, error) {
	return newWorker(db, brokerURL, line, prefetch, concurrency, sender, reservations)
}

func newWorker(db *gorm.DB, brokerURL, line string, prefetch, concurrency int, sender Sender, reservations app.ReservationStore) (*Worker, error) {
	if line != "express" && line != "normal" {
		return nil, fmt.Errorf("invalid worker line %q", line)
	}
	if concurrency < 1 || prefetch < concurrency {
		return nil, fmt.Errorf("prefetch (%d) must be at least concurrency (%d)", prefetch, concurrency)
	}
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		return nil, err
	}
	return &Worker{db: db, conn: conn, line: line, prefetch: prefetch, concurrency: concurrency, sender: sender, reservations: reservations}, nil
}

func (w *Worker) Close() error { return w.conn.Close() }

func (w *Worker) Run(ctx context.Context) error {
	ch, err := w.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := Declare(ch); err != nil {
		return err
	}
	if err := ch.Qos(w.prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume("sms."+w.line, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	errCh := make(chan error, w.concurrency)
	for i := 0; i < w.concurrency; i++ {
		go w.consume(ctx, deliveries, errCh)
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (w *Worker) consume(ctx context.Context, deliveries <-chan amqp.Delivery, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				select {
				case errCh <- fmt.Errorf("delivery channel closed"):
				default:
				}
				return
			}
			if err := w.process(ctx, d.Body); err != nil {
				_ = d.Nack(false, true)
			} else {
				_ = d.Ack(false)
			}
		}
	}
}

func (w *Worker) process(ctx context.Context, body []byte) error {
	var event app.DeliveryEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	if w.reservations == nil {
		return w.processLegacy(ctx, event)
	}
	return w.processReserved(ctx, event)
}

func (w *Worker) processReserved(ctx context.Context, event app.DeliveryEvent) error {
	terminal, err := w.beginDelivery(ctx, event)
	if err != nil || terminal {
		if terminal {
			return w.reservations.Commit(ctx, event.MessageID)
		}
		return err
	}
	if !event.DeadlineAt.IsZero() && time.Now().UTC().After(event.DeadlineAt) {
		if err := w.reservations.Refund(ctx, event.WalletID, event.MessageID); err != nil {
			return err
		}
		return w.markDelivery(ctx, event.MessageID, "refunded", "express SMS SLA expired before provider attempt", nil)
	}
	reserved, err := w.reservations.IsReserved(ctx, event.MessageID)
	if err != nil {
		return err
	}
	if !reserved {
		return w.markDelivery(ctx, event.MessageID, "refunded", "credit reservation is missing", nil)
	}
	if err := w.sender.Send(ctx, event); err != nil {
		if refundErr := w.reservations.Refund(ctx, event.WalletID, event.MessageID); refundErr != nil {
			return refundErr
		}
		return w.markDelivery(ctx, event.MessageID, "refunded", err.Error(), nil)
	}
	if err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := persistence.NewPostgresWalletRepository(tx)
		amount, err := wallet.MoneyFromUnits(event.CostUnits)
		if err != nil {
			return fmt.Errorf("invalid reserved amount: %w", err)
		}
		if _, err := repo.DeductAndRecord(ctx, event.WalletID, amount, event.MessageID, "SMS delivery"); err != nil {
			return err
		}
		return tx.Model(&persistence.SMSDeliveryModel{}).Where("message_id = ?", event.MessageID).Updates(map[string]any{"status": "delivered", "delivered_at": time.Now().UTC(), "attempts": gorm.Expr("attempts + 1")}).Error
	}); err != nil {
		if refundErr := w.reservations.Refund(ctx, event.WalletID, event.MessageID); refundErr != nil {
			return fmt.Errorf("commit debit: %w; refund reservation: %v", err, refundErr)
		}
		return w.markDelivery(ctx, event.MessageID, "refunded", err.Error(), nil)
	}
	return w.reservations.Commit(ctx, event.MessageID)
}

func (w *Worker) beginDelivery(ctx context.Context, event app.DeliveryEvent) (bool, error) {
	var terminal bool
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record persistence.SMSDeliveryModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "message_id = ?", event.MessageID).Error
		if err == gorm.ErrRecordNotFound {
			record = persistence.SMSDeliveryModel{MessageID: event.MessageID, WalletID: event.WalletID, Destination: string(event.Destination), Message: event.Message, Line: string(event.Line), ChannelName: event.ChannelName, Status: "processing"}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if record.Status == "delivered" || record.Status == "refunded" {
			terminal = true
			return nil
		}
		return nil
	})
	return terminal, err
}

func (w *Worker) markDelivery(ctx context.Context, messageID, status, lastError string, deliveredAt *time.Time) error {
	updates := map[string]any{"status": status, "last_error": lastError, "attempts": gorm.Expr("attempts + 1")}
	if deliveredAt != nil {
		updates["delivered_at"] = deliveredAt
	}
	return w.db.WithContext(ctx).Model(&persistence.SMSDeliveryModel{}).Where("message_id = ?", messageID).Updates(updates).Error
}

func (w *Worker) processLegacy(ctx context.Context, event app.DeliveryEvent) error {
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record persistence.SMSDeliveryModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "message_id = ?", event.MessageID).Error
		if err == gorm.ErrRecordNotFound {
			record = persistence.SMSDeliveryModel{MessageID: event.MessageID, WalletID: event.WalletID, Destination: string(event.Destination), Message: event.Message, Line: string(event.Line), ChannelName: event.ChannelName, Status: "processing"}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if record.Status == "delivered" || record.Status == "refunded" {
			return nil
		}
		if !event.DeadlineAt.IsZero() && time.Now().UTC().After(event.DeadlineAt) {
			return w.refund(tx, ctx, &record, event, "express SMS SLA expired before provider attempt")
		}
		if err := w.sender.Send(ctx, event); err != nil {
			return w.refund(tx, ctx, &record, event, err.Error())
		}
		return tx.Model(&record).Updates(map[string]any{"status": "delivered", "delivered_at": time.Now().UTC(), "attempts": gorm.Expr("attempts + 1")}).Error
	})
}

func (w *Worker) refund(tx *gorm.DB, ctx context.Context, record *persistence.SMSDeliveryModel, event app.DeliveryEvent, reason string) error {
	amount, err := wallet.MoneyFromUnits(event.CostUnits)
	if err != nil {
		return err
	}
	repo := persistence.NewPostgresWalletRepository(tx)
	if _, err := repo.CreditAndRecord(ctx, event.WalletID, amount, event.MessageID, "Refund for failed SMS delivery"); err != nil {
		return err
	}
	return tx.Model(record).Updates(map[string]any{"status": "refunded", "last_error": reason, "attempts": gorm.Expr("attempts + 1")}).Error
}
