package models

import (
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Enums and Constants
type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusInactive  SubscriptionStatus = "inactive"
	SubscriptionStatusCanceled  SubscriptionStatus = "canceled"
	SubscriptionStatusSuspended SubscriptionStatus = "suspended"
	SubscriptionStatusTrial     SubscriptionStatus = "trial"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
)

type PlanType string

const (
	PlanTypeFree    PlanType = "free"
	PlanTypeBasic   PlanType = "basic"
	PlanTypePremium PlanType = "premium"
	PlanTypeCustom  PlanType = "custom"
)

type BillingCycle string

const (
	BillingCycleMonthly BillingCycle = "monthly"
	BillingCycleYearly  BillingCycle = "yearly"
)

// Core Models
type Plan struct {
	ID           bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name         string        `json:"name" bson:"name"`
	Description  string        `json:"description,omitempty" bson:"description,omitempty"`
	PlanType     PlanType      `json:"planType" bson:"planType"`
	Price        float64       `json:"price" bson:"price"`
	Currency     string        `json:"currency" bson:"currency"`
	BillingCycle BillingCycle  `json:"billingCycle" bson:"billingCycle"`
	Features     []Feature     `json:"features,omitempty" bson:"features,omitempty"`
	IsActive     bool          `json:"isActive" bson:"isActive"`
	TrialDays    int           `json:"trialDays,omitempty" bson:"trialDays,omitempty"`
	Metadata     Metadata      `json:"metadata" bson:"metadata"`
}

type Feature struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Enabled     bool   `json:"enabled" bson:"enabled"`
}

type PlanStats struct {
	TotalPlans  int64 `json:"totalPlans"`
	ActivePlans int64 `json:"activePlans"`
}

type Subscription struct {
	ID                 bson.ObjectID      `json:"id,omitempty" bson:"_id,omitempty"`
	UserID             string             `json:"userId" bson:"userId"`
	PlanID             bson.ObjectID      `json:"planId" bson:"planId"`
	Status             SubscriptionStatus `json:"status" bson:"status"`
	StartDate          int64              `json:"startDate" bson:"startDate"`
	EndDate            int64              `json:"endDate,omitempty" bson:"endDate,omitempty"`
	NextBillingDate    int64              `json:"nextBillingDate,omitempty" bson:"nextBillingDate,omitempty"`
	TrialStartDate     int64              `json:"trialStartDate,omitempty" bson:"trialStartDate,omitempty"`
	TrialEndDate       int64              `json:"trialEndDate,omitempty" bson:"trialEndDate,omitempty"`
	CanceledAt         int64              `json:"canceledAt,omitempty" bson:"canceledAt,omitempty"`
	CancelReason       string             `json:"cancelReason,omitempty" bson:"cancelReason,omitempty"`
	AutoRenew          bool               `json:"autoRenew" bson:"autoRenew"`
	PaymentMethodID    string             `json:"paymentMethodId,omitempty" bson:"paymentMethodId,omitempty"`
	CurrentPeriodStart int64              `json:"currentPeriodStart" bson:"currentPeriodStart"`
	CurrentPeriodEnd   int64              `json:"currentPeriodEnd" bson:"currentPeriodEnd"`
	Metadata           Metadata           `json:"metadata" bson:"metadata"`
}

type Metadata struct {
	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}

// DTOs and Requests
type CreatePlanRequest struct {
	Name         string       `json:"name" binding:"required"`
	Description  string       `json:"description"`
	PlanType     PlanType     `json:"planType" binding:"required"`
	Price        float64      `json:"price" binding:"required,min=0"`
	Currency     string       `json:"currency" binding:"required"`
	BillingCycle BillingCycle `json:"billingCycle" binding:"required"`
	Features     []Feature    `json:"features"`
	TrialDays    int          `json:"trialDays"`
}

type UpdatePlanRequest struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Price        float64      `json:"price"`
	BillingCycle BillingCycle `json:"billingCycle"`
	Features     []Feature    `json:"features"`
	TrialDays    int          `json:"trialDays"`
	IsActive     *bool        `json:"isActive"`
}

type CreateSubscriptionRequest struct {
	UserID          string `json:"userId" binding:"required"`
	PlanID          string `json:"planId" binding:"required"`
	PaymentMethodID string `json:"paymentMethodId"`
	AutoRenew       *bool  `json:"autoRenew"`
	StartTrial      *bool  `json:"startTrial"`
}

type UpdateSubscriptionRequest struct {
	PlanID          string `json:"planId"`
	PaymentMethodID string `json:"paymentMethodId"`
	AutoRenew       *bool  `json:"autoRenew"`
}

type CancelSubscriptionRequest struct {
	Reason    string `json:"reason"`
	Immediate *bool  `json:"immediate"` // Cancel immediately or at period end
}

type SubscriptionSearchQuery struct {
	UserID   string             `form:"userId"`
	Status   SubscriptionStatus `form:"status"`
	PlanType PlanType           `form:"planType"`
	Page     int                `form:"page,default=1"`
	PageSize int                `form:"pageSize,default=20"`
}

// Response DTOs
type SubscriptionWithPlan struct {
	Subscription *Subscription `json:"subscription"`
	Plan         *Plan         `json:"plan"`
}

type BillingDashboard struct {
	ActiveSubscriptions   int64   `json:"activeSubscriptions"`
	TotalRevenue          float64 `json:"totalRevenue"`
	MonthlyRevenue        float64 `json:"monthlyRevenue"`
	OverdueInvoices       int64   `json:"overdueInvoices"`
	TrialSubscriptions    int64   `json:"trialSubscriptions"`
	ChurnRate             float64 `json:"churnRate"`
	AverageRevenuePerUser float64 `json:"averageRevenuePerUser"`
}

// Search Results
type SubscriptionSearchResult struct {
	Subscriptions []*SubscriptionWithPlan `json:"subscriptions"`
	TotalCount    int64                   `json:"totalCount"`
	PageCount     int                     `json:"pageCount"`
	CurrentPage   int                     `json:"currentPage"`
}
