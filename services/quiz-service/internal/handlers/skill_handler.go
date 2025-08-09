package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

type SkillHandler struct {
	Service *service.SkillService
}

func NewSkillHandler(service *service.SkillService) *SkillHandler {
	return &SkillHandler{
		Service: service,
	}
}

// GetAllSkills returns all active skills
func (h *SkillHandler) GetAllSkills(c *gin.Context) {
	skills, err := h.Service.GetAllActiveSkills(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills})
}

// GetSkillByID returns a specific skill
func (h *SkillHandler) GetSkillByID(c *gin.Context) {
	id := c.Param("id")
	skill, err := h.Service.GetSkillByID(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}
	c.JSON(http.StatusOK, skill)
}

// GetSkillsByCategory returns skills by category
func (h *SkillHandler) GetSkillsByCategory(c *gin.Context) {
	categoryID := c.Param("categoryId")
	skills, err := h.Service.GetSkillsByCategory(context.Background(), categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": skills})
}
