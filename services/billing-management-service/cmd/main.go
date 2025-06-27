package main

import (
	"billing-management-service/internal/config"
	"billing-management-service/internal/database/mongo"
	"billing-management-service/internal/event"
	"billing-management-service/internal/handlers"
	"billing-management-service/internal/repository"
	"billing-management-service/internal/services"
	"billing-management-service/utils/discovery"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
)

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "billing_management_service")
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
		return c.Status(fiber.StatusOK).SendString("Billing Management Service is healthy")
	})
	var eventConsumer event.Consumer
	// Initialize repositories
	subscriptionRepo := repository.NewSubscriptionRepository(mongo.Mongo_Database)
	planRepo := repository.NewPlanRepository(mongo.Mongo_Database)

	// Create database indexes
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := subscriptionRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create subscription indexes: %v", err)
	}
	if err := planRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create plan indexes: %v", err)
	}
	cancel()

	// Initialize event publisher
	eventPublisher, err := event.NewEventPublisher(cfg.RabbitMQ.URI)
	if err != nil {
		log.Printf("Warning: Failed to initialize event publisher: %v", err)
	}

	// Initialize event consumer
	//	eventConsumer, err := event.NewEventConsumer(cfg.RabbitMQ.URI, subscriptionRepo)
	//	if err != nil {
	//		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	//	} else {
	//		if err := eventConsumer.Start(); err != nil {
	//			log.Printf("Warning: Failed to start event consumer: %v", err)
	//			eventConsumer.Close()
	//		} else {
	//			log.Println("Successfully started event consumer")
	//			defer eventConsumer.Close()
	//		}
	//	}

	// Initialize services
	// billingService := services.NewBillingService(subscriptionRepo, planRepo, invoiceRepo, eventPublisher)
	planService := services.NewPlanService(planRepo, eventPublisher)
	subscriptionService := services.NewSubscriptionService(subscriptionRepo, planRepo, eventPublisher)

	eventConsumer, err = event.NewEventConsumer(cfg.RabbitMQ.URI, subscriptionService)
	if err != nil {
		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	} else {
		if err := eventConsumer.Start(); err != nil {
			log.Printf("Warning: Failed to start event consumer: %v", err)
			eventConsumer.Close()
		} else {
			log.Println("Successfully started payment event consumer")
			defer eventConsumer.Close()
		}
	}
	// Initialize and register handlers
	// billingHandler := handlers.NewBillingHandler(billingService)
	// billingHandler.RegisterRoutes(app)
	planHandler := handlers.NewPlanHandler(planService)
	planHandler.RegisterRoutes(app)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService)
	subscriptionHandler.RegisterRoutes(app)

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

	if eventConsumer != nil {
		if err := eventConsumer.Close(); err != nil {
			log.Printf("Error closing event consumer: %v", err)
		}
	}

	<-doneChan
	log.Println("Server shutdown complete")
}
