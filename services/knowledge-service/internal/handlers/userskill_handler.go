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

type UserSkillHandler struct {
	userSkillService *services.UserSkillService
}

func NewUserSkillHandler(userSkillService *services.UserSkillService) *UserSkillHandler {
	return &UserSkillHandler{
		userSkillService: userSkillService,
	}
}

func (h *UserSkillHandler) RegisterRoutes(app *fiber.App) {
	// All user skill routes are protected and require permissions
	protectedGroup := app.Group("/protected/user-skills")

	// User skill CRUD operations - require specific permissions
	protectedGroup.Post("/", h.AddUserSkill, utils.PermissionRequired(middleware.WriteUserSkillPermission))
	protectedGroup.Get("/user/:userID", h.GetUserSkills, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/user/:userID/skill/:skillID", h.GetUserSkill, utils.OwnerPermissionRequired(""))
	protectedGroup.Put("/user/:userID/skill/:skillID", h.UpdateUserSkill, utils.OwnerPermissionRequired(""))
	protectedGroup.Delete("/user/:userID/skill/:skillID", h.RemoveUserSkill, utils.OwnerPermissionRequired(""))

	// User skill management operations
	protectedGroup.Patch("/user/:userID/skill/:skillID/last-used", h.UpdateLastUsed, utils.OwnerPermissionRequired(""))
	protectedGroup.Patch("/user/:userID/skill/:skillID/endorse", h.EndorseUserSkill, utils.PermissionRequired(middleware.EndorseUserSkillPermission))
	protectedGroup.Patch("/user/:userID/skill/:skillID/verify", h.VerifyUserSkill, utils.RequireAnyPermission(middleware.AdminPermission, middleware.ManagerPermission, middleware.VerifyUserSkillPermission))

	// Query operations - require read permissions
	protectedGroup.Get("/skill/:skillID/users", h.GetUsersWithSkill, utils.PermissionRequired(middleware.ReadUserSkillPermission))
	protectedGroup.Get("/skill/:skillID/top-users", h.GetTopUsersForSkill, utils.PermissionRequired(middleware.ReadUserSkillPermission))
	protectedGroup.Get("/user/:userID/matrix", h.GetUserSkillMatrix, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/user/:userID/gaps/:targetSkillID", h.GetSkillGaps, utils.OwnerPermissionRequired(""))

	protectedGroup.Put("/user/:userID/skill/:skillID/blooms", h.UpdateBloomsAssessment, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/user/:userID/skill/:skillID/blooms", h.GetBloomsAssessment, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/user/:userID/blooms-analytics", h.GetBloomsAnalytics, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/user/:userID/skill/:skillID/focus-area", h.GetRecommendedFocusArea, utils.OwnerPermissionRequired(""))
	protectedGroup.Get("/skill/:skillID/blooms-experts/:bloomsLevel", h.GetBloomsExperts, utils.PermissionRequired(middleware.ReadUserSkillPermission))
	protectedGroup.Patch("/user/:userID/skill/:skillID/auto-level", h.UpdateSkillLevelFromBlooms, utils.OwnerPermissionRequired(""))

	// Batch operations - require admin permissions
	protectedGroup.Post("/batch", h.BatchAddUserSkills, utils.PermissionRequired(middleware.AdminUserSkillPermission))
}

func (h *UserSkillHandler) AddUserSkill(c fiber.Ctx) error {
	var userSkill models.UserSkill

	if err := c.Bind().Body(&userSkill); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Ensure users can only add skills for themselves unless they have admin/manager permissions
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")

	hasElevatedPermissions := strings.Contains(userPermissions, middleware.AdminPermission) || strings.Contains(userPermissions, middleware.ManagerPermission)
	if !hasElevatedPermissions && userSkill.UserID.Hex() != currentUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You can only add skills for yourself",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createdUserSkill, err := h.userSkillService.AddUserSkill(ctx, &userSkill)
	if err != nil {
		log.Printf("Failed to add user skill: %v", err)

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if strings.Contains(err.Error(), "skill not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		if strings.Contains(err.Error(), "already has this skill") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "User already has this skill",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add user skill",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User skill added successfully",
		"data": fiber.Map{
			"userSkill": createdUserSkill,
		},
	})
}

func (h *UserSkillHandler) GetUserSkill(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userSkill, err := h.userSkillService.GetUserSkill(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to get user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve user skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"userSkill": userSkill,
		},
	})
}

func (h *UserSkillHandler) GetUserSkills(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	if userIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	sortBy := c.Query("sortBy", "confidence")
	sortDesc := c.Query("sortDesc", "true") == "true"
	level := models.SkillLevel(c.Query("level"))
	minConfidence, _ := strconv.ParseFloat(c.Query("minConfidence", "0"), 64)
	verifiedOnly := c.Query("verifiedOnly", "false") == "true"

	if limit < 1 || limit > 100 {
		limit = 50
	}

	opts := repository.UserSkillListOptions{
		Limit:         limit,
		Offset:        offset,
		SortBy:        sortBy,
		SortDesc:      sortDesc,
		Level:         level,
		MinConfidence: minConfidence,
		VerifiedOnly:  verifiedOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userSkills, err := h.userSkillService.GetUserSkills(ctx, userID, opts)
	if err != nil {
		log.Printf("Failed to get user skills for %s: %v", userIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve user skills",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"userSkills": userSkills,
			"userID":     userID,
			"count":      len(userSkills),
			"pagination": fiber.Map{
				"limit":  limit,
				"offset": offset,
			},
		},
	})
}

func (h *UserSkillHandler) GetUsersWithSkill(c fiber.Ctx) error {
	skillIDStr := c.Params("skillID")
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

	// Parse query parameters
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	level := models.SkillLevel(c.Query("level"))
	minConfidence, _ := strconv.ParseFloat(c.Query("minConfidence", "0"), 64)
	verifiedOnly := c.Query("verifiedOnly", "false") == "true"

	if limit < 1 || limit > 100 {
		limit = 20
	}

	opts := repository.UserSkillListOptions{
		Limit:         limit,
		Offset:        offset,
		Level:         level,
		MinConfidence: minConfidence,
		VerifiedOnly:  verifiedOnly,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userSkills, err := h.userSkillService.GetUsersWithSkill(ctx, skillID, opts)
	if err != nil {
		log.Printf("Failed to get users with skill %s: %v", skillIDStr, err)

		if strings.Contains(err.Error(), "skill not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve users with skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"userSkills": userSkills,
			"skillID":    skillID,
			"count":      len(userSkills),
			"pagination": fiber.Map{
				"limit":  limit,
				"offset": offset,
			},
		},
	})
}

func (h *UserSkillHandler) UpdateUserSkill(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	var updates services.UserSkillUpdate

	if err := c.Bind().Body(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updatedUserSkill, err := h.userSkillService.UpdateUserSkill(ctx, userID, skillID, &updates)
	if err != nil {
		log.Printf("Failed to update user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		if strings.Contains(err.Error(), "validation failed") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User skill updated successfully",
		"data": fiber.Map{
			"userSkill": updatedUserSkill,
		},
	})
}

func (h *UserSkillHandler) RemoveUserSkill(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.userSkillService.RemoveUserSkill(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to remove user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove user skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User skill removed successfully",
	})
}

func (h *UserSkillHandler) UpdateLastUsed(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = h.userSkillService.UpdateLastUsed(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to update last used for user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update last used",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Last used updated successfully",
	})
}

