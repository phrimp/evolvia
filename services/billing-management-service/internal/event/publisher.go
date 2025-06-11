package event

import (
	"billing-management-service/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	PublishPlanEvent(event *PlanEvent) error
	PublishSubscriptionEvent(event *SubscriptionEvent) error
	Close() error
}

type EventPublisher struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	exchangeName string
	enabled      bool
}

func NewEventPublisher(rabbitURI string) (*EventPublisher, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event publishing is disabled")
		return &EventPublisher{
			enabled: false,
		}, nil
	}

	// Connect to RabbitMQ
	conn, err := amqp091.Dial(rabbitURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Declare the exchange
	exchangeName := "billing.events"
	err = channel.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &EventPublisher{
		conn:         conn,
		channel:      channel,
		exchangeName: exchangeName,
		enabled:      true,
	}, nil
}

func (p *EventPublisher) publishEvent(ctx context.Context, routingKey string, event any) error {
	if !p.enabled {
		log.Printf("Event publishing is disabled, skipping event: %s", routingKey)
		return nil
	}

	// Convert event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create publishing context with timeout
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Publish the event
	err = p.channel.PublishWithContext(
		pubCtx,
		p.exchangeName, // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("Published event: %s", routingKey)
	return nil
}

// Plan event publishing
func (p *EventPublisher) PublishPlanEvent(event *PlanEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

// Subscription event publishing
func (p *EventPublisher) PublishSubscriptionEvent(event *SubscriptionEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

func (p *EventPublisher) Close() error {
	if !p.enabled {
		return nil
	}

	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		}
	}

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ connection: %w", err)
		}
	}

	return nil
}

// Event factory functions for common events

// CreatePlanCreatedEvent creates a plan created event
func CreatePlanCreatedEvent(planID string, planType models.PlanType) *PlanEvent {
	return &PlanEvent{
		EventType: EventTypePlanCreated,
		PlanID:    planID,
		PlanType:  planType,
		Timestamp: time.Now().Unix(),
	}
}

// CreatePlanUpdatedEvent creates a plan updated event
func CreatePlanUpdatedEvent(planID string, planType models.PlanType, changedFields []string, oldValues, newValues map[string]any) *PlanEvent {
	return &PlanEvent{
		EventType:     EventTypePlanUpdated,
		PlanID:        planID,
		PlanType:      planType,
		Timestamp:     time.Now().Unix(),
		ChangedFields: changedFields,
		OldValues:     oldValues,
		NewValues:     newValues,
	}
}

// CreatePlanDeletedEvent creates a plan deleted event
func CreatePlanDeletedEvent(planID string, planType models.PlanType) *PlanEvent {
	return &PlanEvent{
		EventType: EventTypePlanDeleted,
		PlanID:    planID,
		PlanType:  planType,
		Timestamp: time.Now().Unix(),
	}
}

// CreatePlanActivatedEvent creates a plan activated event
func CreatePlanActivatedEvent(planID string, planType models.PlanType) *PlanEvent {
	return &PlanEvent{
		EventType: EventTypePlanActivated,
		PlanID:    planID,
		PlanType:  planType,
		Timestamp: time.Now().Unix(),
		OldValues: map[string]any{"isActive": false},
		NewValues: map[string]any{"isActive": true},
	}
}

// CreatePlanDeactivatedEvent creates a plan deactivated event
func CreatePlanDeactivatedEvent(planID string, planType models.PlanType) *PlanEvent {
	return &PlanEvent{
		EventType: EventTypePlanDeactivated,
		PlanID:    planID,
		PlanType:  planType,
		Timestamp: time.Now().Unix(),
		OldValues: map[string]any{"isActive": true},
		NewValues: map[string]any{"isActive": false},
	}
}

// CreateSubscriptionCreatedEvent creates a subscription created event
func CreateSubscriptionCreatedEvent(subscriptionID, userID, planID string, status models.SubscriptionStatus) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionCreated,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         status,
		Timestamp:      time.Now().Unix(),
	}
}

// CreateSubscriptionUpdatedEvent creates a subscription updated event
func CreateSubscriptionUpdatedEvent(subscriptionID, userID, planID string, status models.SubscriptionStatus, changedFields []string, oldValues, newValues map[string]any) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionUpdated,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         status,
		Timestamp:      time.Now().Unix(),
		ChangedFields:  changedFields,
		OldValues:      oldValues,
		NewValues:      newValues,
	}
}

// CreateSubscriptionCanceledEvent creates a subscription canceled event
func CreateSubscriptionCanceledEvent(subscriptionID, userID, planID string, oldStatus models.SubscriptionStatus, reason string) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionCanceled,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         models.SubscriptionStatusCanceled,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": oldStatus},
		NewValues:      map[string]any{"status": models.SubscriptionStatusCanceled, "reason": reason},
	}
}

// CreateSubscriptionSuspendedEvent creates a subscription suspended event
func CreateSubscriptionSuspendedEvent(subscriptionID, userID, planID string, oldStatus models.SubscriptionStatus, reason string) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionSuspended,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         models.SubscriptionStatusSuspended,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": oldStatus},
		NewValues:      map[string]any{"status": models.SubscriptionStatusSuspended, "reason": reason},
	}
}

// CreateSubscriptionReactivatedEvent creates a subscription reactivated event
func CreateSubscriptionReactivatedEvent(subscriptionID, userID, planID string) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionReactivated,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         models.SubscriptionStatusActive,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": models.SubscriptionStatusSuspended},
		NewValues:      map[string]any{"status": models.SubscriptionStatusActive},
	}
}

// CreateSubscriptionRenewedEvent creates a subscription renewed event
func CreateSubscriptionRenewedEvent(subscriptionID, userID, planID string) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeSubscriptionRenewed,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         models.SubscriptionStatusActive,
		Timestamp:      time.Now().Unix(),
	}
}

// CreateTrialExpiredEvent creates a trial expired event
func CreateTrialExpiredEvent(subscriptionID, userID, planID string, newStatus models.SubscriptionStatus) *SubscriptionEvent {
	return &SubscriptionEvent{
		EventType:      EventTypeTrialExpired,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
		Status:         newStatus,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": models.SubscriptionStatusTrial},
		NewValues:      map[string]any{"status": newStatus},
	}
}
