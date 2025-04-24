package main

import (
	"fmt"
	"log"
	"middleware/internal/config"
	_ "middleware/internal/database/redis"
	grpcServer "middleware/internal/grpc"
	_ "middleware/pkg/discovery"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	pb "proto-gen/middleware"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func setupLogging() (*os.File, error) {
	logDir := filepath.Join("/evolvia", "log", "middleware")
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
	grpcServer := setupGRPCServer()
	app := fiber.New(fiber.Config{})
	app.Use(func(c fiber.Ctx) error {
		log.Printf("Received request for path: %s", c.Path())
		return c.Next()
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
		if err := grpcServer.Serve(grpcListener); err != nil {
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

	grpcServer.GracefulStop()

	<-doneChan
	log.Println("Server exited, goodbye!")
}

func setupGRPCServer() *grpc.Server {
	s := grpc.NewServer()
	middlewareServer := grpcServer.NewMiddlewareServer()

	pb.RegisterMiddlewareServiceServer(s, middlewareServer)

	reflection.Register(s)
	return s
}
