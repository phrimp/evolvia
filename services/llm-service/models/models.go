package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents user information for RAG context
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"userId" json:"userId"`
	Name      string             `bson:"name" json:"name"`
	Email     string             `bson:"email" json:"email"`
	Profile   UserProfile        `bson:"profile" json:"profile"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type UserProfile struct {
	FirstName string `bson:"firstName" json:"firstName"`
	LastName  string `bson:"lastName" json:"lastName"`
	Avatar    string `bson:"avatar,omitempty" json:"avatar,omitempty"`
	Phone     string `bson:"phone,omitempty" json:"phone,omitempty"`
}

// Order represents user orders for RAG context
type Order struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"userId" json:"userId"`
	OrderID   string             `bson:"orderId" json:"orderId"`
	Products  []OrderProduct     `bson:"products" json:"products"`
	Total     float64            `bson:"total" json:"total"`
	Status    string             `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type OrderProduct struct {
	ProductID string  `bson:"productId" json:"productId"`
	Name      string  `bson:"name" json:"name"`
	Quantity  int     `bson:"quantity" json:"quantity"`
	Price     float64 `bson:"price" json:"price"`
}

// LLMRequest represents a request to the LLM service
type LLMRequest struct {
	Message   string                 `json:"message" binding:"required"`
	SessionID string                 `json:"sessionId,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// LLMResponse represents a response from the LLM service
type LLMResponse struct {
	Message   string    `json:"message"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
	Sources   []string  `json:"sources,omitempty"`
}

// ServiceStatus represents the status of the LLM service
type ServiceStatus struct {
	Service        string            `json:"service"`
	Version        string            `json:"version"`
	Status         string            `json:"status"`
	Timestamp      time.Time         `json:"timestamp"`
	Connection     ServiceConnection `json:"connection"`
	LLMModel       ModelStatus       `json:"llmModel"`
	EmbeddingModel ModelStatus       `json:"embeddingModel"`
}

type ServiceConnection struct {
	MongoDB  bool `json:"mongodb"`
	RabbitMQ bool `json:"rabbitmq"`
	LLMModel bool `json:"llmModel"`
}

// ModelStatus represents the status of a model (LLM or embedding)
type ModelStatus struct {
	Status   string `json:"status"`
	Model    string `json:"model"`
	BaseURL  string `json:"baseUrl"`
	Provider string `json:"provider"`
}

// ChatSession represents a chat session
type ChatSession struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID string             `bson:"sessionId" json:"sessionId"`
	UserID    string             `bson:"userId" json:"userId"`
	Title     string             `bson:"title,omitempty" json:"title,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	IsActive  bool               `bson:"isActive" json:"isActive"`
}

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID string             `bson:"sessionId" json:"sessionId"`
	UserID    string             `bson:"userId" json:"userId"`
	Message   string             `bson:"message" json:"message"`
	Response  string             `bson:"response" json:"response"`
	Role      string             `bson:"role" json:"role"` // "user" or "assistant"
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

// StreamingResponse represents a single chunk in a streaming response
type StreamingResponse struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object"`
	Created int64  `json:"created,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// StreamChunk represents a chunk of streamed data
type StreamChunk struct {
	Content   string    `json:"content"`
	IsEnd     bool      `json:"isEnd"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
