package models

// Dashboard Models
type ComprehensiveDashboard struct {
	// User metrics
	TotalUsers             int64   `json:"totalUsers"`
	ActiveSubscriptions    int64   `json:"activeSubscriptions"`
	TrialSubscriptions     int64   `json:"trialSubscriptions"`
	CanceledSubscriptions  int64   `json:"canceledSubscriptions"`
	SuspendedSubscriptions int64   `json:"suspendedSubscriptions"`
	PastDueSubscriptions   int64   `json:"pastDueSubscriptions"`
	InactiveSubscriptions  int64   `json:"inactiveSubscriptions"`
	SubscriptionPercentage float64 `json:"subscriptionPercentage"`

	// Revenue metrics
	TotalRevenue          float64 `json:"totalRevenue"`
	MonthlyRevenue        float64 `json:"monthlyRevenue"`
	YearlyRevenue         float64 `json:"yearlyRevenue"`
	AverageRevenue        float64 `json:"averageRevenue"`
	AverageRevenuePerUser float64 `json:"averageRevenuePerUser"`

	// Growth metrics
	NewSubscriptionsToday    int64   `json:"newSubscriptionsToday"`
	NewSubscriptionsThisWeek int64   `json:"newSubscriptionsThisWeek"`
	ChurnRate                float64 `json:"churnRate"`
}

// User Metrics
type UserMetrics struct {
	TotalUsers             int64   `json:"totalUsers"`
	ActiveSubscriptions    int64   `json:"activeSubscriptions"`
	TrialSubscriptions     int64   `json:"trialSubscriptions"`
	CanceledSubscriptions  int64   `json:"canceledSubscriptions"`
	SuspendedSubscriptions int64   `json:"suspendedSubscriptions"`
	PastDueSubscriptions   int64   `json:"pastDueSubscriptions"`
	InactiveSubscriptions  int64   `json:"inactiveSubscriptions"`
	SubscriptionPercentage float64 `json:"subscriptionPercentage"`
}

// Revenue Metrics
type RevenueMetrics struct {
	TotalRevenue          float64 `json:"totalRevenue"`
	MonthlyRevenue        float64 `json:"monthlyRevenue"`
	YearlyRevenue         float64 `json:"yearlyRevenue"`
	AveragePrice          float64 `json:"averagePrice"`
	AverageRevenuePerUser float64 `json:"averageRevenuePerUser"`
}

// Real-time Metrics
type RealTimeMetrics struct {
	NewSubscriptionsToday    int64 `json:"newSubscriptionsToday"`
	NewSubscriptionsThisWeek int64 `json:"newSubscriptionsThisWeek"`
	ActiveSubscriptions      int64 `json:"activeSubscriptions"`
	Timestamp                int64 `json:"timestamp"`
}

// Subscription Trends
type SubscriptionTrends struct {
	Period string      `json:"period"` // "hours", "days", "weeks", "months"
	Data   []TrendData `json:"data"`
}

type TrendData struct {
	Period interface{} `json:"period"` // Could be different structures based on grouping
	Count  int64       `json:"count"`
	Date   string      `json:"date,omitempty"` // Formatted date string for frontend
}

// Plan Analytics
type PlanPopularity struct {
	Plans           []PlanStatistics `json:"plans"`
	MostPopularPlan string           `json:"mostPopularPlan"`
	TotalRevenue    float64          `json:"totalRevenue"`
}

type PlanStatistics struct {
	PlanID          string   `json:"planId"`
	PlanName        string   `json:"planName"`
	PlanType        PlanType `json:"planType"`
	Price           float64  `json:"price"`
	SubscriberCount int64    `json:"subscriberCount"`
	Revenue         float64  `json:"revenue"`
	Percentage      float64  `json:"percentage"`
}

// Advanced Analytics
type AdvancedAnalytics struct {
	TimeRange             string            `json:"timeRange"`
	ConversionRate        float64           `json:"conversionRate"`
	CustomerLifetimeValue float64           `json:"customerLifetimeValue"`
	ChurnRate             float64           `json:"churnRate"`
	CohortData            []CohortData      `json:"cohortData"`
	GrowthProjection      *GrowthProjection `json:"growthProjection"`
}

