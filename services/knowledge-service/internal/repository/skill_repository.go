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

	for i, skill_related := range skill.Relations {
		related_skill, err := r.GetByID(ctx, skill_related.SkillID)
		if err != nil {
			log.Println("Find skill related by id failed: ", err)
			continue
		}
		skill.Relations[i].Description = related_skill.Description
		skill.Relations[i].TaggedSkill = related_skill.TaggedSkill
		skill.Relations[i].Name = related_skill.Name
	}

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

func (r *SkillRepository) SearchByKeywordsWithCategories(ctx context.Context, keywords string, limit int) ([]*models.SkillWithCategory, error) {
	if keywords == "" {
		return []*models.SkillWithCategory{}, nil
	}

	// Use aggregation pipeline to join with categories and search
	pipeline := []bson.M{
		// Match only active skills first
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Left join with categories collection
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category_info",
			},
		},
		// Add category name field for easier searching
		{
			"$addFields": bson.M{
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category_info.name", 0}},
						"",
					},
				},
				"category_path": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category_info.path", 0}},
						"",
					},
				},
			},
		},
		// Filter by search keywords across multiple fields
		{
			"$match": bson.M{
				"$or": []bson.M{
					{"name": bson.M{"$regex": keywords, "$options": "i"}},
					{"description": bson.M{"$regex": keywords, "$options": "i"}},
					{"common_names": bson.M{"$regex": keywords, "$options": "i"}},
					{"technical_terms": bson.M{"$regex": keywords, "$options": "i"}},
					{"tags": bson.M{"$regex": keywords, "$options": "i"}},
					{"category_name": bson.M{"$regex": keywords, "$options": "i"}},
				},
			},
		},
		// Sort by relevance (usage count) and name
		{
			"$sort": bson.M{
				"usage_count": -1,
				"name":        1,
			},
		},
		// Remove the temporary category_info array
		{
			"$unset": "category_info",
		},
	}

	// Add limit if specified
	if limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": limit})
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to search skills with categories: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*models.SkillWithCategory
	for cursor.Next(ctx) {
		// First decode into a map to handle the flat structure
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode document: %w", err)
		}

		// Create the skill object
		skill := &models.Skill{
			ID:             doc["_id"].(bson.ObjectID),
			Name:           getStringFromDoc(doc, "name"),
			Description:    getStringFromDoc(doc, "description"),
			CommonNames:    getStringArrayFromDoc(doc, "common_names"),
			Abbreviations:  getStringArrayFromDoc(doc, "abbreviations"),
			TechnicalTerms: getStringArrayFromDoc(doc, "technical_terms"),
			Tags:           getStringArrayFromDoc(doc, "tags"),
			IsActive:       getBoolFromDoc(doc, "is_active"),
			Version:        getIntFromDoc(doc, "version"),
			UsageCount:     getIntFromDoc(doc, "usage_count"),
			CreatedAt:      getTimeFromDoc(doc, "created_at"),
			UpdatedAt:      getTimeFromDoc(doc, "updated_at"),
		}

		// Handle optional fields
		if catID, ok := doc["category_id"].(bson.ObjectID); ok {
			skill.CategoryID = &catID
		}

		if lastUsed, ok := doc["last_used"].(time.Time); ok {
			skill.LastUsed = &lastUsed
		}

		// Handle nested structs
		if identRules, ok := doc["identification_rules"].(bson.M); ok {
			skill.IdentificationRules = parseIdentificationRules(identRules)
		}

		if metadata, ok := doc["metadata"].(bson.M); ok {
			skill.Metadata = parseMetadata(metadata)
		}

		if relations, ok := doc["relations"].([]interface{}); ok {
			skill.Relations = parseRelations(relations)
		}

		if category, ok := doc["category"].(bson.M); ok {
			skill.Category = parseCategory(category)
		}

		if createdBy, ok := doc["created_by"].(bson.ObjectID); ok {
			skill.CreatedBy = &createdBy
		}

		if updatedBy, ok := doc["updated_by"].(bson.ObjectID); ok {
			skill.UpdatedBy = &updatedBy
		}

		// Create the result with category information
		result := &models.SkillWithCategory{
			Skill:        skill,
			CategoryName: getStringFromDoc(doc, "category_name"),
			CategoryPath: getStringFromDoc(doc, "category_path"),
		}

		results = append(results, result)
	}

	return results, nil
}

