package events

import (
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	utils "proto-gen/utils"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Consumer defines the interface for event consumption
type Consumer interface {
	Start() error
	Close() error
}

// EventConsumer implements the Consumer interface using RabbitMQ
type EventConsumer struct {
	conn           *amqp091.Connection
	channel        *amqp091.Channel
	queueName      string
	redisRepo      *repository.RedisRepo
	userRepo       *repository.UserAuthRepository
	roleRepo       *repository.RoleRepository
	userRoleRepo   *repository.UserRoleRepository
	permissionRepo *repository.PermissionRepository
	eventPublisher *EventPublisher
	shutdown       chan struct{}
	wg             sync.WaitGroup
	enabled        bool
}

// Exchange configuration
type ExchangeConfig struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp091.Table
}

// Binding configuration
type BindingConfig struct {
	Exchange   string
	RoutingKey string
}

// NewEventConsumer creates a new event consumer
func NewEventConsumer(rabbitURI string, redisRepo *repository.RedisRepo, userRepo *repository.UserAuthRepository, roleRepo *repository.RoleRepository, permissionRepo *repository.PermissionRepository, userRoleRepo *repository.UserRoleRepository, eventPublisher *EventPublisher) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consumption is disabled")
		return &EventConsumer{
			redisRepo:      redisRepo,
			userRepo:       userRepo,
			roleRepo:       roleRepo,
			userRoleRepo:   userRoleRepo,
			permissionRepo: permissionRepo,
			eventPublisher: eventPublisher,
			shutdown:       make(chan struct{}),
			enabled:        false,
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

	// Set QoS/prefetch
	err = channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return &EventConsumer{
		conn:           conn,
		channel:        channel,
		queueName:      "auth-service-events",
		redisRepo:      redisRepo,
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		userRoleRepo:   userRoleRepo,
		permissionRepo: permissionRepo,
		eventPublisher: eventPublisher,
		shutdown:       make(chan struct{}),
		enabled:        true,
	}, nil
}

