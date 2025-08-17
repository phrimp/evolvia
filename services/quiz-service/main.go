package main

import (
	"context"
	"log"
	"os"
	"quiz-service/internal/db"
	"quiz-service/internal/event"
	"quiz-service/internal/handlers"
	"quiz-service/internal/repository"
	"quiz-service/internal/service"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI is required")
	}
	db.InitMongo(mongoURI)

	// RabbitMQ event publisher
	rabbitURL := os.Getenv("RABBITMQ_URI")
	// Use skills.events exchange to integrate with knowledge service
	eventExchange := "skills.events"
	var publisher *event.EventPublisher
	if rabbitURL != "" {
		var err error
		publisher, err = event.NewEventPublisher(rabbitURL, eventExchange)
		if err != nil {
			log.Fatalf("Failed to connect to RabbitMQ: %v", err)
		}
		defer publisher.Close()
		log.Printf("Connected to RabbitMQ exchange: %s", eventExchange)
	} else {
		log.Println("RabbitMQ not configured, public events will not be published")
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://evolvia.phrimp.io.vn"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// Khởi tạo repository, service, handler cho quiz và question
	mongoClient := db.Client
	database := mongoClient.Database("quiz_service")

	// Global configuration setup
	configRepo := repository.NewConfigRepository(database)
	configService := service.NewConfigService(configRepo)
	configHandler := handlers.NewConfigHandler(configService)

	// Ensure default configuration exists
	if err := configService.EnsureDefaultExists(context.Background()); err != nil {
		log.Fatalf("Failed to ensure default config exists: %v", err)
	}

	resultRepo := repository.NewResultRepository(database)
	resultService := service.NewResultService(resultRepo, publisher)
	quizRepo := repository.NewQuizRepository(database)
	quizService := service.NewQuizService(quizRepo)
	quizHandler := handlers.NewQuizHandler(quizService)
	questionRepo := repository.NewQuestionRepository(database)
	questionService := service.NewQuestionService(questionRepo)
	questionHandler := handlers.NewQuestionHandler(questionService)
	sessionRepo := repository.NewSessionRepository(database)
	sessionService := service.NewSessionService(
		sessionRepo,
		questionRepo,
		configService,
		resultService,
	)
	sessionService.SetEventPublisher(publisher)
	sessionHandler := handlers.NewSessionHandler(sessionService, questionService)

	// Timeout handler for session timeout management
	timeoutHandler := handlers.NewTimeoutHandler(sessionService)

	// Public routes
	resultHandler := handlers.NewResultHandler(resultService)
	publicQuiz := r.Group("/public/quizz/quiz")
	{
		publicQuiz.GET("/", func(c *gin.Context) {
			quizHandler.ListQuizzes(c)
			if publisher != nil {
				publisher.Publish("quiz.list", nil)
			}
		})
		publicQuiz.GET(":id", func(c *gin.Context) {
			quizHandler.GetQuiz(c)
			if publisher != nil {
				publisher.Publish("quiz.get", gin.H{"id": c.Param("id")})
			}
		})
		publicQuiz.GET(":id/results", func(c *gin.Context) {
			resultHandler.GetResultsByQuiz(c)
			if publisher != nil {
				publisher.Publish("quiz.results", gin.H{"id": c.Param("id")})
			}
		})
	}
	publicQuestion := r.Group("/public/quizz/question")
	{
		publicQuestion.GET("/", func(c *gin.Context) {
			questionHandler.ListQuestions(c)
			if publisher != nil {
				publisher.Publish("question.list", nil)
			}
		})
		publicQuestion.GET(":id", func(c *gin.Context) {
			questionHandler.GetQuestion(c)
			if publisher != nil {
				publisher.Publish("question.get", gin.H{"id": c.Param("id")})
			}
		})
		// Get supported question types information
		publicQuestion.GET("/types", questionHandler.GetSupportedQuestionTypes)
	}

	// Protected routes
	protectedQuiz := r.Group("/protected/quizz/quiz")
	{
		protectedQuiz.POST("/", quizHandler.CreateQuiz)
		protectedQuiz.GET("/:id", quizHandler.GetQuiz)
		protectedQuiz.PUT("/:id", quizHandler.UpdateQuiz)
		protectedQuiz.DELETE("/:id", quizHandler.DeleteQuiz)
	}

	protectedQuestion := r.Group("/protected/quizz/question")
	{
		protectedQuestion.POST("/", questionHandler.CreateQuestion)
		protectedQuestion.PUT("/:id", questionHandler.UpdateQuestion)
		protectedQuestion.DELETE("/:id", questionHandler.DeleteQuestion)
		protectedQuestion.POST("/bulk", questionHandler.BulkQuestionOps)

		// Type-specific creation endpoints
		protectedQuestion.POST("/true-false", questionHandler.CreateTrueFalseQuestion)
		protectedQuestion.POST("/single-choice", questionHandler.CreateSingleChoiceQuestion)
		protectedQuestion.POST("/multiple-choice", questionHandler.CreateMultipleChoiceQuestion)
	}

	publicUser := r.Group("/public/quizz/user")
	{
		publicUser.GET(":id/results", func(c *gin.Context) {
			resultHandler.GetResultsByUser(c)
			if publisher != nil {
				publisher.Publish("user.results", gin.H{"id": c.Param("id")})
			}
		})
	}

	protectedResult := r.Group("/protected/quizz/result")
	{
		protectedResult.POST("/", resultHandler.CreateResult)
	}

	// Global Configuration routes
	protectedConfig := r.Group("/protected/quizz/config")
	{
		protectedConfig.POST("/", configHandler.CreateConfig)
		protectedConfig.GET("/:id", configHandler.GetConfig)
		protectedConfig.GET("/", configHandler.GetAllConfigs)
		protectedConfig.GET("/default", configHandler.GetDefaultConfig)
		protectedConfig.PUT("/:id", configHandler.UpdateConfig)
		protectedConfig.DELETE("/:id", configHandler.DeleteConfig)
		protectedConfig.POST("/:id/set-default", configHandler.SetDefaultConfig)
	}

	setupGlobalSessionRoutes(r, sessionHandler, publisher) // Global session routes (no quiz dependency)
	setupTimeoutRoutes(r, timeoutHandler)                  // Session timeout management routes

	r.Run(":6666")
}

