package service

import (
	"context"
	"fmt"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

// ConfigService handles global quiz configuration operations
type ConfigService struct {
	Repo *repository.ConfigRepository
}

// NewConfigService creates a new configuration service
func NewConfigService(repo *repository.ConfigRepository) *ConfigService {
	return &ConfigService{
		Repo: repo,
	}
}

// CreateConfig creates a new global configuration
func (s *ConfigService) CreateConfig(ctx context.Context, config *models.GlobalQuizConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return s.Repo.Create(ctx, config)
}

// GetConfig retrieves a configuration by ID
func (s *ConfigService) GetConfig(ctx context.Context, id string) (*models.GlobalQuizConfig, error) {
	return s.Repo.FindByID(ctx, id)
}

// GetDefaultConfig retrieves the default configuration
func (s *ConfigService) GetDefaultConfig(ctx context.Context) (*models.GlobalQuizConfig, error) {
	config, err := s.Repo.FindDefault(ctx)
	if err != nil {
		// If no default exists, ensure one is created
		if err := s.Repo.EnsureDefaultExists(ctx); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		// Try again
		config, err = s.Repo.FindDefault(ctx)
	}
	return config, err
}

// GetAllConfigs retrieves all active configurations
func (s *ConfigService) GetAllConfigs(ctx context.Context) ([]models.GlobalQuizConfig, error) {
	return s.Repo.FindAll(ctx)
}

// UpdateConfig updates a configuration
func (s *ConfigService) UpdateConfig(ctx context.Context, id string, update map[string]interface{}) error {
	// Validate the update contains valid fields
	if stageConfig, exists := update["stage_config"]; exists {
		// Create a temporary config to validate stage config
		tempConfig := &models.GlobalQuizConfig{
			Name:        "temp",
			StageConfig: stageConfig.(map[string]models.EnhancedStageConfig),
		}
		if err := tempConfig.Validate(); err != nil {
			return fmt.Errorf("invalid stage configuration: %w", err)
		}
	}

	return s.Repo.Update(ctx, id, update)
}

// DeleteConfig soft deletes a configuration
func (s *ConfigService) DeleteConfig(ctx context.Context, id string) error {
	// Check if this is the default config
	config, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if config.IsDefault {
		return fmt.Errorf("cannot delete default configuration")
	}

	return s.Repo.Delete(ctx, id)
}

// SetDefaultConfig sets a configuration as default
func (s *ConfigService) SetDefaultConfig(ctx context.Context, id string) error {
	return s.Repo.SetAsDefault(ctx, id)
}

// EnsureDefaultExists ensures a default configuration exists
func (s *ConfigService) EnsureDefaultExists(ctx context.Context) error {
	return s.Repo.EnsureDefaultExists(ctx)
}

// GetConfigForSession gets the appropriate configuration for a session
// If configID is empty, returns default configuration
func (s *ConfigService) GetConfigForSession(ctx context.Context, configID string) (*models.GlobalQuizConfig, error) {
	if configID == "" {
		return s.GetDefaultConfig(ctx)
	}
	return s.GetConfig(ctx, configID)
}