// Start starts consuming events
func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consumption is disabled, not starting consumer")
		return nil
	}

	// Define all exchanges this service needs to consume from
	exchanges := []ExchangeConfig{
		{
			Name:       "profile-events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
		{
			Name:       "google.events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
		{
			Name:       "billing.events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
	}

	// Declare all exchanges
	for _, exchange := range exchanges {
		err := c.channel.ExchangeDeclare(
			exchange.Name,
			exchange.Type,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.Internal,
			exchange.NoWait,
			exchange.Args,
		)
		if err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}
		log.Printf("Declared exchange: %s", exchange.Name)
	}

	// Declare the queue
	_, err := c.channel.QueueDeclare(
		c.queueName, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	log.Printf("Declared queue: %s", c.queueName)

	// Define all bindings
	bindings := []BindingConfig{
		// Profile events
		{Exchange: "profile-events", RoutingKey: "profile.updated"},
		{Exchange: "profile-events", RoutingKey: "profile.deleted"},

		// Google events
		{Exchange: "google.events", RoutingKey: "google.login"},
		{Exchange: "google.events", RoutingKey: "google.login.request"},

		// Billing events
		{Exchange: "billing.events", RoutingKey: "plan.created"},
		{Exchange: "billing.events", RoutingKey: "plan.updated"},
		{Exchange: "billing.events", RoutingKey: "plan.deleted"},
		{Exchange: "billing.events", RoutingKey: "subscription.updated"},
		{Exchange: "google.events", RoutingKey: "email.verification.success"},
	}

	// Bind the queue to all exchanges with their routing keys
	for _, binding := range bindings {
		err := c.channel.QueueBind(
			c.queueName,        // queue name
			binding.RoutingKey, // routing key
			binding.Exchange,   // exchange
			false,              // no-wait
			nil,                // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue to exchange %s with key %s: %w",
				binding.Exchange, binding.RoutingKey, err)
		}
		log.Printf("Bound queue %s to exchange %s with routing key %s",
			c.queueName, binding.Exchange, binding.RoutingKey)
	}

	// Start consuming messages
	msgs, err := c.channel.Consume(
		c.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.consume(msgs)
	}()

	log.Println("Event consumer started and listening to multiple exchanges")
	return nil
}

// consume handles incoming messages
func (c *EventConsumer) consume(msgs <-chan amqp091.Delivery) {
	for {
		select {
		case <-c.shutdown:
			log.Println("Stopping event consumer")
			return
		case msg, ok := <-msgs:
			if !ok {
				log.Println("Message channel closed, reconnecting...")
				// Attempt to reconnect
				time.Sleep(5 * time.Second)
				return
			}

			// Process the message
			err := c.processMessage(msg)
			if err != nil {
				// Log detailed error information for monitoring
				log.Printf("FAILED to process message - Exchange: %s, RoutingKey: %s, Error: %v",
					msg.Exchange, msg.RoutingKey, err)
				log.Printf("Failed message body: %s", string(msg.Body))

				// Acknowledge failed message to prevent infinite requeuing
				// This removes the message from the queue permanently
				if ackErr := msg.Ack(false); ackErr != nil {
					log.Printf("Error acknowledging failed message: %v", ackErr)
				} else {
					log.Printf("Acknowledged and discarded failed message (routing key: %s)", msg.RoutingKey)
				}
			} else {
				// Acknowledge successful message processing
				if err := msg.Ack(false); err != nil {
					log.Printf("Error acknowledging successful message: %v", err)
				}
			}
		}
	}
}

func (c *EventConsumer) processMessage(msg amqp091.Delivery) error {
	routingKey := msg.RoutingKey
	exchange := msg.Exchange

	log.Printf("Processing message from exchange '%s' with routing key: %s", exchange, routingKey)

	switch routingKey {
	// Profile events
	case "profile.updated":
		return c.handleProfileUpdated(msg.Body)
	case "profile.deleted":
		return c.handleProfileDeleted(msg.Body)

	// Google events
	case "google.login":
		return c.handleGoogleLogin(msg.Body)
	case "google.login.request":
		return c.handleGoogleLoginRequest(msg.Body)
	case "email.verification.success":
		return c.handleEmailVerificationSuccess(msg.Body)

	// Billing events
	case "plan.created":
		return c.handlePlanCreated(msg.Body)
	case "plan.updated":
		return c.handlePlanUpdated(msg.Body)
	case "plan.deleted":
		return c.handlePlanDeleted(msg.Body)

	case "subscription.updated":
		return c.handleSubscriptionUpdated(msg.Body)

	default:
		log.Printf("Unknown routing key: %s from exchange: %s", routingKey, exchange)
		return nil // Acknowledge the message to avoid requeuing
	}
}

// Event handler functions

func (c *EventConsumer) handleProfileUpdated(body []byte) error {
	var event ProfileUpdatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal profile updated event: %w", err)
	}

	log.Printf("Profile updated: Username=%s", event.Username)

	// Invalidate the cache
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.redisRepo.InvalidateProfileCache(ctx, event.Username)
	if err != nil {
		return fmt.Errorf("error invalidating profile cache for user %s: %w", event.Username, err)
	}

	log.Printf("Successfully invalidated profile cache for user: %s", event.Username)
	return nil
}

func (c *EventConsumer) handleProfileDeleted(body []byte) error {
	return nil
}

func (c *EventConsumer) handleGoogleLogin(body []byte) error {
	var event GoogleLoginEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarsal google login event: %w", err)
	}

	log.Printf("Google login event received: Email=%s", event.Email)

	//user, err := c.userRepo.FindByEmail(context.Background(), event.Email)
	//log.Println(err)
	//if user != nil || err != nil {
	//	log.Printf("user Google login exist: no new user created; detail: %v", err)
	//	return fmt.Errorf("user Google login exist: no new user created; detail: %v", err)
	//}

	//user = &models.UserAuth{
	//	ID:              bson.NewObjectID(),
	//	Username:        event.Email,
	//	Email:           event.Email,
	//	PasswordHash:    event.Email + utils.GenerateRandomStringWithLength(10),
	//	IsActive:        true,
	//	IsEmailVerified: true,
	//	CreatedAt:       int(time.Now().Unix()),
	//	UpdatedAt:       int(time.Now().Unix()),
	//}
	//log.Printf("New user init: %v", user)

	//_, err = c.userRepo.NewUser(context.Background(), user)
	//if err != nil {
	//	log.Printf("error create new auth user: %v", err)
	//	return fmt.Errorf("error create new auth user: %v", err)
	//}
	//profile := map[string]string{"fullname": event.Name, "locale": event.Locale, "avatar": event.Avatar}
	//log.Printf("new user created with profile: %v", profile)

	//if c.eventPublisher != nil {
	//	err := c.eventPublisher.PublishUserRegister(
	//		context.Background(),
	//		user.ID.Hex(),
	//		user.Username,
	//		user.Email,
	//		profile,
	//	)
	//	if err != nil {
	//		// Log the error but don't fail the registration
	//		log.Printf("Warning: Failed to publish user created event: %v", err)
	//	} else {
	//		log.Printf("Published user created event for user: %s", user.Username)
	//	}
	//}

	return nil
}

