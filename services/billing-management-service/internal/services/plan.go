package services

import (
	"billing-management-service/internal/event"
	"billing-management-service/internal/models"
	"billing-management-service/internal/repository"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type PlanService struct {
	planRepo  *repository.PlanRepository
	publisher event.Publisher
}

func NewPlanService(planRepo *repository.PlanRepository, publisher event.Publisher) *PlanService {
	return &PlanService{
		planRepo:  planRepo,
		publisher: publisher,
	}
}

// CreatePlan creates a new subscription plan
func (s *PlanService) CreatePlan(ctx context.Context, req *models.CreatePlanRequest) (*models.Plan, error) {
	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create new plan
	plan := &models.Plan{
		Name:         req.Name,
		Description:  req.Description,
		PlanType:     req.PlanType,
		Price:        req.Price,
		Currency:     strings.ToUpper(req.Currency),
		BillingCycle: req.BillingCycle,
		Features:     req.Features,
		IsActive:     true,
		TrialDays:    req.TrialDays,
		Metadata: models.Metadata{
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	// Save to database
	createdPlan, err := s.planRepo.New(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	// Publish plan created event
	planEvent := event.CreatePlanCreatedEvent(createdPlan)

	if err := s.publisher.PublishPlanEvent(planEvent); err != nil {
		log.Printf("Failed to publish plan created event: %v", err)
	} else {
		log.Printf("Published plan created event with %d features and %d total permissions",
			len(planEvent.Features), len(planEvent.RoleMetadata.AllPermissions))
	}

	return createdPlan, nil
}

// GetPlan retrieves a plan by ID
func (s *PlanService) GetPlan(ctx context.Context, planID string) (*models.Plan, error) {
	if planID == "" {
		return nil, fmt.Errorf("plan ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(planID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID format: %w", err)
	}

	plan, err := s.planRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return plan, nil
}

// GetPlansByType retrieves plans by plan type
func (s *PlanService) GetPlansByType(ctx context.Context, planType models.PlanType) ([]*models.Plan, error) {
	if planType == "" {
		return nil, fmt.Errorf("plan type is required")
	}

	plans, err := s.planRepo.FindByPlanType(ctx, planType)
	if err != nil {
		return nil, fmt.Errorf("failed to get plans by type: %w", err)
	}

	return plans, nil
}

// UpdatePlan updates an existing plan
func (s *PlanService) UpdatePlan(ctx context.Context, planID string, req *models.UpdatePlanRequest) (*models.Plan, error) {
	if planID == "" {
		return nil, fmt.Errorf("plan ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(planID)
	if err != nil {
		return nil, fmt.Errorf("invalid plan ID format: %w", err)
	}

	// Get existing plan
	existingPlan, err := s.planRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("plan not found")
		}
		return nil, fmt.Errorf("failed to get existing plan: %w", err)
	}

	// Track changes for event
	changedFields := []string{}
	oldValues := make(map[string]any)
	newValues := make(map[string]any)

	// Update fields
	updatedPlan := *existingPlan
	updatedPlan.Metadata.UpdatedAt = time.Now().Unix()

	if req.Name != "" && req.Name != existingPlan.Name {
		changedFields = append(changedFields, "name")
		oldValues["name"] = existingPlan.Name
		newValues["name"] = req.Name
		updatedPlan.Name = req.Name
	}

	if req.Description != existingPlan.Description {
		changedFields = append(changedFields, "description")
		oldValues["description"] = existingPlan.Description
		newValues["description"] = req.Description
		updatedPlan.Description = req.Description
	}

	if req.Price > 0 && req.Price != existingPlan.Price {
		changedFields = append(changedFields, "price")
		oldValues["price"] = existingPlan.Price
		newValues["price"] = req.Price
		updatedPlan.Price = req.Price
	}

	if req.BillingCycle != "" && req.BillingCycle != existingPlan.BillingCycle {
		changedFields = append(changedFields, "billingCycle")
		oldValues["billingCycle"] = existingPlan.BillingCycle
		newValues["billingCycle"] = req.BillingCycle
		updatedPlan.BillingCycle = req.BillingCycle
	}

	if req.Features != nil && !s.compareFeaturesSlice(existingPlan.Features, req.Features) {
		changedFields = append(changedFields, "features")
		oldValues["features"] = existingPlan.Features
		newValues["features"] = req.Features
		updatedPlan.Features = req.Features
	}

	if req.TrialDays != existingPlan.TrialDays {
		changedFields = append(changedFields, "trialDays")
		oldValues["trialDays"] = existingPlan.TrialDays
		newValues["trialDays"] = req.TrialDays
		updatedPlan.TrialDays = req.TrialDays
	}

	if req.IsActive != nil && *req.IsActive != existingPlan.IsActive {
		changedFields = append(changedFields, "isActive")
		oldValues["isActive"] = existingPlan.IsActive
		newValues["isActive"] = *req.IsActive
		updatedPlan.IsActive = *req.IsActive
	}

	if len(changedFields) == 0 {
		return existingPlan, nil // No changes
	}

	// Save changes
	savedPlan, err := s.planRepo.Update(ctx, objectID, &updatedPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	// Publish plan updated event
	planEvent := event.CreatePlanUpdatedEvent(&updatedPlan, changedFields, oldValues, newValues)
	fmt.Println(savedPlan, "\n===========================")
	fmt.Println(updatedPlan)

	if err := s.publisher.PublishPlanEvent(planEvent); err != nil {
		log.Printf("Failed to publish plan updated event: %v", err)
	} else {
		log.Printf("Published plan updated event with changes: %v", changedFields)
		if planEvent.RoleMetadata != nil {
			log.Printf("Role metadata includes %d permissions across %d features",
				len(planEvent.RoleMetadata.AllPermissions), len(planEvent.RoleMetadata.FeaturePermissionMap))
		}
	}

	return savedPlan, nil
}

// DeletePlan deletes a plan (soft delete by setting isActive to false)
func (s *PlanService) DeletePlan(ctx context.Context, planID string) error {
	if planID == "" {
		return fmt.Errorf("plan ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(planID)
	if err != nil {
		return fmt.Errorf("invalid plan ID format: %w", err)
	}

	// Get plan before deletion for event
	plan, err := s.planRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("plan not found")
		}
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Soft delete by setting isActive to false
	if err := s.planRepo.UpdateStatus(ctx, objectID, false); err != nil {
		return fmt.Errorf("failed to delete plan: %w", err)
	}

	// Publish plan deleted event
	planEvent := event.CreatePlanDeletedEvent(plan)

	if err := s.publisher.PublishPlanEvent(planEvent); err != nil {
		log.Printf("Failed to publish plan created event: %v", err)
	} else {
		log.Printf("Published plan deleted event with id: %s", planEvent.PlanID)
	}

	return nil
}

// ListPlans retrieves plans with pagination
func (s *PlanService) ListPlans(ctx context.Context, page, limit int, activeOnly bool) ([]*models.Plan, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var plans []*models.Plan
	var err error

	if activeOnly {
		plans, err = s.planRepo.FindActivePlans(ctx, page, limit)
	} else {
		plans, err = s.planRepo.FindAll(ctx, page, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}

	return plans, nil
}

// GetPlanStats returns plan statistics
func (s *PlanService) GetPlanStats(ctx context.Context) (*models.PlanStats, error) {
	totalPlans, err := s.planRepo.CountPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total plans: %w", err)
	}

	activePlans, err := s.planRepo.CountActivePlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active plans: %w", err)
	}

	return &models.PlanStats{
		TotalPlans:  totalPlans,
		ActivePlans: activePlans,
	}, nil
}

// ActivatePlan activates a plan
func (s *PlanService) ActivatePlan(ctx context.Context, planID string) error {
	return s.updatePlanStatus(ctx, planID, true)
}

// DeactivatePlan deactivates a plan
func (s *PlanService) DeactivatePlan(ctx context.Context, planID string) error {
	return s.updatePlanStatus(ctx, planID, false)
}

// Helper methods

func (s *PlanService) validateCreateRequest(req *models.CreatePlanRequest) error {
	if req.Name == "" {
		return fmt.Errorf("plan name is required")
	}
	if req.PlanType == "" {
		return fmt.Errorf("plan type is required")
	}
	if req.Price < 0 {
		return fmt.Errorf("price cannot be negative")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.BillingCycle == "" {
		return fmt.Errorf("billing cycle is required")
	}
	if !s.isValidCurrency(req.Currency) {
		return fmt.Errorf("invalid currency code")
	}
	if !s.isValidBillingCycle(req.BillingCycle) {
		return fmt.Errorf("invalid billing cycle")
	}
	if !s.isValidPlanType(req.PlanType) {
		return fmt.Errorf("invalid plan type")
	}
	if req.TrialDays < 0 {
		return fmt.Errorf("trial days cannot be negative")
	}
	return nil
}

func (s *PlanService) isValidCurrency(currency string) bool {
	validCurrencies := map[string]bool{
		"USD": true, "EUR": true, "GBP": true, "JPY": true,
		"CAD": true, "AUD": true, "CHF": true, "CNY": true,
		"SEK": true, "NZD": true, "MXN": true, "SGD": true,
		"HKD": true, "NOK": true, "TRY": true, "RUB": true,
		"INR": true, "VND": true, "ZAR": true, "KRW": true,
	}
	return validCurrencies[strings.ToUpper(currency)]
}

func (s *PlanService) isValidBillingCycle(cycle models.BillingCycle) bool {
	return cycle == models.BillingCycleMonthly || cycle == models.BillingCycleYearly
}

func (s *PlanService) isValidPlanType(planType models.PlanType) bool {
	return planType == models.PlanTypeFree ||
		planType == models.PlanTypeBasic ||
		planType == models.PlanTypePremium ||
		planType == models.PlanTypeCustom
}

func (s *PlanService) updatePlanStatus(ctx context.Context, planID string, isActive bool) error {
	if planID == "" {
		return fmt.Errorf("plan ID is required")
	}

	objectID, err := bson.ObjectIDFromHex(planID)
	if err != nil {
		return fmt.Errorf("invalid plan ID format: %w", err)
	}

	// Get plan before update for event
	plan, err := s.planRepo.FindByID(ctx, objectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("plan not found")
		}
		return fmt.Errorf("failed to get plan: %w", err)
	}

	if plan.IsActive == isActive {
		return nil // No change needed
	}

	// Update status
	if err := s.planRepo.UpdateStatus(ctx, objectID, isActive); err != nil {
		return fmt.Errorf("failed to update plan status: %w", err)
	}

	// Publish plan status changed event
	eventType := event.EventTypePlanActivated
	if !isActive {
		eventType = event.EventTypePlanDeactivated
	}

	planEvent := &event.PlanEvent{
		EventType: eventType,
		PlanID:    plan.ID.Hex(),
		PlanType:  plan.PlanType,
		Timestamp: time.Now().Unix(),
		OldValues: map[string]any{"isActive": plan.IsActive},
		NewValues: map[string]any{"isActive": isActive},
	}

	if err := s.publisher.PublishPlanEvent(planEvent); err != nil {
		log.Printf("Failed to publish plan status changed event: %v", err)
	}

	return nil
}

// Comparison methods for tracking changes
func (s *PlanService) compareFeaturesSlice(old, new []models.Feature) bool {
	if len(old) != len(new) {
		return false
	}
	for i, o := range old {
		n := new[i]
		if o.Name != n.Name ||
			o.Description != n.Description ||
			o.Enabled != n.Enabled {
			return false
		}
	}
	return true
}
