package handlers

import (
	"context"
	"log"
	"profile-service/internal/middleware"
	"profile-service/internal/models"
	"profile-service/internal/service"
	"proto-gen/utils"
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
	// Health check - always public
	app.Get("/health", h.HealthCheck)

	// PUBLIC ROUTES - No authentication required
	// These are useful for public profile viewing, networking, research discovery
	publicGroup := app.Group("/public/profiles")
	publicGroup.Get("/search", h.PublicSearchProfiles)     // Public profile discovery
	publicGroup.Get("/user/:userId", h.GetPublicProfile)   // Public profile view by user ID
	publicGroup.Get("/:id/public", h.GetPublicProfileByID) // Public profile view by profile ID

	// PROTECTED ROUTES - Authentication required
	protectedGroup := app.Group("/protected/profiles")

	// Self-service endpoints - users can manage their own profiles
	protectedGroup.Get("/me", h.GetMe)                                                                     // Get own profile
	protectedGroup.Put("/me", h.UpdateMe, utils.PermissionRequired(middleware.UpdateProfilePermission))    // Update own profile
	protectedGroup.Delete("/me", h.DeleteMe, utils.PermissionRequired(middleware.DeleteProfilePermission)) // Delete own profile
	protectedGroup.Get("/me/completeness", h.GetMyProfileCompleteness)                                     // Get own profile completeness

	// Owner-specific access (users can access their own profiles or admins can access any)
	protectedGroup.Get("/user/:userId", h.GetProfileByUserID, utils.OwnerPermissionRequired(""))
	protectedGroup.Put("/user/:userId", h.UpdateProfileByUserID, utils.OwnerPermissionRequired(""), utils.PermissionRequired(middleware.UpdateProfilePermission))

	// Profile management by ID - requires permission checking and ownership validation
	protectedGroup.Get("/:id", h.GetProfile, utils.PermissionRequired(middleware.ReadProfilePermission))
	protectedGroup.Put("/:id", h.UpdateProfile, utils.PermissionRequired(middleware.UpdateProfilePermission))
	protectedGroup.Delete("/:id", h.DeleteProfile, utils.PermissionRequired(middleware.DeleteProfilePermission))
	protectedGroup.Get("/:id/completeness", h.GetProfileCompleteness, utils.PermissionRequired(middleware.ReadProfilePermission))

	// Admin-only operations
	protectedGroup.Get("/", h.ListProfiles, utils.PermissionRequired(middleware.ReadAllProfilePermission))
	protectedGroup.Get("/search", h.SearchProfiles)
	protectedGroup.Get("/analytics", h.GetProfileAnalytics)
}

// PUBLIC ENDPOINTS - No authentication required

func (h *ProfileHandler) PublicSearchProfiles(c fiber.Ctx) error {
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

	if pageSize, err := strconv.Atoi(c.Query("pageSize", "20")); err == nil && pageSize > 0 && pageSize <= 50 {
		query.PageSize = pageSize
	}

	// Limit page size for public access
	if query.PageSize > 50 {
		query.PageSize = 50
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := h.profileService.SearchProfiles(ctx, query)
	if err != nil {
		log.Printf("Failed to search public profiles: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search profiles",
		})
	}

	// Filter profiles for public consumption
	publicProfiles := h.filterProfilesForPublic(result.Profiles)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profiles":    publicProfiles,
			"totalCount":  result.TotalCount,
			"pageCount":   result.PageCount,
			"currentPage": result.CurrentPage,
		},
	})
}

func (h *ProfileHandler) GetPublicProfile(c fiber.Ctx) error {
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
		log.Printf("Failed to get public profile for user %s: %v", userID, err)

		if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve profile",
		})
	}

	// Check if profile is public (assuming you have a privacy setting)
	// For now, we'll filter sensitive information
	publicProfile := h.filterProfileForPublic(profile)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profile": publicProfile,
		},
	})
}

func (h *ProfileHandler) GetPublicProfileByID(c fiber.Ctx) error {
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
		log.Printf("Failed to get public profile %s: %v", profileID, err)

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

	// Filter for public consumption
	publicProfile := h.filterProfileForPublic(profile)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"profile": publicProfile,
		},
	})
}

// PROTECTED ENDPOINTS - Require authentication