func (c *EventConsumer) handleGoogleLoginRequest(body []byte) error {
	var event GoogleLoginRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal google login request event: %w", err)
	}

	log.Printf("Google login request received: RequestID=%s, Email=%s", event.RequestID, event.Email)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to login with email as both username and password (Google OAuth pattern)
	userAuth, err := c.userRepo.FindByEmail(ctx, event.Email)
	var sessionToken string
	var userID string

	if err != nil || userAuth == nil {
		// User doesn't exist, create new user
		log.Printf("Creating new user for Google login: %s", event.Email)

		newUser := &models.UserAuth{
			ID:              bson.NewObjectID(),
			Username:        event.Email,
			Email:           event.Email,
			PasswordHash:    event.Email, // Set password hash as email for Google users
			IsActive:        true,
			IsEmailVerified: true, // Google users are pre-verified
			CreatedAt:       int(time.Now().Unix()),
			UpdatedAt:       int(time.Now().Unix()),
		}

		// Create user in database
		createdUser, err := c.userRepo.NewUser(ctx, newUser)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			// Publish failure response
			c.publishLoginResponse(ctx, event.RequestID, false, "", "Failed to create user", "")
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Assign default role to new user
		err = c.assignDefaultRoleToUser(ctx, createdUser.ID)
		if err != nil {
			log.Printf("Warning: Failed to assign default role to user: %v", err)
		}

		userAuth = createdUser
		userID = createdUser.ID.Hex()

		// Publish user registration event if event publisher is available
		if c.eventPublisher != nil {
			err := c.eventPublisher.PublishUserRegister(
				ctx,
				userAuth.ID.Hex(),
				userAuth.Username,
				userAuth.Email,
				event.Profile,
			)
			if err != nil {
				log.Printf("Warning: Failed to publish user created event: %v", err)
			} else {
				log.Printf("Published user created event for user: %s", userAuth.Username)
			}
		}
	} else {
		userID = userAuth.ID.Hex()
		log.Printf("Existing user found for Google login: %s", event.Email)
	}

	// Create session token (this logic should match your existing login flow)
	sessionToken, err = c.createSessionForUser(ctx, userAuth)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		// Publish failure response
		c.publishLoginResponse(ctx, event.RequestID, false, "", "Failed to create session", userID)
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Publish success response
	c.publishLoginResponse(ctx, event.RequestID, true, sessionToken, "", userID)

	log.Printf("Successfully processed Google login request for user: %s", event.Email)
	return nil
}

// Helper function to create session for user (extracted from existing login logic)
func (c *EventConsumer) createSessionForUser(ctx context.Context, userAuth *models.UserAuth) (string, error) {
	// This should implement the same session creation logic as in your existing login handlers
	// For now, I'll create a simplified version - you may need to adjust this

	// Get user permissions (simplified - just set empty permissions for Google users)
	permissions := []string{} // Google users get basic permissions

	// Create session (simplified - you may need to adjust based on your session service implementation)
	sessionData := map[string]interface{}{
		"user_id":     userAuth.ID.Hex(),
		"username":    userAuth.Username,
		"email":       userAuth.Email,
		"permissions": permissions,
	}

	// Generate session token (you might have a different method for this)
	sessionToken := generateSessionToken()

	// Store session in Redis with 24-hour expiration
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	_, sessionErr := c.redisRepo.SaveStructCached(ctx, "", sessionKey, sessionData, 24*60) // 24 hours in minutes
	if sessionErr != nil {
		return "", fmt.Errorf("failed to store session: %w", sessionErr)
	}

	return sessionToken, nil
}

// Helper function to publish login response
func (c *EventConsumer) publishLoginResponse(ctx context.Context, requestID string, success bool, sessionToken, errorMsg, userID string) {
	if c.eventPublisher != nil {
		err := c.eventPublisher.PublishGoogleLoginResponse(ctx, requestID, success, sessionToken, errorMsg, userID)
		if err != nil {
			log.Printf("Failed to publish Google login response: %v", err)
		} else {
			log.Printf("Published Google login response for request %s", requestID)
		}
	}
}

