package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"object-storage-service/internal/config"
	"object-storage-service/internal/database/minio"
	"object-storage-service/internal/events"
	"object-storage-service/internal/models"
	"object-storage-service/internal/repository"
	"object-storage-service/pkg/utils"
	"path/filepath"
	"strings"
	"time"

	miniogh "github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type FileService struct {
	fileRepository *repository.FileRepository
	eventPublisher events.Publisher
	config         *config.Config
}

// NewFileService creates a new file service
func NewFileService(repo *repository.FileRepository, eventPublisher events.Publisher, config *config.Config) *FileService {
	return &FileService{
		fileRepository: repo,
		eventPublisher: eventPublisher,
		config:         config,
	}
}

// UploadFile uploads a file to MinIO and saves its metadata
func (s *FileService) UploadFile(ctx context.Context, fileHeader *multipart.FileHeader, ownerID, description, folderPath string, isPublic bool, tags []string, metadata map[string]string) (*models.File, error) {
	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a MD5 hash calculator
	hash := md5.New()

	// Create a reader that will calculate the hash while streaming
	hashingReader := utils.CreateHashingReader(file, hash)

	// Generate unique object name (we'll update with the checksum after the upload)
	tempObjectName := fmt.Sprintf("%s/%d-%s", ownerID, time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
	fileExt := filepath.Ext(fileHeader.Filename)

	// Get content type
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Stream directly to MinIO without buffering
	uploadInfo, err := minio.UploadFileStream(
		ctx,
		s.config.MinIO.FileBucket,
		tempObjectName,
		hashingReader,
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		return nil, fmt.Errorf("error uploading to MinIO: %w", err)
	}
	log.Printf("File uploaded successfully: %v", uploadInfo)

	// Get the checksum after the upload is complete
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Create the final object name with the checksum
	objectName := fmt.Sprintf("%s/%s%s", ownerID, checksum, fileExt)

	// If the temporary name is different from the final name with checksum,
	// we need to copy the object with the new name and delete the old one
	if tempObjectName != objectName {
		// Copy object to the new name (with checksum)
		srcOpts := miniogh.CopySrcOptions{
			Bucket: s.config.MinIO.FileBucket,
			Object: tempObjectName,
		}

		dstOpts := miniogh.CopyDestOptions{
			Bucket: s.config.MinIO.FileBucket,
			Object: objectName,
		}

		// Copy the object with the new name
		_, err = minio.MinioClient.CopyObject(ctx, dstOpts, srcOpts)
		if err != nil {
			return nil, fmt.Errorf("error copying file with checksum name: %w", err)
		}

		// Delete the temporary object
		err = minio.DeleteFile(ctx, s.config.MinIO.FileBucket, tempObjectName)
		if err != nil {
			log.Printf("Warning: Failed to delete temporary file %s: %v", tempObjectName, err)
		}
	}

	// Create file metadata
	file_metadata := &models.File{
		OwnerID:      ownerID,
		Name:         fileHeader.Filename,
		Description:  description,
		Size:         fileHeader.Size,
		ContentType:  contentType,
		StoragePath:  objectName,
		BucketName:   s.config.MinIO.FileBucket,
		IsPublic:     isPublic,
		Checksum:     checksum,
		VersionCount: 1,
		FolderPath:   folderPath,
		Tags:         tags,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Rest of the method remains the same...
	createdFile, err := s.fileRepository.Create(ctx, file_metadata)
	if err != nil {
		return nil, fmt.Errorf("error creating file metadata: %s", err)
	}
	log.Printf("File Metadata created successfully: %v", createdFile)

	return createdFile, nil
}

// GetFile retrieves a file by ID
func (s *FileService) GetFile(ctx context.Context, id string) (*models.File, error) {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file not found")
	}

	// Update last accessed time
	_ = s.fileRepository.UpdateAccessTime(ctx, id)

	// Publish file accessed event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishFileAccessed(ctx, file.ID.Hex(), file.OwnerID)
		if err != nil {
			log.Printf("Error publishing file accessed event: %v", err)
		}
	}

	return file, nil
}

// GetFileContent retrieves a file's content from MinIO
func (s *FileService) GetFileContent(ctx context.Context, id string) (io.ReadCloser, string, int64, error) {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return nil, "", 0, fmt.Errorf("error retrieving file metadata: %w", err)
	}
	if file == nil {
		return nil, "", 0, errors.New("file not found")
	}

	// Get file from MinIO
	obj, err := minio.GetFile(ctx, file.BucketName, file.StoragePath)
	if err != nil {
		return nil, "", 0, fmt.Errorf("error retrieving file from storage: %w", err)
	}

	// Get object info
	stat, err := obj.Stat()
	if err != nil {
		return nil, "", 0, fmt.Errorf("error getting file stats: %w", err)
	}

	// Update last accessed time
	_ = s.fileRepository.UpdateAccessTime(ctx, id)

	return obj, file.ContentType, stat.Size, nil
}

