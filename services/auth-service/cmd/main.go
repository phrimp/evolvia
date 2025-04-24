package main

import (
	"auth_service/internal/config"
	_ "auth_service/internal/database/mongo"
	_ "auth_service/internal/database/redis"
	grpcServer "auth_service/internal/grpc"
	"auth_service/internal/models"
	"auth_service/pkg/discovery"

	//_ "auth_service/pkg/discovery"
	pb "auth_service/pkg/proto/auth"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

func main() {
	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logFile.Close()
	_grpcServer := setupGRPCServer()
	app := fiber.New(fiber.Config{})
	app.Use(func(c fiber.Ctx) error {
		if !slices.Contains(ignore_log_path, c.Path()) {
			log.Printf("Received request for path: %s", c.Path())
		}
		return c.Next()
	})

	app.Get("/auth", func(c fiber.Ctx) error {
		test := models.Session{
			Token: "test-Token",
		}
		a := grpcServer.NewSessionSenderService(discovery.ServiceDiscovery)
		err := a.SendSession(context.Background(), &test, "middleware")
		if err != nil {
			fmt.Println(err)
		}

		return c.Status(200).SendString("Auth Service Root")
	})

	app.Get("/auth/*", func(c fiber.Ctx) error {
		path := c.Params("*")
		return c.Status(200).SendString("Auth Service Path: " + path)
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(200).SendString("OK")
	})

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
