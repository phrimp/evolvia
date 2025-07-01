package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type RAGService struct {
	DatabasePrompt string
	GuardPrompt    string
	SystemPrompt   string
}

var ragService *RAGService

func InitRAGService() error {
	ragService = &RAGService{}

	// Load RAG prompts from files
	if err := ragService.loadRAGFiles(); err != nil {
		return fmt.Errorf("failed to load RAG files: %v", err)
	}

	log.Println("RAG Service initialized successfully")
	return nil
}

func GetRAGService() *RAGService {
	return ragService
}

func (r *RAGService) loadRAGFiles() error {
	// Load database.md
	databasePath := filepath.Join("rag", "database.md")
	if content, err := os.ReadFile(databasePath); err == nil {
		r.DatabasePrompt = string(content)
	} else {
		log.Printf("Warning: Could not load %s: %v", databasePath, err)
	}

	// Load guard.md
	guardPath := filepath.Join("rag", "guard.md")
	if content, err := os.ReadFile(guardPath); err == nil {
		r.GuardPrompt = string(content)
	} else {
		log.Printf("Warning: Could not load %s: %v", guardPath, err)
	}

	// Load prompt.md
	promptPath := filepath.Join("rag", "prompt.md")
	if content, err := os.ReadFile(promptPath); err == nil {
		r.SystemPrompt = string(content)
	} else {
		log.Printf("Warning: Could not load %s: %v", promptPath, err)
	}

	return nil
}

func (r *RAGService) GetSystemPrompt() string {
	if r.SystemPrompt != "" {
		return r.SystemPrompt
	}
	// Fallback nếu không load được file
	return `Bạn là trợ lý ảo thông minh của nền tảng Evolvia.`
}

func (r *RAGService) GetGuardPrompt() string {
	if r.GuardPrompt != "" {
		return r.GuardPrompt
	}
	// Fallback nếu không load được file
	return `Chỉ trả lời các câu hỏi liên quan đến dịch vụ của chúng tôi.`
}

func (r *RAGService) GetDatabasePrompt() string {
	if r.DatabasePrompt != "" {
		return r.DatabasePrompt
	}
	// Fallback with comprehensive database info
	return `Bạn có thể truy vấn các database MongoDB sau:
- llm_service: Lưu session chat
- auth_service: Xác thực người dùng  
- profile_service: Thông tin hồ sơ (Collection: Profile)
- payos_service: Thanh toán
- billing_management_service: Hóa đơn
- knowledge_service: Cơ sở tri thức`
}

// BuildRAGContext - Tạo context đơn giản từ database.md
func (r *RAGService) BuildRAGContext(userID string, userMessage string) string {
	basePrompt := r.GetDatabasePrompt()

	// Add user data if userID exists
	if userID != "" {
		if results, err := r.ExecuteCustomQuery(userID, "profile_service", "Profile", map[string]interface{}{}); err == nil && len(results) > 0 {
			return fmt.Sprintf("%s\n\nDỮ LIỆU NGƯỜI DÙNG:\n%+v", basePrompt, results[0])
		}
	}

	return basePrompt
}

// ExecuteCustomQuery - Cho phép LLM tự tạo và thực hiện query với multiple databases
func (r *RAGService) ExecuteCustomQuery(userID string, databaseName string, collection string, query map[string]interface{}) ([]bson.M, error) {
	dbService := GetDatabaseService()
	if dbService == nil {
		return nil, fmt.Errorf("database service not available")
	}

	// Select database based on databaseName parameter
	var db *mongo.Database
	switch databaseName {
	case "profile_service":
		db = dbService.ProfileDatabase
	case "llm_service":
		db = dbService.Database
	case "auth_service", "payos_service", "billing_management_service", "knowledge_service":
		// These would need to be added to DatabaseService if needed
		return nil, fmt.Errorf("database %s not configured", databaseName)
	default:
		// Use default database
		db = dbService.Database
	}

	// Security: always add userID to query for user-related collections
	secureCollections := []string{"users", "orders", "subscriptions", "profiles", "Profile"}
	for _, secureCol := range secureCollections {
		if collection == secureCol {
			query["userId"] = userID
			break
		}
	}

	coll := db.Collection(collection)
	cursor, err := coll.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	err = cursor.All(context.Background(), &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}
