package main

import (
	"context"
	"fmt"
	"knowledge-service/internal/config"
	"knowledge-service/internal/database/mongo"
	"knowledge-service/internal/event"
	"knowledge-service/internal/handlers"
	"knowledge-service/internal/repository"
	"knowledge-service/internal/services"
	"knowledge-service/utils/discovery"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
)

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "knowledge_service")
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

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("Knowledge Service is healthy")
	})

	// Initialize repositories
	skillRepo := repository.NewSkillRepository(mongo.Mongo_Database, "skills")
	userSkillRepo := repository.NewUserSkillRepository(mongo.Mongo_Database, "user_skills")

	// Initialize event publisher
	eventPublisher, err := event.NewEventPublisher(cfg.RabbitMQ.URI)
	if err != nil {
		log.Printf("Warning: Failed to initialize event publisher: %v", err)
	}

	// Initialize services with data directory
	dataDir := getEnv("DATA_DIR", "/data")
	skillService, err := services.NewSkillService(skillRepo, dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize skill service: %v", err)
	}

	userSkillService, err := services.NewUserSkillService(userSkillRepo, skillRepo)
	if err != nil {
		log.Fatalf("Failed to initialize user skill service: %v", err)
	}

	// Initialize event consumer with both services for skill detection
	eventConsumer, err := event.NewEventConsumer(cfg.RabbitMQ.URI, userSkillService, skillService)
	if err != nil {
		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	} else {
		if err := eventConsumer.Start(); err != nil {
			log.Printf("Warning: Failed to start event consumer: %v", err)
			eventConsumer.Close()
		} else {
			log.Println("Successfully started event consumer for skill detection")
			defer eventConsumer.Close()
		}
	}

	// Initialize and register handlers
	skillHandler := handlers.NewSkillHandler(skillService)
	skillHandler.RegisterRoutes(app)

	userSkillHandler := handlers.NewUserSkillHandler(userSkillService)
	userSkillHandler.RegisterRoutes(app)

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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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

	// Close event consumer
	if eventConsumer != nil {
		if err := eventConsumer.Close(); err != nil {
			log.Printf("Error closing event consumer: %v", err)
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

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
