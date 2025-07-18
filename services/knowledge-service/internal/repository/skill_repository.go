package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"knowledge-service/internal/models"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SkillRepository struct {
	collection *mongo.Collection
}

// NewSkillRepository creates a new skill repository instance
func NewSkillRepository(database *mongo.Database, collection string) *SkillRepository {
	repo := &SkillRepository{
		collection: database.Collection(collection),
	}

	return repo
}

// InitializeIndexes creates MongoDB indexes for optimal performance
func (r *SkillRepository) InitializeIndexes(ctx context.Context) error {
	indexes := models.GetSkillIndexes()
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	return nil
}

// InitializeData loads skill data from /data/skills/* directory
func (r *SkillRepository) InitializeData(ctx context.Context, dataDir string) error {
	skillsDir := filepath.Join(dataDir, "skills")

	// Check if directory exists
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return fmt.Errorf("skills directory not found: %s", skillsDir)
	}

	var skillsLoaded int
	err := filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		log.Println("Found:", d.Name(), "Path:", path)

		// Process only JSON files
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		// Read and parse skill file
		skill, err := r.loadSkillFromFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to load skill from %s: %v\n", path, err)
			return nil // Continue processing other files
		}

		// Check if skill already exists
		exists, err := r.ExistsByName(ctx, skill.Name)
		if err != nil {
			return fmt.Errorf("failed to check if skill exists: %w", err)
		}

		if exists {
			fmt.Printf("Skill '%s' already exists, skipping...\n", skill.Name)
			return nil
		}

		// Insert skill
		_, err = r.Create(ctx, skill)
		if err != nil {
			fmt.Printf("Warning: Failed to insert skill '%s': %v\n", skill.Name, err)
			return nil // Continue processing other files
		}

		skillsLoaded++
		fmt.Printf("Loaded skill: %s\n", skill.Name)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk skills directory: %w", err)
	}

	fmt.Printf("Successfully loaded %d skills from %s\n", skillsLoaded, skillsDir)
	return nil
}

// loadSkillFromFile reads and parses a skill JSON file
func (r *SkillRepository) loadSkillFromFile(filePath string) (*models.Skill, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var skill models.Skill
	if err := json.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Set default values
	now := time.Now()
	skill.CreatedAt = now
	skill.UpdatedAt = now
	skill.IsActive = true
	skill.Version = 1
	skill.UsageCount = 0

	return &skill, nil
}

// Create inserts a new skill
func (r *SkillRepository) Create(ctx context.Context, skill *models.Skill) (*models.Skill, error) {
	if skill.ID.IsZero() {
		skill.ID = bson.NewObjectID()
	}

	now := time.Now()
	skill.CreatedAt = now
	skill.UpdatedAt = now
	skill.IsActive = true
	skill.Version = 1

	_, err := r.collection.InsertOne(ctx, skill)
	if err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}

	return skill, nil
}

// GetByID retrieves a skill by its ID
func (r *SkillRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.Skill, error) {
	var skill models.Skill
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&skill)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get skill by ID: %w", err)
	}

	return &skill, nil
}

// GetByName retrieves a skill by its name
func (r *SkillRepository) GetByName(ctx context.Context, name string) (*models.Skill, error) {
	var skill models.Skill
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&skill)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get skill by name: %w", err)
	}

	return &skill, nil
}

// ExistsByName checks if a skill with the given name exists
func (r *SkillRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"name": name})
	if err != nil {
		return false, fmt.Errorf("failed to check skill existence: %w", err)
	}
	return count > 0, nil
}

// Update modifies an existing skill
func (r *SkillRepository) Update(ctx context.Context, id bson.ObjectID, skill *models.Skill) (*models.Skill, error) {
	skill.ID = id
	skill.UpdatedAt = time.Now()
	skill.Version++

	update := bson.M{"$set": skill}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update skill: %w", err)
	}

	return skill, nil
}

// Delete removes a skill (soft delete by setting is_active to false)
func (r *SkillRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("failed to delete skill: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("skill not found")
	}

	return nil
}

// HardDelete permanently removes a skill from the database
func (r *SkillRepository) HardDelete(ctx context.Context, id bson.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to hard delete skill: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("skill not found")
	}

	return nil
}

// List retrieves skills with pagination and filtering
func (r *SkillRepository) List(ctx context.Context, opts ListOptions) ([]*models.Skill, int64, error) {
	filter := r.buildFilter(opts)

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count skills: %w", err)
	}

	// Build find options
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.SetLimit(int64(opts.Limit))
	}
	if opts.Offset > 0 {
		findOpts.SetSkip(int64(opts.Offset))
	}
	if opts.SortBy != "" {
		sortOrder := 1
		if opts.SortDesc {
			sortOrder = -1
		}
		findOpts.SetSort(bson.M{opts.SortBy: sortOrder})
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, 0, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("cursor error: %w", err)
	}

	return skills, total, nil
}

