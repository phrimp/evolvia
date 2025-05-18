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
	"path/filepath"
	"time"
)

type AvatarService struct {
	avatarRepository *repository.AvatarRepository
	eventPublisher   events.Publisher
	config           *config.Config
}

// NewAvatarService creates a new avatar service
func NewAvatarService(repo *repository.AvatarRepository, eventPublisher events.Publisher, config *config.Config) *AvatarService {
	return &AvatarService{
		avatarRepository: repo,
		eventPublisher:   eventPublisher,
		config:           config,
	}
}

// UploadAvatar uploads an avatar
func (s *AvatarService) UploadAvatar(ctx context.Context, fileHeader *multipart.FileHeader, userID string, isDefault bool) (*models.Avatar, error) {
	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Read the file into a buffer to calculate checksum
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, file); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Calculate MD5 checksum
	hash := md5.New()
	if _, err := io.Copy(hash, bytes.NewReader(buffer.Bytes())); err != nil {
		return nil, fmt.Errorf("error calculating checksum: %w", err)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Generate unique object name using the checksum
	fileExt := filepath.Ext(fileHeader.Filename)
	objectName := fmt.Sprintf("%s/%s%s", userID, checksum, fileExt)

	// Upload to MinIO
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // Default to JPEG for avatars
	}

	uploadInfo, err := minio.UploadFile(
		ctx,
		s.config.MinIO.AvatarBucket,
		objectName,
		bytes.NewReader(buffer.Bytes()),
		contentType,
		fileHeader.Size,
	)
	if err != nil {
		return nil, fmt.Errorf("error uploading to MinIO: %w", err)
	}

	log.Printf("avatar uploaded successfully: %v", uploadInfo)

	// Create avatar metadata
	avatar := &models.Avatar{
		UserID:      userID,
		FileName:    fileHeader.Filename,
		Size:        fileHeader.Size,
		ContentType: contentType,
		StoragePath: objectName,
		BucketName:  s.config.MinIO.AvatarBucket,
		IsDefault:   isDefault,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save avatar metadata to MongoDB
	createdAvatar, err := s.avatarRepository.Create(ctx, avatar)
	if err != nil {
		// Try to delete the file from MinIO if MongoDB insert fails
		_ = minio.DeleteFile(ctx, s.config.MinIO.AvatarBucket, objectName)
		return nil, fmt.Errorf("error saving avatar metadata: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishAvatarUploaded(ctx, createdAvatar.ID.Hex(), userID)
		if err != nil {
			log.Printf("Error publishing avatar uploaded event: %v", err)
		}
	}

	return createdAvatar, nil
}

// GetAvatar retrieves an avatar by ID
func (s *AvatarService) GetAvatar(ctx context.Context, id string) (*models.Avatar, error) {
	avatar, err := s.avatarRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error retrieving avatar: %w", err)
	}
	if avatar == nil {
		return nil, errors.New("avatar not found")
	}

	return avatar, nil
}

// GetAvatarContent retrieves an avatar's content from MinIO
func (s *AvatarService) GetAvatarContent(ctx context.Context, id string) (io.ReadCloser, string, int64, error) {
	avatar, err := s.avatarRepository.GetByID(ctx, id)
	if err != nil {
		return nil, "", 0, fmt.Errorf("error retrieving avatar metadata: %w", err)
	}
	if avatar == nil {
		return nil, "", 0, errors.New("avatar not found")
	}

	// Get file from MinIO
	obj, err := minio.GetFile(ctx, avatar.BucketName, avatar.StoragePath)
	if err != nil {
		return nil, "", 0, fmt.Errorf("error retrieving avatar from storage: %w", err)
	}

	// Get object info
	stat, err := obj.Stat()
	if err != nil {
		return nil, "", 0, fmt.Errorf("error getting avatar stats: %w", err)
	}

	return obj, avatar.ContentType, stat.Size, nil
}

// GetUserAvatars retrieves all avatars for a user
func (s *AvatarService) GetUserAvatars(ctx context.Context, userID string) ([]*models.Avatar, error) {
	return s.avatarRepository.GetByUserID(ctx, userID)
}

// GetDefaultAvatar retrieves the default avatar for a user
func (s *AvatarService) GetDefaultAvatar(ctx context.Context, userID string) (*models.Avatar, error) {
	avatar, err := s.avatarRepository.GetDefaultAvatar(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving default avatar: %w", err)
	}
	if avatar == nil {
		// No default avatar found
		return nil, nil
	}

	return avatar, nil
}

// SetDefaultAvatar sets an avatar as the default
func (s *AvatarService) SetDefaultAvatar(ctx context.Context, id string) error {
	// Check if avatar exists
	avatar, err := s.avatarRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error retrieving avatar: %w", err)
	}
	if avatar == nil {
		return errors.New("avatar not found")
	}

	// Set as default
	err = s.avatarRepository.SetDefault(ctx, id)
	if err != nil {
		return fmt.Errorf("error setting default avatar: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishAvatarUpdated(ctx, id, avatar.UserID)
		if err != nil {
			log.Printf("Error publishing avatar updated event: %v", err)
		}
	}

	return nil
}

// DeleteAvatar deletes an avatar
func (s *AvatarService) DeleteAvatar(ctx context.Context, id string) error {
	// Check if avatar exists
	avatar, err := s.avatarRepository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error retrieving avatar: %w", err)
	}
	if avatar == nil {
		return errors.New("avatar not found")
	}

	// Delete avatar from MinIO
	err = minio.DeleteFile(ctx, avatar.BucketName, avatar.StoragePath)
	if err != nil {
		return fmt.Errorf("error deleting avatar from storage: %w", err)
	}

	// Delete metadata from MongoDB
	err = s.avatarRepository.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting avatar metadata: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		err = s.eventPublisher.PublishAvatarDeleted(ctx, id, avatar.UserID)
		if err != nil {
			log.Printf("Error publishing avatar deleted event: %v", err)
		}
	}

	return nil
}

// GetAvatarURL generates a presigned URL for avatar access
func (s *AvatarService) GetAvatarURL(ctx context.Context, id string, expiry int) (string, error) {
	avatar, err := s.avatarRepository.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("error retrieving avatar: %w", err)
	}
	if avatar == nil {
		return "", errors.New("avatar not found")
	}

	// Generate presigned URL
	url, err := minio.GetPresignedURL(ctx, avatar.BucketName, avatar.StoragePath, expiry)
	if err != nil {
		return "", fmt.Errorf("error generating presigned URL: %w", err)
	}

	return url, nil
}
