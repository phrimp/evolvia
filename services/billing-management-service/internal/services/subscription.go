package services

import (
	"billing-management-service/internal/event"
	"billing-management-service/internal/models"
	"billing-management-service/internal/repository"
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SubscriptionService struct {
	subscriptionRepo *repository.SubscriptionRepository
	planRepo         *repository.PlanRepository
	publisher        event.Publisher
}

func NewSubscriptionService(
	subscriptionRepo *repository.SubscriptionRepository,
	planRepo *repository.PlanRepository,
	publisher event.Publisher,
) *SubscriptionService {
	return &SubscriptionService{
		subscriptionRepo: subscriptionRepo,
		planRepo:         planRepo,
		publisher:        publisher,
	}
}

// CreateSubscription creates a new subscription
func (s *SubscriptionService) CreateSubscription(ctx context.Context, req *models.CreateSubscriptionRequest) (*models.Subscription, error) {
	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Validate plan exists and is active
	planObjectID, err := bson.ObjectIDFromHex(req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID format: %w", err)
	}

	plan, err := s.planRepo.FindByID(ctx, planObjectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	if !plan.IsActive {
		return nil, fmt.Errorf("plan is not active")
	}

	// Check if user already has an active subscription
	existingSubscription, err := s.subscriptionRepo.FindActiveByUserID(ctx, req.UserID)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("failed to check existing subscription: %w", err)
	}
	if existingSubscription != nil {
		return nil, fmt.Errorf("user already has an active subscription")
	}

	// Calculate subscription dates
	currentTime := time.Now()
	startDate := currentTime.Unix()
	var endDate, nextBillingDate, trialStartDate, trialEndDate int64

	// Set default values
	autoRenew := true
	if req.AutoRenew != nil {
		autoRenew = *req.AutoRenew
	}

	startTrial := false
	if req.StartTrial != nil {
		startTrial = *req.StartTrial
	}

	status := models.SubscriptionStatusActive

	// Handle trial period
	if startTrial && plan.TrialDays > 0 {
		status = models.SubscriptionStatusTrial
		trialStartDate = startDate
		trialEndDate = currentTime.AddDate(0, 0, plan.TrialDays).Unix()
		nextBillingDate = trialEndDate
	} else {
		// Calculate next billing date based on billing cycle
		if plan.BillingCycle == models.BillingCycleMonthly {
			nextBillingDate = currentTime.AddDate(0, 1, 0).Unix()
		} else {
			nextBillingDate = currentTime.AddDate(1, 0, 0).Unix()
		}
	}

	// Calculate current period
	currentPeriodStart := startDate
	if plan.BillingCycle == models.BillingCycleMonthly {
		currentPeriodEnd := currentTime.AddDate(0, 1, 0).Unix()
		if status == models.SubscriptionStatusTrial {
			currentPeriodEnd = trialEndDate
		}
		endDate = currentPeriodEnd
	} else {
		currentPeriodEnd := currentTime.AddDate(1, 0, 0).Unix()
		if status == models.SubscriptionStatusTrial {
			currentPeriodEnd = trialEndDate
		}
		endDate = currentPeriodEnd
	}

	// Create new subscription
	subscription := &models.Subscription{
		UserID:             req.UserID,
		PlanID:             planObjectID,
		Status:             status,
		StartDate:          startDate,
		EndDate:            endDate,
		NextBillingDate:    nextBillingDate,
		TrialStartDate:     trialStartDate,
		TrialEndDate:       trialEndDate,
		AutoRenew:          autoRenew,
		PaymentMethodID:    req.PaymentMethodID,
		CurrentPeriodStart: currentPeriodStart,
		CurrentPeriodEnd:   endDate,
		Metadata: models.Metadata{
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	// Save to database
	createdSubscription, err := s.subscriptionRepo.New(ctx, subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Publish subscription created event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionCreated,
		SubscriptionID: createdSubscription.ID.Hex(),
		UserID:         createdSubscription.UserID,
		PlanID:         createdSubscription.PlanID.Hex(),
		Status:         createdSubscription.Status,
		Timestamp:      time.Now().Unix(),
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription created event: %v", err)
	}

	return createdSubscription, nil
}

// GetSubscription retrieves a subscription by ID
func (s *SubscriptionService) GetSubscription(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID format: %w", err)
	}

	subscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("subscription not found")
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return subscription, nil
}

// GetSubscriptionByUserID retrieves active subscription for a user
func (s *SubscriptionService) GetSubscriptionByUserID(ctx context.Context, userID string) (*models.Subscription, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	subscription, err := s.subscriptionRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no active subscription found for user %s", userID)
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return subscription, nil
}

// GetSubscriptionWithPlan retrieves subscription with plan details
func (s *SubscriptionService) GetSubscriptionWithPlan(ctx context.Context, subscriptionID string) (*models.SubscriptionWithPlan, error) {
	subscription, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	plan, err := s.planRepo.FindByID(ctx, subscription.PlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return &models.SubscriptionWithPlan{
		Subscription: subscription,
		Plan:         plan,
	}, nil
}

// UpdateSubscription updates an existing subscription
func (s *SubscriptionService) UpdateSubscription(ctx context.Context, subscriptionID string, req *models.UpdateSubscriptionRequest) (*models.Subscription, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID format: %w", err)
	}

	// Get existing subscription
	existingSubscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("subscription not found")
		}
		return nil, fmt.Errorf("failed to get existing subscription: %w", err)
	}

	// Track changes for event
	changedFields := []string{}
	oldValues := make(map[string]any)
	newValues := make(map[string]any)

	// Update fields
	updatedSubscription := *existingSubscription
	updatedSubscription.Metadata.UpdatedAt = time.Now().Unix()

	// Update plan if specified
	if req.PlanID != "" {
		planObjectID, err := bson.ObjectIDFromHex(req.PlanID)
		if err != nil {
			return nil, fmt.Errorf("invalid plan ID format: %w", err)
		}

		// Validate new plan exists and is active
		plan, err := s.planRepo.FindByID(ctx, planObjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to get new plan: %w", err)
		}
		if !plan.IsActive {
			return nil, fmt.Errorf("new plan is not active")
		}

		if !planObjectID.IsZero() && planObjectID != existingSubscription.PlanID {
			changedFields = append(changedFields, "planId")
			oldValues["planId"] = existingSubscription.PlanID.Hex()
			newValues["planId"] = req.PlanID
			updatedSubscription.PlanID = planObjectID
		}
	}

	if req.PaymentMethodID != existingSubscription.PaymentMethodID {
		changedFields = append(changedFields, "paymentMethodId")
		oldValues["paymentMethodId"] = existingSubscription.PaymentMethodID
		newValues["paymentMethodId"] = req.PaymentMethodID
		updatedSubscription.PaymentMethodID = req.PaymentMethodID
	}

	if req.AutoRenew != nil && *req.AutoRenew != existingSubscription.AutoRenew {
		changedFields = append(changedFields, "autoRenew")
		oldValues["autoRenew"] = existingSubscription.AutoRenew
		newValues["autoRenew"] = *req.AutoRenew
		updatedSubscription.AutoRenew = *req.AutoRenew
	}

	if len(changedFields) == 0 {
		return existingSubscription, nil // No changes
	}

	// Save changes
	savedSubscription, err := s.subscriptionRepo.Update(ctx, objectID, &updatedSubscription)
	if err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	// Publish subscription updated event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionUpdated,
		SubscriptionID: savedSubscription.ID.Hex(),
		UserID:         savedSubscription.UserID,
		PlanID:         savedSubscription.PlanID.Hex(),
		Status:         savedSubscription.Status,
		Timestamp:      time.Now().Unix(),
		ChangedFields:  changedFields,
		OldValues:      oldValues,
		NewValues:      newValues,
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription updated event: %v", err)
	}

	return savedSubscription, nil
}

