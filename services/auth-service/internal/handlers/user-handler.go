package handlers

import (
	"auth_service/internal/models"
	"auth_service/internal/service"
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserHandler permission constants
const (
	ReadUserPermission     = "read:user"
	ReadAllUsersPermission = "read:user:all"
	UpdateUserPermission   = "update:user"
	DeleteUserPermission   = "delete:user"
	ManageUserPermission   = "manage:user"
)

type UserHandler struct {
	userService     *service.UserService
	userRoleService *service.UserRoleService
}

func NewUserHandler(userService *service.UserService, userRoleService *service.UserRoleService) *UserHandler {
	return &UserHandler{
		userService:     userService,
		userRoleService: userRoleService,
	}
}

func (h *UserHandler) RegisterRoutes(app *fiber.App) {
	userGroup := app.Group("/protected/auth/users")

	// Admin endpoints for user management
	userGroup.Get("/", h.ListAllUsers)                 // List all users
	userGroup.Get("/:id", h.GetUserByID)               // Get user by ID
	userGroup.Put("/:id", h.UpdateUser)                // Update user
	userGroup.Delete("/:id", h.DeleteUser)             // Delete/deactivate user
	userGroup.Put("/:id/activate", h.ActivateUser)     // Activate user
	userGroup.Put("/:id/deactivate", h.DeactivateUser) // Deactivate user
	userGroup.Get("/search", h.SearchUsers)            // Search users
}

// ListAllUsers returns all users with pagination
func (h *UserHandler) ListAllUsers(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, ReadAllUsersPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to list all users",
		})
	}

	// Parse pagination parameters
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

	// DEBUG: Add these lines temporarily
	h.userService.UserRepo.DebugFieldNames(ctx)

	// Test with the specific user ID from your response
	userObjectID, _ := bson.ObjectIDFromHex("6891d3c11e8582266745d2b6")
	h.userService.UserRepo.DebugUserRoles(ctx, userObjectID)
	h.userService.UserRepo.DebugRoles(ctx)
	h.userService.UserRepo.DebugAggregationSteps(ctx, userObjectID)

	users, err := h.userService.ListAllUsers(ctx, page, limit)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve users",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"users": users,
			"pagination": fiber.Map{
				"page":  page,
				"limit": limit,
				"count": len(users),
			},
		},
	})
}

// GetUserByID returns a specific user by ID
func (h *UserHandler) GetUserByID(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, ReadUserPermission) && !h.hasPermission(c, ReadAllUsersPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to view users",
		})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := h.userService.GetUserByID(ctx, objectID)
	if err != nil {
		log.Printf("Failed to get user %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Remove sensitive information from response
	userResponse := h.sanitizeUserResponse(user)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"user": userResponse,
		},
	})
}

// UpdateUser updates user information
func (h *UserHandler) UpdateUser(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, UpdateUserPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to update users",
		})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	var updateRequest struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		IsActive        *bool  `json:"isActive"`
		IsEmailVerified *bool  `json:"isEmailVerified"`
	}

	if err := c.Bind().Body(&updateRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updatedUser, err := h.userService.UpdateUserByAdmin(ctx, objectID, &updateRequest)
	if err != nil {
		log.Printf("Failed to update user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user",
		})
	}

	userResponse := h.sanitizeUserResponse(updatedUser)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User updated successfully",
		"data": fiber.Map{
			"user": userResponse,
		},
	})
}

// DeleteUser deletes or deactivates a user
func (h *UserHandler) DeleteUser(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, DeleteUserPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to delete users",
		})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// For now, we'll deactivate instead of hard delete for safety
	err = h.userService.DeactivateUser(ctx, objectID)
	if err != nil {
		log.Printf("Failed to deactivate user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to deactivate user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User deactivated successfully",
	})
}

// ActivateUser activates a user account
func (h *UserHandler) ActivateUser(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, ManageUserPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to manage users",
		})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.userService.ActivateUser(ctx, objectID)
	if err != nil {
		log.Printf("Failed to activate user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to activate user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User activated successfully",
	})
}

// DeactivateUser deactivates a user account
func (h *UserHandler) DeactivateUser(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, ManageUserPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to manage users",
		})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = h.userService.DeactivateUser(ctx, objectID)
	if err != nil {
		log.Printf("Failed to deactivate user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to deactivate user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User deactivated successfully",
	})
}

// SearchUsers searches for users based on query parameters
func (h *UserHandler) SearchUsers(c fiber.Ctx) error {
	// Check permissions
	if !h.hasPermission(c, ReadAllUsersPermission) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to search users",
		})
	}

	// Parse search parameters
	username := c.Query("username")
	email := c.Query("email")
	isActive := c.Query("isActive")
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

	users, err := h.userService.SearchUsers(ctx, username, email, isActive, page, limit)
	if err != nil {
		log.Printf("Failed to search users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search users",
		})
	}

	// Sanitize user responses
	sanitizedUsers := make([]fiber.Map, len(users))
	for i, user := range users {
		sanitizedUsers[i] = h.sanitizeUserResponse(user)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"users": sanitizedUsers,
			"pagination": fiber.Map{
				"page":  page,
				"limit": limit,
				"count": len(users),
			},
			"query": fiber.Map{
				"username": username,
				"email":    email,
				"isActive": isActive,
			},
		},
	})
}

// Helper methods

// hasPermission checks if user has the required permission
func (h *UserHandler) hasPermission(c fiber.Ctx, requiredPermission string) bool {
	userPermissions := c.Get("X-User-Permissions")
	if userPermissions == "" {
		return false
	}

	permissions := strings.Split(userPermissions, ",")
	for _, perm := range permissions {
		perm = strings.TrimSpace(perm)
		if perm == requiredPermission || perm == "admin" || perm == "read:admin" || perm == "update:admin" || perm == "delete:admin" {
			return true
		}
	}

	return false
}

// sanitizeUserResponse removes sensitive information from user response
func (h *UserHandler) sanitizeUserResponse(user interface{}) fiber.Map {
	if userAuth, ok := user.(*models.UserAuth); ok {
		return fiber.Map{
			"id":                  userAuth.ID,
			"username":            userAuth.Username,
			"email":               userAuth.Email,
			"isActive":            userAuth.IsActive,
			"isEmailVerified":     userAuth.IsEmailVerified,
			"createdAt":           userAuth.CreatedAt,
			"updatedAt":           userAuth.UpdatedAt,
			"failedLoginAttempts": userAuth.FailedLoginAttempts,
			"lastLoginAttempt":    userAuth.LastLoginAttempt,
			"lastLoginAt":         userAuth.LastLoginAt,
			"basicProfile":        userAuth.BasicProfile,
			// Exclude: passwordHash, sensitive fields
		}
	}

	// Fallback for unknown types
	return fiber.Map{
		"error": "Invalid user data type",
	}
}
