package service

import (
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
)

type AvatarService struct {
	avatarRepository *repository.AvatarRepository
	redisRepository  *repository.RedisRepo
	eventPublisher   events.Publisher
	config           *config.Config
}

// NewAvatarService creates a new avatar service
func NewAvatarService(repo *repository.AvatarRepository, eventPublisher events.Publisher, config *config.Config, redisRepo *repository.RedisRepo) *AvatarService {
	return &AvatarService{
		avatarRepository: repo,
		redisRepository:  redisRepo,
		eventPublisher:   eventPublisher,
		config:           config,
	}
}

func (s *AvatarService) UploadAvatar(ctx context.Context, fileHeader *multipart.FileHeader, userID string, isDefault bool) (*models.Avatar, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a MD5 hash calculator
	hash := md5.New()

	// Create a reader that will calculate the hash while streaming
	hashingReader := utils.CreateHashingReader(file, hash)

	// Generate temporary object name
	tempObjectName := fmt.Sprintf("%s/%d-%s", userID, time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
	fileExt := filepath.Ext(fileHeader.Filename)

	// Get content type
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // Default to JPEG for avatars
	}

	uploadInfo, err := minio.UploadFileStream(
		ctx,
		s.config.MinIO.AvatarBucket,
		tempObjectName,
		hashingReader,
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		return nil, fmt.Errorf("error uploading to MinIO: %w", err)
	}
	log.Printf("Avatar uploaded successfully: %v", uploadInfo)

	// Get the checksum after the upload is complete
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Create the final object name with the checksum
	objectName := fmt.Sprintf("%s/%s%s", userID, checksum, fileExt)

	// If the temporary name is different from the final name with checksum,
	// we need to copy the object with the new name and delete the old one
	if tempObjectName != objectName {
		// Copy object to the new name (with checksum)
		srcOpts := miniogh.CopySrcOptions{
			Bucket: s.config.MinIO.AvatarBucket,
			Object: tempObjectName,
		}

		dstOpts := miniogh.CopyDestOptions{
			Bucket: s.config.MinIO.AvatarBucket,
			Object: objectName,
		}

		// Copy the object with the new name
		_, err = minio.MinioClient.CopyObject(ctx, dstOpts, srcOpts)
		if err != nil {
			return nil, fmt.Errorf("error copying file with checksum name: %w", err)
		}

		// Delete the temporary object
		err = minio.DeleteFile(ctx, s.config.MinIO.AvatarBucket, tempObjectName)
		if err != nil {
			log.Printf("Warning: Failed to delete temporary file %s: %v", tempObjectName, err)
		}
	}

	// Create avatar metadata
	avatar := &models.Avatar{
		UserID:      userID,
		FileName:    fileHeader.Filename,
		Size:        fileHeader.Size,
		ContentType: contentType,
		StoragePath: objectName,
		BucketName:  s.config.MinIO.AvatarBucket,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	createdAvatar, err := s.avatarRepository.Create(ctx, avatar)
	if err != nil {
		return nil, fmt.Errorf("error creating avatar: %s", err)
	}
	log.Printf("Avatar added successfully: %v", createdAvatar)

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

func (s *AvatarService) GetDefaultAvatar(ctx context.Context, userID string) (*models.Avatar, error) {
	avatars, err := s.avatarRepository.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving user avatars: %w", err)
	}

	if len(avatars) > 0 {
		return avatars[0], nil
	}

	return s.getSystemDefaultAvatar(ctx)
}

func (s *AvatarService) getSystemDefaultAvatar(ctx context.Context) (*models.Avatar, error) {
	avatars, err := s.avatarRepository.GetByUserID(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error retrieving system avatars: %w", err)
	}

	if len(avatars) > 0 {
		for _, avatar := range avatars {
			if strings.Contains(avatar.FileName, "default_avatar") {
				return avatar, nil
			}
		}
		return avatars[0], nil // Return first system avatar
	}

	return nil, errors.New("no default avatar available")
}

// SetDefaultAvatar sets an avatar as the default
//func (s *AvatarService) SetDefaultAvatar(ctx context.Context, id string) error {
//	// Check if avatar exists
//	avatar, err := s.avatarRepository.GetByID(ctx, id)
//	if err != nil {
//		return fmt.Errorf("error retrieving avatar: %w", err)
//	}
//	if avatar == nil {
//		return errors.New("avatar not found")
//	}
//
//	// Set as default
//	err = s.avatarRepository.SetDefault(ctx, id)
//	if err != nil {
//		return fmt.Errorf("error setting default avatar: %w", err)
//	}
//
//	// Publish event
//	if s.eventPublisher != nil {
//		err = s.eventPublisher.PublishAvatarUpdated(ctx, id, avatar.UserID)
//		if err != nil {
//			log.Printf("Error publishing avatar updated event: %v", err)
//		}
//	}
//
//	return nil
//}

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

func (s *AvatarService) GetAvatarURLSystem(ctx context.Context, avatar *models.Avatar, user_id string, expiry int) (string, error) {
	url, err := minio.GetPresignedURL(ctx, avatar.BucketName, avatar.StoragePath, expiry)
	if err != nil {
		return "", fmt.Errorf("error generating presigned URL: %w", err)
	}
	s.redisRepository.SaveStructCached(ctx, user_id, "avatar-cached:", url, 24)

	return url, nil
}

// GetAvatarURL generates a presigned URL for avatar access
func (s *AvatarService) GetAvatarURL(ctx context.Context, user_id string, expiry int) (string, error) {
	avatars, err := s.avatarRepository.GetByUserID(ctx, user_id)
	if err != nil {
		return "", fmt.Errorf("error retrieving avatar: %w", err)
	}
	if len(avatars) == 0 {
		log.Printf("no avatar (even default avatar) found by user: %s", user_id)
		return "", errors.New("avatar not found")
	}
	avatar := avatars[0]

	// Generate presigned URL
	url, err := minio.GetPresignedURL(ctx, avatar.BucketName, avatar.StoragePath, expiry)
	if err != nil {
		return "", fmt.Errorf("error generating presigned URL: %w", err)
	}
	s.redisRepository.SaveStructCached(ctx, user_id, "avatar-cached:", url, 24)

	return url, nil
}

func (s *AvatarService) CreateDefaultAvatar(ctx context.Context, df_avatar *models.Avatar) error {
	_, err := s.avatarRepository.Create(context.Background(), df_avatar)
	if err != nil {
		return fmt.Errorf("error creating avatar metadata for %s: %v", df_avatar.FileName, err)
	}
	log.Printf("Added default avatar: %s", df_avatar.FileName)
	return nil
}
