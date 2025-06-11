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

type PlanEvent struct {
	EventType     string          `json:"eventType"`
	PlanID        string          `json:"planId"`
	PlanType      models.PlanType `json:"planType"`
	Timestamp     int64           `json:"timestamp"`
	ChangedFields []string        `json:"changedFields,omitempty"`
	OldValues     map[string]any  `json:"oldValues,omitempty"`
	NewValues     map[string]any  `json:"newValues,omitempty"`
}

type SubscriptionEvent struct {
	EventType      string                    `json:"eventType"`
	SubscriptionID string                    `json:"subscriptionId"`
	UserID         string                    `json:"userId"`
	PlanID         string                    `json:"planId"`
	Status         models.SubscriptionStatus `json:"status"`
	Timestamp      int64                     `json:"timestamp"`
	ChangedFields  []string                  `json:"changedFields,omitempty"`
	OldValues      map[string]any            `json:"oldValues,omitempty"`
	NewValues      map[string]any            `json:"newValues,omitempty"`
}
