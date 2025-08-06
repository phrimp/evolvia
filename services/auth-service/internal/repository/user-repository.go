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
				"basicProfile.roles": "$roleDetails.name",
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

// 1. First, let's check what's actually in your UserRole collection
func (r *UserAuthRepository) DebugUserRoles(ctx context.Context, userID bson.ObjectID) {
	userRoleCollection := r.collection.Database().Collection("UserRole")

	// Check all UserRole documents for this user (ignore isActive for now)
	cursor, err := userRoleCollection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		log.Printf("Error finding UserRoles: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var userRoles []bson.M
	if err = cursor.All(ctx, &userRoles); err != nil {
		log.Printf("Error decoding UserRoles: %v", err)
		return
	}

	log.Printf("Found %d UserRole records for user %s:", len(userRoles), userID.Hex())
	for i, ur := range userRoles {
		log.Printf("UserRole %d: %+v", i, ur)
	}
}

// 2. Check what's in your Role collection
func (r *UserAuthRepository) DebugRoles(ctx context.Context) {
	roleCollection := r.collection.Database().Collection("Role")

	cursor, err := roleCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Printf("Error finding Roles: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var roles []bson.M
	if err = cursor.All(ctx, &roles); err != nil {
		log.Printf("Error decoding Roles: %v", err)
		return
	}

	log.Printf("Found %d Role records:", len(roles))
	for i, role := range roles {
		log.Printf("Role %d: %+v", i, role)
	}
}

// 3. Test the aggregation step by step
func (r *UserAuthRepository) DebugAggregationSteps(ctx context.Context, userID bson.ObjectID) {
	// Test just the UserRole lookup
	pipeline1 := []bson.M{
		{
			"$match": bson.M{"_id": userID},
		},
		{
			"$lookup": bson.M{
				"from": "UserRole",
				"let":  bson.M{"userId": "$_id"},
				"pipeline": bson.A{
					bson.M{
						"$match": bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$userId", "$$userId"},
							},
						},
					},
				},
				"as": "userRoles",
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline1)
	if err != nil {
		log.Printf("Error in step 1 aggregation: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var result1 []bson.M
	if err = cursor.All(ctx, &result1); err != nil {
		log.Printf("Error decoding step 1: %v", err)
		return
	}

	log.Printf("Step 1 result (UserRole lookup): %+v", result1)

	// Test with Role lookup
	pipeline2 := []bson.M{
		{
			"$match": bson.M{"_id": userID},
		},
		{
			"$lookup": bson.M{
				"from": "UserRole",
				"let":  bson.M{"userId": "$_id"},
				"pipeline": bson.A{
					bson.M{
						"$match": bson.M{
							"$expr": bson.M{
								"$eq": bson.A{"$userId", "$$userId"},
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
	}

	cursor2, err := r.collection.Aggregate(ctx, pipeline2)
	if err != nil {
		log.Printf("Error in step 2 aggregation: %v", err)
		return
	}
	defer cursor2.Close(ctx)

	var result2 []bson.M
	if err = cursor2.All(ctx, &result2); err != nil {
		log.Printf("Error decoding step 2: %v", err)
		return
	}

	log.Printf("Step 2 result (Role lookup): %+v", result2)
}

// 4. Alternative approach - check field names by looking at first document
func (r *UserAuthRepository) DebugFieldNames(ctx context.Context) {
	userRoleCollection := r.collection.Database().Collection("UserRole")
	roleCollection := r.collection.Database().Collection("Role")

	// Get first UserRole document
	var userRole bson.M
	err := userRoleCollection.FindOne(ctx, bson.M{}).Decode(&userRole)
	if err != nil {
		log.Printf("No UserRole documents found or error: %v", err)
	} else {
		log.Printf("Sample UserRole document fields: %+v", userRole)
	}

	// Get first Role document
	var role bson.M
	err = roleCollection.FindOne(ctx, bson.M{}).Decode(&role)
	if err != nil {
		log.Printf("No Role documents found or error: %v", err)
	} else {
		log.Printf("Sample Role document fields: %+v", role)
	}
}