func (h *UserSkillHandler) EndorseUserSkill(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = h.userSkillService.EndorseUserSkill(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to endorse user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to endorse user skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User skill endorsed successfully",
	})
}

func (h *UserSkillHandler) VerifyUserSkill(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	var req struct {
		Verified bool `json:"verified"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = h.userSkillService.VerifyUserSkill(ctx, userID, skillID, req.Verified)
	if err != nil {
		log.Printf("Failed to verify user skill %s-%s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify user skill",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User skill verification updated successfully",
	})
}

func (h *UserSkillHandler) GetTopUsersForSkill(c fiber.Ctx) error {
	skillIDStr := c.Params("skillID")
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

	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userSkills, err := h.userSkillService.GetTopUsersForSkill(ctx, skillID, limit)
	if err != nil {
		log.Printf("Failed to get top users for skill %s: %v", skillIDStr, err)

		if strings.Contains(err.Error(), "skill not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve top users",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"userSkills": userSkills,
			"skillID":    skillID,
			"count":      len(userSkills),
		},
	})
}

func (h *UserSkillHandler) GetUserSkillMatrix(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	if userIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	matrix, err := h.userSkillService.GetUserSkillMatrix(ctx, userID)
	if err != nil {
		log.Printf("Failed to get user skill matrix for %s: %v", userIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve user skill matrix",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"matrix": matrix,
		},
	})
}

func (h *UserSkillHandler) GetSkillGaps(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	targetSkillIDStr := c.Params("targetSkillID")

	if userIDStr == "" || targetSkillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Target Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	targetSkillID, err := bson.ObjectIDFromHex(targetSkillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid target skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gaps, err := h.userSkillService.GetSkillGaps(ctx, userID, targetSkillID)
	if err != nil {
		log.Printf("Failed to get skill gaps for user %s target %s: %v", userIDStr, targetSkillIDStr, err)

		if strings.Contains(err.Error(), "target skill not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Target skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve skill gaps",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"gaps":          gaps,
			"userID":        userID,
			"targetSkillID": targetSkillID,
			"count":         len(gaps),
		},
	})
}

func (h *UserSkillHandler) BatchAddUserSkills(c fiber.Ctx) error {
	var userSkills []*models.UserSkill

	if err := c.Bind().Body(&userSkills); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(userSkills) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No user skills provided",
		})
	}

	if len(userSkills) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Too many user skills (maximum 100 allowed)",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.userSkillService.BatchAddUserSkills(ctx, userSkills)
	if err != nil {
		log.Printf("Failed to batch add user skills: %v", err)

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
			"error": "Failed to add user skills",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User skills added successfully",
		"data": fiber.Map{
			"count": len(userSkills),
		},
	})
}

func (h *UserSkillHandler) UpdateBloomsAssessment(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	var assessment models.BloomsTaxonomyAssessment
	if err := c.Bind().Body(&assessment); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.userSkillService.UpdateBloomsAssessment(ctx, userID, skillID, &assessment)
	if err != nil {
		log.Printf("Failed to update Bloom's assessment for user %s skill %s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update Bloom's assessment",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Bloom's assessment updated successfully",
		"data": fiber.Map{
			"assessment": assessment,
		},
	})
}

// GetBloomsAssessment retrieves Bloom's taxonomy scores for a user skill
func (h *UserSkillHandler) GetBloomsAssessment(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assessment, err := h.userSkillService.GetBloomsAssessment(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to get Bloom's assessment for user %s skill %s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve Bloom's assessment",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"assessment":       assessment,
			"overall_score":    assessment.GetOverallScore(),
			"primary_strength": assessment.GetPrimaryStrength(),
			"weakest_area":     assessment.GetWeakestArea(),
		},
	})
}

// GetBloomsAnalytics retrieves aggregated Bloom's data for a user
func (h *UserSkillHandler) GetBloomsAnalytics(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	if userIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	analytics, err := h.userSkillService.GetBloomsAnalytics(ctx, userID)
	if err != nil {
		log.Printf("Failed to get Bloom's analytics for user %s: %v", userIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve Bloom's analytics",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"analytics": analytics,
		},
	})
}

// GetRecommendedFocusArea suggests which Bloom's level to focus on next
func (h *UserSkillHandler) GetRecommendedFocusArea(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	focusArea, err := h.userSkillService.GetRecommendedFocusArea(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to get recommended focus area for user %s skill %s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get recommended focus area",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"recommended_focus": focusArea,
		},
	})
}

// GetBloomsExperts finds users with high proficiency in specific Bloom's level
func (h *UserSkillHandler) GetBloomsExperts(c fiber.Ctx) error {
	skillIDStr := c.Params("skillID")
	bloomsLevel := c.Params("bloomsLevel")

	if skillIDStr == "" || bloomsLevel == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Skill ID and Bloom's level are required",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	minScore, _ := strconv.ParseFloat(c.Query("minScore", "70"), 64)
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	if limit < 1 || limit > 50 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	experts, err := h.userSkillService.GetUsersWithBloomsExpertise(ctx, skillID, bloomsLevel, minScore, limit)
	if err != nil {
		log.Printf("Failed to get Bloom's experts for skill %s level %s: %v", skillIDStr, bloomsLevel, err)

		if strings.Contains(err.Error(), "skill not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Skill not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve experts",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"experts":      experts,
			"skill_id":     skillID,
			"blooms_level": bloomsLevel,
			"min_score":    minScore,
			"count":        len(experts),
		},
	})
}

// UpdateSkillLevelFromBlooms automatically updates skill level based on Bloom's assessment
func (h *UserSkillHandler) UpdateSkillLevelFromBlooms(c fiber.Ctx) error {
	userIDStr := c.Params("userID")
	skillIDStr := c.Params("skillID")

	if userIDStr == "" || skillIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and Skill ID are required",
		})
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	skillID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid skill ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.userSkillService.UpdateSkillLevelFromBlooms(ctx, userID, skillID)
	if err != nil {
		log.Printf("Failed to update skill level from Bloom's for user %s skill %s: %v", userIDStr, skillIDStr, err)

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User skill not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update skill level",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Skill level updated based on Bloom's assessment",
	})
}
