package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	quizRepo := repository.NewQuizRepository(database)
	quizService := service.NewQuizService(quizRepo)
	quizHandler := handlers.NewQuizHandler(quizService)
	questionRepo := repository.NewQuestionRepository(database)
	questionService := service.NewQuestionService(questionRepo)
	questionHandler := handlers.NewQuestionHandler(questionService)
	sessionRepo := repository.NewSessionRepository(database)
	sessionService := service.NewSessionService(
		sessionRepo,
		quizRepo, // DEPRECATED: Will be removed
		questionRepo,
		configService, // NEW: For global configurations
	)
	// DEPRECATED: Answer persistence is now handled via SessionService caching
	// These components are maintained for backward compatibility and potential rollback
	// TODO: Consider removing after cache-based implementation is proven stable in production
	answerRepo := repository.NewAnswerRepository(database)
	answerService := service.NewAnswerService(answerRepo)
	_ = handlers.NewAnswerHandler(answerService) // Handler returns deprecation notices
	sessionHandler := handlers.NewSessionHandler(sessionService, answerService, questionService)
	
	// Timeout handler for session timeout management
	timeoutHandler := handlers.NewTimeoutHandler(sessionService)

	// Public routes
	resultRepo := repository.NewResultRepository(database)
	resultService := service.NewResultService(resultRepo, publisher)
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

	setupSessionRoutes(r, sessionHandler, publisher)       // DEPRECATED: Quiz-dependent sessions
	setupGlobalSessionRoutes(r, sessionHandler, publisher) // RECOMMENDED: Global session routes (no quiz dependency)
	setupTimeoutRoutes(r, timeoutHandler)                  // Session timeout management routes

	r.Run(":6666")
}

