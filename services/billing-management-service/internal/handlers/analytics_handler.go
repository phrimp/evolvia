package handlers

import (
	"billing-management-service/internal/middleware"
	"billing-management-service/internal/services"
	"context"
	"log"
	"proto-gen/utils"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
)

type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
	}
}

func (h *AnalyticsHandler) RegisterRoutes(app *fiber.App) {
	// Analytics routes group
	analyticsGroup := app.Group("/protected/billing/analytics")

	// Dashboard endpoints
	analyticsGroup.Get("/dashboard/comprehensive", h.GetComprehensiveDashboard, utils.PermissionRequired(middleware.ReadBillingDashboardPermission))
	analyticsGroup.Get("/dashboard/user-metrics", h.GetUserMetrics, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))
	analyticsGroup.Get("/dashboard/revenue-metrics", h.GetRevenueMetrics, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))
	analyticsGroup.Get("/dashboard/real-time", h.GetRealTimeMetrics, utils.PermissionRequired(middleware.ReadBillingDashboardPermission))

	// Trend analysis endpoints
	analyticsGroup.Get("/trends/subscriptions", h.GetSubscriptionTrends, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))
	analyticsGroup.Get("/trends/plans", h.GetPlanPopularity, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))

	// Advanced analytics endpoints
	analyticsGroup.Get("/advanced", h.GetAdvancedAnalytics, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))

	// Admin-specific endpoints
	analyticsGroup.Get("/admin/subscription-stats", h.GetAdminSubscriptionStats, utils.PermissionRequired(middleware.ReadBillingDashboardPermission))
	analyticsGroup.Get("/admin/cancellation-analytics", h.GetCancellationAnalytics, utils.PermissionRequired(middleware.ReadBillingAnalyticsPermission))

	// Health check for analytics service
	analyticsGroup.Get("/health", h.HealthCheck)
}

func (h *AnalyticsHandler) GetComprehensiveDashboard(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dashboard, err := h.analyticsService.GetComprehensiveDashboard(ctx)
	if err != nil {
		log.Printf("Failed to get comprehensive dashboard: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve comprehensive dashboard",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"dashboard": dashboard,
		},
		"meta": fiber.Map{
			"generated_at":   time.Now().Unix(),
			"cache_duration": 300, // 5 minutes cache recommendation
		},
	})
}

func (h *AnalyticsHandler) GetUserMetrics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics, err := h.analyticsService.GetUserMetrics(ctx)
	if err != nil {
		log.Printf("Failed to get user metrics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve user metrics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"metrics": metrics,
		},
	})
}

func (h *AnalyticsHandler) GetRevenueMetrics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics, err := h.analyticsService.GetRevenueMetrics(ctx)
	if err != nil {
		log.Printf("Failed to get revenue metrics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve revenue metrics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"metrics": metrics,
		},
	})
}

func (h *AnalyticsHandler) GetRealTimeMetrics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := h.analyticsService.GetRealTimeMetrics(ctx)
	if err != nil {
		log.Printf("Failed to get real-time metrics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve real-time metrics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"metrics": metrics,
		},
	})
}

func (h *AnalyticsHandler) GetSubscriptionTrends(c fiber.Ctx) error {
	period := c.Query("period", "days") // hours, days, weeks, months
	limit, _ := strconv.Atoi(c.Query("limit", "30"))

	if limit < 1 || limit > 365 {
		limit = 30
	}

	validPeriods := []string{"hours", "days", "weeks", "months"}
	isValidPeriod := false
	for _, p := range validPeriods {
		if period == p {
			isValidPeriod = true
			break
		}
	}

	if !isValidPeriod {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid period. Must be one of: hours, days, weeks, months",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	trends, err := h.analyticsService.GetSubscriptionTrends(ctx, period, limit)
	if err != nil {
		log.Printf("Failed to get subscription trends: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve subscription trends",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"trends": trends,
		},
		"params": fiber.Map{
			"period": period,
			"limit":  limit,
		},
	})
}

func (h *AnalyticsHandler) GetPlanPopularity(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	popularity, err := h.analyticsService.GetPlanPopularity(ctx)
	if err != nil {
		log.Printf("Failed to get plan popularity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve plan popularity",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"popularity": popularity,
		},
	})
}

func (h *AnalyticsHandler) GetAdvancedAnalytics(c fiber.Ctx) error {
	timeRange := c.Query("timeRange", "30d") // 7d, 30d, 90d, 1y

	// Validate time range
	validRanges := []string{"7d", "30d", "90d", "1y"}
	isValidRange := false
	for _, r := range validRanges {
		if timeRange == r {
			isValidRange = true
			break
		}
	}

	if !isValidRange {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid time range. Must be one of: 7d, 30d, 90d, 1y",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	analytics, err := h.analyticsService.GetAdvancedAnalytics(ctx, timeRange)
	if err != nil {
		log.Printf("Failed to get advanced analytics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve advanced analytics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"analytics": analytics,
		},
		"params": fiber.Map{
			"timeRange": timeRange,
		},
	})
}

func (h *AnalyticsHandler) GetAdminSubscriptionStats(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := h.analyticsService.GetAdminSubscriptionStats(ctx)
	if err != nil {
		log.Printf("Failed to get admin subscription stats: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve admin subscription statistics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"stats": stats,
		},
		"meta": fiber.Map{
			"generated_at":   stats.GeneratedAt,
			"cache_duration": 300, // 5 minutes cache recommendation
		},
	})
}

func (h *AnalyticsHandler) GetCancellationAnalytics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	analytics, err := h.analyticsService.GetCancellationAnalytics(ctx)
	if err != nil {
		log.Printf("Failed to get cancellation analytics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve cancellation analytics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"analytics": analytics,
		},
		"meta": fiber.Map{
			"generated_at":   time.Now().Unix(),
			"cache_duration": 600, // 10 minutes cache recommendation
		},
	})
}

func (h *AnalyticsHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "healthy",
		"service":   "analytics",
		"timestamp": time.Now().Unix(),
	})
}
