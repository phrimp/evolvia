package repository

import (
	"context"
	"log"
	"object-storage-service/internal/database/mongo"
	"object-storage-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodb "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type FileRepository struct {
	fileCollection    *mongodb.Collection
	versionCollection *mongodb.Collection
}

// NewFileRepository creates a new file repository
func NewFileRepository() *FileRepository {
	return &FileRepository{
		fileCollection:    mongo.GetCollection("files"),
		versionCollection: mongo.GetCollection("file_versions"),
	}
}

// Create saves a new file metadata
func (r *FileRepository) Create(ctx context.Context, file *models.File) (*models.File, error) {
	file.CreatedAt = time.Now()
	file.UpdatedAt = time.Now()
	file.VersionCount = 1

	result, err := r.fileCollection.InsertOne(ctx, file)
	if err != nil {
		log.Printf("Error creating file: %v", err)
		return nil, err
	}

	file.ID = result.InsertedID.(bson.ObjectID)
	return file, nil
}

// GetByID retrieves a file by ID
func (r *FileRepository) GetByID(ctx context.Context, id string) (*models.File, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var file models.File
	err = r.fileCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&file)
	if err != nil {
		if err == mongodb.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &file, nil
}

// Update updates a file's metadata
func (r *FileRepository) Update(ctx context.Context, file *models.File) error {
	file.UpdatedAt = time.Now()

	_, err := r.fileCollection.UpdateOne(
		ctx,
		bson.M{"_id": file.ID},
		bson.M{"$set": file},
	)
	return err
}

// Delete deletes a file by ID
func (r *FileRepository) Delete(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.fileCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	// Also delete all versions
	_, err = r.versionCollection.DeleteMany(ctx, bson.M{"fileId": objectID})
	return err
}

// List retrieves a paginated list of files
func (r *FileRepository) List(ctx context.Context, ownerID string, folderPath string, page, pageSize int) ([]*models.File, int64, error) {
	filter := bson.M{}
	if ownerID != "" {
		filter["ownerId"] = ownerID
	}
	if folderPath != "" {
		filter["folderPath"] = folderPath
	}

	// Count total documents
	count, err := r.fileCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination
	opts := options.Find()
	opts.SetSort(bson.M{"createdAt": -1})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	cursor, err := r.fileCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var files []*models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, 0, err
	}

	return files, count, nil
}

// AddVersion adds a new version for a file
func (r *FileRepository) AddVersion(ctx context.Context, version *models.FileVersion) error {
	version.CreatedAt = time.Now()

	_, err := r.versionCollection.InsertOne(ctx, version)
	if err != nil {
		return err
	}

	// Update the file's version count and current version
	_, err = r.fileCollection.UpdateOne(
		ctx,
		bson.M{"_id": version.FileID},
		bson.M{
			"$inc": bson.M{"versionCount": 1},
			"$set": bson.M{
				"currentVersion": version.ID.Hex(),
				"updatedAt":      time.Now(),
			},
		},
	)
	return err
}

// GetVersions retrieves all versions of a file
func (r *FileRepository) GetVersions(ctx context.Context, fileID string) ([]*models.FileVersion, error) {
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, err
	}

	opts := options.Find().SetSort(bson.M{"versionNumber": -1})
	cursor, err := r.versionCollection.Find(ctx, bson.M{"fileId": objectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var versions []*models.FileVersion
	if err = cursor.All(ctx, &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

// UpdateAccessTime updates the last accessed time of a file
func (r *FileRepository) UpdateAccessTime(ctx context.Context, fileID string) error {
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = r.fileCollection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"lastAccessedAt": now}},
	)
	return err
}

// Search searches for files by name, tags, or metadata
func (r *FileRepository) Search(ctx context.Context, ownerID, query string, page, pageSize int) ([]*models.File, int64, error) {
	filter := bson.M{}
	if ownerID != "" {
		filter["ownerId"] = ownerID
	}

	if query != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
			{"tags": query},
		}
	}

	// Count total documents
	count, err := r.fileCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination
	opts := options.Find()
	opts.SetSort(bson.M{"createdAt": -1})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	cursor, err := r.fileCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var files []*models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, 0, err
	}

	return files, count, nil
}

// UpdatePermissions updates the permissions for a file
func (r *FileRepository) UpdatePermissions(ctx context.Context, fileID string, permissions []models.Permission) error {
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return err
	}

	_, err = r.fileCollection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{
			"$set": bson.M{
				"permissions": permissions,
				"updatedAt":   time.Now(),
			},
		},
	)
	return err
}