// UpdateFile updates a file's metadata
func (s *FileService) UpdateFile(ctx context.Context, id string, update *models.FileUpdateRequest) (*models.File, error) {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file not found")
	}

	// Update fields if provided
	if update.Description != "" {
		file.Description = update.Description
	}
	if update.FolderPath != "" {
		file.FolderPath = update.FolderPath
	}
	if update.IsPublic != nil {
		file.IsPublic = *update.IsPublic
	}
	if update.Tags != nil {
		file.Tags = update.Tags
	}
	if update.Metadata != nil {
		file.Metadata = update.Metadata
	}

	file.UpdatedAt = time.Now()

	// Save updates
	err = s.fileRepository.Update(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("error updating file: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishFileUpdated(ctx, file.ID.Hex(), file.OwnerID)
		if err != nil {
			log.Printf("Error publishing file updated event: %v", err)
		}
	}

	return file, nil
}

// DeleteFile deletes a file and its metadata
func (s *FileService) DeleteFile(ctx context.Context, id string) error {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return errors.New("file not found")
	}

	// Delete file from MinIO
	err = minio.DeleteFile(ctx, file.BucketName, file.StoragePath)
	if err != nil {
		return fmt.Errorf("error deleting file from storage: %w", err)
	}

	// Delete versions from MinIO
	versions, err := s.fileRepository.GetVersions(ctx, id)
	if err != nil {
		log.Printf("Error retrieving file versions: %v", err)
	} else {
		for _, version := range versions {
			// Skip the current version which was already deleted
			if version.StoragePath != file.StoragePath {
				err = minio.DeleteFile(ctx, file.BucketName, version.StoragePath)
				if err != nil {
					log.Printf("Error deleting version from storage: %v", err)
				}
			}
		}
	}

	// Delete metadata from MongoDB
	err = s.fileRepository.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting file metadata: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishFileDeleted(ctx, id, file.OwnerID)
		if err != nil {
			log.Printf("Error publishing file deleted event: %v", err)
		}
	}

	return nil
}

// ListFiles lists all files for an owner
func (s *FileService) ListFiles(ctx context.Context, ownerID string, folderPath string, page, pageSize int) ([]*models.File, int64, error) {
	return s.fileRepository.List(ctx, ownerID, folderPath, page, pageSize)
}

// GetFileURL generates a presigned URL for file access
func (s *FileService) GetFileURL(ctx context.Context, id string, expiry int) (string, error) {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return "", errors.New("file not found")
	}

	// Generate presigned URL
	url, err := minio.GetPresignedURL(ctx, file.BucketName, file.StoragePath, expiry)
	if err != nil {
		return "", fmt.Errorf("error generating presigned URL: %w", err)
	}

	return url, nil
}

// UpdatePermissions updates a file's permissions
func (s *FileService) UpdatePermissions(ctx context.Context, id string, permissions []models.Permission, grantedBy string) error {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return errors.New("file not found")
	}

	// Set granted by and time
	now := time.Now()
	for i := range permissions {
		permissions[i].GrantedBy = grantedBy
		permissions[i].GrantedAt = now
	}

	return s.fileRepository.UpdatePermissions(ctx, id, permissions)
}

// NewVersion uploads a new version of a file
func (s *FileService) NewVersion(ctx context.Context, id string, fileHeader *multipart.FileHeader, userID string) (*models.FileVersion, error) {
	file, err := s.fileRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file not found")
	}

	// Open the uploaded file
	uploadedFile, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer uploadedFile.Close()

	// Read the file into a buffer to calculate checksum
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, uploadedFile); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Calculate MD5 checksum
	hash := md5.New()
	if _, err := io.Copy(hash, bytes.NewReader(buffer.Bytes())); err != nil {
		return nil, fmt.Errorf("error calculating checksum: %w", err)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Generate unique object name using the checksum
	fileExt := filepath.Ext(file.Name)
	objectName := fmt.Sprintf("%s/%s_v%d%s", file.OwnerID, strings.TrimSuffix(file.Name, fileExt), file.VersionCount+1, fileExt)

	// Upload to MinIO
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = file.ContentType
	}

	_, err = minio.UploadFile(
		ctx,
		file.BucketName,
		objectName,
		bytes.NewReader(buffer.Bytes()),
		contentType,
		fileHeader.Size,
	)
	if err != nil {
		return nil, fmt.Errorf("error uploading to MinIO: %w", err)
	}

	// Create version
	version := &models.FileVersion{
		ID:            bson.NewObjectID(),
		FileID:        file.ID,
		VersionNumber: file.VersionCount + 1,
		Size:          fileHeader.Size,
		StoragePath:   objectName,
		Checksum:      checksum,
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	// Save version
	err = s.fileRepository.AddVersion(ctx, version)
	if err != nil {
		// Try to delete the file from MinIO if MongoDB insert fails
		_ = minio.DeleteFile(ctx, file.BucketName, objectName)
		return nil, fmt.Errorf("error saving file version: %w", err)
	}

	// Update file size, content type and path to point to the new version
	file.Size = fileHeader.Size
	file.ContentType = contentType
	file.StoragePath = objectName
	file.Checksum = checksum
	file.UpdatedAt = time.Now()
	file.CurrentVersion = version.ID.Hex()

	err = s.fileRepository.Update(ctx, file)
	if err != nil {
		log.Printf("Error updating file with new version details: %v", err)
	}

	return version, nil
}

// GetVersions retrieves all versions of a file
func (s *FileService) GetVersions(ctx context.Context, id string) ([]*models.FileVersion, error) {
	return s.fileRepository.GetVersions(ctx, id)
}
