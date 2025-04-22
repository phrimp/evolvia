package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserAuthRepository struct {
	collection *mongo.Collection
}

func NewUserAuthRepository(db *mongo.Database) *UserAuthRepository {
	return &UserAuthRepository{
		collection: db.Collection("UserAuth"),
	}
}

func (r *UserAuthRepository) New()

func (r *UserAuthRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.UserAuth, error) {
	var user models.UserAuth
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserAuthRepository) FindByUsernamePassword(ctx context.Context, username, password string) (*models.UserAuth, error) {
	var user models.UserAuth
	err := r.collection.FindOne(ctx, bson.M{"username": username, "password": password}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserAuthRepository) FindByEmail(ctx context.Context, email string) (*models.UserAuth, error) {
	var user models.UserAuth
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserAuthRepository) FindAll(ctx context.Context, page, limit int) ([]*models.UserAuth, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*models.UserAuth
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}