type CohortData struct {
	CohortMonth    string    `json:"cohortMonth"`
	InitialUsers   int64     `json:"initialUsers"`
	RetentionRates []float64 `json:"retentionRates"` // Retention rate for each subsequent month
}

type GrowthProjection struct {
	NextMonth   int64 `json:"nextMonth"`
	NextQuarter int64 `json:"nextQuarter"`
	NextYear    int64 `json:"nextYear"`
}

// Performance Metrics
type PerformanceMetrics struct {
	ConversionFunnel *ConversionFunnel `json:"conversionFunnel"`
	RevenueBreakdown *RevenueBreakdown `json:"revenueBreakdown"`
	CustomerSegments []CustomerSegment `json:"customerSegments"`
	SeasonalTrends   []SeasonalTrend   `json:"seasonalTrends"`
}

type ConversionFunnel struct {
	Visitors          int64   `json:"visitors"`
	TrialSignups      int64   `json:"trialSignups"`
	PaidConversions   int64   `json:"paidConversions"`
	VisitorToTrial    float64 `json:"visitorToTrial"`
	TrialToPaid       float64 `json:"trialToPaid"`
	OverallConversion float64 `json:"overallConversion"`
}

type RevenueBreakdown struct {
	ByPlanType     []PlanTypeRevenue `json:"byPlanType"`
	ByBillingCycle []BillingRevenue  `json:"byBillingCycle"`
	MonthlyGrowth  float64           `json:"monthlyGrowth"`
	YearlyGrowth   float64           `json:"yearlyGrowth"`
}

type PlanTypeRevenue struct {
	PlanType PlanType `json:"planType"`
	Revenue  float64  `json:"revenue"`
	Count    int64    `json:"count"`
}

type BillingRevenue struct {
	BillingCycle BillingCycle `json:"billingCycle"`
	Revenue      float64      `json:"revenue"`
	Count        int64        `json:"count"`
}

type CustomerSegment struct {
	SegmentName    string  `json:"segmentName"`
	UserCount      int64   `json:"userCount"`
	AverageRevenue float64 `json:"averageRevenue"`
	ChurnRate      float64 `json:"churnRate"`
	LifetimeValue  float64 `json:"lifetimeValue"`
}

type SeasonalTrend struct {
	Month             int     `json:"month"`
	MonthName         string  `json:"monthName"`
	SubscriptionCount int64   `json:"subscriptionCount"`
	Revenue           float64 `json:"revenue"`
	GrowthRate        float64 `json:"growthRate"`
}

// Admin Subscription Statistics
type AdminSubscriptionStats struct {
	// Basic counts
	TotalSubscriptions    int64   `json:"totalSubscriptions"`
	ActiveSubscriptions   int64   `json:"activeSubscriptions"`
	CanceledSubscriptions int64   `json:"canceledSubscriptions"`
	TrialSubscriptions    int64   `json:"trialSubscriptions"`
	SuspendedSubscriptions int64  `json:"suspendedSubscriptions"`
	PastDueSubscriptions  int64   `json:"pastDueSubscriptions"`
	InactiveSubscriptions int64   `json:"inactiveSubscriptions"`

	// Rate calculations
	SubscriptionRate      float64 `json:"subscriptionRate"`      // Active/Total users
	CancelRate            float64 `json:"cancelRate"`            // Canceled/Total subscriptions
	TrialConversionRate   float64 `json:"trialConversionRate"`   // Active/(Active+Trial)
	ChurnRate             float64 `json:"churnRate"`             // Monthly churn percentage
	GrowthRate            float64 `json:"growthRate"`            // Monthly growth percentage

	// Time-based metrics
	NewSubscriptionsToday    int64 `json:"newSubscriptionsToday"`
	NewSubscriptionsThisWeek int64 `json:"newSubscriptionsThisWeek"`
	NewSubscriptionsThisMonth int64 `json:"newSubscriptionsThisMonth"`
	CancelationsToday        int64 `json:"cancelationsToday"`
	CancelationsThisWeek     int64 `json:"cancelationsThisWeek"`
	CancelationsThisMonth    int64 `json:"cancelationsThisMonth"`

	// Revenue metrics
	TotalRevenue          float64 `json:"totalRevenue"`
	MonthlyRevenue        float64 `json:"monthlyRevenue"`
	AverageRevenuePerUser float64 `json:"averageRevenuePerUser"`

	// Timestamp
	GeneratedAt int64 `json:"generatedAt"`
}

