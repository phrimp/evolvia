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

type CategoryRepository struct {
	collection *mongo.Collection
}

func NewCategoryRepository(database *mongo.Database, collection string) *CategoryRepository {
	return &CategoryRepository{
		collection: database.Collection(collection),
	}
}

// InitializeIndexes creates MongoDB indexes for categories
func (r *CategoryRepository) InitializeIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "parent_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "parent_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "path", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "level", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "updated_at", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create category indexes: %w", err)
	}
	return nil
}

// Create inserts a new category
func (r *CategoryRepository) Create(ctx context.Context, category *models.SkillCategory) (*models.SkillCategory, error) {
	if category.ID.IsZero() {
		category.ID = bson.NewObjectID()
	}

	now := time.Now()
	category.CreatedAt = now
	category.UpdatedAt = now

	_, err := r.collection.InsertOne(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return category, nil
}

// GetByID retrieves a category by ID
func (r *CategoryRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.SkillCategory, error) {
	var category models.SkillCategory
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get category by ID: %w", err)
	}

	return &category, nil
}

// GetByName retrieves a category by name
func (r *CategoryRepository) GetByName(ctx context.Context, name string) (*models.SkillCategory, error) {
	var category models.SkillCategory
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get category by name: %w", err)
	}

	return &category, nil
}

// Update modifies an existing category
func (r *CategoryRepository) Update(ctx context.Context, id bson.ObjectID, category *models.SkillCategory) (*models.SkillCategory, error) {
	category.ID = id
	category.UpdatedAt = time.Now()

	update := bson.M{"$set": category}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("category not found")
	}

	return category, nil
}

// Delete removes a category (only if it has no children or associated skills)
func (r *CategoryRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	// Check if category has children
	childCount, err := r.collection.CountDocuments(ctx, bson.M{"parent_id": id})
	if err != nil {
		return fmt.Errorf("failed to check for child categories: %w", err)
	}

	if childCount > 0 {
		return fmt.Errorf("cannot delete category with child categories")
	}

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// GetAll retrieves all categories
func (r *CategoryRepository) GetAll(ctx context.Context) ([]*models.SkillCategory, error) {
	findOpts := options.Find().SetSort(bson.M{"level": 1, "name": 1})

	cursor, err := r.collection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find categories: %w", err)
	}
	defer cursor.Close(ctx)

	var categories []*models.SkillCategory
	for cursor.Next(ctx) {
		var category models.SkillCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// GetByParentID retrieves categories by parent ID
func (r *CategoryRepository) GetByParentID(ctx context.Context, parentID *bson.ObjectID) ([]*models.SkillCategory, error) {
	filter := bson.M{}
	if parentID != nil {
		filter["parent_id"] = *parentID
	} else {
		filter["parent_id"] = bson.M{"$exists": false}
	}

	findOpts := options.Find().SetSort(bson.M{"name": 1})

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find categories by parent: %w", err)
	}
	defer cursor.Close(ctx)

	var categories []*models.SkillCategory
	for cursor.Next(ctx) {
		var category models.SkillCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, nil
}

// GetRootCategories retrieves top-level categories (no parent)
func (r *CategoryRepository) GetRootCategories(ctx context.Context) ([]*models.SkillCategory, error) {
	return r.GetByParentID(ctx, nil)
}

// GetCategoryTree retrieves the full category hierarchy
func (r *CategoryRepository) GetCategoryTree(ctx context.Context) ([]*models.CategoryNode, error) {
	// Get all categories
	categories, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Build tree structure
	categoryMap := make(map[bson.ObjectID]*models.CategoryNode)
	var roots []*models.CategoryNode

	// First pass: create all nodes
	for _, cat := range categories {
		node := &models.CategoryNode{
			Category: cat,
			Children: []*models.CategoryNode{},
		}
		categoryMap[cat.ID] = node

		if cat.ParentID == nil {
			roots = append(roots, node)
		}
	}

	// Second pass: build parent-child relationships
	for _, cat := range categories {
		if cat.ParentID != nil {
			if parent, exists := categoryMap[*cat.ParentID]; exists {
				if child, exists := categoryMap[cat.ID]; exists {
					parent.Children = append(parent.Children, child)
				}
			}
		}
	}

	return roots, nil
}

// ExistsByName checks if a category with the given name exists
func (r *CategoryRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"name": name})
	if err != nil {
		return false, fmt.Errorf("failed to check category existence: %w", err)
	}
	return count > 0, nil
}

