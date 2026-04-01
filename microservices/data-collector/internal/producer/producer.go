package producer

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer wraps a kafka writer.
type Producer struct {
	writer *kafka.Writer
}

// New creates a Kafka producer connected to the given broker list (comma-separated).
func New(brokers string) *Producer {
	brokerList := strings.Split(brokers, ",")
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokerList...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}
	return &Producer{writer: w}
}

// Publish encodes v as JSON and sends it to the given topic.
func (p *Producer) Publish(ctx context.Context, topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	msg := kafka.Message{
		Topic: topic,
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("producer: failed to write to %s: %v", topic, err)
		return err
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
