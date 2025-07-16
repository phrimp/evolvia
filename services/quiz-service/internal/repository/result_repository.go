package repository

import (
	"context"

	"quiz-service/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ResultRepository struct {
	Col *mongo.Collection
}

func NewResultRepository(db *mongo.Database) *ResultRepository {
	return &ResultRepository{Col: db.Collection("results")}
}

func (r *ResultRepository) FindBySession(ctx context.Context, sessionID string) (*models.QuizResult, error) {
	var result models.QuizResult
	err := r.Col.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *ResultRepository) FindByUser(ctx context.Context, userID string) ([]models.QuizResult, error) {
	cur, err := r.Col.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var results []models.QuizResult
	for cur.Next(ctx) {
		var res models.QuizResult
		if err := cur.Decode(&res); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (r *ResultRepository) FindByQuiz(ctx context.Context, quizID string) ([]models.QuizResult, error) {
	cur, err := r.Col.Find(ctx, bson.M{"quiz_id": quizID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var results []models.QuizResult
	for cur.Next(ctx) {
		var res models.QuizResult
		if err := cur.Decode(&res); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (r *ResultRepository) Create(ctx context.Context, result *models.QuizResult) error {
	_, err := r.Col.InsertOne(ctx, result)
	return err
}
