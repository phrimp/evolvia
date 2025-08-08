package repository

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SkillVerificationHistoryRepository struct {
	collection *mongo.Collection
}

func NewSkillVerificationHistoryRepository(database *mongo.Database, collection string) *SkillVerificationHistoryRepository {
	return &SkillVerificationHistoryRepository{
		collection: database.Collection(collection),
	}
}

// Create inserts a new verification history record
func (r *SkillVerificationHistoryRepository) Create(ctx context.Context, history *models.SkillProgressHistory) (*models.SkillProgressHistory, error) {
	if history.ID.IsZero() {
		history.ID = bson.NewObjectID()
	}
	if history.Timestamp.IsZero() {
		history.Timestamp = time.Now()
	}

	_, err := r.collection.InsertOne(ctx, history)
	if err != nil {
		return nil, fmt.Errorf("failed to insert verification history: %w", err)
	}
	return history, nil
}

// GetByID retrieves a verification history record by ID
func (r *SkillVerificationHistoryRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.SkillProgressHistory, error) {
	var history models.SkillProgressHistory
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&history)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get verification history: %w", err)
	}
	return &history, nil
}

// GetByUserAndSkill retrieves all history records for a specific user and skill
func (r *SkillVerificationHistoryRepository) GetByUserAndSkill(ctx context.Context, userID, skillID bson.ObjectID) ([]*models.SkillProgressHistory, error) {
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}
	findOpts := options.Find().SetSort(bson.M{"timestamp": -1})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to query verification history: %w", err)
	}
	defer cursor.Close(ctx)

	var histories []*models.SkillProgressHistory
	for cursor.Next(ctx) {
		var history models.SkillProgressHistory
		if err := cursor.Decode(&history); err != nil {
			return nil, fmt.Errorf("failed to decode verification history: %w", err)
		}
		histories = append(histories, &history)
	}
	return histories, nil
}

// DeleteByID removes a verification history record by ID
func (r *SkillVerificationHistoryRepository) DeleteByID(ctx context.Context, id bson.ObjectID) error {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete verification history: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("verification history not found")
	}
	return nil
}

// InitializeIndexes creates useful indexes for fast lookups
func (r *SkillVerificationHistoryRepository) InitializeIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "skill_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create verification history indexes: %w", err)
	}
	return nil
}
