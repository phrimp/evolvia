package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/models"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	Service *service.ConfigService
}

func NewConfigHandler(service *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		Service: service,
	}
}

// CreateConfig creates a new global configuration
func (h *ConfigHandler) CreateConfig(c *gin.Context) {
	var config models.GlobalQuizConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if err := h.Service.CreateConfig(context.Background(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Configuration created successfully",
		"config":  config,
	})
}

// GetConfig retrieves a configuration by ID
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration ID is required",
		})
		return
	}

	config, err := h.Service.GetConfig(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Configuration not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetDefaultConfig retrieves the default configuration
func (h *ConfigHandler) GetDefaultConfig(c *gin.Context) {
	config, err := h.Service.GetDefaultConfig(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get default configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetAllConfigs retrieves all configurations
func (h *ConfigHandler) GetAllConfigs(c *gin.Context) {
	configs, err := h.Service.GetAllConfigs(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get configurations",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
		"count":   len(configs),
	})
}

// UpdateConfig updates a configuration
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration ID is required",
		})
		return
	}

	var update map[string]interface{}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if err := h.Service.UpdateConfig(context.Background(), id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
	})
}

// DeleteConfig deletes a configuration
func (h *ConfigHandler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration ID is required",
		})
		return
	}

	if err := h.Service.DeleteConfig(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration deleted successfully",
	})
}

// SetDefaultConfig sets a configuration as default
func (h *ConfigHandler) SetDefaultConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Configuration ID is required",
		})
		return
	}

	if err := h.Service.SetDefaultConfig(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to set default configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Default configuration updated successfully",
	})
}
