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

type PlanHandler struct {
	planService *services.PlanService
}

func NewPlanHandler(planService *services.PlanService) *PlanHandler {
	return &PlanHandler{
		planService: planService,
	}
}

func (h *PlanHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HealthCheck)

	// Protected routes group
	protectedGroup := app.Group("/protected/plans")

	protectedGroup.Post("/", h.CreatePlan, utils.PermissionRequired(middleware.WritePlanPermission))
	protectedGroup.Get("/", h.ListPlans, utils.PermissionRequired(middleware.ReadAllPlanPermission))
	protectedGroup.Get("/active", h.ListActivePlans)
	protectedGroup.Get("/stats", h.GetPlanStats)
	protectedGroup.Get("/types/:planType", h.GetPlansByType)
	protectedGroup.Get("/:id", h.GetPlan)
	protectedGroup.Put("/:id", h.UpdatePlan, utils.PermissionRequired(middleware.UpdatePlanPermission))
	protectedGroup.Delete("/:id", h.DeletePlan, utils.PermissionRequired(middleware.DeletePlanPermission))
	protectedGroup.Patch("/:id/activate", h.ActivatePlan, utils.PermissionRequired(middleware.UpdatePlanPermission))
	protectedGroup.Patch("/:id/deactivate", h.DeactivatePlan, utils.PermissionRequired(middleware.UpdatePlanPermission))
}

func (h *PlanHandler) CreatePlan(c fiber.Ctx) error {
	var req models.CreatePlanRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plan, err := h.planService.CreatePlan(ctx, &req)
	if err != nil {
		log.Printf("Failed to create plan: %v", err)

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create plan",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Plan created successfully",
		"data": fiber.Map{
			"plan": plan,
		},
	})
}

func (h *PlanHandler) GetPlan(c fiber.Ctx) error {
	planID := c.Params("id")
	if planID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	plan, err := h.planService.GetPlan(ctx, planID)
	if err != nil {
		log.Printf("Failed to get plan %s: %v", planID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid plan ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve plan",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"plan": plan,
		},
	})
}

func (h *PlanHandler) GetPlansByType(c fiber.Ctx) error {
	planTypeStr := c.Params("planType")
	if planTypeStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan type is required",
		})
	}

	planType := models.PlanType(planTypeStr)
	if !h.isValidPlanType(planType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid plan type",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	plans, err := h.planService.GetPlansByType(ctx, planType)
	if err != nil {
		log.Printf("Failed to get plans by type %s: %v", planType, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve plans",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"plans":    plans,
			"planType": planType,
			"count":    len(plans),
		},
	})
}

func (h *PlanHandler) UpdatePlan(c fiber.Ctx) error {
	planID := c.Params("id")
	if planID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID is required",
		})
	}

	var req models.UpdatePlanRequest

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plan, err := h.planService.UpdatePlan(ctx, planID, &req)
	if err != nil {
		log.Printf("Failed to update plan %s: %v", planID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid plan ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update plan",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Plan updated successfully",
		"data": fiber.Map{
			"plan": plan,
		},
	})
}

func (h *PlanHandler) DeletePlan(c fiber.Ctx) error {
	planID := c.Params("id")
	if planID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.planService.DeletePlan(ctx, planID)
	if err != nil {
		log.Printf("Failed to delete plan %s: %v", planID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid plan ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete plan",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Plan deleted successfully",
	})
}

func (h *PlanHandler) ListPlans(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	activeOnly := c.Query("activeOnly", "false") == "true"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plans, err := h.planService.ListPlans(ctx, page, limit, activeOnly)
	if err != nil {
		log.Printf("Failed to list plans: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve plans",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"plans": plans,
			"pagination": fiber.Map{
				"page":       page,
				"limit":      limit,
				"count":      len(plans),
				"activeOnly": activeOnly,
			},
		},
	})
}

func (h *PlanHandler) ListActivePlans(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plans, err := h.planService.ListPlans(ctx, page, limit, true)
	if err != nil {
		log.Printf("Failed to list active plans: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve active plans",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"plans": plans,
			"pagination": fiber.Map{
				"page":  page,
				"limit": limit,
				"count": len(plans),
			},
		},
	})
}

func (h *PlanHandler) GetPlanStats(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := h.planService.GetPlanStats(ctx)
	if err != nil {
		log.Printf("Failed to get plan stats: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve plan statistics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"stats": stats,
		},
	})
}

func (h *PlanHandler) ActivatePlan(c fiber.Ctx) error {
	planID := c.Params("id")
	if planID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.planService.ActivatePlan(ctx, planID)
	if err != nil {
		log.Printf("Failed to activate plan %s: %v", planID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid plan ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to activate plan",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Plan activated successfully",
	})
}

func (h *PlanHandler) DeactivatePlan(c fiber.Ctx) error {
	planID := c.Params("id")
	if planID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.planService.DeactivatePlan(ctx, planID)
	if err != nil {
		log.Printf("Failed to deactivate plan %s: %v", planID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Plan not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid plan ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to deactivate plan",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Plan deactivated successfully",
	})
}

func (h *PlanHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Billing Management Service - Plans is healthy")
}

// Helper functions
func (h *PlanHandler) isValidPlanType(planType models.PlanType) bool {
	return planType == models.PlanTypeFree ||
		planType == models.PlanTypeBasic ||
		planType == models.PlanTypePremium ||
		planType == models.PlanTypeCustom
}
