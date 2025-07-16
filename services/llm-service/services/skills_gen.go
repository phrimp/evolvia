package services

import (
	// "encoding/json"
	"llm-service/models"
	"strings"
	"time"
)

// GenerateSkillsFromContent analyzes the given content and returns extracted skills in the required format.
func GenerateSkillsFromContent(content string) (*models.SkillExtractionResponse, error) {
	// This is a placeholder for actual LLM or pattern-based extraction logic.
	// For now, we mock a response for demonstration.
	// In production, you would call your LLM or use a skill extraction pipeline here.

	// Example: If content contains "JavaScript", return a JavaScript skill
	skills := []models.Skill{}
	if strings.Contains(strings.ToLower(content), "javascript") {
		skills = append(skills, models.Skill{
			Name:        "JavaScript",
			Description: "A high-level, dynamic programming language used for web development.",
			CommonNames: []string{"JavaScript", "JS"},
			Abbreviations: []string{"JS"},
			TechnicalTerms: []string{"ECMAScript"},
			Category: models.SkillCategory{
				Name:  "Programming Languages",
				Path:  "Technology/Programming/Languages",
				Level: 3,
			},
			Tags: []string{"frontend", "web", "dynamic"},
			Relations: []models.SkillRelation{},
			Metadata: models.SkillMetadata{
				Industry:     []string{"Technology", "Web Development"},
				JobRoles:     []string{"Frontend Developer", "Full Stack Developer"},
				Difficulty:   3,
				TimeToLearn:  100,
				Trending:     true,
				MarketDemand: 0.9,
			},
			IsActive: true,
			Version: 1,
		})
	}

	response := &models.SkillExtractionResponse{
		Skills: skills,
		ExtractionMetadata: models.ExtractionMetadata{
			SourceType:         "text",
			TotalSkillsFound:   len(skills),
			ConfidenceThreshold: 0.6,
			ExtractionDate:     time.Now().Format(time.RFC3339),
			ProcessingNotes:    "Mock extraction for demonstration.",
		},
	}
	return response, nil
}
