package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type UserAuthRepository struct {
	collection *mongo.Collection
}

func NewUserAuthRepository(db *mongo.Database) *UserAuthRepository {
	return &UserAuthRepository{
		collection: db.Collection("UserAuth"),
	}
}

func (r *UserAuthRepository) NewUser(ctx context.Context, user *models.UserAuth) (*models.UserAuth, error) {
	existingUserByEmail, err := r.FindByEmail(ctx, user.Email)
	if err != nil {
		return nil, fmt.Errorf("error checking existing email: %w", err)
	}
	if existingUserByEmail != nil {
		return nil, errors.New("user with this email already exists")
	}

	if user.Username != "" {
		err := r.collection.FindOne(ctx, bson.M{"username": user.Username}).Decode(&models.UserAuth{})
		if err == nil {
			return nil, errors.New("user with this username already exists")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(hashedPassword)

	_, err = r.collection.InsertOne(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return user, nil
}

func (r *UserAuthRepository) Update(ctx context.Context, user *models.UserAuth) error {
	user.UpdatedAt = int(time.Now().Unix())

	filter := bson.M{"_id": user.ID}
	_, err := r.collection.UpdateOne(ctx, filter, bson.M{"$set": user})
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (r *UserAuthRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.UserAuth, error) {
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

func (r *UserAuthRepository) FindByUsername(ctx context.Context, username string) (*models.UserAuth, error) {
	var user models.UserAuth

	err := Repositories_instance.RedisRepository.GetStructCached(ctx, "auth-service-auth-user-"+username, username, &user)
	if err == nil {
		return &user, nil
	}
	log.Printf("Failed to find User in Cached: %s, Find User in DB", err)

	err = r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	isCached, err := Repositories_instance.RedisRepository.SaveStructCached(ctx, username, "auth-service-auth-user-"+username, user, 24)
	if !isCached {
		log.Printf("Failed to save Auth User to Cache: %s", err)
	}
	return &user, nil
}

func (r *UserAuthRepository) VerifyPassword(user *models.UserAuth, password string) bool {
	return user != nil && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
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
	skip := int64((page - 1) * limit)

	pipeline := []bson.M{
		{
			"$sort": bson.M{"createdAt": -1},
		},
		{
			"$skip": skip,
		},
		{
			"$limit": int64(limit),
		},
		{
			"$lookup": bson.M{
				"from": "UserRole",
				"let":  bson.M{"userId": "$_id"},
				"pipeline": bson.A{
					bson.M{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": bson.A{
									bson.M{"$eq": bson.A{"$userId", "$$userId"}},
									bson.M{"$eq": bson.A{"$isActive", true}},
								},
							},
						},
					},
				},
				"as": "userRoles",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "Role",
				"localField":   "userRoles.roleId",
				"foreignField": "_id",
				"as":           "roleDetails",
			},
		},
		{
			"$addFields": bson.M{
				"basicProfile.roles": bson.M{
					"$map": bson.M{
						"input": "$roleDetails",
						"as":    "role",
						"in":    "$$role.name",
					},
				},
			},
		},
		{
			"$project": bson.M{
				"userRoles":   0,
				"roleDetails": 0,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
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
