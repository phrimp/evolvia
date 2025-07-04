package services

import (
	"context"
	"log"
	"time"

	"llm-service/configs"
	"llm-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DatabaseService struct {
	Client          *mongo.Client
	Database        *mongo.Database // llm_service database for sessions
	ProfileDatabase *mongo.Database // profile_service database for user data
}

var dbService *DatabaseService

func InitDatabase() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(configs.AppConfig.MongoURI))
	if err != nil {
		return err
	}

	// Test connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}

	dbService = &DatabaseService{
		Client:          client,
		Database:        client.Database(configs.AppConfig.MongoDatabase),        // llm_service
		ProfileDatabase: client.Database(configs.AppConfig.MongoProfileDatabase), // profile_service
	}

	return nil
}

func GetDatabaseService() *DatabaseService {
	return dbService
}

func (db *DatabaseService) IsConnected() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.Client.Ping(ctx, nil)
	return err == nil
}

// RAG Database Operations
func (db *DatabaseService) GetUserInfo(userID string) (*models.User, error) {
	collection := db.Database.Collection("users")

	var user models.User
	err := collection.FindOne(context.Background(), bson.M{"userId": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *DatabaseService) GetUserOrders(userID string, limit int) ([]models.Order, error) {
	collection := db.Database.Collection("orders")

	filter := bson.M{"userId": userID}
	opts := options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))

	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var orders []models.Order
	if err = cursor.All(context.Background(), &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

func (db *DatabaseService) CountUserOrders(userID string) (int64, error) {
	collection := db.Database.Collection("orders")

	count, err := collection.CountDocuments(context.Background(), bson.M{"userId": userID})
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Chat Session Operations
func (db *DatabaseService) CreateChatSession(sessionID, userID string) (*models.ChatSession, error) {
	collection := db.Database.Collection("chat_sessions")

	session := &models.ChatSession{
		SessionID: sessionID,
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsActive:  true,
	}

	result, err := collection.InsertOne(context.Background(), session)
	if err != nil {
		return nil, err
	}

	session.ID = result.InsertedID.(primitive.ObjectID)
	return session, nil
}

func (db *DatabaseService) GetChatSession(sessionID string) (*models.ChatSession, error) {
	collection := db.Database.Collection("chat_sessions")

	var session models.ChatSession
	err := collection.FindOne(context.Background(), bson.M{"sessionId": sessionID}).Decode(&session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (db *DatabaseService) GetUserSessions(userID string, limit int) ([]models.ChatSession, error) {
	collection := db.Database.Collection("chat_sessions")

	filter := bson.M{"userId": userID, "isActive": true}
	opts := options.Find().SetSort(bson.D{bson.E{Key: "updatedAt", Value: -1}}) // Sort by most recent first

	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var sessions []models.ChatSession
	if err = cursor.All(context.Background(), &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (db *DatabaseService) ValidateSession(sessionID, userID string) bool {
	session, err := db.GetChatSession(sessionID)
	if err != nil {
		log.Printf("Session validation failed - session not found: %v", err)
		return false
	}

	log.Printf("🔍 Validating session: sessionID=%s, requestUserID=%s, sessionUserID=%s",
		sessionID, userID, session.UserID)

	// Allow session access if:
	// 1. Session is active AND
	// 2. (UserID matches OR both are anonymous OR request is anonymous)
	isValid := session.IsActive &&
		(session.UserID == userID ||
			(session.UserID == "anonymous" && userID == "anonymous") ||
			userID == "anonymous")

	if !isValid {
		log.Printf("Session validation failed - userID mismatch or inactive session")
	} else {
		log.Printf("Session validation passed")
	}

	return isValid
}

func (db *DatabaseService) SaveChatMessage(sessionID, userID, message, response string) (*models.ChatMessage, error) {
	collection := db.Database.Collection("chat_messages")

	// Save user message
	userMsg := &models.ChatMessage{
		SessionID: sessionID,
		UserID:    userID,
		Message:   message,
		Role:      "user",
		CreatedAt: time.Now(),
	}

	_, err := collection.InsertOne(context.Background(), userMsg)
	if err != nil {
		return nil, err
	}

	// Save assistant response
	assistantMsg := &models.ChatMessage{
		SessionID: sessionID,
		UserID:    userID,
		Response:  response,
		Role:      "assistant",
		CreatedAt: time.Now(),
	}

	result, err := collection.InsertOne(context.Background(), assistantMsg)
	if err != nil {
		return nil, err
	}

	assistantMsg.ID = result.InsertedID.(primitive.ObjectID)
	return assistantMsg, nil
}

func (db *DatabaseService) GetChatHistory(sessionID string, limit int) ([]models.ChatMessage, error) {
	collection := db.Database.Collection("chat_messages")

	filter := bson.M{"sessionId": sessionID}
	opts := options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: 1}}) // Ascending order

	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var messages []models.ChatMessage
	if err = cursor.All(context.Background(), &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (db *DatabaseService) Close() error {
	if dbService != nil && dbService.Client != nil {
		return dbService.Client.Disconnect(context.Background())
	}
	return nil
}
