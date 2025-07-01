package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	// Fallback nếu không load được file
	return `Bạn có thể truy vấn database MongoDB để lấy thông tin người dùng.`
}

// Tạo context cho RAG với single database
func (r *RAGService) BuildRAGContext(userID string, userMessage string) string {
	if userID == "" {
		return "Người dùng chưa đăng nhập."
	}

	// Tạo context với thông tin cơ bản
	context := fmt.Sprintf(`
THÔNG TIN NGƯỜI DÙNG HIỆN TẠI:
- UserID: %s
- Tin nhắn: %s

DATABASE COLLECTIONS AVAILABLE:
- users: Chứa thông tin cá nhân người dùng (userId, name, email, profile)
- orders: Chứa thông tin đơn hàng (userId, orderId, products, total, status, createdAt)

HƯỚNG DẪN TẠO MONGODB QUERY:
%s

`, userID, userMessage, r.GetDatabasePrompt())

	// Thêm dữ liệu thực tế nếu có thể kết nối database
	if dbData := r.executeSmartQuery(userID, userMessage); dbData != "" {
		context += "\nDỮ LIỆU TỪ DATABASE:\n" + dbData
	}

	return context
}

// Thực hiện query thông minh dựa trên nội dung tin nhắn
func (r *RAGService) executeSmartQuery(userID string, message string) string {
	db := GetDatabaseService()
	if db == nil {
		return "Không thể kết nối database."
	}

	var results []string
	lowerMessage := strings.ToLower(message)

	// Nếu hỏi về thông tin cá nhân
	if r.isAskingAboutPersonalInfo(lowerMessage) {
		if userData := r.getUserData(db, userID); userData != "" {
			results = append(results, "THÔNG TIN CÁ NHÂN:\n"+userData)
		}
	}

	// Nếu hỏi về đơn hàng
	if r.isAskingAboutOrders(lowerMessage) {
		if orderData := r.getOrderData(db, userID); orderData != "" {
			results = append(results, "THÔNG TIN ĐỞN HÀNG:\n"+orderData)
		}
	}

	return strings.Join(results, "\n\n")
}

func (r *RAGService) getUserData(db *DatabaseService, userID string) string {
	// Use profile_service database with Profile collection (primary)
	collections := []string{"Profile", "profiles", "users", "user_profiles"}

	log.Printf("Looking for user with ID: %s in database: profile_service", userID)

	for _, collectionName := range collections {
		collection := db.ProfileDatabase.Collection(collectionName) // Use ProfileDatabase

		var result bson.M
		// Try with userId field first
		err := collection.FindOne(context.Background(), bson.M{"userId": userID}).Decode(&result)
		if err == nil {
			log.Printf("Found user in collection '%s' by userId field", collectionName)
			return r.formatUserData(result)
		}

		// Try with _id field as ObjectID
		if objectID, err := primitive.ObjectIDFromHex(userID); err == nil {
			err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&result)
			if err == nil {
				log.Printf("Found user in collection '%s' by _id field", collectionName)
				return r.formatUserData(result)
			}
		}

		log.Printf("User not found in collection '%s'", collectionName)
	}

	return ""
}

func (r *RAGService) formatUserData(result bson.M) string {
	name := "N/A"
	email := "N/A"
	profile := "N/A"

	// Try different field names for flat structure
	if nameVal, ok := result["name"]; ok && nameVal != nil {
		name = fmt.Sprintf("%v", nameVal)
	} else if nameVal, ok := result["fullName"]; ok && nameVal != nil {
		name = fmt.Sprintf("%v", nameVal)
	} else if nameVal, ok := result["displayName"]; ok && nameVal != nil {
		name = fmt.Sprintf("%v", nameVal)
	}

	// Check nested personalInfo structure
	if personalInfo, ok := result["personalInfo"].(bson.M); ok {
		var nameParts []string
		if firstName, exists := personalInfo["firstName"]; exists && firstName != nil {
			nameParts = append(nameParts, fmt.Sprintf("%v", firstName))
		}
		if lastName, exists := personalInfo["lastName"]; exists && lastName != nil {
			nameParts = append(nameParts, fmt.Sprintf("%v", lastName))
		}
		if len(nameParts) > 0 {
			name = strings.Join(nameParts, " ")
		}
		if displayName, exists := personalInfo["displayName"]; exists && displayName != nil && name == "N/A" {
			name = fmt.Sprintf("%v", displayName)
		}
	}

	// Check flat email field first
	if emailVal, ok := result["email"]; ok && emailVal != nil {
		email = fmt.Sprintf("%v", emailVal)
	}

	// Check nested contactInfo structure
	if contactInfo, ok := result["contactInfo"].(bson.M); ok {
		if emailVal, exists := contactInfo["email"]; exists && emailVal != nil {
			email = fmt.Sprintf("%v", emailVal)
		}
	}

	if profileVal, ok := result["profile"]; ok && profileVal != nil {
		profile = fmt.Sprintf("%v", profileVal)
	}

	log.Printf("User data: name=%s, email=%s", name, email)
	return fmt.Sprintf("- Tên: %s\n- Email: %s\n- Profile: %s", name, email, profile)
}

func (r *RAGService) getOrderData(db *DatabaseService, userID string) string {
	collection := db.Database.Collection("orders")

	// Đếm tổng số đơn hàng
	count, err := collection.CountDocuments(context.Background(), bson.M{"userId": userID})
	if err != nil {
		return ""
	}

	// Lấy 3 đơn hàng gần nhất
	cursor, err := collection.Find(context.Background(),
		bson.M{"userId": userID},
		nil) // Có thể thêm options sort và limit ở đây
	if err != nil {
		return ""
	}
	defer cursor.Close(context.Background())

	var orders []bson.M
	cursor.All(context.Background(), &orders)

	result := fmt.Sprintf("- Tổng số đơn hàng: %d\n", count)
	if len(orders) > 0 {
		result += "- Đơn hàng gần nhất:\n"
		for i, order := range orders {
			if i >= 3 { // Chỉ hiển thị 3 đơn gần nhất
				break
			}
			result += fmt.Sprintf("  + %v: %v (%.0f VND)\n",
				order["orderId"], order["status"], order["total"])
		}
	}

	return result
}

func (r *RAGService) isAskingAboutPersonalInfo(message string) bool {
	keywords := []string{"tên", "email", "thông tin", "profile", "hồ sơ", "cá nhân"}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func (r *RAGService) isAskingAboutOrders(message string) bool {
	keywords := []string{"đơn hàng", "order", "mua", "purchase", "giao dịch", "transaction"}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

// ExecuteCustomQuery - Cho phép LLM tự tạo và thực hiện query với single database
func (r *RAGService) ExecuteCustomQuery(userID string, databaseName string, collection string, query map[string]interface{}) ([]bson.M, error) {
	dbService := GetDatabaseService()
	if dbService == nil {
		return nil, fmt.Errorf("database service not available")
	}

	// Use default database (ignore databaseName parameter for single database setup)
	db := dbService.Database

	// Bảo mật: luôn thêm userID vào query cho các collection có userId
	secureCollections := []string{"users", "orders", "subscriptions", "profiles"}
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