// Helper functions to safely extract values from BSON document
func getStringFromDoc(doc bson.M, key string) string {
	if val, ok := doc[key].(string); ok {
		return val
	}
	return ""
}

func getStringArrayFromDoc(doc bson.M, key string) []string {
	if val, ok := doc[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if str, ok := v.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return []string{}
}

func getBoolFromDoc(doc bson.M, key string) bool {
	if val, ok := doc[key].(bool); ok {
		return val
	}
	return false
}

func getIntFromDoc(doc bson.M, key string) int {
	if val, ok := doc[key].(int32); ok {
		return int(val)
	}
	if val, ok := doc[key].(int64); ok {
		return int(val)
	}
	if val, ok := doc[key].(int); ok {
		return val
	}
	return 0
}

func getTimeFromDoc(doc bson.M, key string) time.Time {
	if val, ok := doc[key].(time.Time); ok {
		return val
	}
	return time.Time{}
}

func parseIdentificationRules(data bson.M) models.SkillIdentificationRules {
	rules := models.SkillIdentificationRules{}

	if primaryPatterns, ok := data["primary_patterns"].([]interface{}); ok {
		rules.PrimaryPatterns = parseKeywordPatterns(primaryPatterns)
	}

	if secondaryPatterns, ok := data["secondary_patterns"].([]interface{}); ok {
		rules.SecondaryPatterns = parseKeywordPatterns(secondaryPatterns)
	}

	if academicPatterns, ok := data["academic_patterns"].([]interface{}); ok {
		rules.AcademicPatterns = parseKeywordPatterns(academicPatterns)
	}

	if negativePatterns, ok := data["negative_patterns"].([]interface{}); ok {
		rules.NegativePatterns = parseKeywordPatterns(negativePatterns)
	}

	rules.MinPrimaryMatches = getIntFromDoc(data, "min_primary_matches")
	rules.MinSecondaryMatches = getIntFromDoc(data, "min_secondary_matches")
	rules.MinAcademicMatch = getIntFromDoc(data, "min_academic_matches")

	if minScore, ok := data["min_total_score"].(float64); ok {
		rules.MinTotalScore = minScore
	}

	rules.ContextWindow = getIntFromDoc(data, "context_window")

	return rules
}

func parseKeywordPatterns(data []interface{}) []models.KeywordPattern {
	patterns := make([]models.KeywordPattern, 0, len(data))
	for _, item := range data {
		if pattern, ok := item.(bson.M); ok {
			kp := models.KeywordPattern{
				Text:            getStringFromDoc(pattern, "text"),
				Type:            getStringFromDoc(pattern, "type"),
				CaseSensitive:   getBoolFromDoc(pattern, "case_sensitive"),
				MinWordBoundary: getBoolFromDoc(pattern, "min_word_boundary"),
			}
			if weight, ok := pattern["weight"].(float64); ok {
				kp.Weight = weight
			}
			patterns = append(patterns, kp)
		}
	}
	return patterns
}

func parseMetadata(data bson.M) models.SkillMetadata {
	metadata := models.SkillMetadata{
		Industry:    getStringArrayFromDoc(data, "industry"),
		JobRoles:    getStringArrayFromDoc(data, "job_roles"),
		Difficulty:  getIntFromDoc(data, "difficulty"),
		TimeToLearn: getIntFromDoc(data, "time_to_learn"),
		Trending:    getBoolFromDoc(data, "trending"),
	}

	if demand, ok := data["market_demand"].(float64); ok {
		metadata.MarketDemand = demand
	}

	return metadata
}

func parseRelations(data []interface{}) []models.SkillRelation {
	relations := make([]models.SkillRelation, 0, len(data))
	for _, item := range data {
		if rel, ok := item.(bson.M); ok {
			relation := models.SkillRelation{
				Description: getStringFromDoc(rel, "description"),
			}

			if skillID, ok := rel["skill_id"].(bson.ObjectID); ok {
				relation.SkillID = skillID
			}

			if relType, ok := rel["relation_type"].(string); ok {
				relation.RelationType = models.RelationType(relType)
			}

			if strength, ok := rel["strength"].(float64); ok {
				relation.Strength = strength
			}

			relations = append(relations, relation)
		}
	}
	return relations
}

func parseCategory(data bson.M) *models.SkillCategory {
	if len(data) == 0 {
		return nil
	}

	category := &models.SkillCategory{
		Name:      getStringFromDoc(data, "name"),
		Path:      getStringFromDoc(data, "path"),
		Level:     getIntFromDoc(data, "level"),
		CreatedAt: getTimeFromDoc(data, "created_at"),
		UpdatedAt: getTimeFromDoc(data, "updated_at"),
	}

	if id, ok := data["_id"].(bson.ObjectID); ok {
		category.ID = id
	}

	if parentID, ok := data["parent_id"].(bson.ObjectID); ok {
		category.ParentID = &parentID
	}

	return category
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

// buildFilter constructs MongoDB filter from ListOptions
func (r *SkillRepository) buildFilter(opts ListOptions) bson.M {
	filter := bson.M{}

	if opts.ActiveOnly {
		filter["is_active"] = true
	}

	if opts.CategoryID != nil {
		filter["category_id"] = *opts.CategoryID
	}

	// Support both legacy tags and new categorized tags
	if len(opts.Tags) > 0 {
		filter["$or"] = []bson.M{
			{"tags": bson.M{"$in": opts.Tags}},
			{"tagged_skill.primary_tags": bson.M{"$in": opts.Tags}},
			{"tagged_skill.secondary_tags": bson.M{"$in": opts.Tags}},
			{"tagged_skill.related_tags": bson.M{"$in": opts.Tags}},
		}
	}

	// Add support for filtering by specific tag categories
	if len(opts.PrimaryTags) > 0 {
		filter["tagged_skill.primary_tags"] = bson.M{"$in": opts.PrimaryTags}
	}

	if len(opts.SecondaryTags) > 0 {
		filter["tagged_skill.secondary_tags"] = bson.M{"$in": opts.SecondaryTags}
	}

	if len(opts.RelatedTags) > 0 {
		filter["tagged_skill.related_tags"] = bson.M{"$in": opts.RelatedTags}
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

type ListOptions struct {
	Limit         int
	Offset        int
	SortBy        string
	SortDesc      bool
	ActiveOnly    bool
	CategoryID    *bson.ObjectID
	Tags          []string // Legacy support
	PrimaryTags   []string // New: filter by primary tags
	SecondaryTags []string // New: filter by secondary tags
	RelatedTags   []string // New: filter by related tags
	Industry      []string
	MinDifficulty int
	MaxDifficulty int
	Trending      *bool
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

func (r *SkillRepository) GetTopSkillsByUsage(ctx context.Context, limit int) ([]*models.SkillWithStats, error) {
	pipeline := []bson.M{
		// Match active skills only
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Join with user_skills to get user count
		{
			"$lookup": bson.M{
				"from":         "user_skills",
				"localField":   "_id",
				"foreignField": "skill_id",
				"as":           "user_skills",
			},
		},
		// Join with categories
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add calculated fields
		{
			"$addFields": bson.M{
				"user_count": bson.M{"$size": "$user_skills"},
				"total_endorsements": bson.M{
					"$sum": "$user_skills.endorsements",
				},
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Sort by usage count descending
		{
			"$sort": bson.M{
				"usage_count": -1,
				"user_count":  -1,
				"name":        1,
			},
		},
		// Limit results
		{
			"$limit": limit,
		},
		// Project final structure
		{
			"$project": bson.M{
				"_id":                1,
				"name":               1,
				"description":        1,
				"category_id":        1,
				"category_name":      1,
				"tags":               1,
				"metadata":           1,
				"usage_count":        1,
				"user_count":         1,
				"total_endorsements": 1,
				"last_used":          1,
				"created_at":         1,
				"updated_at":         1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top skills by usage: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.SkillWithStats
	for cursor.Next(ctx) {
		var skill models.SkillWithStats
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetTopSkillsByPopularity retrieves skills with most users
func (r *SkillRepository) GetTopSkillsByPopularity(ctx context.Context, limit int) ([]*models.SkillWithStats, error) {
	pipeline := []bson.M{
		// Match active skills only
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Join with user_skills to get user count
		{
			"$lookup": bson.M{
				"from":         "user_skills",
				"localField":   "_id",
				"foreignField": "skill_id",
				"as":           "user_skills",
			},
		},
		// Join with categories
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add calculated fields
		{
			"$addFields": bson.M{
				"user_count": bson.M{"$size": "$user_skills"},
				"total_endorsements": bson.M{
					"$sum": "$user_skills.endorsements",
				},
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Filter out skills with no users
		{
			"$match": bson.M{
				"user_count": bson.M{"$gt": 0},
			},
		},
		// Sort by user count descending
		{
			"$sort": bson.M{
				"user_count":  -1,
				"usage_count": -1,
				"name":        1,
			},
		},
		// Limit results
		{
			"$limit": limit,
		},
		// Project final structure
		{
			"$project": bson.M{
				"_id":                1,
				"name":               1,
				"description":        1,
				"category_id":        1,
				"category_name":      1,
				"tags":               1,
				"metadata":           1,
				"usage_count":        1,
				"user_count":         1,
				"total_endorsements": 1,
				"last_used":          1,
				"created_at":         1,
				"updated_at":         1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top skills by popularity: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.SkillWithStats
	for cursor.Next(ctx) {
		var skill models.SkillWithStats
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetTopSkillsByEndorsements retrieves skills with most endorsements
func (r *SkillRepository) GetTopSkillsByEndorsements(ctx context.Context, limit int) ([]*models.SkillWithStats, error) {
	pipeline := []bson.M{
		// Match active skills only
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Join with user_skills to get endorsement data
		{
			"$lookup": bson.M{
				"from":         "user_skills",
				"localField":   "_id",
				"foreignField": "skill_id",
				"as":           "user_skills",
			},
		},
		// Join with categories
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add calculated fields
		{
			"$addFields": bson.M{
				"user_count": bson.M{"$size": "$user_skills"},
				"total_endorsements": bson.M{
					"$sum": "$user_skills.endorsements",
				},
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Filter out skills with no endorsements
		{
			"$match": bson.M{
				"total_endorsements": bson.M{"$gt": 0},
			},
		},
		// Sort by total endorsements descending
		{
			"$sort": bson.M{
				"total_endorsements": -1,
				"user_count":         -1,
				"usage_count":        -1,
				"name":               1,
			},
		},
		// Limit results
		{
			"$limit": limit,
		},
		// Project final structure
		{
			"$project": bson.M{
				"_id":                1,
				"name":               1,
				"description":        1,
				"category_id":        1,
				"category_name":      1,
				"tags":               1,
				"metadata":           1,
				"usage_count":        1,
				"user_count":         1,
				"total_endorsements": 1,
				"last_used":          1,
				"created_at":         1,
				"updated_at":         1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top skills by endorsements: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.SkillWithStats
	for cursor.Next(ctx) {
		var skill models.SkillWithStats
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetTopTrendingSkills retrieves trending skills
func (r *SkillRepository) GetTopTrendingSkills(ctx context.Context, limit int) ([]*models.SkillWithStats, error) {
	pipeline := []bson.M{
		// Match active and trending skills
		{
			"$match": bson.M{
				"is_active":         true,
				"metadata.trending": true,
			},
		},
		// Join with user_skills to get user data
		{
			"$lookup": bson.M{
				"from":         "user_skills",
				"localField":   "_id",
				"foreignField": "skill_id",
				"as":           "user_skills",
			},
		},
		// Join with categories
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add calculated fields
		{
			"$addFields": bson.M{
				"user_count": bson.M{"$size": "$user_skills"},
				"total_endorsements": bson.M{
					"$sum": "$user_skills.endorsements",
				},
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Sort by market demand and usage
		{
			"$sort": bson.M{
				"metadata.market_demand": -1,
				"user_count":             -1,
				"usage_count":            -1,
				"name":                   1,
			},
		},
		// Limit results
		{
			"$limit": limit,
		},
		// Project final structure
		{
			"$project": bson.M{
				"_id":                1,
				"name":               1,
				"description":        1,
				"category_id":        1,
				"category_name":      1,
				"tags":               1,
				"metadata":           1,
				"usage_count":        1,
				"user_count":         1,
				"total_endorsements": 1,
				"last_used":          1,
				"created_at":         1,
				"updated_at":         1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top trending skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.SkillWithStats
	for cursor.Next(ctx) {
		var skill models.SkillWithStats
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetTopRecentlyAddedSkills retrieves recently added skills
func (r *SkillRepository) GetTopRecentlyAddedSkills(ctx context.Context, limit int) ([]*models.SkillWithStats, error) {
	pipeline := []bson.M{
		// Match active skills only
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Join with user_skills to get user data
		{
			"$lookup": bson.M{
				"from":         "user_skills",
				"localField":   "_id",
				"foreignField": "skill_id",
				"as":           "user_skills",
			},
		},
		// Join with categories
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add calculated fields
		{
			"$addFields": bson.M{
				"user_count": bson.M{"$size": "$user_skills"},
				"total_endorsements": bson.M{
					"$sum": "$user_skills.endorsements",
				},
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Sort by creation date descending
		{
			"$sort": bson.M{
				"created_at":  -1,
				"user_count":  -1,
				"usage_count": -1,
			},
		},
		// Limit results
		{
			"$limit": limit,
		},
		// Project final structure
		{
			"$project": bson.M{
				"_id":                1,
				"name":               1,
				"description":        1,
				"category_id":        1,
				"category_name":      1,
				"tags":               1,
				"metadata":           1,
				"usage_count":        1,
				"user_count":         1,
				"total_endorsements": 1,
				"last_used":          1,
				"created_at":         1,
				"updated_at":         1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top recently added skills: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.SkillWithStats
	for cursor.Next(ctx) {
		var skill models.SkillWithStats
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skills = append(skills, &skill)
	}

	return skills, nil
}

func (r *SkillRepository) SearchByKeywords(ctx context.Context, keywords string, limit int) ([]*models.Skill, error) {
	if keywords == "" {
		return []*models.Skill{}, nil
	}

	// Use aggregation pipeline to join with categories and search
	pipeline := []bson.M{
		// Match only active skills first
		{
			"$match": bson.M{
				"is_active": true,
			},
		},
		// Left join with categories collection
		{
			"$lookup": bson.M{
				"from":         "categories",
				"localField":   "category_id",
				"foreignField": "_id",
				"as":           "category",
			},
		},
		// Add category name field for easier searching
		{
			"$addFields": bson.M{
				"category_name": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$category.name", 0}},
						"",
					},
				},
			},
		},
		// Filter by search keywords across multiple fields including categorized tags
		{
			"$match": bson.M{
				"$or": []bson.M{
					{"name": bson.M{"$regex": keywords, "$options": "i"}},
					{"description": bson.M{"$regex": keywords, "$options": "i"}},
					{"common_names": bson.M{"$regex": keywords, "$options": "i"}},
					{"technical_terms": bson.M{"$regex": keywords, "$options": "i"}},
					{"tags": bson.M{"$regex": keywords, "$options": "i"}}, // Legacy
					{"tagged_skill.primary_tags": bson.M{"$regex": keywords, "$options": "i"}},
					{"tagged_skill.secondary_tags": bson.M{"$regex": keywords, "$options": "i"}},
					{"tagged_skill.related_tags": bson.M{"$regex": keywords, "$options": "i"}},
					{"category_name": bson.M{"$regex": keywords, "$options": "i"}},
				},
			},
		},
		// Sort by relevance (usage count) and name
		{
			"$sort": bson.M{
				"usage_count": -1,
				"name":        1,
			},
		},
		// Remove the temporary category_name field and category array
		{
			"$unset": []string{"category_name", "category"},
		},
	}

	// Add limit if specified
	if limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": limit})
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
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
		// Migrate legacy tags if needed
		skill.MigrateLegacyTags()
		skills = append(skills, &skill)
	}

	return skills, nil
}

// SearchByTagCategory searches for skills by specific tag category
func (r *SkillRepository) SearchByTagCategory(ctx context.Context, tagCategory string, tags []string, limit int) ([]*models.Skill, error) {
	if len(tags) == 0 {
		return []*models.Skill{}, nil
	}

	var fieldPath string
	switch tagCategory {
	case "primary":
		fieldPath = "tagged_skill.primary_tags"
	case "secondary":
		fieldPath = "tagged_skill.secondary_tags"
	case "related":
		fieldPath = "tagged_skill.related_tags"
	default:
		// Search across all categories
		return r.SearchByTags(ctx, tags, limit)
	}

	filter := bson.M{
		"is_active": true,
		fieldPath:   bson.M{"$in": tags},
	}

	findOpts := options.Find().
		SetSort(bson.M{"usage_count": -1, "name": 1})

	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search by tag category: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skill.MigrateLegacyTags()
		skills = append(skills, &skill)
	}

	return skills, nil
}

// SearchByTags searches across all tag categories
func (r *SkillRepository) SearchByTags(ctx context.Context, tags []string, limit int) ([]*models.Skill, error) {
	if len(tags) == 0 {
		return []*models.Skill{}, nil
	}

	filter := bson.M{
		"is_active": true,
		"$or": []bson.M{
			{"tags": bson.M{"$in": tags}}, // Legacy support
			{"tagged_skill.primary_tags": bson.M{"$in": tags}},
			{"tagged_skill.secondary_tags": bson.M{"$in": tags}},
			{"tagged_skill.related_tags": bson.M{"$in": tags}},
		},
	}

	findOpts := options.Find().
		SetSort(bson.M{"usage_count": -1, "name": 1})

	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search by tags: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skill.MigrateLegacyTags()
		skills = append(skills, &skill)
	}

	return skills, nil
}

// GetSkillsByWeightedTags retrieves skills matching tags with weight consideration
func (r *SkillRepository) GetSkillsByWeightedTags(ctx context.Context, primaryTags, secondaryTags, relatedTags []string, limit int) ([]*models.Skill, error) {
	// Build a complex query that considers tag weights
	orConditions := []bson.M{}

	// Primary tags have highest priority
	if len(primaryTags) > 0 {
		orConditions = append(orConditions, bson.M{
			"tagged_skill.primary_tags": bson.M{"$in": primaryTags},
		})
	}

	// Secondary tags
	if len(secondaryTags) > 0 {
		orConditions = append(orConditions, bson.M{
			"tagged_skill.secondary_tags": bson.M{"$in": secondaryTags},
		})
	}

	// Related tags
	if len(relatedTags) > 0 {
		orConditions = append(orConditions, bson.M{
			"tagged_skill.related_tags": bson.M{"$in": relatedTags},
		})
	}

	if len(orConditions) == 0 {
		return []*models.Skill{}, nil
	}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"is_active": true,
				"$or":       orConditions,
			},
		},
		// Add a score based on tag matches
		{
			"$addFields": bson.M{
				"match_score": bson.M{
					"$add": []interface{}{
						// Primary tag matches worth 3 points each
						bson.M{
							"$multiply": []interface{}{
								3,
								bson.M{
									"$size": bson.M{
										"$setIntersection": []interface{}{
											"$tagged_skill.primary_tags",
											primaryTags,
										},
									},
								},
							},
						},
						// Secondary tag matches worth 2 points each
						bson.M{
							"$multiply": []interface{}{
								2,
								bson.M{
									"$size": bson.M{
										"$setIntersection": []interface{}{
											"$tagged_skill.secondary_tags",
											secondaryTags,
										},
									},
								},
							},
						},
						// Related tag matches worth 1 point each
						bson.M{
							"$size": bson.M{
								"$setIntersection": []interface{}{
									"$tagged_skill.related_tags",
									relatedTags,
								},
							},
						},
					},
				},
			},
		},
		// Sort by match score, then usage count
		{
			"$sort": bson.M{
				"match_score": -1,
				"usage_count": -1,
				"name":        1,
			},
		},
	}

	if limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": limit})
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get skills by weighted tags: %w", err)
	}
	defer cursor.Close(ctx)

	var skills []*models.Skill
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			return nil, fmt.Errorf("failed to decode skill: %w", err)
		}
		skill.MigrateLegacyTags()
		skills = append(skills, &skill)
	}

	return skills, nil
}

func (r *SkillRepository) MigrateAllLegacyTags(ctx context.Context) error {
	// Find all skills with legacy tags but no categorized tags
	filter := bson.M{
		"tags": bson.M{"$exists": true, "$ne": []string{}},
		"$or": []bson.M{
			{"tagged_skill": bson.M{"$exists": false}},
			{"tagged_skill.primary_tags": bson.M{"$size": 0}},
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to find skills for migration: %w", err)
	}
	defer cursor.Close(ctx)

	var migratedCount int
	for cursor.Next(ctx) {
		var skill models.Skill
		if err := cursor.Decode(&skill); err != nil {
			log.Printf("Failed to decode skill for migration: %v", err)
			continue
		}

		// Migrate the tags
		skill.MigrateLegacyTags()

		// Update the skill in database
		update := bson.M{
			"$set": bson.M{
				"tagged_skill": skill.TaggedSkill,
				"updated_at":   time.Now(),
			},
		}

		_, err = r.collection.UpdateOne(ctx, bson.M{"_id": skill.ID}, update)
		if err != nil {
			log.Printf("Failed to update skill %s during migration: %v", skill.ID.Hex(), err)
			continue
		}

		migratedCount++
	}

	log.Printf("Successfully migrated %d skills from legacy tags to categorized tags", migratedCount)
	return nil
}
