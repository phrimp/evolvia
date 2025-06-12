package utils

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func PermissionRequired(required_permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Println("Permission required function called from", c.IP(), "Calling", c.Method(), "Request", c.OriginalURL())
		userPermissions := c.Get("X-User-Permissions")
		hasPermission := false

		if userPermissions != "" {
			permissions := strings.Split(userPermissions, ",")

			for _, perm := range permissions {
				// Trim whitespace from permission
				perm = strings.TrimSpace(perm)

				// Check for exact match
				if perm == required_permission {
					hasPermission = true
					break
				}

				// Check for admin privileges (admin has all permissions)
				if strings.HasPrefix(perm, "admin") {
					hasPermission = true
					break
				}

				// Check for manager privileges for certain operations
				if strings.HasPrefix(perm, "manager") {
					// Managers can perform most operations except system-level ones
					if !strings.Contains(required_permission, "admin") &&
						!strings.Contains(required_permission, "process:billing:operations") {
						hasPermission = true
						break
					}
				}

				// Check for hierarchical permissions (e.g., read:plan:all includes read:plan)
				if strings.Contains(perm, ":all") && strings.Contains(required_permission, strings.Replace(perm, ":all", "", 1)) {
					hasPermission = true
					break
				}
			}
		}

		if !hasPermission {
			log.Printf("Access denied for user with permissions [%s] requiring [%s]", userPermissions, required_permission)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Insufficient permissions",
			})
		}

		return c.Next()
	}
}

// OwnerPermissionRequired checks if the user is the owner of the resource or has elevated permissions
func OwnerPermissionRequired(userID string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Println("Owner required function called from", c.IP(), "Calling", c.Method(), "Request", c.OriginalURL())

		// Get userID from parameter if not provided
		if userID == "" {
			userID = c.Params("userId")
			if userID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "User ID is required",
				})
			}
		}

		currentUserID := c.Get("X-User-ID")
		userPermissions := c.Get("X-User-Permissions")
		hasPermission := false

		if currentUserID != "" {
			// Check if user is the owner
			if currentUserID == userID {
				hasPermission = true
			} else {
				// Check if user has elevated permissions
				if userPermissions != "" {
					permissions := strings.Split(userPermissions, ",")
					for _, perm := range permissions {
						perm = strings.TrimSpace(perm)
						if strings.HasPrefix(perm, "admin") || strings.HasPrefix(perm, "manager") {
							hasPermission = true
							break
						}
					}
				}
			}
		}

		if !hasPermission {
			log.Printf("Access denied for user [%s] trying to access resource for user [%s] with permissions [%s]",
				currentUserID, userID, userPermissions)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied",
			})
		}

		return c.Next()
	}
}

// RequireAnyPermission checks if the user has any of the specified permissions
func RequireAnyPermission(required_permissions ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Printf("Any permission required from %v called from %s", required_permissions, c.IP())
		userPermissions := c.Get("X-User-Permissions")
		hasPermission := false

		if userPermissions != "" {
			permissions := strings.Split(userPermissions, ",")

			for _, userPerm := range permissions {
				userPerm = strings.TrimSpace(userPerm)

				// Check for admin privileges
				if strings.HasPrefix(userPerm, "admin") {
					hasPermission = true
					break
				}

				// Check against required permissions
				for _, reqPerm := range required_permissions {
					if userPerm == reqPerm {
						hasPermission = true
						break
					}
				}

				if hasPermission {
					break
				}
			}
		}

		if !hasPermission {
			log.Printf("Access denied for user with permissions [%s] requiring any of [%v]", userPermissions, required_permissions)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Insufficient permissions",
			})
		}

		return c.Next()
	}
}

// RequireAllPermissions checks if the user has all of the specified permissions
func RequireAllPermissions(required_permissions ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Printf("All permissions required %v called from %s", required_permissions, c.IP())
		userPermissions := c.Get("X-User-Permissions")

		if userPermissions != "" {
			permissions := strings.Split(userPermissions, ",")

			// Check for admin privileges first
			for _, userPerm := range permissions {
				userPerm = strings.TrimSpace(userPerm)
				if strings.HasPrefix(userPerm, "admin") {
					return c.Next()
				}
			}

			// Check if user has all required permissions
			for _, reqPerm := range required_permissions {
				hasThisPermission := false
				for _, userPerm := range permissions {
					userPerm = strings.TrimSpace(userPerm)
					if userPerm == reqPerm {
						hasThisPermission = true
						break
					}
				}
				if !hasThisPermission {
					log.Printf("Access denied for user with permissions [%s] missing required permission [%s]", userPermissions, reqPerm)
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Insufficient permissions",
					})
				}
			}

			return c.Next()
		}

		log.Printf("Access denied for user with no permissions requiring [%v]", required_permissions)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "No permissions found",
		})
	}
}

// AdminOnly is a convenience function for admin-only endpoints
func AdminOnly() fiber.Handler {
	return PermissionRequired("admin")
}

// ManagerOrAdmin allows both manager and admin access
func ManagerOrAdmin() fiber.Handler {
	return RequireAnyPermission("admin", "manager")
}
