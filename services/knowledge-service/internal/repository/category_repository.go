package repository

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type CategoryRepository struct {
	collection *mongo.Collection
}

func NewCategoryRepository(database *mongo.Database, collection string) *CategoryRepository {
	return &CategoryRepository{
		collection: database.Collection(collection),
	}
}

// InitializeIndexes creates MongoDB indexes for categories
func (r *CategoryRepository) InitializeIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "parent_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "parent_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "path", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "level", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create category indexes: %w", err)
	}
	return nil
}

// GetAll retrieves all categories
func (r *CategoryRepository) GetAll(ctx context.Context) ([]*models.SkillCategory, error) {
	findOpts := options.Find().SetSort(bson.M{"level": 1, "name": 1})

	cursor, err := r.collection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find categories: %w", err)
	}
	defer cursor.Close(ctx)

	var categories []*models.SkillCategory
	for cursor.Next(ctx) {
		var category models.SkillCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// GetByParentID retrieves categories by parent ID
func (r *CategoryRepository) GetByParentID(ctx context.Context, parentID *bson.ObjectID) ([]*models.SkillCategory, error) {
	filter := bson.M{}
	if parentID != nil {
		filter["parent_id"] = *parentID
	} else {
		filter["parent_id"] = bson.M{"$exists": false}
	}

	findOpts := options.Find().SetSort(bson.M{"name": 1})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find categories by parent: %w", err)
	}
	defer cursor.Close(ctx)

	var categories []*models.SkillCategory
	for cursor.Next(ctx) {
		var category models.SkillCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// GetByID retrieves a category by ID
func (r *CategoryRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.SkillCategory, error) {
	var category models.SkillCategory
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get category by ID: %w", err)
	}

	return &category, nil
}

// Create inserts a new category
func (r *CategoryRepository) Create(ctx context.Context, category *models.SkillCategory) (*models.SkillCategory, error) {
	if category.ID.IsZero() {
		category.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return category, nil
}
