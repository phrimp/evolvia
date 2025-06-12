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
				if perm == required_permission || strings.HasPrefix(perm, "admin") || strings.HasPrefix(perm, "manager") {
					hasPermission = true
					break
				}
			}

		}

		if !hasPermission {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}
		return c.Next()
	}
}

func OwnerPermissionRequired(userID string) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Println("Owner required function called from", c.IP(), "Calling", c.Method(), "Request", c.OriginalURL())

		if userID == "" {
			userID = c.Params("userId")
			if userID == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Unauthorized",
				})
			}
		}

		currentUserID := c.Get("X-User-ID")
		hasPermission := false
		if currentUserID != "" {
			if currentUserID == userID {
				hasPermission = true
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}
		return c.Next()
	}
}
