package minio

import (
	"context"
	"errors"
	"io"
	"log"
	"object-storage-service/internal/config"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

func InitMinioClient(cfg *config.MinIOConfig) error {
	var err error

	// Initialize MinIO client
	MinioClient, err = minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		log.Printf("Error initializing MinIO client: %v", err)
		return err
	}

	// Check if buckets exist and create them if they don't
	bucketsToCreate := []string{cfg.AvatarBucket, cfg.FileBucket, cfg.DefaultBucket}
	for _, bucket := range bucketsToCreate {
		exists, err := MinioClient.BucketExists(context.Background(), bucket)
		if err != nil {
			log.Printf("Error checking if bucket %s exists: %v", bucket, err)
			return err
		}

		if !exists {
			err = MinioClient.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{
				Region: cfg.Region,
			})
			if err != nil {
				log.Printf("Error creating bucket %s: %v", bucket, err)
				return err
			}
			log.Printf("Created bucket: %s", bucket)
		}
	}

	log.Println("Successfully initialized MinIO client")
	return nil
}

// UploadFile uploads a file to MinIO
func UploadFile(ctx context.Context, bucketName, objectName string, reader io.Reader, contentType string, size int64) (minio.UploadInfo, error) {
	uploadInfo, err := MinioClient.PutObject(ctx, bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Printf("Error uploading file to MinIO: %v", err)
		return minio.UploadInfo{}, err
	}

	return uploadInfo, nil
}

// GetFile downloads a file from MinIO
func GetFile(ctx context.Context, bucketName, objectName string) (*minio.Object, error) {
	object, err := MinioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Error getting file from MinIO: %v", err)
		return nil, err
	}

	return object, nil
}

// DeleteFile deletes a file from MinIO
func DeleteFile(ctx context.Context, bucketName, objectName string) error {
	err := MinioClient.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("Error deleting file from MinIO: %v", err)
		return err
	}

	return nil
}

// ListFiles lists files in a MinIO bucket with a prefix
func ListFiles(ctx context.Context, bucketName, prefix string) ([]minio.ObjectInfo, error) {
	objectCh := MinioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var objects []minio.ObjectInfo
	for object := range objectCh {
		if object.Err != nil {
			log.Printf("Error listing objects: %v", object.Err)
			return nil, object.Err
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// GetPresignedURL generates a presigned URL for file access
func GetPresignedURL(ctx context.Context, bucketName, objectName string, expiry int) (string, error) {
	// For security, validate the object name to prevent path traversal
	if strings.Contains(objectName, "..") {
		return "", errors.New("invalid object name")
	}

	// Get presigned URL
	presignedURL, err := MinioClient.PresignedGetObject(ctx, bucketName, objectName, time.Duration(expiry)*time.Second, nil)
	if err != nil {
		log.Printf("Error generating presigned URL: %v", err)
		return "", err
	}

	return presignedURL.String(), nil
}

func UploadFileStream(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	return MinioClient.PutObject(ctx, bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

func CountObjectInBucket(bucketName string) (int, error) {
	objectCount := 0
	objectCh := MinioClient.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			log.Printf("Error listing objects in bucket: %v", object.Err)
			return 0, object.Err
		}
		objectCount++
	}
	return objectCount, nil
}
