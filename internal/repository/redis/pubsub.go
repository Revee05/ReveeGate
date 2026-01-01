package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// PubSub channels
const (
	ChannelDonationsNew = "donations:new"
)

// PubSub provides Redis pub/sub functionality
type PubSub struct {
	client *redis.Client
	logger *slog.Logger
}

// NewPubSub creates a new Redis pub/sub client
func NewPubSub(client *redis.Client, logger *slog.Logger) *PubSub {
	return &PubSub{
		client: client,
		logger: logger,
	}
}

// Publish publishes a message to a channel
func (p *PubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := p.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	p.logger.Debug("published message", "channel", channel)
	return nil
}

// Subscribe subscribes to a channel and returns a channel for receiving messages
func (p *PubSub) Subscribe(ctx context.Context, channels ...string) *Subscription {
	pubsub := p.client.Subscribe(ctx, channels...)
	return &Subscription{
		pubsub: pubsub,
		logger: p.logger,
	}
}

// Subscription represents a subscription to Redis channels
type Subscription struct {
	pubsub *redis.PubSub
	logger *slog.Logger
}

// Channel returns the message channel
func (s *Subscription) Channel() <-chan *redis.Message {
	return s.pubsub.Channel()
}

// Close closes the subscription
func (s *Subscription) Close() error {
	return s.pubsub.Close()
}

// ReceiveMessage receives a single message (blocking)
func (s *Subscription) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	return s.pubsub.ReceiveMessage(ctx)
}

// DonationEvent represents a donation event for pub/sub
type DonationEvent struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	DonorName string `json:"donor_name"`
	Message   string `json:"message"`
	Amount    int64  `json:"amount"`
	PaidAt    string `json:"paid_at"`
}

// PublishDonationEvent publishes a new donation event
func (p *PubSub) PublishDonationEvent(ctx context.Context, event *DonationEvent) error {
	return p.Publish(ctx, ChannelDonationsNew, event)
}

// NewDonationEvent creates a new donation event
func NewDonationEvent(id, donorName, message string, amount int64, paidAt string) *DonationEvent {
	return &DonationEvent{
		Type:      "new_donation",
		ID:        id,
		DonorName: donorName,
		Message:   message,
		Amount:    amount,
		PaidAt:    paidAt,
	}
}

// ParseDonationEvent parses a donation event from JSON
func ParseDonationEvent(data []byte) (*DonationEvent, error) {
	var event DonationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse donation event: %w", err)
	}
	return &event, nil
}

// SubscribeDonations subscribes to donation events with a callback
func (p *PubSub) SubscribeDonations(ctx context.Context, callback func(*DonationEvent)) error {
	sub := p.Subscribe(ctx, ChannelDonationsNew)

	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-sub.Channel():
				if msg == nil {
					return
				}
				event, err := ParseDonationEvent([]byte(msg.Payload))
				if err != nil {
					p.logger.Error("failed to parse donation event", "error", err)
					continue
				}
				callback(event)
			}
		}
	}()

	return nil
}
