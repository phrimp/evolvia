package repository

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserSkillRepository struct {
	collection *mongo.Collection
}

// NewUserSkillRepository creates a new user skill repository instance
func NewUserSkillRepository(database *mongo.Database, collection string) *UserSkillRepository {
	return &UserSkillRepository{
		collection: database.Collection(collection),
	}
}

// InitializeIndexes creates MongoDB indexes for optimal performance
func (r *UserSkillRepository) InitializeIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "skill_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "skill_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "level", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "confidence", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "years_experience", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "last_used", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "verified", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "endorsements", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	return nil
}

// Create inserts a new user skill
func (r *UserSkillRepository) Create(ctx context.Context, userSkill *models.UserSkill) (*models.UserSkill, error) {
	if userSkill.ID.IsZero() {
		userSkill.ID = bson.NewObjectID()
	}

	now := time.Now()
	userSkill.CreatedAt = now
	userSkill.UpdatedAt = now

	_, err := r.collection.InsertOne(ctx, userSkill)
	if err != nil {
		return nil, fmt.Errorf("failed to create user skill: %w", err)
	}

	return userSkill, nil
}

// GetByID retrieves a user skill by its ID
func (r *UserSkillRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.UserSkill, error) {
	var userSkill models.UserSkill
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&userSkill)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user skill by ID: %w", err)
	}

	return &userSkill, nil
}

// GetByUserAndSkill retrieves a user's specific skill
func (r *UserSkillRepository) GetByUserAndSkill(ctx context.Context, userID, skillID bson.ObjectID) (*models.UserSkill, error) {
	var userSkill models.UserSkill
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}

	err := r.collection.FindOne(ctx, filter).Decode(&userSkill)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user skill: %w", err)
	}

	return &userSkill, nil
}

// GetByUser retrieves all skills for a user
func (r *UserSkillRepository) GetByUser(ctx context.Context, userID bson.ObjectID, opts UserSkillListOptions) ([]*models.UserSkill, error) {
	filter := bson.M{"user_id": userID}

	// Apply filters
	if opts.Level != "" {
		filter["level"] = opts.Level
	}
	if opts.MinConfidence > 0 {
		filter["confidence"] = bson.M{"$gte": opts.MinConfidence}
	}
	if opts.VerifiedOnly {
		filter["verified"] = true
	}

	// Build find options
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}
	if opts.Offset > 0 {
		findOpts.SetSkip(int64(opts.Offset))
	}

	// Default sort by confidence descending
	sortBy := "confidence"
	sortOrder := -1
	if opts.SortBy != "" {
		sortBy = opts.SortBy
		if opts.SortDesc {
			sortOrder = -1
		} else {
			sortOrder = 1
		}
	}
	findOpts.SetSort(bson.M{sortBy: sortOrder})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find user skills: %w", err)
	}
	defer cursor.Close(ctx)

	var userSkills []*models.UserSkill
	for cursor.Next(ctx) {
		var userSkill models.UserSkill
		if err := cursor.Decode(&userSkill); err != nil {
			return nil, fmt.Errorf("failed to decode user skill: %w", err)
		}
		userSkills = append(userSkills, &userSkill)
	}

	return userSkills, nil
}

// GetBySkill retrieves all users who have a specific skill
func (r *UserSkillRepository) GetBySkill(ctx context.Context, skillID bson.ObjectID, opts UserSkillListOptions) ([]*models.UserSkill, error) {
	filter := bson.M{"skill_id": skillID}

	// Apply filters
	if opts.Level != "" {
		filter["level"] = opts.Level
	}
	if opts.MinConfidence > 0 {
		filter["confidence"] = bson.M{"$gte": opts.MinConfidence}
	}
	if opts.VerifiedOnly {
		filter["verified"] = true
	}

	// Build find options
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}
	if opts.Offset > 0 {
		findOpts.SetSkip(int64(opts.Offset))
	}

	// Default sort by confidence and endorsements
	findOpts.SetSort(bson.M{"confidence": -1, "endorsements": -1})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find users with skill: %w", err)
	}
	defer cursor.Close(ctx)

	var userSkills []*models.UserSkill
	for cursor.Next(ctx) {
		var userSkill models.UserSkill
		if err := cursor.Decode(&userSkill); err != nil {
			return nil, fmt.Errorf("failed to decode user skill: %w", err)
		}
		userSkills = append(userSkills, &userSkill)
	}

	return userSkills, nil
}