// Helper function to assign default role to user (simplified version)
func (c *EventConsumer) assignDefaultRoleToUser(ctx context.Context, userID bson.ObjectID) error {
	// Find the default role (assuming there's a role called "user" or similar)
	defaultRole, err := c.roleRepo.FindByName(ctx, "user")
	if err != nil {
		// If no default role exists, log and continue
		log.Printf("No default role found, skipping role assignment: %v", err)
		return nil
	}

	// Create user role assignment
	currentTime := int(time.Now().Unix())
	userRole := &models.UserRole{
		ID:         bson.NewObjectID(),
		UserID:     userID,
		RoleID:     defaultRole.ID,
		ScopeType:  "global",
		ScopeID:    bson.NilObjectID,
		AssignedBy: bson.NilObjectID, // System assignment
		AssignedAt: currentTime,
		ExpiresAt:  0, // No expiration
		IsActive:   true,
	}

	_, err = c.userRoleRepo.Create(ctx, userRole)
	if err != nil {
		return fmt.Errorf("failed to create user role assignment: %w", err)
	}

	return nil
}

// Helper function to generate session token
func generateSessionToken() string {
	return time.Now().Format("20060102150405") + "-" + utils.GenerateRandomStringWithLength(32)
}

func (c *EventConsumer) handlePlanCreated(body []byte) error {
	var event PlanCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal plan created event: %w", err)
	}

	log.Printf("Plan created event received: PlanID=%s, PlanName=%s", event.PlanID, event.PlanName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if role metadata exists
	if event.RoleMetadata == nil {
		log.Printf("No role metadata in plan created event for plan %s, skipping role creation", event.PlanID)
		return nil
	}

	roleMetadata := event.RoleMetadata

	// Check if role already exists
	existingRole, err := c.roleRepo.FindByName(ctx, roleMetadata.SuggestedRoleName)
	if err == nil && existingRole != nil {
		log.Printf("Role %s already exists, skipping creation", roleMetadata.SuggestedRoleName)
		return nil
	}

	// Validate that all permissions exist
	validPermissions := make([]string, 0, len(roleMetadata.AllPermissions))

	for _, permission := range roleMetadata.AllPermissions {
		_, err := c.permissionRepo.FindAvailablePermission(ctx, permission)
		if err != nil {
			log.Printf("Warning: Permission '%s' not found in system, skipping", permission)
			continue
		}
		validPermissions = append(validPermissions, permission)
	}

	if len(validPermissions) == 0 {
		log.Printf("No valid permissions found for plan %s, skipping role creation", event.PlanID)
		return nil
	}

	// Create the role using repository directly
	log.Printf("Creating role '%s' with %d permissions for plan %s",
		roleMetadata.SuggestedRoleName, len(validPermissions), event.PlanID)

	currentTime := int(time.Now().Unix())
	role := &models.Role{
		Name:        roleMetadata.SuggestedRoleName,
		Description: roleMetadata.RoleDescription,
		Permissions: validPermissions,
		IsSystem:    false, // Not a system role
		CreatedAt:   currentTime,
		UpdatedAt:   currentTime,
	}

	createdRole, err := c.roleRepo.Create(ctx, role)
	if err != nil {
		return fmt.Errorf("failed to create role for plan %s: %w", event.PlanID, err)
	}

	log.Printf("Successfully created role '%s' (ID: %s) for plan %s with permissions: %v",
		createdRole.Name, createdRole.ID.Hex(), event.PlanID, validPermissions)

	// Log feature-permission mapping for debugging
	if len(roleMetadata.FeaturePermissionMap) > 0 {
		log.Printf("Feature-Permission mapping for plan %s:", event.PlanID)
		for feature, permissions := range roleMetadata.FeaturePermissionMap {
			log.Printf("  - Feature '%s': %v", feature, permissions)
		}
	}

	return nil
}

