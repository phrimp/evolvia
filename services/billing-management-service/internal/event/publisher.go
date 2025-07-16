package event

import (
	"billing-management-service/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

// Utility functions for processing features and permissions

// ParseFeaturePermissions extracts permissions from feature description
func ParseFeaturePermissions(description string) []string {
	if description == "" {
		return []string{}
	}

	// Split by comma and trim whitespace
	permissions := strings.Split(description, ",")
	var cleanPermissions []string

	for _, perm := range permissions {
		trimmed := strings.TrimSpace(perm)
		if trimmed != "" {
			cleanPermissions = append(cleanPermissions, trimmed)
		}
	}

	return cleanPermissions
}

// ProcessFeaturesForEvent converts model features to event features with parsed permissions
func ProcessFeaturesForEvent(features []models.Feature) []FeatureDetail {
	eventFeatures := make([]FeatureDetail, len(features))

	for i, feature := range features {
		eventFeatures[i] = FeatureDetail{
			Name:        feature.Name,
			Description: feature.Description,
			Enabled:     feature.Enabled,
			Permissions: ParseFeaturePermissions(feature.Description),
		}
	}

	return eventFeatures
}

// GenerateRoleMetadata creates role metadata for plan events
func GenerateRoleMetadata(plan *models.Plan, eventFeatures []FeatureDetail) *RoleCreationMetadata {
	// Generate suggested role name
	roleName := fmt.Sprintf("%s-plan-role", strings.ToLower(string(plan.PlanType)))

	// Collect all permissions and create feature permission map
	var allPermissions []string
	featurePermissionMap := make(map[string][]string)

	for _, feature := range eventFeatures {
		if feature.Enabled && len(feature.Permissions) > 0 {
			featurePermissionMap[feature.Name] = feature.Permissions
			allPermissions = append(allPermissions, feature.Permissions...)
		}
	}

	// Remove duplicates from allPermissions
	allPermissions = removeDuplicates(allPermissions)

	// Generate role description
	description := fmt.Sprintf("Auto-generated role for %s plan (%s) with %d permissions across %d features",
		plan.Name, plan.PlanType, len(allPermissions), len(featurePermissionMap))

	return &RoleCreationMetadata{
		SuggestedRoleName:    roleName,
		AllPermissions:       allPermissions,
		FeaturePermissionMap: featurePermissionMap,
		RoleDescription:      description,
	}
}

// GenerateUserRoleMetadata creates user role metadata for subscription events
func GenerateUserRoleMetadata(planName string, planType models.PlanType, permissions []string) *UserRoleMetadata {
	roleName := fmt.Sprintf("%s-plan-role", strings.ToLower(string(planType)))

	return &UserRoleMetadata{
		ShouldAssignRole: len(permissions) > 0,
		RoleName:         roleName,
		Permissions:      permissions,
	}
}

// Helper function to remove duplicates
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// Enhanced event factory functions

// CreatePlanCreatedEvent creates an enhanced plan created event with feature details
func CreatePlanCreatedEvent(plan *models.Plan) *PlanEvent {
	eventFeatures := ProcessFeaturesForEvent(plan.Features)
	roleMetadata := GenerateRoleMetadata(plan, eventFeatures)

	return &PlanEvent{
		EventType:    EventTypePlanCreated,
		PlanID:       plan.ID.Hex(),
		PlanName:     plan.Name,
		PlanType:     plan.PlanType,
		Price:        plan.Price,
		Currency:     plan.Currency,
		BillingCycle: plan.BillingCycle,
		Features:     eventFeatures,
		IsActive:     plan.IsActive,
		TrialDays:    plan.TrialDays,
		Timestamp:    time.Now().Unix(),
		RoleMetadata: roleMetadata,
	}
}

// CreatePlanUpdatedEvent creates an enhanced plan updated event
func CreatePlanUpdatedEvent(plan *models.Plan, changedFields []string, oldValues, newValues map[string]any) *PlanEvent {
	eventFeatures := ProcessFeaturesForEvent(plan.Features)
	roleMetadata := GenerateRoleMetadata(plan, eventFeatures)

	return &PlanEvent{
		EventType:     EventTypePlanUpdated,
		PlanID:        plan.ID.Hex(),
		PlanName:      plan.Name,
		PlanType:      plan.PlanType,
		Price:         plan.Price,
		Currency:      plan.Currency,
		BillingCycle:  plan.BillingCycle,
		Features:      eventFeatures,
		IsActive:      plan.IsActive,
		TrialDays:     plan.TrialDays,
		Timestamp:     time.Now().Unix(),
		ChangedFields: changedFields,
		OldValues:     oldValues,
		NewValues:     newValues,
		RoleMetadata:  roleMetadata,
	}
}

// CreateSubscriptionCreatedEvent creates an enhanced subscription created event
func CreateSubscriptionCreatedEvent(subscription *models.Subscription, plan *models.Plan) *SubscriptionEvent {
	eventFeatures := ProcessFeaturesForEvent(plan.Features)

	// Extract permissions from enabled features
	var permissions []string
	for _, feature := range eventFeatures {
		if feature.Enabled {
			permissions = append(permissions, feature.Permissions...)
		}
	}
	permissions = removeDuplicates(permissions)

	userRoleMetadata := GenerateUserRoleMetadata(plan.Name, plan.PlanType, permissions)

	return &SubscriptionEvent{
		EventType:        EventTypeSubscriptionCreated,
		SubscriptionID:   subscription.ID.Hex(),
		UserID:           subscription.UserID,
		PlanID:           subscription.PlanID.Hex(),
		PlanName:         plan.Name,
		PlanType:         plan.PlanType,
		Status:           subscription.Status,
		Timestamp:        time.Now().Unix(),
		UserRoleMetadata: userRoleMetadata,
	}
}
