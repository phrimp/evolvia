package event

import "billing-management-service/internal/models"

const (
	EventTypePlanCreated     = "plan.created"
	EventTypePlanUpdated     = "plan.updated"
	EventTypePlanDeleted     = "plan.deleted"
	EventTypePlanActivated   = "plan.activated"
	EventTypePlanDeactivated = "plan.deactivated"

	EventTypeSubscriptionCreated     = "subscription.created"
	EventTypeSubscriptionUpdated     = "subscription.updated"
	EventTypeSubscriptionCanceled    = "subscription.canceled"
	EventTypeSubscriptionSuspended   = "subscription.suspended"
	EventTypeSubscriptionReactivated = "subscription.reactivated"
	EventTypeSubscriptionRenewed     = "subscription.renewed"
	EventTypeTrialExpired            = "subscription.trial_expired"
)

// Enhanced Feature structure for events
type FeatureDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Permissions []string `json:"permissions"` // Parsed from description
}

type PlanEvent struct {
	EventType     string              `json:"eventType"`
	PlanID        string              `json:"planId"`
	PlanName      string              `json:"planName"`
	PlanType      models.PlanType     `json:"planType"`
	Price         float64             `json:"price"`
	Currency      string              `json:"currency"`
	BillingCycle  models.BillingCycle `json:"billingCycle"`
	Features      []FeatureDetail     `json:"features"`
	IsActive      bool                `json:"isActive"`
	TrialDays     int                 `json:"trialDays"`
	Timestamp     int64               `json:"timestamp"`
	ChangedFields []string            `json:"changedFields,omitempty"`
	OldValues     map[string]any      `json:"oldValues,omitempty"`
	NewValues     map[string]any      `json:"newValues,omitempty"`

	// Role creation metadata
	RoleMetadata *RoleCreationMetadata `json:"roleMetadata,omitempty"`
}

type RoleCreationMetadata struct {
	SuggestedRoleName    string              `json:"suggestedRoleName"`    // e.g., "premium-plan-role"
	AllPermissions       []string            `json:"allPermissions"`       // All permissions from all features
	FeaturePermissionMap map[string][]string `json:"featurePermissionMap"` // Map feature name to its permissions
	RoleDescription      string              `json:"roleDescription"`      // Generated description for the role
}

type SubscriptionEvent struct {
	EventType      string                    `json:"eventType"`
	SubscriptionID string                    `json:"subscriptionId"`
	UserID         string                    `json:"userId"`
	PlanID         string                    `json:"planId"`
	PlanName       string                    `json:"planName,omitempty"`
	PlanType       models.PlanType           `json:"planType,omitempty"`
	Status         models.SubscriptionStatus `json:"status"`
	Timestamp      int64                     `json:"timestamp"`
	ChangedFields  []string                  `json:"changedFields,omitempty"`
	OldValues      map[string]any            `json:"oldValues,omitempty"`
	NewValues      map[string]any            `json:"newValues,omitempty"`

	// User role assignment metadata
	UserRoleMetadata *UserRoleMetadata `json:"userRoleMetadata,omitempty"`
}

type UserRoleMetadata struct {
	ShouldAssignRole bool     `json:"shouldAssignRole"`
	RoleName         string   `json:"roleName"`
	Permissions      []string `json:"permissions"`
	PreviousRoles    []string `json:"previousRoles,omitempty"`
}
