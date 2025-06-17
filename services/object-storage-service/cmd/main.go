package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"object-storage-service/internal/api/handlers"
	"object-storage-service/internal/config"
	"object-storage-service/internal/database/minio"
	"object-storage-service/internal/database/mongo"
	"object-storage-service/internal/events"
	"object-storage-service/internal/models"
	"object-storage-service/internal/repository"
	"object-storage-service/internal/service"
	"object-storage-service/pkg/discovery"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "object_storage_service")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	currentTime := time.Now()
	logFileName := fmt.Sprintf("log_%s.log", currentTime.Format("2006-01-02"))
	logFile := filepath.Join(logDir, logFileName)

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return file, nil
}

// ServiceContainer holds all service dependencies
type ServiceContainer struct {
	FileRepository   *repository.FileRepository
	AvatarRepository *repository.AvatarRepository
	RedisRepository  *repository.RedisRepo
	FileService      *service.FileService
	AvatarService    *service.AvatarService
	EventPublisher   events.Publisher
	EventConsumer    events.Consumer
	ServiceDiscovery *discovery.ServiceRegistry
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logging
	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logFile.Close()

	// Initialize MongoDB
	if err := mongo.InitMongoDB(&cfg.MongoDB); err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	defer mongo.CloseDB()

	// Initialize MinIO client
	if err := minio.InitMinioClient(&cfg.MinIO); err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	// Initialize repositories
	fileRepository := repository.NewFileRepository()
	avatarRepository := repository.NewAvatarRepository()
	redisRepository := repository.NewRedisRepo()

	// Simple startup retry
	var eventPublisher events.Publisher
	for i := range 3 {
		eventPublisher, err = events.NewEventPublisher(cfg.RabbitMQ.URI)
		if err == nil {
			defer eventPublisher.Close()
			break
		}
		if i < 2 {
			time.Sleep(time.Second * 5)
		}
	}

	if eventPublisher == nil {
		log.Println("Starting without event publisher - events will be skipped")
	}

	// Initialize service container
	container := &ServiceContainer{
		FileRepository:   fileRepository,
		AvatarRepository: avatarRepository,
		RedisRepository:  redisRepository,
		FileService:      service.NewFileService(fileRepository, eventPublisher, cfg),
		AvatarService:    service.NewAvatarService(avatarRepository, eventPublisher, cfg, redisRepository),
		EventPublisher:   eventPublisher,
	}

	// Initialize event consumer
	var eventConsumer events.Consumer
	maxRetries := 3
	retryDelays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

	for i := range maxRetries {
		eventConsumer, err = events.NewEventConsumer(
			cfg.RabbitMQ.URI,
			fileRepository,
			avatarRepository,
			redisRepository,
		)
		if err == nil {
			// Successfully created consumer, now try to start it
			if err := eventConsumer.Start(); err != nil {
				log.Printf("Failed to start event consumer (attempt %d/%d): %v", i+1, maxRetries, err)
				eventConsumer.Close()
				eventConsumer = nil
			} else {
				// Successfully started
				log.Println("Successfully started event consumer")
				container.EventConsumer = eventConsumer
				defer eventConsumer.Close()
				break
			}
		} else {
			log.Printf("Failed to initialize event consumer (attempt %d/%d): %v", i+1, maxRetries, err)
		}

		// If this was the last attempt, give up
		if i == maxRetries-1 {
			log.Printf("Failed to initialize event consumer after %d attempts", maxRetries)
			eventConsumer = nil
			break
		}

		// Wait before retrying
		delay := retryDelays[i]
		log.Printf("Retrying in %v", delay)
		time.Sleep(delay)
	}

	if eventConsumer == nil {
		log.Println("Starting without event consumer - user registration events will not be processed")
	}

	// Initialize service discovery
	serviceRegistry, err := discovery.NewServiceRegistry(
		cfg.Consul.Address,
		cfg.Server.ServiceName,
		cfg.Server.ServiceID,
		cfg.Server.Port,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize service discovery: %v", err)
	} else {
		container.ServiceDiscovery = serviceRegistry
		// Register with Consul
		if err := serviceRegistry.Register(); err != nil {
			log.Printf("Warning: Failed to register with Consul: %v", err)
		} else {
			log.Println("Successfully registered with Consul")
			// Ensure service is deregistered when application exits
			defer serviceRegistry.Deregister()
		}
	}

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://evolvia.phrimp.io.vn"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))

	// Set up routes
	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("Object Storage Service is healthy")
	})

	// Setup graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	doneChan := make(chan bool, 1)

	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize handlers
	fileHandler := handlers.NewFileHandler(container.FileService)
	avatarHandler := handlers.NewAvatarHandler(container.AvatarService)

	// Register routes
	fileHandler.RegisterRoutes(app)
	avatarHandler.RegisterRoutes(app)

	go func() {
		log.Printf("Starting server on port %s", cfg.Server.Port)
		if err := app.Listen(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
		doneChan <- true
	}()

	LoadDefaultAssets(container, cfg.MinIO.DefaultBucket)

	<-shutdownChan
	log.Println("Shutting down server...")

	// Create a deadline context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	<-doneChan
	log.Println("Server exited, goodbye!")
}

