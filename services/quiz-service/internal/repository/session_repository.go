package repository

import (
	"context"

	"quiz-service/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type SessionRepository struct {
	Col *mongo.Collection
}

func NewSessionRepository(db *mongo.Database) *SessionRepository {
	return &SessionRepository{Col: db.Collection("sessions")}
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*models.QuizSession, error) {
	var session models.QuizSession
	err := r.Col.FindOne(ctx, bson.M{"_id": id}).Decode(&session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepository) Create(ctx context.Context, session *models.QuizSession) error {
	_, err := r.Col.InsertOne(ctx, session)
	return err
}

func (r *SessionRepository) Update(ctx context.Context, id string, update bson.M) error {
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}
