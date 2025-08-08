package services

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type CategoryService struct {
	categoryRepo *repository.CategoryRepository
	skillRepo    *repository.SkillRepository
}

// NewCategoryService creates a new category service
func NewCategoryService(categoryRepo *repository.CategoryRepository, skillRepo *repository.SkillRepository, dataDir string) (*CategoryService, error) {
	service := &CategoryService{
		categoryRepo: categoryRepo,
		skillRepo:    skillRepo,
	}

	// Initialize the service
	if err := service.initialize(context.Background(), dataDir); err != nil {
		return nil, fmt.Errorf("failed to initialize category service: %w", err)
	}

	return service, nil
}

// initialize sets up indexes and loads initial data
func (s *CategoryService) initialize(ctx context.Context, dataDir string) error {
	log.Println("Initializing Category Service...")

	// Create database indexes
	log.Println("Creating category database indexes...")
	if err := s.categoryRepo.InitializeIndexes(ctx); err != nil {
		return fmt.Errorf("failed to initialize indexes: %w", err)
	}
	log.Println("Category database indexes created successfully")

	// Load initial data from /data/categories/*
	log.Println("Loading category data from directory:", dataDir)
	if err := s.categoryRepo.InitializeData(ctx, dataDir); err != nil {
		return fmt.Errorf("failed to initialize category data: %w", err)
	}
	log.Println("Category data loaded successfully")

	return nil
}

// CreateCategory creates a new category
func (s *CategoryService) CreateCategory(ctx context.Context, category *models.SkillCategory) (*models.SkillCategory, error) {
	// Validate category
	if err := s.validateCategory(category); err != nil {
		return nil, fmt.Errorf("category validation failed: %w", err)
	}

	// Check if category already exists with same name and parent
	exists, err := s.categoryRepo.ExistsByNameAndParent(ctx, category.Name, category.ParentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check category existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("category with name '%s' already exists under this parent", category.Name)
	}

	// If has parent, validate parent exists and calculate level
	if category.ParentID != nil {
		parent, err := s.categoryRepo.GetByID(ctx, *category.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent category: %w", err)
		}
		if parent == nil {
			return nil, fmt.Errorf("parent category not found")
		}
		category.Level = parent.Level + 1
	} else {
		category.Level = 0
	}

	// Calculate path
	path, err := s.calculateCategoryPath(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate category path: %w", err)
	}
	category.Path = path

	return s.categoryRepo.Create(ctx, category)
}

// GetCategoryByID retrieves a category by ID
func (s *CategoryService) GetCategoryByID(ctx context.Context, id bson.ObjectID) (*models.SkillCategory, error) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, fmt.Errorf("category not found")
	}

	return category, nil
}

// GetCategoryByName retrieves a category by name
func (s *CategoryService) GetCategoryByName(ctx context.Context, name string) (*models.SkillCategory, error) {
	category, err := s.categoryRepo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, fmt.Errorf("category not found")
	}

	return category, nil
}

// UpdateCategory updates an existing category
func (s *CategoryService) UpdateCategory(ctx context.Context, id bson.ObjectID, category *models.SkillCategory) (*models.SkillCategory, error) {
	// Check if category exists
	existing, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("category not found")
	}

	// Validate updated category
	if err := s.validateCategory(category); err != nil {
		return nil, fmt.Errorf("category validation failed: %w", err)
	}

	// Prevent circular references
	if category.ParentID != nil && *category.ParentID == id {
		return nil, fmt.Errorf("category cannot be its own parent")
	}

	// Check if name is being changed and if new name already exists under same parent
	if category.Name != existing.Name ||
		(category.ParentID != existing.ParentID) {
		exists, err := s.categoryRepo.ExistsByNameAndParent(ctx, category.Name, category.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to check category existence: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("category with name '%s' already exists under this parent", category.Name)
		}
	}

	// Recalculate level and path if parent changed
	if category.ParentID != existing.ParentID {
		if category.ParentID != nil {
			parent, err := s.categoryRepo.GetByID(ctx, *category.ParentID)
			if err != nil {
				return nil, fmt.Errorf("failed to get parent category: %w", err)
			}
			if parent == nil {
				return nil, fmt.Errorf("parent category not found")
			}
			category.Level = parent.Level + 1
		} else {
			category.Level = 0
		}

		path, err := s.calculateCategoryPath(ctx, category)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate category path: %w", err)
		}
		category.Path = path
	}

	return s.categoryRepo.Update(ctx, id, category)
}

