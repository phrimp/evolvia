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

type SkillHandler struct {
	skillService *services.SkillService
}

func NewSkillHandler(skillService *services.SkillService) *SkillHandler {
	return &SkillHandler{
		skillService: skillService,
	}
}

func (h *SkillHandler) RegisterRoutes(app *fiber.App) {
	// All skill routes are protected and require permissions
	protectedGroup := app.Group("/protected/skills")

	// Skill CRUD operations - require specific permissions
	protectedGroup.Post("/", h.CreateSkill, utils.PermissionRequired(middleware.WriteSkillPermission))
	protectedGroup.Get("/", h.ListSkills)
	protectedGroup.Get("/:id", h.GetSkill)
	protectedGroup.Put("/:id", h.UpdateSkill, utils.PermissionRequired(middleware.UpdateSkillPermission))
	protectedGroup.Delete("/:id", h.DeleteSkill, utils.PermissionRequired(middleware.DeleteSkillPermission))

	// Skill search and query operations - require read permissions
	protectedGroup.Get("/search", h.SearchSkills)
	protectedGroup.Get("/category/:categoryID", h.GetSkillsByCategory)
	protectedGroup.Get("/popular", h.GetMostUsedSkills)
	protectedGroup.Get("/:id/related/:relationType", h.GetRelatedSkills)

	// Skill management operations - require admin permissions
	protectedGroup.Post("/batch", h.BatchCreateSkills, utils.PermissionRequired(middleware.AdminSkillPermission))
	protectedGroup.Post("/reload-data", h.ReloadSkillData, utils.PermissionRequired(middleware.AdminSkillPermission))
	protectedGroup.Get("/statistics", h.GetSkillStatistics, utils.RequireAnyPermission(middleware.AdminSkillPermission, middleware.ReadKnowledgeAnalyticsPermission))

	// Health check
	protectedGroup.Get("/health", h.HealthCheck)
}

func (h *SkillHandler) CreateSkill(c fiber.Ctx) error {
	var skill models.Skill

	if err := c.Bind().Body(&skill); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createdSkill, err := h.skillService.CreateSkill(ctx, &skill)
	if err != nil {
		log.Printf("Failed to create skill: %v", err)

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

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create skill",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Skill created successfully",
		"data": fiber.Map{
			"skill": createdSkill,
		},
	})
}

func (h *SkillHandler) GetSkill(c fiber.Ctx) error {
	skillID := c.Params("id")
	if skillID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(skillID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	skill, err := h.skillService.GetSkillByID(ctx, objectID)
	if err != nil {
		log.Printf("Failed to get skill %s: %v", skillID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"skill": skill,
		},
	})
}

func (h *SkillHandler) UpdateSkill(c fiber.Ctx) error {
	skillID := c.Params("id")
	if skillID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(skillID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	var skill models.Skill

	if err := c.Bind().Body(&skill); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updatedSkill, err := h.skillService.UpdateSkill(ctx, objectID, &skill)
	if err != nil {
		log.Printf("Failed to update skill %s: %v", skillID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
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

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Skill updated successfully",
		"data": fiber.Map{
			"skill": updatedSkill,
		},
	})
}

func (h *SkillHandler) DeleteSkill(c fiber.Ctx) error {
	skillID := c.Params("id")
	if skillID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(skillID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.skillService.DeleteSkill(ctx, objectID)
	if err != nil {
		log.Printf("Failed to delete skill %s: %v", skillID, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Skill deleted successfully",
	})
}

func (h *SkillHandler) ListSkills(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	sortBy := c.Query("sortBy", "name")
	sortDesc := c.Query("sortDesc", "false") == "true"
	activeOnly := c.Query("activeOnly", "true") == "true"

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Parse optional filters
	var categoryID *bson.ObjectID
	if categoryIDStr := c.Query("categoryID"); categoryIDStr != "" {
		if objID, err := bson.ObjectIDFromHex(categoryIDStr); err == nil {
			categoryID = &objID
		}
	}

	var tags []string
	if tagsStr := c.Query("tags"); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	var industry []string
	if industryStr := c.Query("industry"); industryStr != "" {
		industry = strings.Split(industryStr, ",")
		for i, ind := range industry {
			industry[i] = strings.TrimSpace(ind)
		}
	}

	minDifficulty, _ := strconv.Atoi(c.Query("minDifficulty", "0"))
	maxDifficulty, _ := strconv.Atoi(c.Query("maxDifficulty", "0"))

	var trending *bool
	if trendingStr := c.Query("trending"); trendingStr != "" {
		trendingBool := trendingStr == "true"
		trending = &trendingBool
	}

	opts := repository.ListOptions{
		Limit:         limit,
		Offset:        (page - 1) * limit,
		SortBy:        sortBy,
		SortDesc:      sortDesc,
		ActiveOnly:    activeOnly,
		CategoryID:    categoryID,
		Tags:          tags,
		Industry:      industry,
		MinDifficulty: minDifficulty,
		MaxDifficulty: maxDifficulty,
		Trending:      trending,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skills, total, err := h.skillService.ListSkills(ctx, opts)
	if err != nil {
		log.Printf("Failed to list skills: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve skills",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"skills": skills,
			"pagination": fiber.Map{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": (total + int64(limit) - 1) / int64(limit),
			},
		},
	})
}

func (h *SkillHandler) SearchSkills(c fiber.Ctx) error {
	keywords := c.Query("q")
	if keywords == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Search keywords are required (use 'q' parameter)",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Check if enhanced search is requested
	includeCategory := c.Query("includeCategory", "true") == "true"
	searchMode := c.Query("mode", "basic") // basic, enhanced, advanced

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch searchMode {
	case "advanced":
		// Advanced search with match scoring
		results, err := h.skillService.SearchSkillsAdvanced(ctx, keywords, limit)
		if err != nil {
			log.Printf("Failed to perform advanced search for skills: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to search skills",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": fiber.Map{
				"results":  results,
				"keywords": keywords,
				"count":    len(results),
				"mode":     "advanced",
			},
		})

	case "enhanced":
		// Enhanced search with category information
		results, err := h.skillService.SearchSkillsWithCategories(ctx, keywords, limit, includeCategory)
		if err != nil {
			log.Printf("Failed to perform enhanced search for skills: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to search skills",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": fiber.Map{
				"skills":           results,
				"keywords":         keywords,
				"count":            len(results),
				"mode":             "enhanced",
				"include_category": includeCategory,
			},
		})

	default:
		// Basic search (backward compatibility)
		skills, err := h.skillService.SearchSkills(ctx, keywords, limit)
		if err != nil {
			log.Printf("Failed to search skills: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to search skills",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": fiber.Map{
				"skills":   skills,
				"keywords": keywords,
				"count":    len(skills),
				"mode":     "basic",
			},
		})
	}
}

