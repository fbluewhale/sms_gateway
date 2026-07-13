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
		if err := d.publishBatch(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) publishBatch(ctx context.Context) error {
	var rows []persistence.SMSOutboxModel
	if err := d.db.WithContext(ctx).Where("published_at IS NULL").Order("id").Limit(100).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	ch, err := d.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := Declare(ch); err != nil {
		return err
	}
	if err := ch.Confirm(false); err != nil {
		return err
	}
	for _, row := range rows {
		dc, err := ch.PublishWithDeferredConfirmWithContext(ctx, Exchange, row.RoutingKey, true, false, amqp.Publishing{
			DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: row.MessageID, Timestamp: row.CreatedAt, Body: row.Payload,
		})
		if err != nil {
			return err
		}
		if dc == nil || !dc.Wait() {
			return fmt.Errorf("publish not confirmed: %s", row.MessageID)
		}
		now := time.Now().UTC()
		if err := d.db.WithContext(ctx).Model(&persistence.SMSOutboxModel{}).Where("id = ? AND published_at IS NULL", row.ID).Update("published_at", now).Error; err != nil {
			return err
		}
	}
	return nil
}

type Sender interface {
	Send(context.Context, app.DeliveryEvent) error
}

type Worker struct {
	db       *gorm.DB
	conn     *amqp.Connection
	line     string
	prefetch int
	sender   Sender
}

func NewWorker(db *gorm.DB, brokerURL, line string, prefetch int, sender Sender) (*Worker, error) {
	if line != "express" && line != "normal" {
		return nil, fmt.Errorf("invalid worker line %q", line)
	}
	conn, err := amqp.Dial(brokerURL)
	if err != nil {
		return nil, err
	}
	return &Worker{db: db, conn: conn, line: line, prefetch: prefetch, sender: sender}, nil
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
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
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record persistence.SMSDeliveryModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, "message_id = ?", event.MessageID).Error
		if err == gorm.ErrRecordNotFound {
			record = persistence.SMSDeliveryModel{MessageID: event.MessageID, Status: "processing"}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if record.Status == "delivered" || record.Status == "refunded" {
			return nil
		}
		if err := w.sender.Send(ctx, event); err != nil {
			amount, moneyErr := wallet.MoneyFromUnits(event.CostUnits)
			if moneyErr != nil {
				return moneyErr
			}
			repo := persistence.NewPostgresWalletRepository(tx)
			if _, refundErr := repo.CreditAndRecord(ctx, event.WalletID, amount, event.MessageID, "Refund for failed SMS delivery"); refundErr != nil {
				return refundErr
			}
			return tx.Model(&record).Updates(map[string]any{"status": "refunded", "last_error": err.Error(), "attempts": gorm.Expr("attempts + 1")}).Error
		}
		now := time.Now().UTC()
		return tx.Model(&record).Updates(map[string]any{"status": "delivered", "delivered_at": now, "attempts": gorm.Expr("attempts + 1")}).Error
	})
}