// CancelSubscription cancels a subscription
func (s *SubscriptionService) CancelSubscription(ctx context.Context, subscriptionID string, req *models.CancelSubscriptionRequest) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID format: %w", err)
	}

	// Get subscription before cancellation for event
	subscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("subscription not found")
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if subscription.Status == models.SubscriptionStatusCanceled {
		return fmt.Errorf("subscription is already canceled")
	}

	immediate := false
	if req.Immediate != nil {
		immediate = *req.Immediate
	}

	// Cancel subscription
	if err := s.subscriptionRepo.CancelSubscription(ctx, objectID, req.Reason, immediate); err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	// Publish subscription canceled event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionCanceled,
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID,
		PlanID:         subscription.PlanID.Hex(),
		Status:         models.SubscriptionStatusCanceled,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": subscription.Status},
		NewValues:      map[string]any{"status": models.SubscriptionStatusCanceled, "reason": req.Reason},
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription canceled event: %v", err)
	}

	return nil
}

// SearchSubscriptions searches subscriptions based on query parameters
func (s *SubscriptionService) SearchSubscriptions(ctx context.Context, query *models.SubscriptionSearchQuery) (*models.SubscriptionSearchResult, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	subscriptions, totalCount, err := s.subscriptionRepo.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search subscriptions: %w", err)
	}

	// Get plan details for each subscription
	var subscriptionsWithPlan []*models.SubscriptionWithPlan
	for _, subscription := range subscriptions {
		plan, err := s.planRepo.FindByID(ctx, subscription.PlanID)
		if err != nil {
			log.Printf("Failed to get plan for subscription %s: %v", subscription.ID.Hex(), err)
			// Continue with nil plan rather than failing the entire request
			subscriptionsWithPlan = append(subscriptionsWithPlan, &models.SubscriptionWithPlan{
				Subscription: subscription,
				Plan:         nil,
			})
		} else {
			subscriptionsWithPlan = append(subscriptionsWithPlan, &models.SubscriptionWithPlan{
				Subscription: subscription,
				Plan:         plan,
			})
		}
	}

	pageCount := int((totalCount + int64(query.PageSize) - 1) / int64(query.PageSize))

	result := &models.SubscriptionSearchResult{
		Subscriptions: subscriptionsWithPlan,
		TotalCount:    totalCount,
		PageCount:     pageCount,
		CurrentPage:   query.Page,
	}

	return result, nil
}

