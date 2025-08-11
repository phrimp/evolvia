package repository

import (
	"context"
	"quiz-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ConfigRepository struct {
	Col *mongo.Collection
}

func NewConfigRepository(db *mongo.Database) *ConfigRepository {
	return &ConfigRepository{
		Col: db.Collection("global_configs"),
	}
}

// Create creates a new global configuration
func (r *ConfigRepository) Create(ctx context.Context, config *models.GlobalQuizConfig) error {
	if config.ID == "" {
		config.ID = primitive.NewObjectID().Hex()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	_, err := r.Col.InsertOne(ctx, config)
	return err
}

// FindByID finds a configuration by ID
func (r *ConfigRepository) FindByID(ctx context.Context, id string) (*models.GlobalQuizConfig, error) {
	var config models.GlobalQuizConfig
	err := r.Col.FindOne(ctx, bson.M{"_id": id}).Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// FindDefault finds the default configuration
func (r *ConfigRepository) FindDefault(ctx context.Context) (*models.GlobalQuizConfig, error) {
	var config models.GlobalQuizConfig
	err := r.Col.FindOne(ctx, bson.M{"is_default": true, "status": "active"}).Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// FindAll finds all configurations
func (r *ConfigRepository) FindAll(ctx context.Context) ([]models.GlobalQuizConfig, error) {
	cur, err := r.Col.Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var configs []models.GlobalQuizConfig
	for cur.Next(ctx) {
		var config models.GlobalQuizConfig
		if err := cur.Decode(&config); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

// Update updates a configuration
func (r *ConfigRepository) Update(ctx context.Context, id string, update map[string]interface{}) error {
	update["updated_at"] = time.Now()
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

// Delete soft deletes a configuration
func (r *ConfigRepository) Delete(ctx context.Context, id string) error {
	update := bson.M{
		"status":     "deleted",
		"updated_at": time.Now(),
	}
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

// EnsureDefaultExists ensures a default configuration exists
func (r *ConfigRepository) EnsureDefaultExists(ctx context.Context) error {
	count, err := r.Col.CountDocuments(ctx, bson.M{"is_default": true, "status": "active"})
	if err != nil {
		return err
	}

	if count == 0 {
		defaultConfig := models.DefaultGlobalConfig()
		return r.Create(ctx, defaultConfig)
	}

	return nil
}

// SetAsDefault sets a configuration as default (and unsets others)
func (r *ConfigRepository) SetAsDefault(ctx context.Context, id string) error {
	// First, unset all existing defaults
	_, err := r.Col.UpdateMany(ctx,
		bson.M{"is_default": true},
		bson.M{"$set": bson.M{"is_default": false, "updated_at": time.Now()}})
	if err != nil {
		return err
	}

	// Then set the new default
	_, err = r.Col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"is_default": true, "updated_at": time.Now()}})
	return err
}

