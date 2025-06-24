package handlers

import (
	"billing-management-service/internal/middleware"
	"billing-management-service/internal/models"
	"billing-management-service/internal/services"
	"context"
	"log"
	"proto-gen/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SubscriptionHandler struct {
	subscriptionService *services.SubscriptionService
}

func NewSubscriptionHandler(subscriptionService *services.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
	}
}

func (h *SubscriptionHandler) RegisterRoutes(app *fiber.App) {
	// Protected routes group
	protectedGroup := app.Group("/protected/billing/subscriptions")

	// Subscription CRUD operations
	protectedGroup.Post("/", h.CreateSubscription, utils.PermissionRequired(middleware.WriteSubscriptionPermission))
	protectedGroup.Get("/:id", h.GetSubscription, utils.PermissionRequired(middleware.ReadSubscriptionPermission))
	protectedGroup.Put("/:id", h.UpdateSubscription, utils.PermissionRequired(middleware.UpdateSubscriptionPermission))
	protectedGroup.Delete("/:id", h.CancelSubscription, utils.PermissionRequired(middleware.DeleteSubscriptionPermission))

	// User-specific subscription access (users can access their own subscriptions)
	protectedGroup.Get("/user/:userId", h.GetSubscriptionByUserID, utils.OwnerPermissionRequired(""))

	// Subscription management operations - require manage permissions
	protectedGroup.Patch("/:id/renew", h.RenewSubscription, utils.PermissionRequired(middleware.ManageSubscriptionPermission))
	protectedGroup.Patch("/:id/suspend", h.SuspendSubscription, utils.PermissionRequired(middleware.ManageSubscriptionPermission))
	protectedGroup.Patch("/:id/reactivate", h.ReactivateSubscription, utils.PermissionRequired(middleware.ManageSubscriptionPermission))

	// Admin-only operations
	protectedGroup.Get("/", h.SearchSubscriptions, utils.PermissionRequired(middleware.ReadAllSubscriptionPermission))
	protectedGroup.Get("/search", h.SearchSubscriptions, utils.PermissionRequired(middleware.ReadAllSubscriptionPermission))
	protectedGroup.Get("/:id/with-plan", h.GetSubscriptionWithPlan, utils.PermissionRequired(middleware.ReadAllSubscriptionPermission))
	protectedGroup.Get("/expiring", h.GetExpiringSubscriptions, utils.PermissionRequired(middleware.ReadAllSubscriptionPermission))

	// Billing dashboard and analytics - require specific billing permissions
	protectedGroup.Get("/dashboard", h.GetBillingDashboard, utils.PermissionRequired(middleware.ReadBillingDashboardPermission))

	// System operations - require billing operations permission
	protectedGroup.Post("/process-trial-expirations", h.ProcessTrialExpirations, utils.PermissionRequired(middleware.ProcessBillingOperationsPermission))
}

func (h *SubscriptionHandler) CreateSubscription(c fiber.Ctx) error {
	var req models.CreateSubscriptionRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Ensure users can only create subscriptions for themselves unless they have admin/manager permissions
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")

	// Check if user has admin/manager permissions or is creating for themselves
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") || strings.Contains(userPermissions, "manager")
	if !hasElevatedPermissions && req.UserID != currentUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You can only create subscriptions for yourself",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	subscription, err := h.subscriptionService.CreateSubscription(ctx, &req)
	if err != nil {
		log.Printf("Failed to create subscription: %v", err)

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "plan not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "plan is not active") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Selected plan is not active",
			})
		}

		if strings.Contains(err.Error(), "already has an active subscription") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "User already has an active subscription",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create subscription",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Subscription created successfully",
		"data": fiber.Map{
			"subscription": subscription,
		},
	})
}

func (h *SubscriptionHandler) GetSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subscription, err := h.subscriptionService.GetSubscription(ctx, subscriptionID)
	if err != nil {
		log.Printf("Failed to get subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve subscription",
		})
	}

	// Check if user can access this subscription (owner or elevated permissions)
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") ||
		strings.Contains(userPermissions, "manager") ||
		strings.Contains(userPermissions, middleware.ReadAllSubscriptionPermission)

	if !hasElevatedPermissions && subscription.UserID != currentUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"subscription": subscription,
		},
	})
}

func (h *SubscriptionHandler) GetSubscriptionByUserID(c fiber.Ctx) error {
	userID := c.Params("userId")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subscription, err := h.subscriptionService.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		log.Printf("Failed to get subscription for user %s: %v", userID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No active subscription found for this user",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve subscription",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"subscription": subscription,
		},
	})
}

func (h *SubscriptionHandler) GetSubscriptionWithPlan(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.subscriptionService.GetSubscriptionWithPlan(ctx, subscriptionID)
	if err != nil {
		log.Printf("Failed to get subscription with plan %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve subscription details",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": result,
	})
}

