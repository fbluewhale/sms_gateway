package channel

import "context"

type ChannelRepository interface {
	GetByName(ctx context.Context, name string) (*Channel, error)
	Create(ctx context.Context, channel *Channel) error
	List(ctx context.Context) ([]Channel, error)
}
