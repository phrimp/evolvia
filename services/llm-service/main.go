package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"llm-service/configs"
	"llm-service/controllers"
	"llm-service/services"
	"llm-service/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	configs.LoadConfig()

	// Set Gin mode
	gin.SetMode(configs.AppConfig.GinMode)

	// Initialize services
	if err := initServices(); err != nil {
		log.Fatal("Failed to initialize services:", err)
	}

	// Setup routes
	r := setupRoutes()

	// Register service with RabbitMQ
	registerService()

	// Start server
	log.Printf("Starting LLM Service on port %s", configs.AppConfig.Port)
	if err := r.Run(":" + configs.AppConfig.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func initServices() error {
	// Initialize Database
	if err := services.InitDatabase(); err != nil {
		log.Printf("Warning: Failed to connect to MongoDB: %v", err)
	} else {
		log.Println("MongoDB connected successfully")
	}

	// Initialize RabbitMQ
	if configs.AppConfig.RabbitMQEnabled {
		if err := services.InitRabbitMQ(); err != nil {
			log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
		} else {
			log.Println("RabbitMQ connected successfully")
		}
	} else {
		log.Println("RabbitMQ disabled in configuration")
	}

	// Initialize RAG Service
	if err := services.InitRAGService(); err != nil {
		log.Printf("Warning: Failed to initialize RAG service: %v", err)
	} else {
		log.Println("RAG Service initialized successfully")
	}

	// Initialize LLM Service
	if err := services.InitLLMService(); err != nil {
		log.Printf("Warning: Failed to initialize LLM service: %v", err)
	} else {
		log.Println("LLM Service initialized successfully")
	}

	return nil
}

func setupRoutes() *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://evolvia.phrimp.io.vn"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Add logging middleware
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Recovery middleware
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "llm-service",
			"timestamp": time.Now(),
		})
	})

	// Initialize controller
	llmController := controllers.NewLLMController()

	// Public routes group
	public := r.Group("/public/llm")
	{
		// Basic ping endpoint
		public.GET("/ping", llmController.Ping)

		// Model status endpoint
		public.GET("/model", llmController.GetModelStatus)

		// Model interaction endpoints
		model := public.Group("/model")
		{
			// Create new chat session
			model.POST("/session", llmController.CreateChatSession)

			// Chat with the model (requires session ID in URL path)
			model.POST("/:sessionId/chat", llmController.Chat)

			// Streaming chat with the model
			model.POST("/:sessionId/stream", llmController.ChatStream)

			// Get chat history
			model.GET("/history/:sessionId", llmController.GetChatHistory)
		}

		// Database query endpoint (for LLM to execute custom queries)
		public.POST("/query", llmController.ExecuteQuery)
	}

	// Protected routes group (requires JWT token)
	protected := r.Group("/protected/llm")
	protected.Use(authMiddleware())
	{
		// Add protected endpoints here if needed
		protected.GET("/user/sessions", func(c *gin.Context) {
			userID, _ := c.Get("userID")
			utils.SuccessResponse(c, "User sessions", gin.H{
				"userId":  userID,
				"message": "This is a protected endpoint",
			})
		})
	}

	return r
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := utils.GetUserIDFromToken(c)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or missing token")
			c.Abort()
			return
		}

		if userID == "" {
			utils.UnauthorizedResponse(c, "Token is required for this endpoint")
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func registerService() {
	if !configs.AppConfig.RabbitMQEnabled {
		log.Println("Skipping service registration - RabbitMQ is disabled")
		return
	}

	if rabbitmq := services.GetRabbitMQService(); rabbitmq != nil {
		if err := rabbitmq.PublishServiceRegistration(); err != nil {
			log.Printf("Warning: Failed to register service: %v", err)
		} else {
			log.Println("Service registered successfully")
		}
	} else {
		log.Println("Skipping service registration - RabbitMQ not connected")
	}
}