func (h *SubscriptionHandler) UpdateSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	var req models.UpdateSubscriptionRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check ownership for non-admin users
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") || strings.Contains(userPermissions, "manager")

	if !hasElevatedPermissions {
		// Get subscription to check ownership
		existingSubscription, err := h.subscriptionService.GetSubscription(ctx, subscriptionID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Subscription not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify subscription ownership",
			})
		}

		if existingSubscription.UserID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

	subscription, err := h.subscriptionService.UpdateSubscription(ctx, subscriptionID, &req)
	if err != nil {
		log.Printf("Failed to update subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		if strings.Contains(err.Error(), "plan is not active") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Selected plan is not active",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update subscription",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription updated successfully",
		"data": fiber.Map{
			"subscription": subscription,
		},
	})
}

func (h *SubscriptionHandler) CancelSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	var req models.CancelSubscriptionRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check ownership for non-admin users
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") || strings.Contains(userPermissions, "manager")

	if !hasElevatedPermissions {
		// Get subscription to check ownership
		existingSubscription, err := h.subscriptionService.GetSubscription(ctx, subscriptionID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Subscription not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify subscription ownership",
			})
		}

		if existingSubscription.UserID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

	err := h.subscriptionService.CancelSubscription(ctx, subscriptionID, &req)
	if err != nil {
		log.Printf("Failed to cancel subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		if strings.Contains(err.Error(), "already canceled") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Subscription is already canceled",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to cancel subscription",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription canceled successfully",
	})
}

func (h *SubscriptionHandler) SearchSubscriptions(c fiber.Ctx) error {
	query := &models.SubscriptionSearchQuery{
		UserID:   c.Query("userId"),
		Status:   models.SubscriptionStatus(c.Query("status")),
		PlanType: models.PlanType(c.Query("planType")),
		Page:     1,
		PageSize: 20,
	}

	if page, err := strconv.Atoi(c.Query("page", "1")); err == nil && page > 0 {
		query.Page = page
	}

	if pageSize, err := strconv.Atoi(c.Query("pageSize", "20")); err == nil && pageSize > 0 && pageSize <= 100 {
		query.PageSize = pageSize
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := h.subscriptionService.SearchSubscriptions(ctx, query)
	if err != nil {
		log.Printf("Failed to search subscriptions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search subscriptions",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": result,
	})
}

func (h *SubscriptionHandler) RenewSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.subscriptionService.RenewSubscription(ctx, subscriptionID)
	if err != nil {
		log.Printf("Failed to renew subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		if strings.Contains(err.Error(), "only active subscriptions") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Only active subscriptions can be renewed",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to renew subscription",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription renewed successfully",
	})
}

func (h *SubscriptionHandler) SuspendSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	var req struct {
		Reason string `json:"reason"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Reason == "" {
		req.Reason = "Administrative action"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.subscriptionService.SuspendSubscription(ctx, subscriptionID, req.Reason)
	if err != nil {
		log.Printf("Failed to suspend subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		if strings.Contains(err.Error(), "already suspended") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Subscription is already suspended",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to suspend subscription",
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription suspended successfully",
	})
}

func (h *SubscriptionHandler) ReactivateSubscription(c fiber.Ctx) error {
	subscriptionID := c.Params("id")
	if subscriptionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Subscription ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.subscriptionService.ReactivateSubscription(ctx, subscriptionID)
	if err != nil {
		log.Printf("Failed to reactivate subscription %s: %v", subscriptionID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Subscription not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid subscription ID format",
			})
		}

		if strings.Contains(err.Error(), "only suspended subscriptions") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Only suspended subscriptions can be reactivated",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to reactivate subscription",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Subscription reactivated successfully",
	})
}

func (h *SubscriptionHandler) GetBillingDashboard(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dashboard, err := h.subscriptionService.GetBillingDashboard(ctx)
	if err != nil {
		log.Printf("Failed to get billing dashboard: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve billing dashboard",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"dashboard": dashboard,
		},
	})
}

func (h *SubscriptionHandler) GetExpiringSubscriptions(c fiber.Ctx) error {
	daysAhead, _ := strconv.Atoi(c.Query("daysAhead", "7"))
	if daysAhead < 1 {
		daysAhead = 7
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subscriptions, err := h.subscriptionService.GetExpiringSubscriptions(ctx, daysAhead)
	if err != nil {
		log.Printf("Failed to get expiring subscriptions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve expiring subscriptions",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"subscriptions": subscriptions,
			"daysAhead":     daysAhead,
			"count":         len(subscriptions),
		},
	})
}

func (h *SubscriptionHandler) ProcessTrialExpirations(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.subscriptionService.ProcessTrialExpiration(ctx)
	if err != nil {
		log.Printf("Failed to process trial expirations: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process trial expirations",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Trial expirations processed successfully",
	})
}

func (h *SubscriptionHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Billing Management Service - Subscriptions is healthy")
}
