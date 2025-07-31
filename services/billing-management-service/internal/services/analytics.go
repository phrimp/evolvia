package services

import (
	"billing-management-service/internal/models"
	"billing-management-service/internal/repository"
	"context"
	"fmt"
	"time"
)

type AnalyticsService struct {
	analyticsRepo    *repository.AnalyticsRepository
	subscriptionRepo *repository.SubscriptionRepository
	planRepo         *repository.PlanRepository
}

func NewAnalyticsService(
	analyticsRepo *repository.AnalyticsRepository,
	subscriptionRepo *repository.SubscriptionRepository,
	planRepo *repository.PlanRepository,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo:    analyticsRepo,
		subscriptionRepo: subscriptionRepo,
		planRepo:         planRepo,
	}
}

// GetComprehensiveDashboard returns complete dashboard metrics
func (s *AnalyticsService) GetComprehensiveDashboard(ctx context.Context) (*models.ComprehensiveDashboard, error) {
	// Get user metrics
	userMetrics, err := s.analyticsRepo.GetUserMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user metrics: %w", err)
	}

	// Get revenue metrics
	revenueMetrics, err := s.analyticsRepo.GetRevenueMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue metrics: %w", err)
	}

	// Get real-time metrics
	realTimeMetrics, err := s.analyticsRepo.GetRealTimeMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get real-time metrics: %w", err)
	}

	// Calculate churn rate (simplified - should be based on historical data)
	churnRate := s.calculateChurnRate(userMetrics)

	dashboard := &models.ComprehensiveDashboard{
		// User metrics
		TotalUsers:             userMetrics.TotalUsers,
		ActiveSubscriptions:    userMetrics.ActiveSubscriptions,
		TrialSubscriptions:     userMetrics.TrialSubscriptions,
		CanceledSubscriptions:  userMetrics.CanceledSubscriptions,
		SuspendedSubscriptions: userMetrics.SuspendedSubscriptions,
		PastDueSubscriptions:   userMetrics.PastDueSubscriptions,
		InactiveSubscriptions:  userMetrics.InactiveSubscriptions,
		SubscriptionPercentage: userMetrics.SubscriptionPercentage,

		// Revenue metrics
		TotalRevenue:          revenueMetrics.TotalRevenue,
		MonthlyRevenue:        revenueMetrics.MonthlyRevenue,
		YearlyRevenue:         revenueMetrics.YearlyRevenue,
		AverageRevenue:        revenueMetrics.AveragePrice,
		AverageRevenuePerUser: revenueMetrics.AverageRevenuePerUser,

		// Growth metrics
		NewSubscriptionsToday:    realTimeMetrics.NewSubscriptionsToday,
		NewSubscriptionsThisWeek: realTimeMetrics.NewSubscriptionsThisWeek,
		ChurnRate:                churnRate,
	}

	return dashboard, nil
}

// GetSubscriptionTrends returns subscription trends over time
func (s *AnalyticsService) GetSubscriptionTrends(ctx context.Context, period string, limit int) (*models.SubscriptionTrends, error) {
	// Validate parameters
	validPeriods := []string{"hours", "days", "weeks", "months"}
	isValid := false
	for _, p := range validPeriods {
		if period == p {
			isValid = true
			break
		}
	}

	if !isValid {
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	if limit < 1 || limit > 365 {
		limit = 30
	}

	trends, err := s.analyticsRepo.GetSubscriptionTrends(ctx, period, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription trends: %w", err)
	}

	// Format dates for frontend consumption
	s.formatTrendDates(trends)

	return trends, nil
}

// GetPlanPopularity returns plan popularity statistics
func (s *AnalyticsService) GetPlanPopularity(ctx context.Context) (*models.PlanPopularity, error) {
	popularity, err := s.analyticsRepo.GetPlanPopularity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan popularity: %w", err)
	}

	return popularity, nil
}

// GetRealTimeMetrics returns current real-time metrics
func (s *AnalyticsService) GetRealTimeMetrics(ctx context.Context) (*models.RealTimeMetrics, error) {
	metrics, err := s.analyticsRepo.GetRealTimeMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get real-time metrics: %w", err)
	}

	return metrics, nil
}

// GetUserMetrics returns detailed user metrics
func (s *AnalyticsService) GetUserMetrics(ctx context.Context) (*models.UserMetrics, error) {
	metrics, err := s.analyticsRepo.GetUserMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user metrics: %w", err)
	}

	return metrics, nil
}

// GetRevenueMetrics returns detailed revenue metrics
func (s *AnalyticsService) GetRevenueMetrics(ctx context.Context) (*models.RevenueMetrics, error) {
	metrics, err := s.analyticsRepo.GetRevenueMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue metrics: %w", err)
	}

	return metrics, nil
}