// SearchByKeywords searches skills by keywords in various fields
func (r *SkillRepository) SearchByKeywords(ctx context.Context, keywords string, limit int) ([]*models.Skill, error) {
	if keywords == "" {
		return []*models.Skill{}, nil
	}

	// Create text search filter
	filter := bson.M{
		"$and": []bson.M{
			{"is_active": true},
			{
				"$or": []bson.M{
					{"name": bson.M{"$regex": keywords, "$options": "i"}},
					{"description": bson.M{"$regex": keywords, "$options": "i"}},
					{"common_names": bson.M{"$regex": keywords, "$options": "i"}},
					{"technical_terms": bson.M{"$regex": keywords, "$options": "i"}},
					{"tags": bson.M{"$regex": keywords, "$options": "i"}},
				},
			},
		},
	}

	findOpts := options.Find()
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}
	findOpts.SetSort(bson.M{"usage_count": -1, "name": 1})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetByCategory retrieves skills by category
func (r *SkillRepository) GetByCategory(ctx context.Context, categoryID bson.ObjectID) ([]*models.Skill, error) {
	filter := bson.M{
		"category_id": categoryID,
		"is_active":   true,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get skills by category: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// IncrementUsageCount increments the usage counter for a skill
func (r *SkillRepository) IncrementUsageCount(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	update := bson.M{
		"$inc": bson.M{"usage_count": 1},
		"$set": bson.M{
			"last_used":  &now,
			"updated_at": now,
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("failed to increment usage count: %w", err)
	}

	return nil
}

// GetMostUsed retrieves the most frequently used skills
func (r *SkillRepository) GetMostUsed(ctx context.Context, limit int) ([]*models.Skill, error) {
	findOpts := options.Find().
		SetSort(bson.M{"usage_count": -1}).
		SetLimit(int64(limit))

	filter := bson.M{"is_active": true}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get most used skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// ListOptions defines options for listing skills
type ListOptions struct {
	Limit         int
	Offset        int
	SortBy        string
	SortDesc      bool
	ActiveOnly    bool
	CategoryID    *bson.ObjectID
	Tags          []string
	Industry      []string
	MinDifficulty int
	MaxDifficulty int
	Trending      *bool
}

// buildFilter constructs MongoDB filter from ListOptions
func (r *SkillRepository) buildFilter(opts ListOptions) bson.M {
	filter := bson.M{}

	if opts.ActiveOnly {
		filter["is_active"] = true
	}

	if opts.CategoryID != nil {
		filter["category_id"] = *opts.CategoryID
	}

	if len(opts.Tags) > 0 {
		filter["tags"] = bson.M{"$in": opts.Tags}
	}

	if len(opts.Industry) > 0 {
		filter["metadata.industry"] = bson.M{"$in": opts.Industry}
	}

	if opts.MinDifficulty > 0 || opts.MaxDifficulty > 0 {
		difficultyFilter := bson.M{}
		if opts.MinDifficulty > 0 {
			difficultyFilter["$gte"] = opts.MinDifficulty
		}
		if opts.MaxDifficulty > 0 {
			difficultyFilter["$lte"] = opts.MaxDifficulty
		}
		filter["metadata.difficulty"] = difficultyFilter
	}

	if opts.Trending != nil {
		filter["metadata.trending"] = *opts.Trending
	}

	return filter
}

// BatchCreate inserts multiple skills at once
func (r *SkillRepository) BatchCreate(ctx context.Context, skills []*models.Skill) error {
	if len(skills) == 0 {
		return nil
	}

	now := time.Now()
	docs := make([]any, len(skills))

	for i, skill := range skills {
		if skill.ID.IsZero() {
			skill.ID = bson.NewObjectID()
		}
		skill.CreatedAt = now
		skill.UpdatedAt = now
		skill.IsActive = true
		if skill.Version == 0 {
			skill.Version = 1
		}
		docs[i] = skill
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("failed to batch create skills: %w", err)
	}

	return nil
}

// GetRelatedSkills finds skills related to the given skill
func (r *SkillRepository) GetRelatedSkills(ctx context.Context, skillID bson.ObjectID, relationType models.RelationType) ([]*models.Skill, error) {
	filter := bson.M{
		"relations": bson.M{
			"$elemMatch": bson.M{
				"skill_id":      skillID,
				"relation_type": relationType,
			},
		},
		"is_active": true,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get related skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}