// DeleteCategory removes a category
func (s *CategoryService) DeleteCategory(ctx context.Context, id bson.ObjectID) error {
	// Check if category exists
	existing, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("category not found")
	}

	// Check if category has associated skills
	skills, err := s.skillRepo.GetByCategory(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check associated skills: %w", err)
	}
	if len(skills) > 0 {
		return fmt.Errorf("cannot delete category with associated skills")
	}

	return s.categoryRepo.Delete(ctx, id)
}

// ListCategories retrieves categories with filtering and pagination
func (s *CategoryService) ListCategories(ctx context.Context, opts repository.CategoryListOptions) ([]*models.SkillCategory, int64, error) {
	return s.categoryRepo.List(ctx, opts)
}

// GetAllCategories retrieves all categories
func (s *CategoryService) GetAllCategories(ctx context.Context) ([]*models.SkillCategory, error) {
	return s.categoryRepo.GetAll(ctx)
}

// GetCategoriesByParent retrieves categories by parent ID
func (s *CategoryService) GetCategoriesByParent(ctx context.Context, parentID *bson.ObjectID) ([]*models.SkillCategory, error) {
	return s.categoryRepo.GetByParentID(ctx, parentID)
}

// GetRootCategories retrieves top-level categories
func (s *CategoryService) GetRootCategories(ctx context.Context) ([]*models.SkillCategory, error) {
	return s.categoryRepo.GetRootCategories(ctx)
}

// GetCategoryTree retrieves the full category hierarchy
func (s *CategoryService) GetCategoryTree(ctx context.Context) ([]*models.CategoryNode, error) {
	return s.categoryRepo.GetCategoryTree(ctx)
}

// GetCategoryStatistics returns various statistics about categories
func (s *CategoryService) GetCategoryStatistics(ctx context.Context) (*models.CategoryStatistics, error) {
	// Get all categories
	categories, err := s.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	// Get root categories
	rootCategories, err := s.categoryRepo.GetRootCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get root categories: %w", err)
	}

	// Calculate statistics
	stats := &models.CategoryStatistics{
		TotalCategories: len(categories),
		RootCategories:  len(rootCategories),
		ByLevel:         make(map[int]int),
		TopCategories:   []*models.CategoryWithSkillCount{},
	}

	maxDepth := 0
	for _, cat := range categories {
		stats.ByLevel[cat.Level]++
		if cat.Level > maxDepth {
			maxDepth = cat.Level
		}
	}
	stats.MaxDepth = maxDepth

	// Get top categories with skill counts (top 10)
	for i, cat := range categories {
		if i >= 10 {
			break
		}

		skills, err := s.skillRepo.GetByCategory(ctx, cat.ID)
		if err != nil {
			log.Printf("Failed to get skills for category %s: %v", cat.ID.Hex(), err)
			continue
		}

		categoryWithCount := &models.CategoryWithSkillCount{
			SkillCategory: cat,
			SkillCount:    len(skills),
		}
		stats.TopCategories = append(stats.TopCategories, categoryWithCount)
	}

	return stats, nil
}

// validateCategory performs validation on category data
func (s *CategoryService) validateCategory(category *models.SkillCategory) error {
	if category == nil {
		return fmt.Errorf("category cannot be nil")
	}

	if category.Name == "" {
		return fmt.Errorf("category name is required")
	}

	if len(category.Name) > 100 {
		return fmt.Errorf("category name cannot exceed 100 characters")
	}

	// Validate name doesn't contain path separators
	if strings.Contains(category.Name, "/") {
		return fmt.Errorf("category name cannot contain '/' character")
	}

	if category.Level < 0 {
		return fmt.Errorf("category level cannot be negative")
	}

	if category.Level > 10 {
		return fmt.Errorf("category level cannot exceed 10 (maximum depth)")
	}

	return nil
}

