package repository

import (
	"context"

	"quiz-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type QuizRepository struct {
	Col *mongo.Collection
}

func NewQuizRepository(db *mongo.Database) *QuizRepository {
	return &QuizRepository{Col: db.Collection("quizzes")}
}

func (r *QuizRepository) FindAll(ctx context.Context) ([]models.Quiz, error) {
	cur, err := r.Col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var quizzes []models.Quiz
	for cur.Next(ctx) {
		var q models.Quiz
		if err := cur.Decode(&q); err != nil {
			return nil, err
		}
		quizzes = append(quizzes, q)
	}
	return quizzes, nil
}

func (r *QuizRepository) FindByID(ctx context.Context, id string) (*models.Quiz, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err // invalid id format
	}
	var quiz models.Quiz
	err = r.Col.FindOne(ctx, bson.M{"_id": objID}).Decode(&quiz)
	if err != nil {
		return nil, err
	}
	return &quiz, nil
}

func (r *QuizRepository) Create(ctx context.Context, quiz *models.Quiz) error {
	_, err := r.Col.InsertOne(ctx, quiz)
	return err
}

func (r *QuizRepository) Update(ctx context.Context, id string, update bson.M) error {
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *QuizRepository) Delete(ctx context.Context, id string) error {
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": "deleted"}})
	return err
}
