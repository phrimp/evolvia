package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"profile-service/internal/config"
	"profile-service/internal/database/mongo"
	"profile-service/internal/event"
	"profile-service/internal/handlers"
	"profile-service/internal/reporsitory"
	"profile-service/internal/service"
	"profile-service/pkg/discovery"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "profile_service")
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

func main() {
	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logFile.Close()

	cfg := config.ServiceConfig

	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"*"},
	}))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("Profile Service is healthy")
	})

	// Initialize repositories
	profileRepo := reporsitory.NewProfileRepository(mongo.Mongo_Database)

	// Create database indexes
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := profileRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create database indexes: %v", err)
	} else {
		log.Println("Database indexes created successfully")
	}
	cancel()

	eventPublisher, err := event.NewEventPublisher(cfg.RabbitMQ.URI)
	if err != nil {
		log.Printf("Warning: Failed to initialize event publisher: %v", err)
	}

	eventConsumer, err := event.NewEventConsumer(cfg.RabbitMQ.URI, profileRepo)
	if err != nil {
		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	} else {
		if err := eventConsumer.Start(); err != nil {
			log.Printf("Warning: Failed to start event consumer: %v", err)
			eventConsumer.Close()
		} else {
			log.Println("Successfully started event consumer")
			defer eventConsumer.Close()
		}
	}

	// Initialize services
	profileService := service.NewProfileService(profileRepo, eventPublisher)

	// Initialize and register handlers
	profileHandler := handlers.NewProfileHandler(profileService)
	profileHandler.RegisterRoutes(app)

	shutdownChan := make(chan os.Signal, 1)
	doneChan := make(chan bool, 1)

	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Starting server on port %s", cfg.Server.Port)
		if err := app.Listen(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
		doneChan <- true
	}()

	<-shutdownChan
	log.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	// Close event publisher
	if eventPublisher != nil {
		if err := eventPublisher.Close(); err != nil {
			log.Printf("Error closing event publisher: %v", err)
		}
	}

	// Disconnect from MongoDB
	mongo.DisconnectMongo()

	// Deregister from service discovery
	if discovery.ServiceDiscovery != nil {
		if err := discovery.ServiceDiscovery.Deregister(); err != nil {
			log.Printf("Error deregistering from service discovery: %v", err)
		}
	}

	<-doneChan
	log.Println("Server shutdown complete")
}
