package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type QuestionHandler struct {
	Service *service.QuestionService
}

func NewQuestionHandler(s *service.QuestionService) *QuestionHandler {
	return &QuestionHandler{Service: s}
}

func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	questions, err := h.Service.ListQuestions(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, questions)
}

func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	id := c.Param("id")

	// Validate ID format
	if strings.TrimSpace(id) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Question ID is required"})
		return
	}

	question, err := h.Service.GetQuestion(context.Background(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Question not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error", "details": err.Error()})
		}
		return
	}

	// Ensure Bloom scores are calculated
	question.EnsureBloomScores()

	response := gin.H{
		"question":  question,
		"type_info": question.GetQuestionTypeInfo(),
	}

	c.JSON(http.StatusOK, response)
}

func (h *QuestionHandler) CreateQuestion(c *gin.Context) {
	var question models.Question
	if err := c.ShouldBindJSON(&question); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate question type and structure
	if err := question.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Question validation failed",
			"details": err.Error(),
		})
		return
	}

	// Calculate Bloom scores
	question.EnsureBloomScores()

	if err := h.Service.CreateQuestion(context.Background(), &question); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Include question type info in response
	response := gin.H{
		"question":  question,
		"type_info": question.GetQuestionTypeInfo(),
		"message":   "Question created successfully",
	}

	c.JSON(http.StatusCreated, response)
}

func (h *QuestionHandler) UpdateQuestion(c *gin.Context) {
	id := c.Param("id")
	var update map[string]interface{}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If type is being changed, validate the new type
	if newType, exists := update["type"].(string); exists {
		if !models.IsValidQuestionType(newType) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Invalid question type",
				"supported_types": models.GetPrimaryQuestionTypes(),
			})
			return
		}
	}

	if err := h.Service.UpdateQuestion(context.Background(), id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Question updated successfully",
		"id":      id,
	})
}

func (h *QuestionHandler) DeleteQuestion(c *gin.Context) {
	id := c.Param("id")
	if err := h.Service.DeleteQuestion(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *QuestionHandler) BulkQuestionOps(c *gin.Context) {
	var request struct {
		Operation string            `json:"operation" binding:"required"`
		Questions []models.Question `json:"questions"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch request.Operation {
	case "create":
		h.bulkCreateQuestions(c, request.Questions)
	case "validate":
		h.bulkValidateQuestions(c, request.Questions)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Unsupported operation",
			"supported": []string{"create", "validate"},
		})
	}
}

func (h *QuestionHandler) bulkCreateQuestions(c *gin.Context, questions []models.Question) {
	var results []gin.H
	var errors []gin.H

	for i, question := range questions {
		// Validate question
		if err := question.Validate(); err != nil {
			errors = append(errors, gin.H{
				"index": i,
				"id":    question.ID,
				"error": err.Error(),
			})
			continue
		}

		// Calculate Bloom scores
		question.EnsureBloomScores()

		// Create question
		if err := h.Service.CreateQuestion(context.Background(), &question); err != nil {
			errors = append(errors, gin.H{
				"index": i,
				"id":    question.ID,
				"error": err.Error(),
			})
		} else {
			results = append(results, gin.H{
				"index":  i,
				"id":     question.ID,
				"type":   question.Type,
				"status": "created",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"operation":       "bulk_create",
		"total_processed": len(questions),
		"successful":      len(results),
		"failed":          len(errors),
		"results":         results,
		"errors":          errors,
	})
}

func (h *QuestionHandler) bulkValidateQuestions(c *gin.Context, questions []models.Question) {
	var results []gin.H

	for i, question := range questions {
		validationResult := gin.H{
			"index": i,
			"id":    question.ID,
			"type":  question.Type,
		}

		if err := question.Validate(); err != nil {
			validationResult["valid"] = false
			validationResult["error"] = err.Error()
		} else {
			validationResult["valid"] = true
			validationResult["type_info"] = question.GetQuestionTypeInfo()
		}

		results = append(results, validationResult)
	}

	c.JSON(http.StatusOK, gin.H{
		"operation":       "bulk_validate",
		"total_questions": len(questions),
		"results":         results,
	})
}

// CreateTrueFalseQuestion creates a true/false question using the factory method
func (h *QuestionHandler) CreateTrueFalseQuestion(c *gin.Context) {
	var request struct {
		Content         string   `json:"content" binding:"required"`
		SkillID         string   `json:"skill_id" binding:"required"`
		BloomLevel      string   `json:"bloom_level" binding:"required"`
		CorrectAnswer   bool     `json:"correct_answer" binding:"required"`
		Explanation     string   `json:"explanation"`
		Points          int      `json:"points"`
		EstimatedTime   int      `json:"estimated_time_seconds"`
		DifficultyLevel string   `json:"difficulty_level"`
		TopicTags       []string `json:"topic_tags"`
		QuestionPoolID  string   `json:"question_pool_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create true/false question using factory method
	question := models.NewTrueFalseQuestion(
		request.Content,
		request.SkillID,
		request.BloomLevel,
		request.CorrectAnswer,
	)

	// Set additional fields
	question.Explanation = request.Explanation
	question.Points = request.Points
	question.EstimatedTimeSeconds = request.EstimatedTime
	question.DifficultyLevel = request.DifficultyLevel
	question.TopicTags = request.TopicTags
	question.QuestionPoolID = request.QuestionPoolID

	// Calculate Bloom scores
	question.EnsureBloomScores()

	if err := h.Service.CreateQuestion(context.Background(), question); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"question":  question,
		"type_info": question.GetQuestionTypeInfo(),
		"message":   "True/False question created successfully",
	})
}