// Update modifies an existing user skill
func (r *UserSkillRepository) Update(ctx context.Context, id bson.ObjectID, userSkill *models.UserSkill) (*models.UserSkill, error) {
	userSkill.ID = id
	userSkill.UpdatedAt = time.Now()

	update := bson.M{"$set": userSkill}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update user skill: %w", err)
	}

	return userSkill, nil
}

// Delete removes a user skill
func (r *UserSkillRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete user skill: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("user skill not found")
	}

	return nil
}

// DeleteByUserAndSkill removes a specific skill from a user
func (r *UserSkillRepository) DeleteByUserAndSkill(ctx context.Context, userID, skillID bson.ObjectID) error {
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete user skill: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("user skill not found")
	}

	return nil
}

// UpdateLastUsed updates the last used timestamp for a user skill
func (r *UserSkillRepository) UpdateLastUsed(ctx context.Context, userID, skillID bson.ObjectID) error {
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"last_used":  &now,
			"updated_at": now,
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	return nil
}

// IncrementEndorsements increments the endorsement count for a user skill
func (r *UserSkillRepository) IncrementEndorsements(ctx context.Context, userID, skillID bson.ObjectID) error {
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}

	update := bson.M{
		"$inc": bson.M{"endorsements": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment endorsements: %w", err)
	}

	return nil
}

// SetVerified marks a user skill as verified or unverified
func (r *UserSkillRepository) SetVerified(ctx context.Context, userID, skillID bson.ObjectID, verified bool) error {
	filter := bson.M{
		"user_id":  userID,
		"skill_id": skillID,
	}

	update := bson.M{
		"$set": bson.M{
			"verified":   verified,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to set verified status: %w", err)
	}

	return nil
}

// GetTopSkillsByLevel retrieves users with highest skill levels
func (r *UserSkillRepository) GetTopSkillsByLevel(ctx context.Context, skillID bson.ObjectID, limit int) ([]*models.UserSkill, error) {
	filter := bson.M{"skill_id": skillID}

	levelOrder := bson.M{
		"$switch": bson.M{
			"branches": []bson.M{
				{"case": bson.M{"$eq": []any{"$level", "expert"}}, "then": 4},
				{"case": bson.M{"$eq": []any{"$level", "advanced"}}, "then": 3},
				{"case": bson.M{"$eq": []any{"$level", "intermediate"}}, "then": 2},
				{"case": bson.M{"$eq": []any{"$level", "beginner"}}, "then": 1},
			},
			"default": 0,
		},
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$addFields": bson.M{"level_order": levelOrder}},
		{"$sort": bson.M{"level_order": -1, "confidence": -1, "years_experience": -1}},
		{"$limit": limit},
		{"$unset": "level_order"},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top skills by level: %w", err)
	}
	defer cursor.Close(ctx)

	var userSkills []*models.UserSkill
	for cursor.Next(ctx) {
		var userSkill models.UserSkill
		if err := cursor.Decode(&userSkill); err != nil {
			return nil, fmt.Errorf("failed to decode user skill: %w", err)
		}
		userSkills = append(userSkills, &userSkill)
	}

	return userSkills, nil
}

// BatchCreate inserts multiple user skills at once
func (r *UserSkillRepository) BatchCreate(ctx context.Context, userSkills []*models.UserSkill) error {
	if len(userSkills) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]any, len(userSkills))

	for i, userSkill := range userSkills {
		if userSkill.ID.IsZero() {
			userSkill.ID = bson.NewObjectID()
		}
		userSkill.CreatedAt = now
		userSkill.UpdatedAt = now
		docs[i] = userSkill
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("failed to batch create user skills: %w", err)
	}

	return nil
}

// UserSkillListOptions defines options for listing user skills
type UserSkillListOptions struct {
	Limit         int
	Offset        int
	SortBy        string
	SortDesc      bool
	Level         models.SkillLevel
	MinConfidence float64
	VerifiedOnly  bool
}