func (h *ProfileHandler) GetMe(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User authentication required",
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

func (h *ProfileHandler) UpdateMe(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User authentication required",
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

	// First get the profile to find the profile ID
	existingProfile, err := h.profileService.GetProfileByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found for this user",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find profile",
		})
	}

	profile, err := h.profileService.UpdateProfile(ctx, existingProfile.ID.Hex(), req)
	if err != nil {
		log.Printf("Failed to update profile for user %s: %v", userID, err)
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

func (h *ProfileHandler) DeleteMe(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User authentication required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First get the profile to find the profile ID
	existingProfile, err := h.profileService.GetProfileByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found for this user",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find profile",
		})
	}

	err = h.profileService.DeleteProfile(ctx, existingProfile.ID.Hex())
	if err != nil {
		log.Printf("Failed to delete profile for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete profile",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Profile deleted successfully",
	})
}

func (h *ProfileHandler) GetMyProfileCompleteness(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User authentication required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First get the profile to find the profile ID
	existingProfile, err := h.profileService.GetProfileByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found for this user",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find profile",
		})
	}

	completeness, err := h.profileService.GetProfileCompleteness(ctx, existingProfile.ID.Hex())
	if err != nil {
		log.Printf("Failed to get profile completeness for user %s: %v", userID, err)
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

	// Check if user can access this profile (owner or elevated permissions)
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") ||
		strings.Contains(userPermissions, "manager") ||
		strings.Contains(userPermissions, middleware.ReadAllProfilePermission)

	if !hasElevatedPermissions && profile.UserID != currentUserID {
		// Return filtered profile for non-owners
		publicProfile := h.filterProfileForPublic(profile)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": fiber.Map{
				"profile": publicProfile,
			},
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

func (h *ProfileHandler) UpdateProfileByUserID(c fiber.Ctx) error {
	userID := c.Params("userId")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
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

	// First get the profile to find the profile ID
	existingProfile, err := h.profileService.GetProfileByUserID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found for this user",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find profile",
		})
	}

	profile, err := h.profileService.UpdateProfile(ctx, existingProfile.ID.Hex(), req)
	if err != nil {
		log.Printf("Failed to update profile for user %s: %v", userID, err)
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

	// Check ownership for non-admin users
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") || strings.Contains(userPermissions, "manager")

	if !hasElevatedPermissions {
		// Get profile to check ownership
		existingProfile, err := h.profileService.GetProfile(ctx, profileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Profile not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify profile ownership",
			})
		}

		if existingProfile.UserID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

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

	// Check ownership for non-admin users
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") || strings.Contains(userPermissions, "manager")

	if !hasElevatedPermissions {
		// Get profile to check ownership
		existingProfile, err := h.profileService.GetProfile(ctx, profileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Profile not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify profile ownership",
			})
		}

		if existingProfile.UserID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

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

	// Check ownership for non-admin users
	currentUserID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")
	hasElevatedPermissions := strings.Contains(userPermissions, "admin") ||
		strings.Contains(userPermissions, "manager") ||
		strings.Contains(userPermissions, middleware.ReadAllProfilePermission)

	if !hasElevatedPermissions {
		// Get profile to check ownership
		existingProfile, err := h.profileService.GetProfile(ctx, profileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Profile not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify profile ownership",
			})
		}

		if existingProfile.UserID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}
	}

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

func (h *ProfileHandler) GetProfileAnalytics(c fiber.Ctx) error {
	// This would be implemented based on your analytics needs
	// For example: profile completion rates, most common fields, geographic distribution, etc.
	analytics := fiber.Map{
		"message": "Profile analytics endpoint - implement based on your needs",
		"suggestions": []string{
			"Profile completion rates",
			"Most common institutions",
			"Geographic distribution",
			"Field distribution",
			"Profile creation trends",
		},
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"analytics": analytics,
		},
	})
}

func (h *ProfileHandler) HealthCheck(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("Profile Service is healthy")
}

// HELPER FUNCTIONS

// filterProfileForPublic removes sensitive information from a profile for public access
func (h *ProfileHandler) filterProfileForPublic(profile *models.Profile) interface{} {
	// Return only public fields, hiding sensitive personal data
	return fiber.Map{
		"id":          profile.ID,
		"name":        profile.PersonalInfo.DisplayName,
		"bio":         profile.PersonalInfo.Biography,
		"publicEmail": profile.ContactInfo.Email,              // Only if they have a public email field
		"socialLinks": profile.ContactInfo.SocialMediaHandles, // Only public social links
		// Hide: personal email, phone, address, private fields, internal metadata
	}
}

// filterProfilesForPublic filters an array of profiles for public consumption
func (h *ProfileHandler) filterProfilesForPublic(profiles []*models.Profile) []interface{} {
	var publicProfiles []interface{}
	for _, profile := range profiles {
		// Only include profiles that are marked as public (if you have this field)
		// For now, we'll include all but filter the sensitive data
		publicProfiles = append(publicProfiles, h.filterProfileForPublic(profile))
	}
	return publicProfiles
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
