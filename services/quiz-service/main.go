package main

import (
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
	eventExchange := os.Getenv("RABBITMQ_EXCHANGE")
	var publisher *event.EventPublisher
	if rabbitURL != "" && eventExchange != "" {
		var err error
		publisher, err = event.NewEventPublisher(rabbitURL, eventExchange)
		if err != nil {
			log.Fatalf("Failed to connect to RabbitMQ: %v", err)
		}
		defer publisher.Close()
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
	quizRepo := repository.NewQuizRepository(database)
	quizService := service.NewQuizService(quizRepo)
	quizHandler := handlers.NewQuizHandler(quizService)
	questionRepo := repository.NewQuestionRepository(database)
	questionService := service.NewQuestionService(questionRepo)
	questionHandler := handlers.NewQuestionHandler(questionService)
	sessionRepo := repository.NewSessionRepository(database)
	sessionService := service.NewSessionService(
		sessionRepo,
		quizRepo,
		questionRepo,
	)
	answerRepo := repository.NewAnswerRepository(database)
	answerService := service.NewAnswerService(answerRepo)
	_ = handlers.NewAnswerHandler(answerService)
	sessionHandler := handlers.NewSessionHandler(sessionService, answerService)

	// Public routes
	resultRepo := repository.NewResultRepository(database)
	resultService := service.NewResultService(resultRepo)
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

	setupSessionRoutes(r, sessionHandler, publisher)

	r.Run(":6666")
}

func setupSessionRoutes(r *gin.Engine, sessionHandler *handlers.SessionHandler, publisher *event.EventPublisher) {
	// Protected session routes with adaptive logic
	protectedSession := r.Group("/protected/quizz/session")
	{
		// === CORE SESSION MANAGEMENT ===

		// Create new adaptive session with skill validation
		protectedSession.POST("/", func(c *gin.Context) {
			sessionHandler.CreateSession(c)
			if publisher != nil {
				publisher.Publish("quiz.session.creation_requested", gin.H{
					"user_id":   c.GetHeader("X-User-ID"),
					"timestamp": time.Now(),
				})
			}
		})

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
