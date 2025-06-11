package handlers

import (
	"context"
	"log"
	"profile-service/internal/models"
	"profile-service/internal/service"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

func (h *ProfileHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HealthCheck)

	// Protected routes group
	protectedGroup := app.Group("/protected/profiles")

	protectedGroup.Get("/search", h.SearchProfiles)
	protectedGroup.Get("/user/:userId", h.GetProfileByUserID)
	protectedGroup.Get("/:id", h.GetProfile)
	protectedGroup.Put("/:id", h.UpdateProfile)
	protectedGroup.Delete("/:id", h.DeleteProfile)
	protectedGroup.Get("/", h.ListProfiles)
	protectedGroup.Get("/:id/completeness", h.GetProfileCompleteness)
}

func (h *ProfileHandler) GetProfile(c fiber.Ctx) error {
	profileID := c.Params("id")
	if profileID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Profile ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profile, err := h.profileService.GetProfile(ctx, profileID)
	if err != nil {
		log.Printf("Failed to get profile %s: %v", profileID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid profile ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve profile",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profile": profile,
		},
	})
}

func (h *ProfileHandler) GetProfileByUserID(c fiber.Ctx) error {
	userID := c.Params("userId")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profile, err := h.profileService.GetProfileByUserID(ctx, userID)
	if err != nil {
		log.Printf("Failed to get profile for user %s: %v", userID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found for this user",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve profile",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profile": profile,
		},
	})
}

func (h *ProfileHandler) UpdateProfile(c fiber.Ctx) error {
	profileID := c.Params("id")
	if profileID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Profile ID is required",
		})
	}

	var updateRequest struct {
		Profile models.ProfileDTO `json:"profile"`
	}

	if err := c.Bind().Body(&updateRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	req := &models.UpdateProfileRequest{
		ProfileDTO: updateRequest.Profile,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profile, err := h.profileService.UpdateProfile(ctx, profileID, req)
	if err != nil {
		log.Printf("Failed to update profile %s: %v", profileID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid profile ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update profile",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Profile updated successfully",
		"data": fiber.Map{
			"profile": profile,
		},
	})
}

func (h *ProfileHandler) DeleteProfile(c fiber.Ctx) error {
	profileID := c.Params("id")
	if profileID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Profile ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.profileService.DeleteProfile(ctx, profileID)
	if err != nil {
		log.Printf("Failed to delete profile %s: %v", profileID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found",
			})
		}

		if strings.Contains(err.Error(), "invalid") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid profile ID format",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete profile",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Profile deleted successfully",
	})
}

func (h *ProfileHandler) ListProfiles(c fiber.Ctx) error {
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

	profiles, err := h.profileService.ListProfiles(ctx, page, limit)
	if err != nil {
		log.Printf("Failed to list profiles: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve profiles",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profiles": profiles,
			"pagination": fiber.Map{
				"page":  page,
				"limit": limit,
				"count": len(profiles),
			},
		},
	})
}

func (h *ProfileHandler) SearchProfiles(c fiber.Ctx) error {
	query := &models.ProfileSearchQuery{
		Name:        c.Query("name"),
		Institution: c.Query("institution"),
		Field:       c.Query("field"),
		Country:     c.Query("country"),
		Page:        1,
		PageSize:    20,
	}

	if page, err := strconv.Atoi(c.Query("page", "1")); err == nil && page > 0 {
		query.Page = page
	}

	if pageSize, err := strconv.Atoi(c.Query("pageSize", "20")); err == nil && pageSize > 0 && pageSize <= 100 {
		query.PageSize = pageSize
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := h.profileService.SearchProfiles(ctx, query)
	if err != nil {
		log.Printf("Failed to search profiles: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search profiles",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profiles":    result.Profiles,
			"totalCount":  result.TotalCount,
			"pageCount":   result.PageCount,
			"currentPage": result.CurrentPage,
		},
	})
}

func (h *ProfileHandler) GetProfileCompleteness(c fiber.Ctx) error {
	profileID := c.Params("id")
	if profileID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Profile ID is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	completeness, err := h.profileService.GetProfileCompleteness(ctx, profileID)
	if err != nil {
		log.Printf("Failed to get profile completeness for %s: %v", profileID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to calculate profile completeness",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"completeness": completeness,
		},
	})
}

func (h *ProfileHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Profile Service is healthy")
}

// Helper functions
func (h *ProfileHandler) isValidEmail(email string) bool {
	if len(email) < 3 {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]
	if len(local) == 0 || len(domain) == 0 {
		return false
	}

	return strings.Contains(domain, ".")
}