// Cancellation Analytics
type CancellationAnalytics struct {
	TotalCancellations     int64                    `json:"totalCancellations"`
	CancelRate             float64                  `json:"cancelRate"`
	CancelationsByReason   []CancellationByReason   `json:"cancelationsByReason"`
	CancelationsByPlan     []CancellationByPlan     `json:"cancelationsByPlan"`
	CancelationTrends      []CancellationTrendData  `json:"cancelationTrends"`
	AverageSubscriptionLife float64                 `json:"averageSubscriptionLife"` // days
}

type CancellationByReason struct {
	Reason string  `json:"reason"`
	Count  int64   `json:"count"`
	Rate   float64 `json:"rate"`
}

type CancellationByPlan struct {
	PlanName   string  `json:"planName"`
	PlanType   string  `json:"planType"`
	Count      int64   `json:"count"`
	Rate       float64 `json:"rate"`
	TotalUsers int64   `json:"totalUsers"`
}

type CancellationTrendData struct {
	Period         string `json:"period"`
	Cancellations  int64  `json:"cancellations"`
	NewSignups     int64  `json:"newSignups"`
	CancelRate     float64 `json:"cancelRate"`
	Date           string `json:"date,omitempty"`
}

// Geographic Analytics
type GeographicAnalytics struct {
	UsersByCountry  []CountryData `json:"usersByCountry"`
	RevenueByRegion []RegionData  `json:"revenueByRegion"`
	TopMarkets      []MarketData  `json:"topMarkets"`
}

type CountryData struct {
	CountryCode string  `json:"countryCode"`
	CountryName string  `json:"countryName"`
	UserCount   int64   `json:"userCount"`
	Revenue     float64 `json:"revenue"`
}

type RegionData struct {
	Region    string  `json:"region"`
	UserCount int64   `json:"userCount"`
	Revenue   float64 `json:"revenue"`
}

type MarketData struct {
	Market            string  `json:"market"`
	UserCount         int64   `json:"userCount"`
	Revenue           float64 `json:"revenue"`
	GrowthRate        float64 `json:"growthRate"`
	MarketPenetration float64 `json:"marketPenetration"`
}

// Financial Analytics
type FinancialAnalytics struct {
	MRR                 float64             `json:"mrr"`           // Monthly Recurring Revenue
	ARR                 float64             `json:"arr"`           // Annual Recurring Revenue
	LTV                 float64             `json:"ltv"`           // Customer Lifetime Value
	CAC                 float64             `json:"cac"`           // Customer Acquisition Cost
	PaybackPeriod       float64             `json:"paybackPeriod"` // Months to recover CAC
	RevenueChurn        float64             `json:"revenueChurn"`  // Revenue churn rate
	NetRevenueRetention float64             `json:"netRevenueRetention"`
	CashFlow            []CashFlowData      `json:"cashFlow"`
	RevenueForecasting  *RevenueForecasting `json:"revenueForecasting"`
}

type CashFlowData struct {
	Month   string  `json:"month"`
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
	NetFlow float64 `json:"netFlow"`
}

type RevenueForecasting struct {
	Next30Days  float64 `json:"next30Days"`
	Next60Days  float64 `json:"next60Days"`
	Next90Days  float64 `json:"next90Days"`
	Confidence  float64 `json:"confidence"`
	Methodology string  `json:"methodology"`
}
