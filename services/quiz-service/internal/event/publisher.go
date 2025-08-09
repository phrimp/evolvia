package event

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/streadway/amqp"
)

type EventPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

func NewEventPublisher(amqpURL, exchange string) (*EventPublisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	err = ch.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return &EventPublisher{conn: conn, channel: ch, exchange: exchange}, nil
}

func (p *EventPublisher) Publish(eventType string, payload interface{}) error {
	event := map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Log to console
	fmt.Printf("[EVENT] %s: %v\n", eventType, payload)

	// Log to file
	f, ferr := os.OpenFile("event.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[EVENT] %s: %v\n", eventType, payload))
	}

	// Use the event type as the routing key for topic exchange
	return p.channel.Publish(
		p.exchange,
		eventType, // routing key
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func (p *EventPublisher) Close() {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
