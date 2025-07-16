package repository

import (
	"context"

	"quiz-service/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AnswerRepository struct {
	Col *mongo.Collection
}

func NewAnswerRepository(db *mongo.Database) *AnswerRepository {
	return &AnswerRepository{Col: db.Collection("answers")}
}

func (r *AnswerRepository) Create(ctx context.Context, answer *models.QuizAnswer) error {
	_, err := r.Col.InsertOne(ctx, answer)
	return err
}

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
