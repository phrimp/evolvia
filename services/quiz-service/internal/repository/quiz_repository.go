package repository

import (
	"context"
	"errors"
	"fmt"

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
	// 1. 将字符串 ID 转成 Mongo ObjectID
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid object id: %w", err)
	}

	// 2. 查询
	var quiz models.Quiz
	err = r.Col.FindOne(ctx, bson.M{"_id": objID}).Decode(&quiz)
	switch {
	case err == nil:
		return &quiz, nil
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, err // 自定义 error，方便上层判断
	default:
		// 3. 打日志 / 上报 metrics
		return nil, fmt.Errorf("find quiz: %w", err)
	}
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