func LoadDefaultAssets(services *ServiceContainer, defaultBucket string) error {
	assetDir := filepath.Join("/evolvia", "assets")
	imageFileExts := []string{"jpg", "png", "jpeg", "gif", "webp", "svg"}

	// Count file in assets
	assetCount := 0
	err := filepath.Walk(assetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			assetCount++
		}
		return nil
	})
	if err != nil {
		log.Printf("Error counting asset files: %v", err)
		return err
	}

	if assetCount == 0 {
		log.Println("No asset files found to load")
		return nil
	}

	objectCount, err := minio.CountObjectInBucket(defaultBucket)
	if err != nil {
		log.Printf("error counting object in default bucket: %s", err)
		return nil
	}

	if objectCount >= assetCount {
		log.Printf("Default assets already loaded (%d objects in bucket), skipping...", objectCount)
		return nil
	}

	log.Printf("Loading %d default assets to %s bucket...", assetCount, defaultBucket)

	err = filepath.Walk(assetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Open the file
		file, err := os.Open(path)
		if err != nil {
			log.Printf("Error opening file %s: %v", path, err)
			return nil // Continue with next file
		}
		defer file.Close()

		fileName := filepath.Base(path)
		fileExt := strings.ToLower(filepath.Ext(fileName))

		// Determine content type
		contentType := ""
		if fileExt != "" {
			contentType = mime.TypeByExtension(fileExt)
		}

		if contentType == "" {
			buffer := make([]byte, 512)
			_, err = file.Read(buffer)
			if err != nil && err != io.EOF {
				log.Printf("Error reading file header: %v", err)
				return nil
			}

			contentType = http.DetectContentType(buffer)

			// Reset file position
			_, err = file.Seek(0, 0)
			if err != nil {
				log.Printf("Error resetting file position: %v", err)
				return nil
			}
		}

		// Upload file to default bucket
		_, err = minio.UploadFileStream(
			context.Background(),
			defaultBucket,
			fileName,
			file,
			info.Size(),
			contentType,
		)
		if err != nil {
			log.Printf("Error uploading asset %s to default bucket: %v", fileName, err)
			return nil
		}

		// Create metadata based on file type
		ext := strings.TrimPrefix(fileExt, ".")
		if slices.Contains(imageFileExts, ext) {
			avatar := &models.Avatar{
				UserID:      "", // Empty for system defaults
				FileName:    fileName,
				Size:        info.Size(),
				ContentType: contentType,
				StoragePath: fileName, // Store with same name in default bucket
				BucketName:  defaultBucket,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			err := services.AvatarService.CreateDefaultAvatar(context.Background(), avatar)
			if err != nil {
				log.Printf("error create default avatar: %s", err)
			}
		} else {

			_, err = file.Seek(0, 0)
			if err != nil {
				log.Printf("Error resetting file for checksum: %v", err)
				return nil
			}

			hash := md5.New()
			if _, err := io.Copy(hash, file); err != nil {
				log.Printf("Error calculating checksum: %v", err)
				return nil
			}
			checksum := hex.EncodeToString(hash.Sum(nil))

			// Create file metadata
			fileMetadata := &models.File{
				OwnerID:      "", // Empty for system defaults
				Name:         fileName,
				Description:  "Default System File",
				Size:         info.Size(),
				ContentType:  contentType,
				StoragePath:  fileName, // Store with same name in default bucket
				BucketName:   defaultBucket,
				IsPublic:     true,
				Checksum:     checksum,
				VersionCount: 1,
				FolderPath:   "/system/defaults",
				Tags:         []string{"system", "default"},
				Metadata:     make(map[string]string),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			err := services.FileService.UploadDefaultFile(context.Background(), fileMetadata)
			if err != nil {
				log.Printf("Error create default file: %s", err)
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("Error walking assets directory: %v", err)
		return err
	}

	log.Println("Default assets loaded successfully")
	return nil
}