func (h *SkillHandler) GetSkillsByCategory(c fiber.Ctx) error {
	categoryIDStr := c.Params("categoryID")
	if categoryIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Category ID is required",
		})
	}

	categoryID, err := bson.ObjectIDFromHex(categoryIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid category ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skills, err := h.skillService.GetSkillsByCategory(ctx, categoryID)
	if err != nil {
		log.Printf("Failed to get skills by category %s: %v", categoryIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve skills",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"skills":     skills,
			"categoryID": categoryID,
			"count":      len(skills),
		},
	})
}

func (h *SkillHandler) GetMostUsedSkills(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	skills, err := h.skillService.GetMostUsedSkills(ctx, limit)
	if err != nil {
		log.Printf("Failed to get most used skills: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve most used skills",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"skills": skills,
			"count":  len(skills),
		},
	})
}

func (h *SkillHandler) GetRelatedSkills(c fiber.Ctx) error {
	skillIDStr := c.Params("id")
	if skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID is required",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	relationTypeStr := c.Params("relationType")
	if relationTypeStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Relation type is required",
		})
	}

	relationType := models.RelationType(relationTypeStr)
	if !h.isValidRelationType(relationType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid relation type",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skills, err := h.skillService.GetRelatedSkills(ctx, skillID, relationType)
	if err != nil {
		log.Printf("Failed to get related skills for %s: %v", skillIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve related skills",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"skills":       skills,
			"skillID":      skillID,
			"relationType": relationType,
			"count":        len(skills),
		},
	})
}

func (h *SkillHandler) BatchCreateSkills(c fiber.Ctx) error {
	var skills []*models.Skill

	if err := c.Bind().Body(&skills); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(skills) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No skills provided",
		})
	}

	if len(skills) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Too many skills (maximum 100 allowed)",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.skillService.BatchCreateSkills(ctx, skills)
	if err != nil {
		log.Printf("Failed to batch create skills: %v", err)

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "already exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create skills",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Skills created successfully",
		"data": fiber.Map{
			"count": len(skills),
		},
	})
}

func (h *SkillHandler) ReloadSkillData(c fiber.Ctx) error {
	dataDir := c.Query("dataDir", "/data")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := h.skillService.ReloadDataFromFiles(ctx, dataDir)
	if err != nil {
		log.Printf("Failed to reload skill data: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to reload skill data",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Skill data reloaded successfully",
		"dataDir": dataDir,
	})
}

func (h *SkillHandler) GetSkillStatistics(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := h.skillService.GetSkillStatistics(ctx)
	if err != nil {
		log.Printf("Failed to get skill statistics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve skill statistics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"statistics": stats,
		},
	})
}

func (h *SkillHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Knowledge Service - Skills is healthy")
}

// Helper functions
func (h *SkillHandler) isValidRelationType(relationType models.RelationType) bool {
	return relationType == models.RelationPrerequisite ||
		relationType == models.RelationBuildsOn ||
		relationType == models.RelationRelated ||
		relationType == models.RelationComplement ||
		relationType == models.RelationAlternative ||
		relationType == models.RelationSpecialization
}
