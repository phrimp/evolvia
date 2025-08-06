package repository

import (
	"context"

	"quiz-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type QuestionRepository struct {
	Col *mongo.Collection
}

func NewQuestionRepository(db *mongo.Database) *QuestionRepository {
	return &QuestionRepository{Col: db.Collection("questions")}
}

func (r *QuestionRepository) FindAll(ctx context.Context) ([]models.Question, error) {
	cur, err := r.Col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var questions []models.Question
	for cur.Next(ctx) {
		var q models.Question
		if err := cur.Decode(&q); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (r *QuestionRepository) FindByID(ctx context.Context, id string) (*models.Question, error) {
	var question models.Question
	err := r.Col.FindOne(ctx, bson.M{"_id": id}).Decode(&question)
	if err != nil {
		return nil, err
	}
	return &question, nil
}

func (r *QuestionRepository) Create(ctx context.Context, question *models.Question) error {
	_, err := r.Col.InsertOne(ctx, question)
	return err
}

func (r *QuestionRepository) Update(ctx context.Context, id string, update bson.M) error {
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *QuestionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.Col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": "deleted"}})
	return err
}

func (r *QuestionRepository) FindByQuizID(ctx context.Context, quizID string) ([]models.Question, error) {
	cur, err := r.Col.Find(ctx, bson.M{"quiz_id": quizID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var questions []models.Question
	for cur.Next(ctx) {
		var q models.Question
		if err := cur.Decode(&q); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}
