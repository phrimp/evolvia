package handlers

import (
	"context"
	"knowledge-service/internal/middleware"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"knowledge-service/internal/services"
	"log"
	"proto-gen/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type CategoryHandler struct {
	categoryService *services.CategoryService
}

func NewCategoryHandler(categoryService *services.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

func (h *CategoryHandler) RegisterRoutes(app *fiber.App) {
	// All category routes are protected and require permissions
	protectedGroup := app.Group("/protected/categories")

	// Category CRUD operations - require specific permissions
	protectedGroup.Post("/", h.CreateCategory, utils.PermissionRequired(middleware.WriteSkillPermission))
	protectedGroup.Get("/", h.ListCategories)
	protectedGroup.Get("/tree", h.GetCategoryTree)
	protectedGroup.Get("/roots", h.GetRootCategories)
	protectedGroup.Get("/:id", h.GetCategory)
	protectedGroup.Put("/:id", h.UpdateCategory, utils.PermissionRequired(middleware.UpdateSkillPermission))
	protectedGroup.Delete("/:id", h.DeleteCategory, utils.PermissionRequired(middleware.DeleteSkillPermission))

	// Category management operations
	protectedGroup.Get("/:id/children", h.GetCategoryChildren)
	protectedGroup.Post("/:id/move", h.MoveCategory, utils.PermissionRequired(middleware.UpdateSkillPermission))
	protectedGroup.Get("/statistics", h.GetCategoryStatistics, utils.RequireAnyPermission(middleware.AdminSkillPermission, middleware.ReadKnowledgeAnalyticsPermission))

	// Health check
	protectedGroup.Get("/health", h.HealthCheck)
}

func (h *CategoryHandler) CreateCategory(c fiber.Ctx) error {
	var category models.SkillCategory

	if err := c.Bind().Body(&category); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createdCategory, err := h.categoryService.CreateCategory(ctx, &category)
	if err != nil {
		log.Printf("Failed to create category: %v", err)

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "already exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "parent category not found") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create category",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Category created successfully",
		"data": fiber.Map{
			"category": createdCategory,
		},
	})
}

func (h *CategoryHandler) GetCategory(c fiber.Ctx) error {
	categoryID := c.Params("id")
	if categoryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(categoryID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	category, err := h.categoryService.GetCategoryByID(ctx, objectID)
	if err != nil {
		log.Printf("Failed to get category %s: %v", categoryID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Category not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve category",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"category": category,
		},
	})
}

func (h *CategoryHandler) UpdateCategory(c fiber.Ctx) error {
	categoryID := c.Params("id")
	if categoryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(categoryID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	var category models.SkillCategory

	if err := c.Bind().Body(&category); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updatedCategory, err := h.categoryService.UpdateCategory(ctx, objectID, &category)
	if err != nil {
		log.Printf("Failed to update category %s: %v", categoryID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Category not found",
			})
		}

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "already exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "circular reference") || strings.Contains(err.Error(), "cannot be its own parent") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update category",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Category updated successfully",
		"data": fiber.Map{
			"category": updatedCategory,
		},
	})
}

func (h *CategoryHandler) DeleteCategory(c fiber.Ctx) error {
	categoryID := c.Params("id")
	if categoryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(categoryID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.categoryService.DeleteCategory(ctx, objectID)
	if err != nil {
		log.Printf("Failed to delete category %s: %v", categoryID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Category not found",
			})
		}

		if strings.Contains(err.Error(), "with child categories") || strings.Contains(err.Error(), "with associated skills") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete category",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Category deleted successfully",
	})
}

func (h *CategoryHandler) ListCategories(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	sortBy := c.Query("sortBy", "name")
	sortDesc := c.Query("sortDesc", "false") == "true"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Parse optional filters
	var parentID *bson.ObjectID
	if parentIDStr := c.Query("parentID"); parentIDStr != "" {
		if objID, err := bson.ObjectIDFromHex(parentIDStr); err == nil {
			parentID = &objID
		}
	}

	level, _ := strconv.Atoi(c.Query("level", "-1"))
	namePattern := c.Query("namePattern")

	opts := repository.CategoryListOptions{
		Limit:       limit,
		Offset:      (page - 1) * limit,
		SortBy:      sortBy,
		SortDesc:    sortDesc,
		ParentID:    parentID,
		Level:       level,
		NamePattern: namePattern,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	categories, total, err := h.categoryService.ListCategories(ctx, opts)
	if err != nil {
		log.Printf("Failed to list categories: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve categories",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"categories": categories,
			"pagination": fiber.Map{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": (total + int64(limit) - 1) / int64(limit),
			},
		},
	})
}

func (h *CategoryHandler) GetRootCategories(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	categories, err := h.categoryService.GetRootCategories(ctx)
	if err != nil {
		log.Printf("Failed to get root categories: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve root categories",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"categories": categories,
			"count":      len(categories),
		},
	})
}

func (h *CategoryHandler) GetCategoryChildren(c fiber.Ctx) error {
	categoryID := c.Params("id")
	if categoryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(categoryID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	children, err := h.categoryService.GetCategoriesByParent(ctx, &objectID)
	if err != nil {
		log.Printf("Failed to get category children for %s: %v", categoryID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve category children",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"children": children,
			"parentID": objectID,
			"count":    len(children),
		},
	})
}

func (h *CategoryHandler) GetCategoryTree(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tree, err := h.categoryService.GetCategoryTree(ctx)
	if err != nil {
		log.Printf("Failed to get category tree: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve category tree",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"tree": tree,
		},
	})
}

func (h *CategoryHandler) MoveCategory(c fiber.Ctx) error {
	categoryID := c.Params("id")
	if categoryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(categoryID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	var req struct {
		NewParentID *string `json:"new_parent_id"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var newParentID *bson.ObjectID
	if req.NewParentID != nil && *req.NewParentID != "" {
		objID, err := bson.ObjectIDFromHex(*req.NewParentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid new parent ID format",
			})
		}
		newParentID = &objID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.categoryService.MoveCategory(ctx, objectID, newParentID)
	if err != nil {
		log.Printf("Failed to move category %s: %v", categoryID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Category not found",
			})
		}

		if strings.Contains(err.Error(), "circular reference") || strings.Contains(err.Error(), "cannot be its own parent") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to move category",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Category moved successfully",
	})
}

func (h *CategoryHandler) GetCategoryStatistics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats, err := h.categoryService.GetCategoryStatistics(ctx)
	if err != nil {
		log.Printf("Failed to get category statistics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve category statistics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"statistics": stats,
		},
	})
}

func (h *CategoryHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Knowledge Service - Categories is healthy")
}