// CreateSingleChoiceQuestion creates a single-choice question using the factory method
func (h *QuestionHandler) CreateSingleChoiceQuestion(c *gin.Context) {
	var request struct {
		Content         string          `json:"content" binding:"required"`
		SkillID         string          `json:"skill_id" binding:"required"`
		BloomLevel      string          `json:"bloom_level" binding:"required"`
		Options         []models.Option `json:"options" binding:"required"`
		CorrectAnswerID string          `json:"correct_answer_id" binding:"required"`
		Explanation     string          `json:"explanation"`
		Points          int             `json:"points"`
		EstimatedTime   int             `json:"estimated_time_seconds"`
		DifficultyLevel string          `json:"difficulty_level"`
		TopicTags       []string        `json:"topic_tags"`
		QuestionPoolID  string          `json:"question_pool_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create single-choice question using factory method
	question := models.NewSingleChoiceQuestion(
		request.Content,
		request.SkillID,
		request.BloomLevel,
		request.Options,
		request.CorrectAnswerID,
	)

	// Set additional fields
	question.Explanation = request.Explanation
	question.Points = request.Points
	question.EstimatedTimeSeconds = request.EstimatedTime
	question.DifficultyLevel = request.DifficultyLevel
	question.TopicTags = request.TopicTags
	question.QuestionPoolID = request.QuestionPoolID

	// Calculate Bloom scores
	question.EnsureBloomScores()

	if err := h.Service.CreateQuestion(context.Background(), question); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"question":  question,
		"type_info": question.GetQuestionTypeInfo(),
		"message":   "Single-choice question created successfully",
	})
}

// CreateMultipleChoiceQuestion creates a multiple-choice question using the factory method
func (h *QuestionHandler) CreateMultipleChoiceQuestion(c *gin.Context) {
	var request struct {
		Content         string          `json:"content" binding:"required"`
		SkillID         string          `json:"skill_id" binding:"required"`
		BloomLevel      string          `json:"bloom_level" binding:"required"`
		Options         []models.Option `json:"options" binding:"required"`
		CorrectAnswerID string          `json:"correct_answer_id" binding:"required"`
		Explanation     string          `json:"explanation"`
		Points          int             `json:"points"`
		EstimatedTime   int             `json:"estimated_time_seconds"`
		DifficultyLevel string          `json:"difficulty_level"`
		TopicTags       []string        `json:"topic_tags"`
		QuestionPoolID  string          `json:"question_pool_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create multiple-choice question using factory method
	question := models.NewMultipleChoiceQuestion(
		request.Content,
		request.SkillID,
		request.BloomLevel,
		request.Options,
		request.CorrectAnswerID,
	)

	// Set additional fields
	question.Explanation = request.Explanation
	question.Points = request.Points
	question.EstimatedTimeSeconds = request.EstimatedTime
	question.DifficultyLevel = request.DifficultyLevel
	question.TopicTags = request.TopicTags
	question.QuestionPoolID = request.QuestionPoolID

	// Calculate Bloom scores
	question.EnsureBloomScores()

	if err := h.Service.CreateQuestion(context.Background(), question); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"question":  question,
		"type_info": question.GetQuestionTypeInfo(),
		"message":   "Multiple-choice question created successfully",
	})
}

// GetSupportedQuestionTypes returns information about supported question types
func (h *QuestionHandler) GetSupportedQuestionTypes(c *gin.Context) {
	types := models.GetPrimaryQuestionTypes()

	typeInfo := make(map[string]interface{})
	for _, qType := range types {
		switch qType {
		case models.QuestionTypeMultipleChoice:
			typeInfo[string(qType)] = gin.H{
				"description":  "Multiple choice questions with multiple options",
				"requirements": []string{"content", "skill_id", "bloom_level", "options (2+)", "correct_answer_id"},
				"features":     []string{"Multiple options", "Single correct answer", "Flexible option count"},
			}
		case models.QuestionTypeTrueFalse:
			typeInfo[string(qType)] = gin.H{
				"description":  "True/False questions with boolean answers",
				"requirements": []string{"content", "skill_id", "bloom_level", "correct_answer (boolean)"},
				"features":     []string{"Boolean answer", "Automatic true/false options", "Support for both string and boolean input"},
			}
		case models.QuestionTypeSingleChoice:
			typeInfo[string(qType)] = gin.H{
				"description":  "Single choice questions (like multiple choice but only one correct option)",
				"requirements": []string{"content", "skill_id", "bloom_level", "options (2+)", "correct_answer_id"},
				"features":     []string{"Multiple options", "Exactly one correct answer", "Enhanced validation"},
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"supported_types":  types,
		"type_information": typeInfo,
		"primary_formats": []string{
			string(models.QuestionTypeMultipleChoice),
			string(models.QuestionTypeTrueFalse),
			string(models.QuestionTypeSingleChoice),
		},
	})
}
