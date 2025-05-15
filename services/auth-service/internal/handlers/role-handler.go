package handlers

import (
	"auth_service/internal/service"
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RoleHandler struct {
	roleService     *service.RoleService
	userRoleService *service.UserRoleService
}

func NewRoleHandler(roleService *service.RoleService, userRoleService *service.UserRoleService) *RoleHandler {
	return &RoleHandler{
		roleService:     roleService,
		userRoleService: userRoleService,
	}
}

func (h *RoleHandler) RegisterRoutes(app *fiber.App) {
	roleGroup := app.Group("/protected/auth/roles")

	roleGroup.Get("/", h.GetAllRoles)
	roleGroup.Get("/:id", h.GetRoleByID)
	roleGroup.Post("/", h.CreateRole)
	roleGroup.Put("/:id", h.UpdateRole)
	roleGroup.Delete("/:id", h.DeleteRole)

	roleGroup.Post("/:id/permissions", h.AddPermissionToRole)
	roleGroup.Delete("/:id/permissions/:permission", h.RemovePermissionFromRole)

	userRoleGroup := app.Group("/protected/auth/user-roles")
	userRoleGroup.Post("/", h.AssignRoleToUser)
	userRoleGroup.Delete("/:id", h.RemoveRoleFromUser)
	userRoleGroup.Get("/users/:userId", h.GetUserRoles)
	userRoleGroup.Get("/roles/:roleName/users", h.GetUsersWithRole)

	err := h.roleService.CreateDefaultRoles(context.Background())
	log.Printf("Error Loading Default Roles: %s", err)
}

func (h *RoleHandler) GetAllRoles(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")

	// Parse pagination parameters
	page := fiber.Query(c, "page", 1)
	limit := fiber.Query(c, "limit", 10)

	// Check if user has permission to view roles
	hasPermission := false
	if userPermissions != "" {
		permissions := strings.SplitSeq(userPermissions, ",")
		// Check for any permission that would allow viewing roles
		for perm := range permissions {
			if perm == "read" || strings.HasPrefix(perm, "role:") || perm == "admin" {
				hasPermission = true
				break
			}
		}
	}

	if !hasPermission {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to view roles",
		})
	}

	log.Printf("User %s requesting all roles (page: %d, limit: %d)", userID, page, limit)

	roles, err := h.roleService.GetAllRoles(c.Context(), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": roles,
	})
}

func (h *RoleHandler) GetRoleByID(c fiber.Ctx) error {
	id := c.Params("id")

	roleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role ID format",
		})
	}

	role, err := h.roleService.GetRoleByID(c.Context(), roleID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": role,
	})
}

func (h *RoleHandler) CreateRole(c fiber.Ctx) error {
	var request struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
		IsSystem    bool     `json:"isSystem"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if request.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Role name is required",
		})
	}

	role, err := h.roleService.CreateRole(c.Context(), request.Name, request.Description, request.Permissions, request.IsSystem)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Role created successfully",
		"data":    role,
	})
}

func (h *RoleHandler) UpdateRole(c fiber.Ctx) error {
	id := c.Params("id")

	roleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role ID format",
		})
	}

	var request struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
		IsSystem    bool     `json:"isSystem"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if request.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Role name is required",
		})
	}

	role, err := h.roleService.UpdateRole(c.Context(), roleID, request.Name, request.Description, request.Permissions, request.IsSystem)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Role updated successfully",
		"data":    role,
	})
}

func (h *RoleHandler) DeleteRole(c fiber.Ctx) error {
	id := c.Params("id")

	roleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role ID format",
		})
	}

	err = h.roleService.DeleteRole(c.Context(), roleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Role deleted successfully",
	})
}

func (h *RoleHandler) AddPermissionToRole(c fiber.Ctx) error {
	id := c.Params("id")

	roleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role ID format",
		})
	}

	var request struct {
		Permission string `json:"permission"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if request.Permission == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Permission name is required",
		})
	}

	err = h.roleService.AddPermissionToRole(c.Context(), roleID, request.Permission)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Permission added to role successfully",
	})
}

func (h *RoleHandler) RemovePermissionFromRole(c fiber.Ctx) error {
	id := c.Params("id")
	permission := c.Params("permission")

	roleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid role ID format",
		})
	}

	err = h.roleService.RemovePermissionFromRole(c.Context(), roleID, permission)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Permission removed from role successfully",
	})
}

func (h *RoleHandler) AssignRoleToUser(c fiber.Ctx) error {
	var request struct {
		UserID        string `json:"userId"`
		RoleName      string `json:"roleName"`
		ScopeType     string `json:"scopeType"`
		ScopeID       string `json:"scopeId"`
		ExpiresInDays int    `json:"expiresInDays"`
	}

	if err := c.Bind().Body(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if request.UserID == "" || request.RoleName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID and role name are required",
		})
	}

	userID, err := primitive.ObjectIDFromHex(request.UserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	role, err := h.roleService.GetRoleByName(c.Context(), request.RoleName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	adminID, _ := primitive.ObjectIDFromHex("000000000000000000000000")

	var scopeID primitive.ObjectID
	if request.ScopeID != "" {
		scopeID, err = primitive.ObjectIDFromHex(request.ScopeID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid scope ID format",
			})
		}
	}

	userRole, err := h.userRoleService.AssignRoleToUser(
		c.Context(),
		userID,
		role.ID,
		adminID,
		request.ScopeType,
		scopeID,
		request.ExpiresInDays,
	)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Role assigned to user successfully",
		"data":    userRole,
	})
}

func (h *RoleHandler) RemoveRoleFromUser(c fiber.Ctx) error {
	id := c.Params("id")

	userRoleID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user role ID format",
		})
	}

	err = h.userRoleService.RemoveRoleFromUser(c.Context(), userRoleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Role removed from user successfully",
	})
}

func (h *RoleHandler) GetUserRoles(c fiber.Ctx) error {
	userID := c.Params("userId")

	uid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID format",
		})
	}

	scopeType := c.Query("scopeType")
	scopeIDStr := c.Query("scopeID")

	var scopeID primitive.ObjectID
	if scopeIDStr != "" {
		scopeID, err = primitive.ObjectIDFromHex(scopeIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid scope ID format",
			})
		}
	}

	var userRoles []*fiber.Map

	roles, err := h.userRoleService.GetUserRolesWithScope(c.Context(), uid, scopeType, scopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	for _, userRole := range roles {
		role, err := h.roleService.GetRoleByID(c.Context(), userRole.RoleID)
		if err != nil {
			continue
		}

		userRoles = append(userRoles, &fiber.Map{
			"id":         userRole.ID,
			"roleId":     userRole.RoleID,
			"roleName":   role.Name,
			"scopeType":  userRole.ScopeType,
			"scopeId":    userRole.ScopeID,
			"assignedAt": userRole.AssignedAt,
			"expiresAt":  userRole.ExpiresAt,
			"isActive":   userRole.IsActive,
		})
	}

	return c.JSON(fiber.Map{
		"data": userRoles,
	})
}

func (h *RoleHandler) GetUsersWithRole(c fiber.Ctx) error {
	roleName := c.Params("roleName")
	page := fiber.Query(c, "page", 1)
	limit := fiber.Query(c, "limit", 10)

	userIDs, err := h.userRoleService.GetUsersWithRole(c.Context(), roleName, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"role":    roleName,
			"userIds": userIDs,
		},
	})
}
