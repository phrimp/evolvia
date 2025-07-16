package main

import (
	"log"
	"os"

	"quiz-service/internal/db"
	"quiz-service/internal/handlers"
	"quiz-service/internal/repository"
	"quiz-service/internal/service"

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

	r := gin.Default()

	// Khởi tạo repository, service, handler cho quiz và question
	mongoClient := db.Client
	database := mongoClient.Database("quizdb")
	quizRepo := repository.NewQuizRepository(database)
	quizService := service.NewQuizService(quizRepo)
	quizHandler := handlers.NewQuizHandler(quizService)
	questionRepo := repository.NewQuestionRepository(database)
	questionService := service.NewQuestionService(questionRepo)
	questionHandler := handlers.NewQuestionHandler(questionService)
	sessionRepo := repository.NewSessionRepository(database)
	sessionService := service.NewSessionService(sessionRepo)
	sessionHandler := handlers.NewSessionHandler(sessionService)
	answerRepo := repository.NewAnswerRepository(database)
	answerService := service.NewAnswerService(answerRepo)
	answerHandler := handlers.NewAnswerHandler(answerService)

	// Public routes
	resultRepo := repository.NewResultRepository(database)
	resultService := service.NewResultService(resultRepo)
	resultHandler := handlers.NewResultHandler(resultService)
	publicQuiz := r.Group("/public/quiz")
	{
		publicQuiz.GET("/", quizHandler.ListQuizzes)
		publicQuiz.GET("/:id", quizHandler.GetQuiz)
		publicQuiz.GET("/:id/results", resultHandler.GetResultsByQuiz)
	}
	publicQuestion := r.Group("/public/question")
	{
		publicQuestion.GET("/", questionHandler.ListQuestions)
		publicQuestion.GET("/:id", questionHandler.GetQuestion)
	}

	// Protected routes
	protectedQuiz := r.Group("/protected/quiz")
	{
		protectedQuiz.POST("/", quizHandler.CreateQuiz)
		protectedQuiz.PUT("/:id", quizHandler.UpdateQuiz)
		protectedQuiz.DELETE("/:id", quizHandler.DeleteQuiz)
	}

	protectedQuestion := r.Group("/protected/question")
	{
		protectedQuestion.POST("/", questionHandler.CreateQuestion)
		protectedQuestion.PUT("/:id", questionHandler.UpdateQuestion)
		protectedQuestion.DELETE("/:id", questionHandler.DeleteQuestion)
		protectedQuestion.POST("/bulk", questionHandler.BulkQuestionOps)
	}

	protectedSession := r.Group("/protected/session")
	{
		protectedSession.POST("/", sessionHandler.CreateSession)
		protectedSession.POST("/:id/answer", answerHandler.CreateAnswer)
		protectedSession.GET("/:id/next", sessionHandler.NextQuestion)
		protectedSession.POST("/:id/submit", sessionHandler.SubmitSession)
		protectedSession.POST("/:id/pause", sessionHandler.PauseSession)
	}

	publicSession := r.Group("/public/session")
	{
		publicSession.GET("/:id", sessionHandler.GetSession)
		publicSession.GET("/:id/answers", answerHandler.GetAnswersBySession)
		publicSession.GET("/:id/result", resultHandler.GetResultBySession)
	}

	publicUser := r.Group("/public/user")
	{
		publicUser.GET("/:id/results", resultHandler.GetResultsByUser)
	}

	protectedResult := r.Group("/protected/result")
	{
		protectedResult.POST("/", resultHandler.CreateResult)
	}

	r.Run()
}