// RenewSubscription renews a subscription for the next billing cycle
func (s *SubscriptionService) RenewSubscription(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID format: %w", err)
	}

	subscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("subscription not found")
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if subscription.Status != models.SubscriptionStatusActive {
		return fmt.Errorf("only active subscriptions can be renewed")
	}

	// Get plan details for billing cycle calculation
	plan, err := s.planRepo.FindByID(ctx, subscription.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Calculate new billing dates
	currentTime := time.Now()
	var nextBillingDate int64
	var currentPeriodEnd int64

	if plan.BillingCycle == models.BillingCycleMonthly {
		nextBillingDate = currentTime.AddDate(0, 1, 0).Unix()
		currentPeriodEnd = nextBillingDate
	} else {
		nextBillingDate = currentTime.AddDate(1, 0, 0).Unix()
		currentPeriodEnd = nextBillingDate
	}

	// Update subscription
	updatedSubscription := *subscription
	updatedSubscription.NextBillingDate = nextBillingDate
	updatedSubscription.CurrentPeriodStart = currentTime.Unix()
	updatedSubscription.CurrentPeriodEnd = currentPeriodEnd
	updatedSubscription.EndDate = currentPeriodEnd
	updatedSubscription.Metadata.UpdatedAt = currentTime.Unix()

	if _, err := s.subscriptionRepo.Update(ctx, objectID, &updatedSubscription); err != nil {
		return fmt.Errorf("failed to renew subscription: %w", err)
	}

	// Publish subscription renewed event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionRenewed,
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID,
		PlanID:         subscription.PlanID.Hex(),
		Status:         subscription.Status,
		Timestamp:      time.Now().Unix(),
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription renewed event: %v", err)
	}

	return nil
}

