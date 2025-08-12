package repository

import (
	"context"
	"quiz-service/internal/models"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SessionRepository struct {
	Col *mongo.Collection
}

func NewSessionRepository(db *mongo.Database) *SessionRepository {
	return &SessionRepository{Col: db.Collection("sessions")}
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*models.QuizSession, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var session models.QuizSession
	err = r.Col.FindOne(ctx, bson.M{"_id": objID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepository) Create(ctx context.Context, session *models.QuizSession) error {
	res, err := r.Col.InsertOne(ctx, session)
	if err != nil {
		return err
	}
	// Gán lại ObjectID vào session (nếu dùng primitive.ObjectID)
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		session.ID = oid.Hex()
	}
	return nil
}

func (r *SessionRepository) Update(ctx context.Context, id string, update bson.M) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	
	// Check if update contains special MongoDB operators that should not be wrapped in $set
	hasOperators := false
	for key := range update {
		if strings.HasPrefix(key, "$") {
			hasOperators = true
			break
		}
	}
	
	// If update contains MongoDB operators, use them directly; otherwise wrap in $set
	var updateDoc bson.M
	if hasOperators {
		updateDoc = update
	} else {
		updateDoc = bson.M{"$set": update}
	}
	
	_, err = r.Col.UpdateOne(ctx, bson.M{"_id": objID}, updateDoc)
	return err
}
