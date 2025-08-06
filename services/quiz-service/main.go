package main

import (
	"log"
	"os"
	"time"

	"quiz-service/internal/db"
	"quiz-service/internal/event"
	"quiz-service/internal/handlers"
	"quiz-service/internal/repository"
	"quiz-service/internal/service"

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
	sessionHandler := handlers.NewSessionHandler(sessionService)
	answerRepo := repository.NewAnswerRepository(database)
	answerService := service.NewAnswerService(answerRepo)
	answerHandler := handlers.NewAnswerHandler(answerService)

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

	protectedSession := r.Group("/protected/quizz/session")
	{
		protectedSession.POST("/", sessionHandler.CreateSession)
		protectedSession.POST("/:id/answer", answerHandler.CreateAnswer)
		protectedSession.GET("/:id/next", sessionHandler.NextQuestion)
		protectedSession.POST("/:id/submit", sessionHandler.SubmitSession)
		protectedSession.POST("/:id/pause", sessionHandler.PauseSession)
	}

	publicSession := r.Group("/public/quizz/session")
	{
		publicSession.GET(":id", func(c *gin.Context) {
			sessionHandler.GetSession(c)
			if publisher != nil {
				publisher.Publish("session.get", gin.H{"id": c.Param("id")})
			}
		})
		publicSession.GET(":id/answers", func(c *gin.Context) {
			answerHandler.GetAnswersBySession(c)
			if publisher != nil {
				publisher.Publish("session.answers", gin.H{"id": c.Param("id")})
			}
		})
		publicSession.GET(":id/result", func(c *gin.Context) {
			resultHandler.GetResultBySession(c)
			if publisher != nil {
				publisher.Publish("session.result", gin.H{"id": c.Param("id")})
			}
		})
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

	r.Run(":6666")
}