func (c *EventConsumer) handlePlanUpdated(body []byte) error {
	var event PlanUpdatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal plan updated event: %w", err)
	}

	log.Printf("Plan updated event received: PlanID=%s, PlanName=%s, ChangedFields=%v",
		event.PlanID, event.PlanName, event.ChangedFields)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if role metadata exists
	if event.RoleMetadata == nil {
		log.Printf("No role metadata in plan updated event for plan %s, skipping role update", event.PlanID)
		return nil
	}

	roleMetadata := event.RoleMetadata

	// Find the existing role for this plan
	existingRole, err := c.roleRepo.FindByName(ctx, roleMetadata.SuggestedRoleName)
	if err != nil {
		log.Printf("Role %s not found for plan %s, skipping update: %v",
			roleMetadata.SuggestedRoleName, event.PlanID, err)
		return nil
	}

	// Validate that all new permissions exist
	validPermissions := make([]string, 0, len(roleMetadata.AllPermissions))

	for _, permission := range roleMetadata.AllPermissions {
		_, err := c.permissionRepo.FindAvailablePermission(ctx, permission)
		if err != nil {
			log.Printf("Warning: Permission '%s' not found in system, skipping", permission)
			continue
		}
		validPermissions = append(validPermissions, permission)
	}

	// Check if permissions actually changed
	if permissionsEqual(existingRole.Permissions, validPermissions) {
		log.Printf("No permission changes detected for role %s (plan %s), skipping update",
			existingRole.Name, event.PlanID)
		return nil
	}

	// Update only the permissions and timestamp
	log.Printf("Updating permissions for role '%s' (plan %s): %d -> %d permissions",
		existingRole.Name, event.PlanID, len(existingRole.Permissions), len(validPermissions))

	existingRole.Permissions = validPermissions
	existingRole.UpdatedAt = int(time.Now().Unix())

	err = c.roleRepo.Update(ctx, existingRole)
	if err != nil {
		return fmt.Errorf("failed to update role for plan %s: %w", event.PlanID, err)
	}

	log.Printf("Successfully updated role '%s' (ID: %s) for plan %s with new permissions: %v",
		existingRole.Name, existingRole.ID.Hex(), event.PlanID, validPermissions)

	// Log feature-permission mapping for debugging
	if len(roleMetadata.FeaturePermissionMap) > 0 {
		log.Printf("Updated Feature-Permission mapping for plan %s:", event.PlanID)
		for feature, permissions := range roleMetadata.FeaturePermissionMap {
			log.Printf("  - Feature '%s': %v", feature, permissions)
		}
	}

	return nil
}

func permissionsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison
	mapA := make(map[string]bool)
	mapB := make(map[string]bool)

	for _, perm := range a {
		mapA[perm] = true
	}

	for _, perm := range b {
		mapB[perm] = true
	}

	// Check if all permissions in A exist in B
	for perm := range mapA {
		if !mapB[perm] {
			return false
		}
	}

	// Check if all permissions in B exist in A
	for perm := range mapB {
		if !mapA[perm] {
			return false
		}
	}

	return true
}

func (c *EventConsumer) handlePlanDeleted(body []byte) error {
	var event PlanDeletedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal plan deleted event: %w", err)
	}

	log.Printf("Plan deleted event received: PlanID=%s, PlanName=%s", event.PlanID, event.PlanName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if role metadata exists
	if event.RoleMetadata == nil {
		log.Printf("No role metadata in plan deleted event for plan %s, skipping role deletion", event.PlanID)
		return nil
	}

	roleMetadata := event.RoleMetadata

	// Find the existing role for this plan
	existingRole, err := c.roleRepo.FindByName(ctx, roleMetadata.SuggestedRoleName)
	if err != nil {
		log.Printf("Role %s not found for deleted plan %s, nothing to delete: %v",
			roleMetadata.SuggestedRoleName, event.PlanID, err)
		return nil // Not an error - role might already be deleted
	}

	// Check if this is a system role (safety check)
	if existingRole.IsSystem {
		log.Printf("WARNING: Attempted to delete system role %s for plan %s - operation blocked",
			existingRole.Name, event.PlanID)
		return fmt.Errorf("cannot delete system role %s", existingRole.Name)
	}

	// Delete the role
	log.Printf("Deleting role '%s' (ID: %s) for deleted plan %s",
		existingRole.Name, existingRole.ID.Hex(), event.PlanID)

	err = c.roleRepo.Delete(ctx, existingRole.ID)
	if err != nil {
		return fmt.Errorf("failed to delete role for plan %s: %w", event.PlanID, err)
	}

	log.Printf("Successfully deleted role '%s' for deleted plan %s (PlanName: %s)",
		existingRole.Name, event.PlanID, event.PlanName)

	// Log additional details for audit purposes
	log.Printf("Deleted role details - Name: %s, Permissions: %v, Created: %d",
		existingRole.Name, existingRole.Permissions, existingRole.CreatedAt)

	return nil
}