// GetAdvancedAnalytics returns advanced analytics data
func (s *AnalyticsService) GetAdvancedAnalytics(ctx context.Context, timeRange string) (*models.AdvancedAnalytics, error) {
	// This could include more complex analytics like:
	// - Cohort analysis
	// - Customer lifetime value
	// - Conversion funnel metrics
	// - Predictive analytics

	userMetrics, err := s.analyticsRepo.GetUserMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user metrics: %w", err)
	}

	revenueMetrics, err := s.analyticsRepo.GetRevenueMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue metrics: %w", err)
	}

	// Calculate additional metrics
	conversionRate := s.calculateConversionRate(userMetrics)
	ltv := s.calculateCustomerLTV(revenueMetrics, userMetrics)

	analytics := &models.AdvancedAnalytics{
		TimeRange:             timeRange,
		ConversionRate:        conversionRate,
		CustomerLifetimeValue: ltv,
		ChurnRate:             s.calculateChurnRate(userMetrics),

		// Cohort data would be calculated here
		CohortData: make([]models.CohortData, 0),

		// Prediction data would be calculated here
		GrowthProjection: &models.GrowthProjection{
			NextMonth:   s.projectGrowth(userMetrics.ActiveSubscriptions, 0.05), // 5% growth estimate
			NextQuarter: s.projectGrowth(userMetrics.ActiveSubscriptions, 0.15), // 15% growth estimate
			NextYear:    s.projectGrowth(userMetrics.ActiveSubscriptions, 0.60), // 60% growth estimate
		},
	}

	return analytics, nil
}

// Helper methods

func (s *AnalyticsService) calculateChurnRate(userMetrics *models.UserMetrics) float64 {
	totalActiveUsers := userMetrics.ActiveSubscriptions + userMetrics.TrialSubscriptions
	if totalActiveUsers == 0 {
		return 0
	}

	// Simplified churn rate calculation
	// In practice, this should be calculated based on historical data
	return float64(userMetrics.CanceledSubscriptions) / float64(totalActiveUsers+userMetrics.CanceledSubscriptions) * 100
}

func (s *AnalyticsService) calculateConversionRate(userMetrics *models.UserMetrics) float64 {
	if userMetrics.TrialSubscriptions == 0 {
		return 0
	}

	// This is a simplified calculation
	// In practice, you'd track trial-to-paid conversions over time
	return float64(userMetrics.ActiveSubscriptions) / float64(userMetrics.TrialSubscriptions+userMetrics.ActiveSubscriptions) * 100
}

func (s *AnalyticsService) calculateCustomerLTV(revenueMetrics *models.RevenueMetrics, userMetrics *models.UserMetrics) float64 {
	if userMetrics.ActiveSubscriptions == 0 {
		return 0
	}

	// Simplified LTV calculation: Average revenue per user * estimated lifetime
	averageLifetimeMonths := 24.0 // Assume 24 months average lifetime
	return revenueMetrics.AverageRevenuePerUser * averageLifetimeMonths
}

func (s *AnalyticsService) projectGrowth(currentValue int64, growthRate float64) int64 {
	return int64(float64(currentValue) * (1 + growthRate))
}

func (s *AnalyticsService) formatTrendDates(trends *models.SubscriptionTrends) {
	for i := range trends.Data {
		// Format the period data into readable date strings
		// This would depend on the period type and structure
		if periodData, ok := trends.Data[i].Period.(map[string]interface{}); ok {
			switch trends.Period {
			case "days":
				if year, ok := periodData["year"].(int32); ok {
					if month, ok := periodData["month"].(int32); ok {
						if day, ok := periodData["day"].(int32); ok {
							date := time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC)
							trends.Data[i].Date = date.Format("2006-01-02")
						}
					}
				}
			case "months":
				if year, ok := periodData["year"].(int32); ok {
					if month, ok := periodData["month"].(int32); ok {
						date := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
						trends.Data[i].Date = date.Format("2006-01")
					}
				}
			case "weeks":
				if year, ok := periodData["year"].(int32); ok {
					if week, ok := periodData["week"].(int32); ok {
						trends.Data[i].Date = fmt.Sprintf("%d-W%02d", year, week)
					}
				}
			case "hours":
				if year, ok := periodData["year"].(int32); ok {
					if month, ok := periodData["month"].(int32); ok {
						if day, ok := periodData["day"].(int32); ok {
							if hour, ok := periodData["hour"].(int32); ok {
								date := time.Date(int(year), time.Month(month), int(day), int(hour), 0, 0, 0, time.UTC)
								trends.Data[i].Date = date.Format("2006-01-02 15:00")
							}
						}
					}
				}
			}
		}
	}
}

// GetAdminSubscriptionStats returns comprehensive admin subscription statistics
func (s *AnalyticsService) GetAdminSubscriptionStats(ctx context.Context) (*models.AdminSubscriptionStats, error) {
	stats, err := s.analyticsRepo.GetAdminSubscriptionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin subscription stats: %w", err)
	}

	return stats, nil
}

// GetCancellationAnalytics returns detailed cancellation analytics
func (s *AnalyticsService) GetCancellationAnalytics(ctx context.Context) (*models.CancellationAnalytics, error) {
	analytics, err := s.analyticsRepo.GetCancellationAnalytics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cancellation analytics: %w", err)
	}

	return analytics, nil
}