func setupSessionRoutes(r *gin.Engine, sessionHandler *handlers.SessionHandler, publisher *event.EventPublisher) {
	// DEPRECATED: Protected session routes with adaptive logic (quiz-dependent)
	// These routes are maintained for backward compatibility
	// New implementations should use setupGlobalSessionRoutes instead
	protectedSession := r.Group("/protected/quizz/session")
	{
		// === CORE SESSION MANAGEMENT ===

		// Update session information
		protectedSession.PUT("/:id", func(c *gin.Context) {
			sessionHandler.UpdateSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.updated", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// === ADAPTIVE QUIZ INTERACTION ===

		// Submit answer - now with adaptive processing
		protectedSession.POST("/:id/answer", func(c *gin.Context) {
			sessionHandler.SubmitAnswer(c)
			if publisher != nil {
				publisher.Publish("quiz.answer.submitted", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get next question based on adaptive logic
		protectedSession.GET("/:id/next", func(c *gin.Context) {
			sessionHandler.NextQuestion(c)
			if publisher != nil {
				publisher.Publish("quiz.question.requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// === SESSION CONTROL ===

		// Submit/complete session
		protectedSession.POST("/:id/submit", func(c *gin.Context) {
			sessionHandler.SubmitSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.completed", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Pause session
		protectedSession.POST("/:id/pause", func(c *gin.Context) {
			sessionHandler.PauseSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.paused", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Resume session
		protectedSession.POST("/:id/resume", func(c *gin.Context) {
			sessionHandler.ResumeSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.resumed", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// === STATUS AND MONITORING ===

		// Get current session status (detailed adaptive status)
		protectedSession.GET("/:id/status", func(c *gin.Context) {
			sessionHandler.GetSessionStatus(c)
			if publisher != nil {
				publisher.Publish("quiz.session.status_checked", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get detailed session progress
		protectedSession.GET("/:id/progress", func(c *gin.Context) {
			sessionHandler.GetSessionProgress(c)
			if publisher != nil {
				publisher.Publish("quiz.session.progress_checked", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get session statistics
		protectedSession.GET("/:id/statistics", func(c *gin.Context) {
			sessionHandler.GetSessionStatistics(c)
			if publisher != nil {
				publisher.Publish("quiz.session.statistics_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Integrity monitoring endpoint
		protectedSession.GET("/:id/integrity", func(c *gin.Context) {
			sessionHandler.GetSessionIntegrityReport(c)
			if publisher != nil {
				publisher.Publish("quiz.session.integrity_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// === ANSWERS AND RESULTS ===

		// Get all answers for a session
		protectedSession.GET("/:id/answers", func(c *gin.Context) {
			sessionHandler.GetSessionAnswers(c)
			if publisher != nil {
				publisher.Publish("quiz.session.answers_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Validate session access
		protectedSession.GET("/:id/validate", func(c *gin.Context) {
			sessionHandler.ValidateSessionAccess(c)
			if publisher != nil {
				publisher.Publish("quiz.session.access_validated", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
		})

		// === QUESTION POOL MANAGEMENT ===

		// Get quiz pool information
		protectedSession.GET("/pool/info", func(c *gin.Context) {
			sessionHandler.GetQuizPoolInfo(c)
			if publisher != nil {
				publisher.Publish("quiz.pool.info_requested", gin.H{
					"quiz_id":   c.Query("quiz_id"),
					"skill_id":  c.Query("skill_id"),
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})

		// Preload questions for a stage
		protectedSession.POST("/pool/preload", func(c *gin.Context) {
			sessionHandler.PreloadQuestions(c)
			if publisher != nil {
				publisher.Publish("quiz.pool.questions_preloaded", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})

		// === ADMIN AND BATCH OPERATIONS ===

		// Get batch sessions (admin endpoint)
		protectedSession.GET("/batch", func(c *gin.Context) {
			sessionHandler.GetBatchSessions(c)
			if publisher != nil {
				publisher.Publish("quiz.session.batch_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"limit":     c.Query("limit"),
					"offset":    c.Query("offset"),
					"timestamp": time.Now(),
				})
			}
		})

		// Get answer cache statistics (admin endpoint)
		protectedSession.GET("/cache-stats", func(c *gin.Context) {
			sessionHandler.GetAnswerCacheStats(c)
			if publisher != nil {
				publisher.Publish("quiz.session.cache_stats_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})

	}

	// === PUBLIC SESSION ROUTES ===
	publicSession := r.Group("/public/quizz/session")
	{
		// Get basic session information (public)
		publicSession.GET("/:id", func(c *gin.Context) {
			sessionHandler.GetSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.public_view", gin.H{
					"session_id": c.Param("id"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get session status (public - limited info)
		publicSession.GET("/:id/status", func(c *gin.Context) {
			sessionHandler.GetSessionStatus(c)
			if publisher != nil {
				publisher.Publish("quiz.session.public_status_check", gin.H{
					"session_id": c.Param("id"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get session progress (public - limited info)
		publicSession.GET("/:id/progress", func(c *gin.Context) {
			sessionHandler.GetSessionProgress(c)
			if publisher != nil {
				publisher.Publish("quiz.session.public_progress_check", gin.H{
					"session_id": c.Param("id"),
					"timestamp":  time.Now(),
				})
			}
		})

		// Get quiz pool information (public)
		publicSession.GET("/pool/info", func(c *gin.Context) {
			sessionHandler.GetQuizPoolInfo(c)
			if publisher != nil {
				publisher.Publish("quiz.pool.public_info_requested", gin.H{
					"quiz_id":   c.Query("quiz_id"),
					"skill_id":  c.Query("skill_id"),
					"timestamp": time.Now(),
				})
			}
		})

	}

	// === MIDDLEWARE SETUP FOR SESSION ROUTES ===

	// Add authentication middleware to protected routes
	protectedSession.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "MISSING_USER_ID",
			})
			c.Abort()
			return
		}
		c.Next()
	})

	// Add rate limiting middleware for intensive operations
	protectedSession.Use(func(c *gin.Context) {
		// Simple rate limiting logic could be added here
		// For production, consider using redis-based rate limiting
		c.Next()
	})

	// Add request logging middleware
	protectedSession.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[SESSION] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			param.Path,
			param.ErrorMessage,
		)
	}))

	// === ERROR HANDLING MIDDLEWARE ===
	protectedSession.Use(func(c *gin.Context) {
		c.Next()

		// Handle any panics that might occur in session handlers
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			if publisher != nil {
				publisher.Publish("quiz.session.error_occurred", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"error":      err.Error(),
					"path":       c.Request.URL.Path,
					"method":     c.Request.Method,
					"timestamp":  time.Now(),
				})
			}
		}
	})
}

// setupGlobalSessionRoutes sets up routes for global session management (no quiz dependency)
// RECOMMENDED: Use these routes for all new implementations
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
			if publisher != nil {
				publisher.Publish("quiz.global_session.question_requested", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
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
			if publisher != nil {
				// Enhanced global session event routed to skills.events exchange
				publisher.Publish("skills.events.session_submitted", gin.H{
					"session_id":   c.Param("id"),
					"user_id":      c.GetHeader("X-User-ID"),
					"timestamp":    time.Now(),
					"session_type": "global",
					"event_type":   "session_submission",
					"exchange":     "skills.events",
					"client_info": gin.H{
						"user_agent": c.GetHeader("User-Agent"),
						"ip_address": c.ClientIP(),
					},
				})
			}
		})

		// Global session status endpoints
		protectedGlobalSession.GET("/:id/status", func(c *gin.Context) {
			sessionHandler.GetSessionStatus(c)
			if publisher != nil {
				publisher.Publish("quiz.global_session.status_checked", gin.H{
					"session_id": c.Param("id"),
					"user_id":    c.GetHeader("X-User-ID"),
					"timestamp":  time.Now(),
				})
			}
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
