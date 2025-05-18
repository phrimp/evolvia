package handlers

import (
	"fmt"
	"log"
	"object-storage-service/internal/models"
	"object-storage-service/internal/service"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type FileHandler struct {
	fileService *service.FileService
}

func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

func (h *FileHandler) RegisterRoutes(app *fiber.App) {
	// Public routes for file access
	publicGroup := app.Group("/public/storage/files")
	publicGroup.Get("/:id", h.GetPublicFile)
	publicGroup.Get("/:id/download", h.DownloadPublicFile)

	// Protected routes for file management
	fileGroup := app.Group("/protected/storage/files")
	fileGroup.Post("/", h.UploadFile)
	fileGroup.Get("/", h.ListFiles)
	fileGroup.Get("/:id", h.GetFile)
	fileGroup.Put("/:id", h.UpdateFile)
	fileGroup.Delete("/:id", h.DeleteFile)
	fileGroup.Get("/:id/download", h.DownloadFile)
	fileGroup.Post("/:id/version", h.AddFileVersion)
	fileGroup.Get("/:id/versions", h.GetFileVersions)
	fileGroup.Put("/:id/permissions", h.UpdateFilePermissions)
	fileGroup.Get("/:id/url", h.GetPresignedURL)
}

func (h *FileHandler) UploadFile(c fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID := c.Get("X-User-ID")
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file provided",
		})
	}

	// Get other form values
	description := c.FormValue("description", "")
	folderPath := c.FormValue("folderPath", "")
	isPublic := c.FormValue("isPublic", "false") == "true"

	// Parse tags
	tags := []string{}
	if tagsStr := c.FormValue("tags", ""); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	// Parse metadata
	metadata := make(map[string]string)
	metadataFields := c.GetReqHeaders()
	for key, value := range metadataFields {
		if strings.HasPrefix(key, "X-Metadata-") {
			fmt.Println("DEBUG:\n", value)
			metadataKey := strings.TrimPrefix(key, "X-Metadata-")
			metadata[metadataKey] = value[0]
		}
	}

	// Upload file
	uploadedFile, err := h.fileService.UploadFile(c.Context(), file, userID, description, folderPath, isPublic, tags, metadata)
	if err != nil {
		log.Printf("Error uploading file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "File uploaded successfully",
		"data":    uploadedFile,
	})
}

func (h *FileHandler) GetFile(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or permissions
	if !file.IsPublic && file.OwnerID != userID {
		// Check if user has permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelRead ||
				perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to access this file",
			})
		}
	}

	return c.JSON(fiber.Map{
		"data": file,
	})
}

func (h *FileHandler) GetPublicFile(c fiber.Ctx) error {
	id := c.Params("id")

	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check if file is public
	if !file.IsPublic {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "This file is not public",
		})
	}

	return c.JSON(fiber.Map{
		"data": file,
	})
}

func (h *FileHandler) DownloadFile(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or permissions
	if !file.IsPublic && file.OwnerID != userID {
		// Check if user has permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelRead ||
				perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to access this file",
			})
		}
	}

	fileContent, contentType, size, err := h.fileService.GetFileContent(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer fileContent.Close()

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename="+file.Name)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	return c.SendStream(fileContent)
}

func (h *FileHandler) DownloadPublicFile(c fiber.Ctx) error {
	id := c.Params("id")

	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check if file is public
	if !file.IsPublic {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "This file is not public",
		})
	}

	fileContent, contentType, size, err := h.fileService.GetFileContent(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer fileContent.Close()

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename="+file.Name)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	return c.SendStream(fileContent)
}

func (h *FileHandler) UpdateFile(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or write permissions
	if file.OwnerID != userID {
		// Check if user has write permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to update this file",
			})
		}
	}

	// Get update data
	var updateRequest models.FileUpdateRequest
	if err := c.Bind().Body(&updateRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update file
	updatedFile, err := h.fileService.UpdateFile(c.Context(), id, &updateRequest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "File updated successfully",
		"data":    updatedFile,
	})
}

func (h *FileHandler) DeleteFile(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or admin permissions
	if file.OwnerID != userID {
		// Check if user has admin permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && perm.AccessLevel == models.AccessLevelAdmin {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to delete this file",
			})
		}
	}

	// Delete file
	if err := h.fileService.DeleteFile(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "File deleted successfully",
	})
}

func (h *FileHandler) ListFiles(c fiber.Ctx) error {
	userID := c.Get("X-User-ID")
	folderPath := c.Query("folderPath", "")
	page := fiber.Query(c, "page", 1)
	pageSize := fiber.Query(c, "pageSize", 10)

	files, count, err := h.fileService.ListFiles(c.Context(), userID, folderPath, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"files":      files,
			"totalCount": count,
			"page":       page,
			"pageSize":   pageSize,
		},
	})
}

func (h *FileHandler) AddFileVersion(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or write permissions
	if file.OwnerID != userID {
		// Check if user has write permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to add versions to this file",
			})
		}
	}

	// Get file from form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file provided",
		})
	}

	// Add new version
	newVersion, err := h.fileService.NewVersion(c.Context(), id, fileHeader, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "New version uploaded successfully",
		"data":    newVersion,
	})
}

func (h *FileHandler) GetFileVersions(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or read permissions
	if !file.IsPublic && file.OwnerID != userID {
		// Check if user has read permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelRead ||
				perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to view versions of this file",
			})
		}
	}

	// Get versions
	versions, err := h.fileService.GetVersions(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": versions,
	})
}

func (h *FileHandler) UpdateFilePermissions(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Only the owner can update permissions
	if file.OwnerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only the owner can update permissions",
		})
	}

	// Get permissions from request
	var permissions []models.Permission
	if err := c.Bind().Body(&permissions); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update permissions
	if err := h.fileService.UpdatePermissions(c.Context(), id, permissions, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Permissions updated successfully",
	})
}

func (h *FileHandler) GetPresignedURL(c fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Get("X-User-ID")
	expiry := fiber.Query(c, "expiry", 3600) // Default 1 hour

	// Get current file to check ownership
	file, err := h.fileService.GetFile(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check ownership or read permissions
	if !file.IsPublic && file.OwnerID != userID {
		// Check if user has read permission
		hasPermission := false
		for _, perm := range file.Permissions {
			if perm.EntityID == userID && (perm.AccessLevel == models.AccessLevelRead ||
				perm.AccessLevel == models.AccessLevelWrite ||
				perm.AccessLevel == models.AccessLevelAdmin) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to access this file",
			})
		}
	}

	// Get presigned URL
	url, err := h.fileService.GetFileURL(c.Context(), id, expiry)
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
