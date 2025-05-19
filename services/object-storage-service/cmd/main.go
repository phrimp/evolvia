package main

import (
	"context"
	"fmt"
	"log"
	"object-storage-service/internal/api/handlers"
	"object-storage-service/internal/config"
	"object-storage-service/internal/database/minio"
	"object-storage-service/internal/database/mongo"
	"object-storage-service/internal/events"
	"object-storage-service/internal/repository"
	"object-storage-service/internal/service"
	"object-storage-service/pkg/discovery"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
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
		log.Printf("Warning: Failed to set up logging: %v", err)
	} else {
		defer logFile.Close()
	}

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

	// Initialize event publisher
	eventPublisher, err := events.NewEventPublisher(cfg.RabbitMQ.URI)
	if err != nil {
		log.Printf("Warning: Failed to initialize event publisher: %v", err)
		eventPublisher = nil
	} else {
		defer eventPublisher.Close()
	}

	// Initialize service container
	container := &ServiceContainer{
		FileRepository:   fileRepository,
		AvatarRepository: avatarRepository,
		FileService:      service.NewFileService(fileRepository, eventPublisher, cfg),
		AvatarService:    service.NewAvatarService(avatarRepository, eventPublisher, cfg),
		EventPublisher:   eventPublisher,
	}

	// Initialize event consumer
	eventConsumer, err := events.NewEventConsumer(
		cfg.RabbitMQ.URI,
		fileRepository,
		avatarRepository,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	} else {
		// Start the consumer
		if err := eventConsumer.Start(); err != nil {
			log.Printf("Warning: Failed to start event consumer: %v", err)
			eventConsumer.Close()
		} else {
			log.Println("Successfully started event consumer")
			container.EventConsumer = eventConsumer
			// Ensure consumer is closed when application exits
			defer eventConsumer.Close()
		}
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

// func LoadDefaultAssets(services *ServiceContainer) error {
// 	dir_path := "../assets/"
// 	err := filepath.Walk(dir_path, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			fmt.Printf("Error accessing path %s: %v\n", path, err)
// 			return err
// 		}
//
// 		// Skip directories
// 		if info.IsDir() {
// 			return nil
// 		}
//
// 		// Open the file
// 		file, err := os.Open(path)
// 		if err != nil {
// 			fmt.Printf("Error opening file %s: %v\n", path, err)
// 			return nil // Continue with next file
// 		}
// 		defer file.Close()
//
// 		data, err := io.ReadAll(file)
// 		if err != nil {
// 			fmt.Printf("Error reading file %s: %v\n", path, err)
// 			return nil // Continue with next file
// 		}
//
// 		return nil
// 	})
// 	return nil
// }