// setupGlobalSessionRoutes sets up routes for global session management (no quiz dependency)
// Benefits: Better performance, no quiz constraints, enhanced caching, simplified management
func setupGlobalSessionRoutes(r *gin.Engine, sessionHandler *handlers.SessionHandler, publisher *event.EventPublisher) {
	// Protected global session routes - RECOMMENDED for new implementations
	protectedGlobalSession := r.Group("/protected/quizz/global-session")
	{
		// === GLOBAL SESSION MANAGEMENT ===

		// Create new global session with skill-based filtering
		protectedGlobalSession.POST("/", func(c *gin.Context) {
			sessionHandler.CreateGlobalSession(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.creation_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})

		// Global session status and management
		protectedGlobalSession.GET("/:id", func(c *gin.Context) {
			sessionHandler.GetSession(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.retrieved", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		protectedGlobalSession.PUT("/:id", func(c *gin.Context) {
			sessionHandler.UpdateSession(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.updated", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Global session questions and answers
		protectedGlobalSession.GET("/:id/next-question", func(c *gin.Context) {
			sessionHandler.NextQuestion(c)
		})

		protectedGlobalSession.POST("/:id/answer", func(c *gin.Context) {
			sessionHandler.SubmitAnswer(c)
			if publisher != nil {
				publisher.Publish("skills.events.answer_submitted", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
					"event_type": "answer_submission",
					"exchange":   "skills.events",
				})
			}
		})

		protectedGlobalSession.POST("/:id/submit", func(c *gin.Context) {
			sessionHandler.SubmitSession(c)
		})

		// Global session status endpoints
		protectedGlobalSession.GET("/:id/status", func(c *gin.Context) {
			sessionHandler.GetSessionStatus(c)
		})

		// Get detailed session progress
		protectedGlobalSession.GET("/:id/progress", func(c *gin.Context) {
			sessionHandler.GetSessionProgress(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.progress_checked", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get session statistics
		protectedGlobalSession.GET("/:id/statistics", func(c *gin.Context) {
			sessionHandler.GetSessionStatistics(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.statistics_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Integrity monitoring endpoint
		protectedGlobalSession.GET("/:id/integrity", func(c *gin.Context) {
			sessionHandler.GetSessionIntegrityReport(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.integrity_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Pause session
		protectedGlobalSession.POST("/:id/pause", func(c *gin.Context) {
			sessionHandler.PauseSession(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.paused", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Resume session
		protectedGlobalSession.POST("/:id/resume", func(c *gin.Context) {
			sessionHandler.ResumeSession(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.resumed", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get all answers for a session
		protectedGlobalSession.GET("/:id/answers", func(c *gin.Context) {
			sessionHandler.GetSessionAnswers(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.answers_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Validate session access
		protectedGlobalSession.GET("/:id/validate", func(c *gin.Context) {
			sessionHandler.ValidateSessionAccess(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.access_validated", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get batch sessions (admin endpoint)
		protectedGlobalSession.GET("/batch", func(c *gin.Context) {
			sessionHandler.GetBatchSessions(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.batch_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"limit":     c.Query("limit"),
					"offset":    c.Query("offset"),
					"timestamp": time.Now(),
				})
			}
		})

		// Get answer cache statistics (admin endpoint)
		protectedGlobalSession.GET("/cache-stats", func(c *gin.Context) {
			sessionHandler.GetAnswerCacheStats(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.cache_stats_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})
	}

	// Public global session routes
	publicGlobalSession := r.Group("/public/quizz/global-session")
	{
		// Public progress check (limited information)
		publicGlobalSession.GET("/:id/progress", func(c *gin.Context) {
			sessionHandler.GetSessionProgress(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.public_progress_check", gin.H{
					"session_id": c.Param("id"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Global pool information for skill tags
		publicGlobalSession.GET("/pool-info", func(c *gin.Context) {
			skillTags := c.QueryArray("skill_tags")
			if len(skillTags) == 0 {
				c.JSON(400, gin.H{
					"error": "skill_tags parameter is required",
				})
				return
			}

			// Create a temporary skill info for pool validation
			skillInfo := struct {
				Tags []string `json:"tags"`
			}{
				Tags: skillTags,
			}

			c.JSON(200, gin.H{
				"message": "Global pool uses all questions filtered by skill tags",
				"skills":  skillInfo,
				"mode":    "global",
			})
		})
	}
}

// setupTimeoutRoutes configures session timeout management routes
func setupTimeoutRoutes(r *gin.Engine, timeoutHandler *handlers.TimeoutHandler) {
	// Protected timeout routes
	protectedTimeout := r.Group("/protected/quizz/timeout")
	{
		// Get session timeout information
		protectedTimeout.GET("/:id", timeoutHandler.GetSessionTimeoutInfo)

		// Extend session timeout
		protectedTimeout.POST("/:id/extend", timeoutHandler.ExtendSessionTimeout)

		// Get current question state for a session
		protectedTimeout.GET("/:id/question-state", timeoutHandler.GetCurrentQuestionState)

		// Get overall question state cache statistics
		protectedTimeout.GET("/stats/question-states", timeoutHandler.GetQuestionStateStats)
	}
}