// calculateCategoryPath builds the full path for a category
func (s *CategoryService) calculateCategoryPath(ctx context.Context, category *models.SkillCategory) (string, error) {
	var pathParts []string

	// Add current category name
	pathParts = append(pathParts, category.Name)

	// Walk up the parent chain
	currentParentID := category.ParentID
	for currentParentID != nil {
		parent, err := s.categoryRepo.GetByID(ctx, *currentParentID)
		if err != nil {
			return "", fmt.Errorf("failed to get parent category: %w", err)
		}
		if parent == nil {
			return "", fmt.Errorf("parent category not found")
		}

		pathParts = append([]string{parent.Name}, pathParts...)
		currentParentID = parent.ParentID
	}

	return "/" + strings.Join(pathParts, "/"), nil
}

// MoveCategory moves a category to a new parent
func (s *CategoryService) MoveCategory(ctx context.Context, categoryID bson.ObjectID, newParentID *bson.ObjectID) error {
	// Get the category to move
	category, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if category == nil {
		return fmt.Errorf("category not found")
	}

	// Validate new parent
	if newParentID != nil {
		// Prevent circular references
		if *newParentID == categoryID {
			return fmt.Errorf("category cannot be its own parent")
		}

		// Check if new parent exists
		newParent, err := s.categoryRepo.GetByID(ctx, *newParentID)
		if err != nil {
			return fmt.Errorf("failed to get new parent category: %w", err)
		}
		if newParent == nil {
			return fmt.Errorf("new parent category not found")
		}

		// Check for circular reference in the hierarchy
		if err := s.checkCircularReference(ctx, categoryID, *newParentID); err != nil {
			return err
		}
	}

	// Update the category
	category.ParentID = newParentID
	_, err = s.UpdateCategory(ctx, categoryID, category)
	return err
}

// BatchCreateCategories creates multiple categories at once
func (s *CategoryService) BatchCreateCategories(ctx context.Context, categories []*models.SkillCategory) error {
	if len(categories) == 0 {
		return fmt.Errorf("no categories provided")
	}

	if len(categories) > 100 {
		return fmt.Errorf("too many categories (maximum 100 allowed)")
	}

	// Validate all categories first
	for i, category := range categories {
		if err := s.validateCategory(category); err != nil {
			return fmt.Errorf("category validation failed at index %d: %w", i, err)
		}
	}

	// Process categories in dependency order (parents before children)
	processedCategories, err := s.processCategoriesInOrder(ctx, categories)
	if err != nil {
		return fmt.Errorf("failed to process categories in order: %w", err)
	}

	// Check for duplicates within the batch
	nameMap := make(map[string]bool)
	parentMap := make(map[string]*bson.ObjectID)

	for i, category := range processedCategories {
		// Create unique key for name+parent combination
		parentKey := "nil"
		if category.ParentID != nil {
			parentKey = category.ParentID.Hex()
		}
		uniqueKey := fmt.Sprintf("%s_%s", category.Name, parentKey)

		log.Println("$$$$$$$$$$$$$$$$", uniqueKey)

		if nameMap[uniqueKey] {
			return fmt.Errorf("duplicate category name '%s' under same parent at index %d", category.Name, i)
		}
		nameMap[uniqueKey] = true
		parentMap[category.Name] = category.ParentID

		// Check against existing categories
		exists, err := s.categoryRepo.ExistsByNameAndParent(ctx, category.Name, category.ParentID)
		if err != nil {
			return fmt.Errorf("failed to check category existence: %w", err)
		}
		if exists {
			return fmt.Errorf("category with name '%s' already exists under this parent", category.Name)
		}
	}

	return s.categoryRepo.BatchCreate(ctx, processedCategories)
}

