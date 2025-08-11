package repository

import (
	"context"
	"quiz-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// DEPRECATED: AnswerRepository is deprecated in favor of cache-based answer storage
// Individual answers are now cached in SessionService instead of persisted to MongoDB
// This repository is maintained for backward compatibility and potential rollback scenarios
// The "answers" collection will no longer receive new data from the current implementation
type AnswerRepository struct {
	Col *mongo.Collection
}

// DEPRECATED: Individual answer persistence is replaced by SessionService caching
func NewAnswerRepository(db *mongo.Database) *AnswerRepository {
	return &AnswerRepository{Col: db.Collection("answers")}
}

// DEPRECATED: Answers are now cached instead of persisted to database
// This method is no longer used by the current implementation
func (r *AnswerRepository) Create(ctx context.Context, answer *models.QuizAnswer) error {
	_, err := r.Col.InsertOne(ctx, answer)
	return err
}

// DEPRECATED: Use SessionService.GetCachedAnswers() for current session data
// This method accesses potentially stale historical data from before cache implementation
func (r *AnswerRepository) FindBySession(ctx context.Context, sessionID string) ([]models.QuizAnswer, error) {
	cur, err := r.Col.Find(ctx, bson.M{"session_id": sessionID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var answers []models.QuizAnswer
	for cur.Next(ctx) {
		var a models.QuizAnswer
		if err := cur.Decode(&a); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}