// ProcessTrialExpiration processes subscriptions whose trial period has ended
func (s *SubscriptionService) ProcessTrialExpiration(ctx context.Context) error {
	// Find subscriptions with expired trials
	currentTime := time.Now().Unix()
	expiredTrialSubscriptions, err := s.subscriptionRepo.FindTrialEndingSubscriptions(ctx, currentTime)
	if err != nil {
		return fmt.Errorf("failed to find expired trial subscriptions: %w", err)
	}

	for _, subscription := range expiredTrialSubscriptions {
		// Update subscription status to active or past due based on payment method
		newStatus := models.SubscriptionStatusActive
		if subscription.PaymentMethodID == "" {
			newStatus = models.SubscriptionStatusPastDue
		}

		if err := s.subscriptionRepo.UpdateStatus(ctx, subscription.ID, newStatus); err != nil {
			log.Printf("Failed to update trial subscription %s: %v", subscription.ID.Hex(), err)
			continue
		}

		// Publish trial expired event
		subscriptionEvent := &event.SubscriptionEvent{
			EventType:      event.EventTypeTrialExpired,
			SubscriptionID: subscription.ID.Hex(),
			UserID:         subscription.UserID,
			PlanID:         subscription.PlanID.Hex(),
			Status:         newStatus,
			Timestamp:      time.Now().Unix(),
			OldValues:      map[string]any{"status": models.SubscriptionStatusTrial},
			NewValues:      map[string]any{"status": newStatus},
		}

		if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
			log.Printf("Failed to publish trial expired event: %v", err)
		}
	}

	return nil
}

// GetBillingDashboard returns billing dashboard statistics
func (s *SubscriptionService) GetBillingDashboard(ctx context.Context) (*models.BillingDashboard, error) {
	dashboard, err := s.subscriptionRepo.GetSubscriptionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription stats: %w", err)
	}

	return dashboard, nil
}

// GetExpiringSubscriptions returns subscriptions that are about to expire
func (s *SubscriptionService) GetExpiringSubscriptions(ctx context.Context, daysAhead int) ([]*models.Subscription, error) {
	if daysAhead < 1 {
		daysAhead = 7 // Default to 7 days
	}

	futureDate := time.Now().AddDate(0, 0, daysAhead).Unix()
	subscriptions, err := s.subscriptionRepo.FindExpiringSubscriptions(ctx, futureDate)
	if err != nil {
		return nil, fmt.Errorf("failed to find expiring subscriptions: %w", err)
	}

	return subscriptions, nil
}

// SuspendSubscription suspends a subscription (e.g., for failed payments)
func (s *SubscriptionService) SuspendSubscription(ctx context.Context, subscriptionID string, reason string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID format: %w", err)
	}

	subscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("subscription not found")
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if subscription.Status == models.SubscriptionStatusSuspended {
		return fmt.Errorf("subscription is already suspended")
	}

	oldStatus := subscription.Status
	if err := s.subscriptionRepo.UpdateStatus(ctx, objectID, models.SubscriptionStatusSuspended); err != nil {
		return fmt.Errorf("failed to suspend subscription: %w", err)
	}

	// Publish subscription suspended event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionSuspended,
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID,
		PlanID:         subscription.PlanID.Hex(),
		Status:         models.SubscriptionStatusSuspended,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": oldStatus},
		NewValues:      map[string]any{"status": models.SubscriptionStatusSuspended, "reason": reason},
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription suspended event: %v", err)
	}

	return nil
}

// ReactivateSubscription reactivates a suspended subscription
func (s *SubscriptionService) ReactivateSubscription(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID format: %w", err)
	}

	subscription, err := s.subscriptionRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("subscription not found")
		}
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if subscription.Status != models.SubscriptionStatusSuspended {
		return fmt.Errorf("only suspended subscriptions can be reactivated")
	}

	if err := s.subscriptionRepo.UpdateStatus(ctx, objectID, models.SubscriptionStatusActive); err != nil {
		return fmt.Errorf("failed to reactivate subscription: %w", err)
	}

	// Publish subscription reactivated event
	subscriptionEvent := &event.SubscriptionEvent{
		EventType:      event.EventTypeSubscriptionReactivated,
		SubscriptionID: subscription.ID.Hex(),
		UserID:         subscription.UserID,
		PlanID:         subscription.PlanID.Hex(),
		Status:         models.SubscriptionStatusActive,
		Timestamp:      time.Now().Unix(),
		OldValues:      map[string]any{"status": models.SubscriptionStatusSuspended},
		NewValues:      map[string]any{"status": models.SubscriptionStatusActive},
	}

	if err := s.publisher.PublishSubscriptionEvent(subscriptionEvent); err != nil {
		log.Printf("Failed to publish subscription reactivated event: %v", err)
	}

	return nil
}

// Helper methods

func (s *SubscriptionService) validateCreateRequest(req *models.CreateSubscriptionRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if req.PlanID == "" {
		return fmt.Errorf("plan ID is required")
	}
	return nil
}