// processCategoriesInOrder sorts categories to ensure parents are created before children
func (s *CategoryService) processCategoriesInOrder(ctx context.Context, categories []*models.SkillCategory) ([]*models.SkillCategory, error) {
	// Create maps for processing
	categoryByName := make(map[string]*models.SkillCategory)
	processed := make([]*models.SkillCategory, 0, len(categories))
	processing := make(map[string]bool)

	for _, category := range categories {
		categoryByName[category.Name] = category
	}

	processCategory := func(name string) error {
		if processing[name] {
			return fmt.Errorf("circular dependency detected for category: %s", name)
		}

		category, exists := categoryByName[name]
		if !exists {
			return nil // Category not in this batch, assume it exists or will be handled separately
		}

		if category.Level == -1 { // Mark as already processed
			return nil
		}

		processing[name] = true
		defer func() { processing[name] = false }()

		// Calculate level and path
		if category.ParentID == nil {
			// Root category
			category.Level = 0
			category.Path = "/" + category.Name
		} else {
			// Find parent to calculate level and path
			parent, err := s.categoryRepo.GetByID(ctx, *category.ParentID)
			if err != nil {
				return fmt.Errorf("failed to get parent category: %w", err)
			}

			if parent != nil {
				// Parent exists in database
				category.Level = parent.Level + 1
				category.Path = parent.Path + "/" + category.Name
			} else {
				// Parent might be in this batch - we'll handle this in a second pass
				category.Level = -1 // Mark as unprocessed
				return nil
			}
		}

		processed = append(processed, category)
		category.Level = -1 // Mark as processed
		return nil
	}

	// First pass: process categories with existing parents or no parents
	for _, category := range categories {
		if err := processCategory(category.Name); err != nil {
			return nil, err
		}
	}

	// Second pass: handle categories whose parents are in this batch
	maxIterations := len(categories)
	for iteration := 0; iteration < maxIterations; iteration++ {
		progressMade := false

		for _, category := range categories {
			if category.Level != -1 { // Already processed
				continue
			}

			if category.ParentID == nil {
				// Root category
				category.Level = 0
				category.Path = "/" + category.Name
				processed = append(processed, category)
				category.Level = -1 // Mark as processed
				progressMade = true
				continue
			}

			// Find parent in processed list
			var parentCategory *models.SkillCategory
			for _, processedCat := range processed {
				if processedCat.ID == *category.ParentID {
					parentCategory = processedCat
					break
				}
			}

			// If parent not found in processed list, try to find by name in batch
			if parentCategory == nil {
				for _, batchCat := range categories {
					if batchCat.ID == *category.ParentID ||
						(category.ParentID != nil && batchCat.Name != "" && category.ParentID.IsZero()) {
						// Handle case where ParentID is referenced by name in batch
						parentCategory = batchCat
						break
					}
				}
			}

			if parentCategory != nil && parentCategory.Level >= 0 {
				category.Level = parentCategory.Level + 1
				category.Path = parentCategory.Path + "/" + category.Name
				processed = append(processed, category)
				category.Level = -1 // Mark as processed
				progressMade = true
			}
		}

		if !progressMade {
			// Check for unprocessed categories
			var unprocessed []string
			for _, category := range categories {
				if category.Level != -1 {
					unprocessed = append(unprocessed, category.Name)
				}
			}
			if len(unprocessed) > 0 {
				return nil, fmt.Errorf("cannot resolve parent dependencies for categories: %v", unprocessed)
			}
			break
		}
	}

	// Reset levels to proper values
	for _, category := range processed {
		if category.Level == -1 {
			// Recalculate level based on path
			parts := strings.Split(strings.Trim(category.Path, "/"), "/")
			category.Level = len(parts) - 1
		}
	}

	return processed, nil
}

// checkCircularReference checks if moving a category would create a circular reference
func (s *CategoryService) checkCircularReference(ctx context.Context, categoryID, newParentID bson.ObjectID) error {
	// Walk up the new parent chain to ensure categoryID is not an ancestor
	currentID := newParentID
	for {
		if currentID == categoryID {
			return fmt.Errorf("circular reference detected: category cannot be moved under its own descendant")
		}

		parent, err := s.categoryRepo.GetByID(ctx, currentID)
		if err != nil {
			return fmt.Errorf("failed to check parent category: %w", err)
		}
		if parent == nil || parent.ParentID == nil {
			break
		}

		currentID = *parent.ParentID
	}

	return nil
}
