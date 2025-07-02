package handlers

import (
	"auth_service/internal/repository"
	"auth_service/internal/service"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type PermissionHanlder struct {
	roleService       *service.RoleService
	permissionService *service.PermissionService
	userRoleService   *service.UserRoleService
}

func NewPermissionHanlder(roleService *service.RoleService, userRoleService *service.UserRoleService, permissionService *service.PermissionService) *PermissionHanlder {
	return &PermissionHanlder{
		roleService:       roleService,
		permissionService: permissionService,
		userRoleService:   userRoleService,
	}
}

func (h *PermissionHanlder) RegisterRoutes(app *fiber.App) {
	permissionGroup := app.Group("/protected/auth/permission")

	permissionGroup.Get("/", h.GetAllPermission)
	permissionGroup.Post("/maintenance", h.Maintenance)
}

func (h *PermissionHanlder) Maintenance(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")

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
			"error": "You don't have permission to view permissions",
		})
	}

	if !hasPermission {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have enough permission",
		})
	}

	log.Printf("User %s request system maintenance", userID)

	var maintenance_status bool

	err := repository.Repositories_instance.RedisRepository.GetStructCached(c.Context(), "maintenance", "", &maintenance_status)
	if err != nil {
		repository.Repositories_instance.RedisRepository.SaveStructCached(c.Context(), "", "maintenance", true, 24000*time.Hour)
		maintenance_status = true
	} else {
		repository.Repositories_instance.RedisRepository.DeleteKey(c.Context(), "maintenance")
		maintenance_status = false
	}

	return c.JSON(fiber.Map{
		"System Maintenance Status": maintenance_status,
	})
}

func (h *PermissionHanlder) GetAllPermission(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	userPermissions := c.Get("X-User-Permissions")

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
			"error": "You don't have permission to view permissions",
		})
	}

	log.Printf("User %s requesting all permissions", userID)

	permissions, err := h.permissionService.GetAllPermission(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": permissions,
	})
}
