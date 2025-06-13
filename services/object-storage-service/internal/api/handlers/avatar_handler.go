package handlers

import (
	"log"
	"object-storage-service/internal/service"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type AvatarHandler struct {
	avatarService *service.AvatarService
}

func NewAvatarHandler(avatarService *service.AvatarService) *AvatarHandler {
	return &AvatarHandler{
		avatarService: avatarService,
	}
}

func (h *AvatarHandler) RegisterRoutes(app *fiber.App) {
	// Public routes for avatar access
	publicGroup := app.Group("/public/storage/avatars")
	publicGroup.Get("/:id", h.GetAvatar)
	publicGroup.Get("/:id/download", h.DownloadAvatar)
	publicGroup.Get("/:id/url", h.GetPresignedURL) // id is user id

	// Protected routes for avatar management
	avatarGroup := app.Group("/protected/storage/avatars")
	avatarGroup.Post("/", h.UploadAvatar)
	avatarGroup.Get("/", h.GetUserAvatars)
	avatarGroup.Get("/default", h.GetDefaultAvatar)
	avatarGroup.Delete("/:id", h.DeleteAvatar)
}

func (h *AvatarHandler) UploadAvatar(c fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get file from form
	file, err := c.FormFile("avatar")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No avatar file provided",
		})
	}

	// Check if this should be the default avatar
	isDefault := c.FormValue("isDefault", "false") == "true"

	// Upload avatar
	avatar, err := h.avatarService.UploadAvatar(c.Context(), file, userID, isDefault)
	if err != nil {
		log.Printf("Error uploading avatar: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Avatar uploaded successfully",
		"data":    avatar,
	})
}

func (h *AvatarHandler) GetAvatar(c fiber.Ctx) error {
	id := c.Params("id")

	avatar, err := h.avatarService.GetAvatar(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": avatar,
	})
}

func (h *AvatarHandler) DownloadAvatar(c fiber.Ctx) error {
	id := c.Params("id")

	avatar, err := h.avatarService.GetAvatar(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	avatarContent, contentType, size, err := h.avatarService.GetAvatarContent(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer avatarContent.Close()

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "inline; filename="+avatar.FileName)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	return c.SendStream(avatarContent)
}

func (h *AvatarHandler) GetUserAvatars(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	avatars, err := h.avatarService.GetUserAvatars(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": avatars,
	})
}

func (h *AvatarHandler) GetDefaultAvatar(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	avatar, err := h.avatarService.GetDefaultAvatar(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	url, err := h.avatarService.GetAvatarURLSystem(c.Context(), avatar, userID, 3600)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if avatar == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No default avatar found",
		})
	}
	result := map[string]any{
		"avatar": avatar,
		"url":    url,
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}

func (h *AvatarHandler) DeleteAvatar(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get avatar to check ownership
	avatar, err := h.avatarService.GetAvatar(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership
	if avatar.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You don't have permission to delete this avatar",
		})
	}

	// Delete avatar
	if err := h.avatarService.DeleteAvatar(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Avatar deleted successfully",
	})
}

//func (h *AvatarHandler) SetDefaultAvatar(c fiber.Ctx) error {
//	id := c.Params("id")
//	userID := c.Get("X-User-ID")
//
//	// Get avatar to check ownership
//	avatar, err := h.avatarService.GetAvatar(c.Context(), id)
//	if err != nil {
//		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
//			"error": err.Error(),
//		})
//	}
//
//	// Check ownership
//	if avatar.UserID != userID {
//		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
//			"error": "You don't have permission to modify this avatar",
//		})
//	}
//
//	// Set as default
//	if err := h.avatarService.SetDefaultAvatar(c.Context(), id); err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
//			"error": err.Error(),
//		})
//	}
//
//	return c.JSON(fiber.Map{
//		"message": "Avatar set as default successfully",
//	})
//}

func (h *AvatarHandler) GetPresignedURL(c fiber.Ctx) error {
	id := c.Params("id")
	expiry := fiber.Query(c, "expiry", 3600) // Default 1 hour

	// Get presigned URL
	url, err := h.avatarService.GetAvatarURL(c.Context(), id, expiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"url":    url,
			"expiry": expiry,
		},
	})
}
