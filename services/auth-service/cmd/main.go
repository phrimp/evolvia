package main

import (
	"auth_service/internal/config"
	_ "auth_service/internal/database/mongo"
	_ "auth_service/internal/database/redis"
	"auth_service/internal/events"
	grpcServer "auth_service/internal/grpc"
	"auth_service/internal/handlers"
	"auth_service/internal/repository"
	"auth_service/internal/service"
	"auth_service/pkg/discovery"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	//_ "auth_service/pkg/discovery"

	pb "proto-gen/auth"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/gofiber/fiber/v3/middleware/cors"
)

var ignore_log_path []string = []string{"/health"}

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "auth_service")
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

type ServerServices struct {
	JwtService         *service.JWTService
	UserService        *service.UserService
	UserRoleService    *service.UserRoleService
	RoleService        *service.RoleService
	PermissionService  *service.PermissionService
	SessionService     *service.SessionService
	gRPCSessionService *grpcServer.SessionSenderService
	gRPCGoogleService  *grpcServer.GoogleAuthService
}

func main() {
	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logFile.Close()

	rabbitmqURI := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		config.ServiceConfig.RabbitMQUSer,
		config.ServiceConfig.RabbitMQPassword,
		"rabbitmq", // host
		config.ServiceConfig.RabbitMQPort)

	eventPublisher, err := events.NewEventPublisher(rabbitmqURI)
	if err != nil {
		log.Printf("Warning: Failed to initialize event publisher: %v", err)
		eventPublisher = nil
	} else {
		defer eventPublisher.Close()
	}

	eventConsumer, err := events.NewEventConsumer(rabbitmqURI, repository.Repositories_instance.RedisRepository, repository.Repositories_instance.UserAuthRepository, eventPublisher)
	if err != nil {
		log.Printf("Warning: Failed to initialize event consumer: %v", err)
	} else {
		// Start the consumer
		if err := eventConsumer.Start(); err != nil {
			log.Printf("Warning: Failed to start event consumer: %v", err)
			eventConsumer.Close()
		} else {
			log.Println("Successfully started event consumer for profile updates")
			// Ensure consumer is closed when application exits
			defer eventConsumer.Close()
		}
	}

	services_init := &ServerServices{
		JwtService:         service.NewJWTService(),
		UserService:        service.NewUserService(eventPublisher),
		UserRoleService:    service.NewUserRoleService(),
		RoleService:        service.NewRoleService(),
		PermissionService:  service.NewPermissionService(),
		SessionService:     service.NewSessionService(),
		gRPCSessionService: grpcServer.NewSessionSenderService(discovery.ServiceDiscovery),
		gRPCGoogleService:  grpcServer.NewGoogleAuthService(discovery.ServiceDiscovery),
	}

	_grpcServer := setupGRPCServer()
	app := fiber.New(fiber.Config{})

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"*"},
	}))

	// Add health check endpoint
	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "healthy",
			"service": "auth-service",
		})
	})

	// Add root endpoint
	app.Get("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Auth Service API",
			"status":  "healthy",
		})
	})

	// Handle all OPTIONS requests
	app.Options("/*", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	app.Use(func(c fiber.Ctx) error {
		if !slices.Contains(ignore_log_path, c.Path()) {
			log.Printf("Received request for path: %s", c.Path())
		}
		return c.Next()
	})

	// Init Handlers
	auth_handler := handlers.NewAuthHandler(services_init.UserService, services_init.JwtService, services_init.SessionService, services_init.UserRoleService, services_init.gRPCSessionService, services_init.gRPCGoogleService)
	role_handler := handlers.NewRoleHandler(services_init.RoleService, services_init.UserRoleService)

	// Register Routes
	auth_handler.RegisterRoutes(app)
	role_handler.RegisterRoutes(app)

	shutdownChan := make(chan os.Signal, 1)
	doneChan := make(chan bool, 1)
	grpcDoneChan := make(chan bool, 1)

	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		grpcPort := config.ServiceConfig.GrpcPort
		grpcListener, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("Failed to listen for gRPC: %v", err)
		}

		log.Printf("Starting gRPC server on port %s", grpcPort)
		if err := _grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
		grpcDoneChan <- true
	}()

	go func() {
		log.Printf("Starting server on port %s", config.ServiceConfig.Port)
		if err := app.Listen(":" + config.ServiceConfig.Port); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
		doneChan <- true
	}()

	<-shutdownChan
	log.Println("Shutting down server...")

	if err := app.Shutdown(); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	_grpcServer.GracefulStop()

	<-doneChan
	log.Println("Server exited, goodbye!")
}

func setupGRPCServer() *grpc.Server {
	s := grpc.NewServer()

	authServer := grpcServer.NewAuthServer()

	pb.RegisterAuthServiceServer(s, authServer)

	reflection.Register(s)

	return s
}