// ExistsByNameAndParent checks if a category with the given name and parent exists
func (r *CategoryRepository) ExistsByNameAndParent(ctx context.Context, name string, parentID *bson.ObjectID) (bool, error) {
	filter := bson.M{"name": name}
	if parentID != nil {
		filter["parent_id"] = *parentID
	} else {
		filter["parent_id"] = bson.M{"$exists": false}
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check category existence: %w", err)
	}
	return count > 0, nil
}

// GetCategoryPath builds the full path for a category
func (r *CategoryRepository) GetCategoryPath(ctx context.Context, categoryID bson.ObjectID) (string, error) {
	category, err := r.GetByID(ctx, categoryID)
	if err != nil {
		return "", err
	}
	if category == nil {
		return "", fmt.Errorf("category not found")
	}

	var pathParts []string
	current := category

	for current != nil {
		pathParts = append([]string{current.Name}, pathParts...)
		if current.ParentID == nil {
			break
		}

		current, err = r.GetByID(ctx, *current.ParentID)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("/%s", string(pathParts[0])), nil
}

// List retrieves categories with pagination and filtering
func (r *CategoryRepository) List(ctx context.Context, opts CategoryListOptions) ([]*models.SkillCategory, int64, error) {
	filter := r.buildFilter(opts)

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count categories: %w", err)
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
	} else {
		findOpts.SetSort(bson.M{"level": 1, "name": 1})
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find categories: %w", err)
	}
	defer cursor.Close(ctx)

	var categories []*models.SkillCategory
	for cursor.Next(ctx) {
		var category models.SkillCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, 0, fmt.Errorf("failed to decode category: %w", err)
		}
		categories = append(categories, &category)
	}

	return categories, total, nil
}

// buildFilter constructs MongoDB filter from CategoryListOptions
func (r *CategoryRepository) buildFilter(opts CategoryListOptions) bson.M {
	filter := bson.M{}

	if opts.ParentID != nil {
		filter["parent_id"] = *opts.ParentID
	}

	if opts.Level >= 0 {
		filter["level"] = opts.Level
	}

	if opts.NamePattern != "" {
		filter["name"] = bson.M{"$regex": opts.NamePattern, "$options": "i"}
	}

	return filter
}

// InitializeData loads category data from /data/categories/* directory
func (r *CategoryRepository) InitializeData(ctx context.Context, dataDir string) error {
	categoriesDir := filepath.Join(dataDir, "categories")

	// Check if directory exists
	if _, err := os.Stat(categoriesDir); os.IsNotExist(err) {
		log.Printf("Categories directory not found: %s, skipping category initialization", categoriesDir)
		return nil // Don't fail if categories directory doesn't exist
	}

	var categoriesLoaded int
	err := filepath.WalkDir(categoriesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		log.Println("Found category file:", d.Name(), "Path:", path)

		// Process only JSON files
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		// Read and parse category file
		categories, err := r.loadCategoriesFromFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to load categories from %s: %v\n", path, err)
			return nil // Continue processing other files
		}

		// Insert categories
		for _, category := range categories {
			// Check if category already exists
			exists, err := r.ExistsByNameAndParent(ctx, category.Name, category.ParentID)
			if err != nil {
				return fmt.Errorf("failed to check if category exists: %w", err)
			}

			if exists {
				fmt.Printf("Category '%s' already exists, skipping...\n", category.Name)
				continue
			}

			// Insert category
			_, err = r.Create(ctx, category)
			if err != nil {
				fmt.Printf("Warning: Failed to insert category '%s': %v\n", category.Name, err)
				continue
			}

			categoriesLoaded++
			fmt.Printf("Loaded category: %s\n", category.Name)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk categories directory: %w", err)
	}

	fmt.Printf("Successfully loaded %d categories from %s\n", categoriesLoaded, categoriesDir)
	return nil
}

// loadCategoriesFromFile reads and parses a category JSON file
func (r *CategoryRepository) loadCategoriesFromFile(filePath string) ([]*models.SkillCategory, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var categories []*models.SkillCategory
	if err := json.Unmarshal(data, &categories); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Set default values
	now := time.Now()
	for _, category := range categories {
		category.CreatedAt = now
		category.UpdatedAt = now
	}

	return categories, nil
}

// CategoryListOptions defines options for listing categories
type CategoryListOptions struct {
	Limit       int
	Offset      int
	SortBy      string
	SortDesc    bool
	ParentID    *bson.ObjectID
	Level       int
	NamePattern string
}