func (c *EventConsumer) handleSubscriptionUpdated(body []byte) error {
	var event SubscriptionEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal subscription updated event: %w", err)
	}

	log.Printf("Subscription updated event received: SubscriptionID=%s, UserID=%s, Status=%s",
		event.SubscriptionID, event.UserID, event.Status)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if this is an activation (status change to active) with role metadata
	if event.Status == "active" && event.UserRoleMetadata != nil && event.UserRoleMetadata.ShouldAssignRole {
		return c.assignRoleToUser(ctx, &event)
	}

	log.Printf("No role assignment needed for subscription %s (status: %s)", event.SubscriptionID, event.Status)
	return nil
}

// Helper method to assign role to user when subscription becomes active
func (c *EventConsumer) assignRoleToUser(ctx context.Context, event *SubscriptionEvent) error {
	log.Printf("Assigning role '%s' to user %s for active subscription %s",
		event.UserRoleMetadata.RoleName, event.UserID, event.SubscriptionID)

	// Find the user
	userObjectID, err := bson.ObjectIDFromHex(event.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	user, err := c.userRepo.FindByID(ctx, userObjectID)
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", event.UserID, err)
	}
	if user == nil {
		return fmt.Errorf("user %s not found", event.UserID)
	}

	// Find the role
	role, err := c.roleRepo.FindByName(ctx, event.UserRoleMetadata.RoleName)
	if err != nil {
		return fmt.Errorf("failed to find role %s: %w", event.UserRoleMetadata.RoleName, err)
	}

	// Check if user already has this role
	var scopeID bson.ObjectID
	existingUserRoles, err := c.userRoleRepo.FindByUserIDAndScope(ctx, userObjectID, "", scopeID)
	if err != nil {
		return fmt.Errorf("failed to get existing user roles: %w", err)
	}

	for _, userRole := range existingUserRoles {
		if userRole.RoleID == role.ID && userRole.IsActive {
			log.Printf("User %s already has role %s, skipping assignment", event.UserID, role.Name)
			return nil
		}
	}

	// Create user role assignment
	systemID, _ := bson.ObjectIDFromHex("000000000000000000000000") // System assignment
	currentTime := int(time.Now().Unix())

	userRole := &models.UserRole{
		ID:         bson.NewObjectID(),
		UserID:     userObjectID,
		RoleID:     role.ID,
		ScopeType:  "subscription",   // Scope to subscription
		ScopeID:    bson.NilObjectID, // Could store subscription ID if needed
		AssignedBy: systemID,
		AssignedAt: currentTime,
		ExpiresAt:  0, // No expiration for subscription-based roles
		IsActive:   true,
	}

	_, err = c.userRoleRepo.Create(ctx, userRole)
	if err != nil {
		return fmt.Errorf("failed to assign role %s to user %s: %w", role.Name, event.UserID, err)
	}

	log.Printf("Successfully assigned role '%s' to user %s for subscription %s with permissions: %v",
		role.Name, event.UserID, event.SubscriptionID, event.UserRoleMetadata.Permissions)

	return nil
}

func (c *EventConsumer) handleEmailVerificationSuccess(body []byte) error {
	var event EmailVerificationSuccessEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal email verification success event: %w", err)
	}

	log.Printf("Email verification success event received: UserID=%s, Email=%s", event.UserID, event.Email)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert UserID string to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(event.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Find the user
	user, err := c.userRepo.FindByID(ctx, userObjectID)
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", event.UserID, err)
	}
	if user == nil {
		return fmt.Errorf("user %s not found", event.UserID)
	}

	// Check if email verification is already completed
	if user.IsEmailVerified {
		log.Printf("User %s email is already verified, skipping update", event.UserID)
		return nil
	}

	// Update email verification status
	user.IsEmailVerified = true
	user.UpdatedAt = int(time.Now().Unix())

	err = c.userRepo.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update email verification status for user %s: %w", event.UserID, err)
	}

	log.Printf("Successfully updated email verification status for user %s (email: %s)", event.UserID, event.Email)

	// Optionally publish a user updated event or invalidate cache
	// Invalidate user cache if you have caching implemented
	if c.redisRepo != nil {
		cacheKey := "auth-service-auth-user-" + user.Username
		err = c.redisRepo.DeleteKey(ctx, cacheKey)
		if err != nil {
			log.Printf("Warning: Failed to invalidate user cache for %s: %v", user.Username, err)
		}
	}

	return nil
}

func (c *EventConsumer) Close() error {
	if !c.enabled {
		return nil
	}

	// Signal the consumer goroutine to stop
	close(c.shutdown)

	// Wait for the consumer goroutine to finish
	c.wg.Wait()

	// Close the RabbitMQ channel and connection
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ connection: %w", err)
		}
	}

	return nil
}
