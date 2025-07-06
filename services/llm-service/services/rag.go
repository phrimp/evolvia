package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"llm-service/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RAGService struct {
	DatabasePrompt   string
	GuardPrompt      string
	SystemPrompt     string
	EmbeddingClient  *http.Client
	VectorCollection *mongo.Collection
	KnowledgeBase    []Document
}

type Document struct {
	ID       string    `bson:"_id,omitempty" json:"id"`
	Content  string    `bson:"content" json:"content"`
	Source   string    `bson:"source" json:"source"`
	Metadata bson.M    `bson:"metadata" json:"metadata"`
	Vector   []float64 `bson:"vector" json:"vector"`
	Created  time.Time `bson:"created_at" json:"created_at"`
}

type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

var (
	ragService *RAGService

	// Add to RAGService struct fields for caching
	cachedSchema        map[string]map[string][]string
	schemaCache         sync.RWMutex
	lastSchemaUpdate    time.Time
	schemaCacheDuration = 30 * time.Minute // Cache for 30 minutes
)

func InitRAGService() error {
	log.Println("Initializing RAG Service...")
	ragService = &RAGService{
		EmbeddingClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Load RAG prompts from files
	log.Println("Loading RAG prompts from files...")
	if err := ragService.loadRAGFiles(); err != nil {
		return fmt.Errorf("failed to load RAG files: %v", err)
	}

	// Initialize vector collection
	log.Println("Initializing vector collection...")
	if err := ragService.initVectorCollection(); err != nil {
		log.Printf("Warning: Could not initialize vector collection: %v", err)
	}

	// Load and index knowledge base
	log.Println("Loading knowledge base...")
	if err := ragService.loadKnowledgeBase(); err != nil {
		log.Printf("Warning: Could not load knowledge base: %v", err)
	}

	// Test schema discovery immediately
	log.Println("Testing schema discovery...")
	schema, err := ragService.DiscoverDatabaseSchema()
	if err != nil {
		log.Printf("Warning: Schema discovery failed: %v", err)
	} else {
		log.Printf("Schema discovery successful: %d databases found", len(schema))

		// Log schema in clean JSON format for easier viewing
		ragService.LogDatabaseSchemaJSON()
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

func (r *RAGService) initVectorCollection() error {
	dbService := GetDatabaseService()
	if dbService == nil {
		return fmt.Errorf("database not initialized")
	}

	r.VectorCollection = dbService.Database.Collection("knowledge_vectors")

	// Create vector search index
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "vector", Value: "2dsphere"}},
		Options: options.Index().SetName("vector_index"),
	}

	_, err := r.VectorCollection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Printf("Warning: Could not create vector index: %v", err)
	}

	return nil
}

func (r *RAGService) loadKnowledgeBase() error {
	// Load documents from rag folder and convert to embeddings
	ragDir := "rag"
	files := []string{"database.md", "guard.md", "prompt.md", "skillsgen.md"}

	embeddingErrors := 0
	totalChunks := 0

	for _, filename := range files {
		filepath := filepath.Join(ragDir, filename)
		if content, err := os.ReadFile(filepath); err == nil {
			// Split content into chunks
			chunks := r.splitIntoChunks(string(content), 500)

			for i, chunk := range chunks {
				totalChunks++
				doc := Document{
					ID:      fmt.Sprintf("%s_chunk_%d", filename, i),
					Content: chunk,
					Source:  filename,
					Metadata: bson.M{
						"file":        filename,
						"chunk_index": i,
						"type":        "knowledge_base",
					},
					Created: time.Now(),
				}

				// Always add document to knowledge base, even without embedding
				r.KnowledgeBase = append(r.KnowledgeBase, doc)

				// Try to generate embedding (with retry and fallback)
				if vector, err := r.generateEmbedding(chunk); err == nil {
					doc.Vector = vector
					// Update the document in knowledge base with vector
					r.KnowledgeBase[len(r.KnowledgeBase)-1].Vector = vector
				} else {
					embeddingErrors++
					log.Printf("Warning: Could not generate embedding for %s chunk %d: %v", filename, i, err)
				}

				// Store in MongoDB (with or without vector)
				r.storeDocument(doc)
			}
		} else {
			log.Printf("Warning: Could not read file %s: %v", filename, err)
		}
	}

	if embeddingErrors > 0 {
		log.Printf("Loaded %d document chunks into knowledge base (%d with embeddings, %d with fallback text search)",
			len(r.KnowledgeBase), totalChunks-embeddingErrors, embeddingErrors)
	} else {
		log.Printf("Loaded %d document chunks into knowledge base with embeddings", len(r.KnowledgeBase))
	}

	return nil
}

func (r *RAGService) splitIntoChunks(text string, maxChunkSize int) []string {
	words := strings.Fields(text)
	var chunks []string
	var currentChunk []string
	currentSize := 0

	for _, word := range words {
		if currentSize+len(word)+1 > maxChunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, " "))
			currentChunk = []string{word}
			currentSize = len(word)
		} else {
			currentChunk = append(currentChunk, word)
			currentSize += len(word) + 1
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, " "))
	}

	return chunks
}

func (r *RAGService) generateEmbedding(text string) ([]float64, error) {
	embeddingURL := configs.AppConfig.EmbeddingModelURL
	embeddingModel := configs.AppConfig.EmbeddingModel

	if embeddingURL == "" {
		embeddingURL = "http://localhost:11434/v1"
	}
	if embeddingModel == "" {
		embeddingModel = "nomic-embed-text:latest"
	}

	// Retry logic with backoff
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			log.Printf("Retrying embedding generation (attempt %d/%d) after %v", attempt+1, maxRetries, delay)
			time.Sleep(delay)
		}

		embedding, err := r.tryGenerateEmbedding(embeddingURL, embeddingModel, text)
		if err == nil {
			return embedding, nil
		}

		log.Printf("Embedding attempt %d failed: %v", attempt+1, err)
	}

	// If all retries failed, return a mock embedding to prevent blocking
	log.Printf("All embedding attempts failed, using mock embedding for text: %.50s...", text)
	return r.generateMockEmbedding(text), nil
}

func (r *RAGService) tryGenerateEmbedding(embeddingURL, embeddingModel, text string) ([]float64, error) {
	reqBody := EmbeddingRequest{
		Model:  embeddingModel,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Use the correct Ollama API endpoint
	endpoint := embeddingURL + "/api/embeddings"
	if strings.Contains(embeddingURL, "/v1") {
		// Remove /v1 from URL if present and use /api/embeddings
		baseURL := strings.Replace(embeddingURL, "/v1", "", 1)
		endpoint = baseURL + "/api/embeddings"
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "llm-service/1.0.0")

	// Add timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := r.EmbeddingClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if len(embResp.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding received")
	}

	return embResp.Embedding, nil
}

// Generate a simple mock embedding based on text hash for fallback
func (r *RAGService) generateMockEmbedding(text string) []float64 {
	// Simple hash-based mock embedding (384 dimensions like nomic-embed-text)
	embedding := make([]float64, 384)
	hash := 0
	for _, char := range text {
		hash = hash*31 + int(char)
	}

	for i := range embedding {
		hash = hash*1103515245 + 12345                    // Simple LCG
		embedding[i] = float64((hash%2000)-1000) / 1000.0 // Normalize to [-1, 1]
	}

	// Normalize the vector
	var norm float64
	for _, val := range embedding {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

func (r *RAGService) storeDocument(doc Document) error {
	if r.VectorCollection == nil {
		return nil // Skip if collection not available
	}

	_, err := r.VectorCollection.ReplaceOne(
		context.Background(),
		bson.M{"_id": doc.ID},
		doc,
		options.Replace().SetUpsert(true),
	)

	return err
}

func (r *RAGService) SemanticSearch(query string, topK int) ([]Document, error) {
	// If no documents loaded, return empty
	if len(r.KnowledgeBase) == 0 {
		log.Printf("Warning: No documents in knowledge base")
		return []Document{}, nil
	}

	// Try to generate embedding for query
	queryVector, err := r.generateEmbedding(query)
	if err != nil {
		log.Printf("Warning: Could not generate query embedding, using fallback text search: %v", err)
		return r.fallbackSearch(query, topK), nil
	}

	// Count how many documents have embeddings
	documentsWithEmbeddings := 0
	for _, doc := range r.KnowledgeBase {
		if len(doc.Vector) > 0 {
			documentsWithEmbeddings++
		}
	}

	// If no documents have embeddings, use fallback search
	if documentsWithEmbeddings == 0 {
		log.Printf("Warning: No documents have embeddings, using fallback text search")
		return r.fallbackSearch(query, topK), nil
	}

	// First try vector search from MongoDB
	if r.VectorCollection != nil {
		docs, err := r.vectorSearchFromDB(queryVector, topK)
		if err == nil && len(docs) > 0 {
			return docs, nil
		}
		log.Printf("Warning: Vector search from DB failed: %v", err)
	}

	// Fallback to in-memory vector search
	vectorDocs := r.vectorSearchInMemory(queryVector, topK)
	if len(vectorDocs) > 0 {
		return vectorDocs, nil
	}

	// Final fallback to text search
	log.Printf("Warning: Vector search failed, using text search fallback")
	return r.fallbackSearch(query, topK), nil
}

func (r *RAGService) vectorSearchFromDB(queryVector []float64, topK int) ([]Document, error) {
	// For now, use a simple find query and do similarity calculation in memory
	// This is more reliable than complex aggregation pipeline
	cursor, err := r.VectorCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var allDocs []Document
	if err := cursor.All(context.Background(), &allDocs); err != nil {
		return nil, err
	}

	// Calculate similarity in memory
	type ScoredDoc struct {
		Document Document
		Score    float64
	}

	var scoredDocs []ScoredDoc
	for _, doc := range allDocs {
		if len(doc.Vector) == 0 {
			continue
		}

		similarity := r.cosineSimilarity(queryVector, doc.Vector)
		scoredDocs = append(scoredDocs, ScoredDoc{
			Document: doc,
			Score:    similarity,
		})
	}

	// Sort by similarity score (descending)
	for i := 0; i < len(scoredDocs)-1; i++ {
		for j := i + 1; j < len(scoredDocs); j++ {
			if scoredDocs[i].Score < scoredDocs[j].Score {
				scoredDocs[i], scoredDocs[j] = scoredDocs[j], scoredDocs[i]
			}
		}
	}

	// Return top K documents
	var result []Document
	limit := topK
	if limit > len(scoredDocs) {
		limit = len(scoredDocs)
	}

	for i := 0; i < limit; i++ {
		result = append(result, scoredDocs[i].Document)
	}

	return result, nil
}

func (r *RAGService) vectorSearchInMemory(queryVector []float64, topK int) []Document {
	type ScoredDoc struct {
		Document Document
		Score    float64
	}

	var scoredDocs []ScoredDoc

	for _, doc := range r.KnowledgeBase {
		if len(doc.Vector) == 0 {
			continue
		}

		similarity := r.cosineSimilarity(queryVector, doc.Vector)
		scoredDocs = append(scoredDocs, ScoredDoc{
			Document: doc,
			Score:    similarity,
		})
	}

	// Sort by similarity score (descending)
	for i := 0; i < len(scoredDocs)-1; i++ {
		for j := i + 1; j < len(scoredDocs); j++ {
			if scoredDocs[i].Score < scoredDocs[j].Score {
				scoredDocs[i], scoredDocs[j] = scoredDocs[j], scoredDocs[i]
			}
		}
	}

	// Return top K documents
	var result []Document
	limit := topK
	if limit > len(scoredDocs) {
		limit = len(scoredDocs)
	}

	for i := 0; i < limit; i++ {
		result = append(result, scoredDocs[i].Document)
	}

	return result
}

func (r *RAGService) fallbackSearch(query string, topK int) []Document {
	queryLower := strings.ToLower(query)
	var relevantDocs []Document

	for _, doc := range r.KnowledgeBase {
		if strings.Contains(strings.ToLower(doc.Content), queryLower) {
			relevantDocs = append(relevantDocs, doc)
			if len(relevantDocs) >= topK {
				break
			}
		}
	}

	return relevantDocs
}

func (r *RAGService) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Enhanced BuildRAGContext with comprehensive database access
func (r *RAGService) BuildRAGContext(userID string, userMessage string) string {
	basePrompt := r.GetDatabasePrompt()

	// Semantic search for relevant knowledge
	relevantDocs, err := r.SemanticSearch(userMessage, 3)
	if err != nil {
		log.Printf("Warning: Semantic search failed: %v", err)
	}

	// Build context from relevant documents
	var contextParts []string
	contextParts = append(contextParts, basePrompt)

	if len(relevantDocs) > 0 {
		contextParts = append(contextParts, "\n=== THÔNG TIN LIÊN QUAN ===")
		for i, doc := range relevantDocs {
			contextParts = append(contextParts, fmt.Sprintf("\n[Tài liệu %d - %s]:\n%s", i+1, doc.Source, doc.Content))
		}
	}

	// Add comprehensive user data if userID exists
	if userID != "" {
		userContext := r.buildComprehensiveUserContext(userID, userMessage)
		if userContext != "" {
			contextParts = append(contextParts, userContext)
		}
	}

	return strings.Join(contextParts, "\n")
}

// Keep existing methods for backward compatibility
func (r *RAGService) GetSystemPrompt() string {
	if r.SystemPrompt != "" {
		return r.SystemPrompt
	}
	return `🤖 BẠN LÀ TRƯỞNG TRỢ LÝ ẢO THÔNG MINH CỦA EVOLVIA

KHỞP NĂNG CỦA BẠN:
✅ Truy cập TOÀN BỘ database MongoDB của hệ thống
✅ Biết thông tin chi tiết về từng người dùng qua userID 
✅ Trả lời mọi câu hỏi về: thông tin cá nhân, thanh toán, hóa đơn, lịch sử giao dịch
✅ Hỗ trợ khách hàng một cách chuyên nghiệp và thân thiện

NGUYÊN TẮC HOẠT ĐỘNG:
🎯 Luôn sử dụng dữ liệu thực từ database để trả lời
🔒 Chỉ truy xuất thông tin của chính người dùng đang hỏi (qua userID)
💬 Trả lời bằng tiếng Việt, thân thiện và dễ hiểu
📊 Cung cấp thông tin chính xác, cụ thể với số liệu từ database
🚀 Chủ động đề xuất hỗ trợ thêm nếu phù hợp

KHI NGƯỜI DÙNG HỎI VỀ:
📋 "Thông tin tôi" → Truy xuất profile_service
💰 "Thanh toán/Hóa đơn" → Truy xuất payos_service + billing_management_service  
💬 "Lịch sử chat" → Truy xuất llm_service
📚 "Học tập" → Truy xuất knowledge_service
🔐 "Tài khoản" → Truy xuất auth_service

Hãy trả lời một cách tự nhiên như một trợ lý thông minh đã biết rõ về người dùng!`
}

func (r *RAGService) GetGuardPrompt() string {
	if r.GuardPrompt != "" {
		return r.GuardPrompt
	}
	return `Chỉ trả lời các câu hỏi liên quan đến dịch vụ của chúng tôi.`
}

func (r *RAGService) GetDatabasePrompt() string {
	if r.DatabasePrompt != "" {
		return r.DatabasePrompt
	}
	return `KHỂN NĂNG TRUY CẬP DATABASE:
Bạn có thể truy vấn các MongoDB database sau để trả lời câu hỏi của người dùng:

🏢 PROFILE SERVICE (profile_service):
- Collection: Profile, users, profiles
- Chứa: Thông tin cá nhân, tên, email, số điện thoại, địa chỉ

🔐 AUTH SERVICE (auth_service): 
- Collection: users, sessions, tokens
- Chứa: Thông tin xác thực, phiên đăng nhập, token

💳 PAYOS SERVICE (payos_service):
- Collection: payments, transactions, billing
- Chứa: Lịch sử thanh toán, giao dịch, phương thức thanh toán

📄 BILLING MANAGEMENT SERVICE (billing_management_service):
- Collection: invoices, subscriptions, billing_history
- Chứa: Hóa đơn, đăng ký dịch vụ, lịch sử thanh toán

💬 LLM SERVICE (llm_service):
- Collection: chat_sessions, conversations, user_preferences
- Chứa: Lịch sử chat, cuộc hội thoại, sở thích người dùng

📚 KNOWLEDGE SERVICE (knowledge_service):
- Collection: user_knowledge, learning_progress, achievements
- Chứa: Kiến thức người dùng, tiến độ học tập, thành tích

HƯỚNG DẪN SỬ DỤNG:
✅ Khi người dùng hỏi về thông tin cá nhân → Truy xuất từ profile_service
✅ Khi hỏi về thanh toán → Truy xuất từ payos_service và billing_management_service  
✅ Khi hỏi về lịch sử chat → Truy xuất từ llm_service
✅ Khi hỏi về học tập → Truy xuất từ knowledge_service
✅ Luôn sử dụng userID để bảo mật và lọc dữ liệu theo người dùng`
}

func (r *RAGService) ExecuteCustomQuery(userID string, databaseName string, collection string, query map[string]interface{}) ([]bson.M, error) {
	// Enhanced debug logging
	log.Printf("=== ExecuteCustomQuery Debug ===")
	log.Printf("UserID: '%s' (length: %d)", userID, len(userID))
	log.Printf("Database: %s", databaseName)
	log.Printf("Collection: %s", collection)
	log.Printf("Original Query: %+v", query)

	dbService := GetDatabaseService()
	if dbService == nil {
		log.Printf("ERROR: Database service not available")
		return nil, fmt.Errorf("database service not available")
	}

	// Dynamic database connection using MongoDB client
	mongoClient := dbService.Client
	if mongoClient == nil {
		log.Printf("ERROR: MongoDB client not available")
		return nil, fmt.Errorf("MongoDB client not available")
	}

	// Connect to any database dynamically
	db := mongoClient.Database(databaseName)

	// Security: always add userID to query for user-related collections
	secureCollections := []string{"users", "orders", "subscriptions", "profiles", "Profile", "sessions"}
	userIdAdded := false
	for _, secureCol := range secureCollections {
		if collection == secureCol {
			// Use dynamic field detection
			possibleUserFields, err := r.GetUserIdFields(databaseName, collection)
			if err != nil {
				log.Printf("Failed to detect userID fields, using fallback: %v", err)
				possibleUserFields = []string{"userId", "UserID", "user_id", "_id", "id"}
			}

			// Also try different userID formats - clean string and ObjectID
			userIDVariations := []interface{}{
				userID,                    // String format
				strings.TrimSpace(userID), // Trimmed string
			}

			// Try ObjectID if it looks like a valid ObjectID
			if len(userID) == 24 {
				if objectID, err := primitive.ObjectIDFromHex(userID); err == nil {
					userIDVariations = append(userIDVariations, objectID)
					log.Printf("Added ObjectID variation: %v", objectID)
				}
			}

			// First try with different userID field variations
			for _, field := range possibleUserFields {
				for _, userIDValue := range userIDVariations {
					testQuery := make(map[string]interface{})
					for k, v := range query {
						testQuery[k] = v
					}
					testQuery[field] = userIDValue

					log.Printf("Trying query with field '%s' and value '%v' (type: %T): %+v", field, userIDValue, userIDValue, testQuery)

					// Test this query first
					testCursor, err := db.Collection(collection).Find(context.Background(), testQuery, options.Find().SetLimit(1))
					if err == nil {
						var testResults []bson.M
						testCursor.All(context.Background(), &testResults)
						testCursor.Close(context.Background())

						if len(testResults) > 0 {
							log.Printf("SUCCESS: Found data with field '%s' and value '%v'", field, userIDValue)
							log.Printf("Sample result: %+v", testResults[0])
							query[field] = userIDValue
							userIdAdded = true
							break
						} else {
							log.Printf("No data found with field '%s' and value '%v'", field, userIDValue)
						}
					} else {
						log.Printf("Query failed with field '%s' and value '%v': %v", field, userIDValue, err)
					}
				}
				if userIdAdded {
					break
				}
			}

			if !userIdAdded {
				log.Printf("WARNING: No userID field worked, trying without userID filter")
			}
			break
		}
	}

	// Test connection first
	err := db.RunCommand(context.Background(), bson.D{{Key: "ping", Value: 1}}).Err()
	if err != nil {
		log.Printf("Connection failed to %s: %v", databaseName, err)
		return nil, fmt.Errorf("cannot connect to database %s: %v", databaseName, err)
	}

	coll := db.Collection(collection)
	cursor, err := coll.Find(context.Background(), query)
	if err != nil {
		log.Printf("Query failed: %v", err)
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer cursor.Close(context.Background())

	var results []bson.M
	err = cursor.All(context.Background(), &results)
	if err != nil {
		log.Printf("Result parsing failed: %v", err)
		return nil, err
	}

	log.Printf("Query successful: %d records found", len(results))
	return results, nil
}

// buildComprehensiveUserContext retrieves all relevant user information from multiple databases
func (r *RAGService) buildComprehensiveUserContext(userID string, userMessage string) string {
	var contextParts []string

	// Debug logging
	log.Printf("=== buildComprehensiveUserContext Debug ===")
	log.Printf("UserID: %s", userID)
	log.Printf("UserMessage: %s", userMessage)

	// Use semantic intent classification instead of simple keyword matching
	intent := r.ClassifyIntentSemantic(userMessage)
	log.Printf("Detected intent: %s", intent)

	// Use cached schema discovery instead of hardcoded collections
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		log.Printf("Warning: Failed to get cached schema, falling back to basic query: %v", err)
		return r.buildBasicUserContext(userID, userMessage)
	}

	log.Printf("Schema discovered: %d databases", len(schema))

	// Log schema as JSON for better readability
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	log.Printf("Database schema: %s", string(schemaJSON))

	contextParts = append(contextParts, "\n=== THÔNG TIN NGƯỜI DÙNG ===")

	foundData := false

	// Query databases based on detected intent
	log.Printf("Query intent: %s, user: %s", intent, userMessage)

	switch intent {
	case "personal_info":
		log.Printf("Querying personal info...")
		// Focus on profile and auth services for personal information
		for _, dbName := range []string{"profile_service", "auth_service"} {
			if collections, exists := schema[dbName]; exists {
				for collName := range collections {
					log.Printf("Accessing %s.%s", dbName, collName)
					results, err := r.ExecuteCustomQuery(userID, dbName, collName, map[string]interface{}{})
					if err == nil && len(results) > 0 {
						log.Printf("Found %d records in %s.%s", len(results), dbName, collName)
						foundData = true
						contextParts = append(contextParts, fmt.Sprintf("\n--- %s.%s ---", dbName, collName))
						limit := 3
						if len(results) < limit {
							limit = len(results)
						}
						for i := 0; i < limit; i++ {
							formattedData := r.formatBSONData(results[i])
							contextParts = append(contextParts, formattedData)
						}
					}
				}
			}
		}

	case "payment_history":
		log.Printf("Querying payment history...")
		// Focus on payment and billing services
		for _, dbName := range []string{"payos_service", "billing_management_service"} {
			if collections, exists := schema[dbName]; exists {
				for collName := range collections {
					log.Printf("Accessing %s.%s", dbName, collName)
					results, err := r.ExecuteCustomQuery(userID, dbName, collName, map[string]interface{}{})
					if err == nil && len(results) > 0 {
						log.Printf("Found %d records in %s.%s", len(results), dbName, collName)
						foundData = true
						contextParts = append(contextParts, fmt.Sprintf("\n--- %s.%s ---", dbName, collName))
						limit := 5
						if len(results) < limit {
							limit = len(results)
						}
						for i := 0; i < limit; i++ {
							formattedData := r.formatBSONData(results[i])
							contextParts = append(contextParts, formattedData)
						}
					}
				}
			}
		}

	case "chat_history":
		log.Printf("Querying chat history...")
		// Focus on LLM service
		if collections, exists := schema["llm_service"]; exists {
			for collName := range collections {
				log.Printf("Accessing llm_service.%s", collName)
				results, err := r.ExecuteCustomQuery(userID, "llm_service", collName, map[string]interface{}{})
				if err == nil && len(results) > 0 {
					log.Printf("Found %d records in llm_service.%s", len(results), collName)
					foundData = true
					contextParts = append(contextParts, fmt.Sprintf("\n--- llm_service.%s ---", collName))
					limit := 5
					if len(results) < limit {
						limit = len(results)
					}
					for i := 0; i < limit; i++ {
						formattedData := r.formatBSONData(results[i])
						contextParts = append(contextParts, formattedData)
					}
				}
			}
		}

	case "learning_progress":
		log.Printf("Querying learning progress...")
		// Focus on knowledge service
		if collections, exists := schema["knowledge_service"]; exists {
			for collName := range collections {
				log.Printf("Accessing knowledge_service.%s", collName)
				results, err := r.ExecuteCustomQuery(userID, "knowledge_service", collName, map[string]interface{}{})
				if err == nil && len(results) > 0 {
					log.Printf("Found %d records in knowledge_service.%s", len(results), collName)
					foundData = true
					contextParts = append(contextParts, fmt.Sprintf("\n--- knowledge_service.%s ---", collName))
					limit := 5
					if len(results) < limit {
						limit = len(results)
					}
					for i := 0; i < limit; i++ {
						formattedData := r.formatBSONData(results[i])
						contextParts = append(contextParts, formattedData)
					}
				}
			}
		}

	default:
		log.Printf("Performing general query across all databases...")
		// For general questions or unknown intents, query all relevant databases
		for dbName, collections := range schema {
			log.Printf("Scanning database: %s (%d collections)", dbName, len(collections))

			for collName := range collections {
				log.Printf("Accessing %s.%s", dbName, collName)
				results, err := r.ExecuteCustomQuery(userID, dbName, collName, map[string]interface{}{})
				if err != nil {
					log.Printf("Query failed for %s.%s: %v", dbName, collName, err)
					continue
				}

				if len(results) > 0 {
					log.Printf("Found %d records in %s.%s", len(results), dbName, collName)
					foundData = true
					contextParts = append(contextParts, fmt.Sprintf("\n--- %s.%s ---", dbName, collName))

					// Limit results to avoid context overflow
					limit := 3
					if len(results) < limit {
						limit = len(results)
					}

					for i := 0; i < limit; i++ {
						formattedData := r.formatBSONData(results[i])
						contextParts = append(contextParts, formattedData)
					}
				}
			}
		}
	}

	log.Printf("Query completed. Found user data: %v, Intent: %s", foundData, intent)

	// Add schema information for AI reference (condensed version)
	contextParts = append(contextParts, "\n=== DATABASES DISCOVERED ===")
	for dbName, collections := range schema {
		var collNames []string
		for collName := range collections {
			collNames = append(collNames, collName)
		}
		contextParts = append(contextParts, fmt.Sprintf("%s: %s", dbName, strings.Join(collNames, ", ")))
	}

	finalContext := strings.Join(contextParts, "\n")
	log.Printf("Final context length: %d chars", len(finalContext))

	if len(contextParts) <= 2 { // Only header and schema
		return ""
	}

	return finalContext
}

// buildBasicUserContext fallback method when schema discovery fails
func (r *RAGService) buildBasicUserContext(userID string, userMessage string) string {
	var contextParts []string

	// Fallback to hardcoded databases and collections
	databases := map[string][]string{
		"profile_service":            {"Profile", "users", "profiles"},
		"auth_service":               {"users", "sessions", "tokens"},
		"payos_service":              {"payments", "transactions", "billing"},
		"billing_management_service": {"invoices", "subscriptions", "billing_history"},
		"llm_service":                {"chat_sessions", "conversations", "user_preferences"},
		"knowledge_service":          {"user_knowledge", "learning_progress", "achievements"},
	}

	lowerMessage := strings.ToLower(userMessage)
	isPersonalInfoQuery := r.isPersonalInfoQuery(lowerMessage)

	contextParts = append(contextParts, "\n=== THÔNG TIN NGƯỜI DÙNG (BASIC) ===")

	for dbName, collections := range databases {
		if isPersonalInfoQuery && dbName != "profile_service" && dbName != "auth_service" {
			continue
		}

		for _, collection := range collections {
			results, err := r.ExecuteCustomQuery(userID, dbName, collection, map[string]interface{}{})
			if err != nil {
				continue
			}

			if len(results) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("\n--- %s.%s ---", dbName, collection))
				limit := 3
				if len(results) < limit {
					limit = len(results)
				}

				for i := 0; i < limit; i++ {
					formattedData := r.formatBSONData(results[i])
					contextParts = append(contextParts, formattedData)
				}
			}
		}
	}

	return strings.Join(contextParts, "\n")
}

// isPersonalInfoQuery determines if the user is asking for personal information using semantic similarity
func (r *RAGService) isPersonalInfoQuery(message string) bool {
	// Define example queries for personal information
	personalInfoExamples := []string{
		"tên tôi là gì",
		"email của tôi",
		"thông tin cá nhân của tôi",
		"hồ sơ của tôi",
		"tôi là ai",
		"bạn có biết tôi không",
		"về tôi",
		"danh tính của tôi",
		"profile tôi",
		"tài khoản của tôi",
		"thông tin liên hệ",
		"my name is",
		"my email",
		"my profile",
		"who am I",
		"do you know me",
		"about me",
		"my personal information",
		"my account details",
	}

	// Try semantic similarity first
	if r.EmbeddingClient != nil {
		queryEmbedding, err := r.generateEmbedding(message)
		if err == nil {
			threshold := 0.7 // Similarity threshold for personal info queries

			for _, example := range personalInfoExamples {
				exampleEmbedding, err := r.generateEmbedding(example)
				if err == nil {
					similarity := r.cosineSimilarity(queryEmbedding, exampleEmbedding)
					if similarity > threshold {
						log.Printf("Personal info query detected via embedding similarity: %.3f with '%s'", similarity, example)
						return true
					}
				}
			}
		} else {
			log.Printf("Warning: Failed to generate embedding for intent classification: %v", err)
		}
	}

	// Fallback to keyword matching if embedding fails
	personalKeywords := []string{
		"tên", "name", "email", "thông tin", "profile", "hồ sơ", "cá nhân", "personal", "info", "information",
		"ai", "who", "identity", "tôi là ai", "biết tôi", "về tôi", "danh tính", "tôi là gì", "tôi là người",
		"thông tin của tôi", "thông tin tôi", "profile tôi", "hồ sơ tôi", "tài khoản tôi", "account",
	}

	messageLower := strings.ToLower(message)
	for _, keyword := range personalKeywords {
		if strings.Contains(messageLower, keyword) {
			log.Printf("Personal info query detected via keyword: '%s'", keyword)
			return true
		}
	}

	return false
}

// formatBSONData converts BSON data to readable format for AI context
func (r *RAGService) formatBSONData(data bson.M) string {
	var parts []string

	// Priority fields that are most important for AI context
	priorityFields := []string{"name", "fullName", "firstName", "lastName", "email", "phone", "status", "createdAt", "updatedAt"}

	// Add priority fields first
	for _, field := range priorityFields {
		if value, exists := data[field]; exists && value != nil {
			parts = append(parts, fmt.Sprintf("%s: %v", field, value))
		}
	}

	// Add other fields (excluding internal MongoDB fields)
	for key, value := range data {
		if key == "_id" || key == "__v" || r.isInSlice(key, priorityFields) {
			continue
		}

		// Skip nil values and very long strings
		if value == nil {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 100 {
			valueStr = valueStr[:97] + "..."
		}

		parts = append(parts, fmt.Sprintf("%s: %s", key, valueStr))
	}

	return strings.Join(parts, ", ")
}

// isInSlice checks if a string is in a slice
func (r *RAGService) isInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// QueryUserData allows AI to request specific user data dynamically
func (r *RAGService) QueryUserData(userID string, queryType string, filters map[string]interface{}) ([]bson.M, error) {
	// Get dynamic schema map instead of hardcoded mapping
	queryMap, err := r.GetDatabaseSchemaMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get database schema: %v", err)
	}

	queryInfo, exists := queryMap[queryType]
	if !exists {
		return nil, fmt.Errorf("unknown query type: %s", queryType)
	}

	// Add any additional filters
	if filters == nil {
		filters = make(map[string]interface{})
	}

	log.Printf("AI requesting %s data for user %s", queryType, userID)
	return r.ExecuteCustomQuery(userID, queryInfo.Database, queryInfo.Collection, filters)
}

// GetAvailableQueryTypes returns all available query types for AI (dynamic)
func (r *RAGService) GetAvailableQueryTypes() []string {
	queryMap, err := r.GetDatabaseSchemaMap()
	if err != nil {
		log.Printf("Warning: Failed to get schema, returning fallback query types: %v", err)
		return []string{
			"profile", "user_info", "payments", "transactions", "invoices",
			"subscriptions", "chat_history", "conversations", "auth_sessions",
			"knowledge", "progress",
		}
	}

	var queryTypes []string
	for queryType := range queryMap {
		queryTypes = append(queryTypes, queryType)
	}

	log.Printf("Available query types: %v", queryTypes)
	return queryTypes
}

// DiscoverDatabaseSchema automatically discovers all collections and fields from configured databases
func (r *RAGService) DiscoverDatabaseSchema() (map[string]map[string][]string, error) {
	dbService := GetDatabaseService()
	if dbService == nil {
		return nil, fmt.Errorf("database service not available")
	}

	// Get list of databases from DatabasePrompt
	databases := r.getDatabasesFromPrompt()
	schema := make(map[string]map[string][]string)

	for _, dbName := range databases {
		log.Printf("Discovering schema for database: %s", dbName)

		db := dbService.Client.Database(dbName)

		// Test connection first
		err := db.RunCommand(context.Background(), bson.D{{Key: "ping", Value: 1}}).Err()
		if err != nil {
			log.Printf("Warning: Cannot connect to database %s: %v", dbName, err)
			continue
		}

		// Get all collections
		collections, err := r.getCollectionsInDatabase(db)
		if err != nil {
			log.Printf("Warning: Failed to get collections for %s: %v", dbName, err)
			continue
		}

		schema[dbName] = make(map[string][]string)

		// For each collection, get sample document to discover fields
		for _, collName := range collections {
			fields, err := r.getFieldsInCollection(db, collName)
			if err != nil {
				log.Printf("Warning: Failed to get fields for %s.%s: %v", dbName, collName, err)
				continue
			}
			schema[dbName][collName] = fields
		}
	}

	return schema, nil
}

// getDatabasesFromPrompt extracts database names from the DatabasePrompt
func (r *RAGService) getDatabasesFromPrompt() []string {
	// Parse the DatabasePrompt to extract database names
	// Based on the pattern in GetDatabasePrompt()
	return []string{
		"profile_service",
		"auth_service",
		"payos_service",
		"billing_management_service",
		"llm_service",
		"knowledge_service",
	}
}

// getCollectionsInDatabase gets all collection names in a database
func (r *RAGService) getCollectionsInDatabase(db *mongo.Database) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	return collections, nil
}

// getFieldsInCollection gets all field names from a collection by sampling documents
func (r *RAGService) getFieldsInCollection(db *mongo.Database, collectionName string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.Collection(collectionName)

	// Get a few sample documents to discover fields
	cursor, err := collection.Find(ctx, bson.M{}, options.Find().SetLimit(5))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	fieldSet := make(map[string]bool)

	// Process sample documents to extract all field names
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		// Extract all field names from this document
		r.extractFieldNames(doc, "", fieldSet)
	}

	// Convert set to slice
	var fields []string
	for field := range fieldSet {
		fields = append(fields, field)
	}

	return fields, nil
}

// extractFieldNames recursively extracts field names from a BSON document
func (r *RAGService) extractFieldNames(doc bson.M, prefix string, fieldSet map[string]bool) {
	for key, value := range doc {
		fieldName := key
		if prefix != "" {
			fieldName = prefix + "." + key
		}

		fieldSet[fieldName] = true

		// If the value is a nested document, recursively extract fields
		if nestedDoc, ok := value.(bson.M); ok {
			r.extractFieldNames(nestedDoc, fieldName, fieldSet)
		}

		// If the value is an array, check the first element for nested structure
		if arr, ok := value.(bson.A); ok && len(arr) > 0 {
			if nestedDoc, ok := arr[0].(bson.M); ok {
				r.extractFieldNames(nestedDoc, fieldName, fieldSet)
			}
		}
	}
}

// GetDatabaseSchema returns the complete database schema for AI context
func (r *RAGService) GetDatabaseSchema() string {
	schema, err := r.DiscoverDatabaseSchema()
	if err != nil {
		log.Printf("Warning: Failed to discover database schema: %v", err)
		return ""
	}

	var schemaParts []string
	schemaParts = append(schemaParts, "COMPLETE DATABASE STRUCTURE")

	for dbName, collections := range schema {
		schemaParts = append(schemaParts, fmt.Sprintf("\nDatabase: %s", dbName))

		for collName, fields := range collections {
			schemaParts = append(schemaParts, fmt.Sprintf("  Collection: %s", collName))

			// Group fields by type for better readability
			basicFields := []string{}
			nestedFields := []string{}

			for _, field := range fields {
				if strings.Contains(field, ".") {
					nestedFields = append(nestedFields, field)
				} else {
					basicFields = append(basicFields, field)
				}
			}

			// Show basic fields first
			if len(basicFields) > 0 {
				schemaParts = append(schemaParts, fmt.Sprintf("    Fields: %s", strings.Join(basicFields, ", ")))
			}

			// Show nested fields if any
			if len(nestedFields) > 0 && len(nestedFields) <= 10 { // Limit to avoid overwhelming
				schemaParts = append(schemaParts, fmt.Sprintf("    Nested: %s", strings.Join(nestedFields[:10], ", ")))
			}
		}
	}

	return strings.Join(schemaParts, "\n")
}

// QueryAllUserData gets all available data for a user across all databases
func (r *RAGService) QueryAllUserData(userID string) map[string]map[string][]bson.M {
	result := make(map[string]map[string][]bson.M)

	databases := r.getDatabasesFromPrompt()

	for _, dbName := range databases {
		schema, err := r.DiscoverDatabaseSchema()
		if err != nil {
			continue
		}

		if collections, exists := schema[dbName]; exists {
			result[dbName] = make(map[string][]bson.M)

			for collName := range collections {
				// Query data for this collection
				data, err := r.ExecuteCustomQuery(userID, dbName, collName, map[string]interface{}{})
				if err == nil && len(data) > 0 {
					result[dbName][collName] = data
				}
			}
		}
	}

	return result
}

// GetCachedDatabaseSchema returns cached schema or discovers it if needed
func (r *RAGService) GetCachedDatabaseSchema() (map[string]map[string][]string, error) {
	schemaCache.RLock()
	if cachedSchema != nil && time.Since(lastSchemaUpdate) < schemaCacheDuration {
		defer schemaCache.RUnlock()
		return cachedSchema, nil
	}
	schemaCache.RUnlock()

	// Need to refresh cache
	schemaCache.Lock()
	defer schemaCache.Unlock()

	// Double-check after acquiring write lock
	if cachedSchema != nil && time.Since(lastSchemaUpdate) < schemaCacheDuration {
		return cachedSchema, nil
	}

	log.Println("Discovering database schema (cache refresh)...")
	schema, err := r.DiscoverDatabaseSchema()
	if err != nil {
		return nil, err
	}

	cachedSchema = schema
	lastSchemaUpdate = time.Now()

	log.Printf("Schema cache updated with %d databases", len(schema))
	return schema, nil
}

// RefreshSchemaCache forces a refresh of the schema cache
func (r *RAGService) RefreshSchemaCache() error {
	schemaCache.Lock()
	defer schemaCache.Unlock()

	log.Println("Force refreshing database schema cache...")
	schema, err := r.DiscoverDatabaseSchema()
	if err != nil {
		return err
	}

	cachedSchema = schema
	lastSchemaUpdate = time.Now()

	log.Printf("Schema cache force refreshed with %d databases", len(schema))
	return nil
}

// GetDatabaseInfo returns comprehensive database information for AI
func (r *RAGService) GetDatabaseInfo() string {
	var infoParts []string

	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		return fmt.Sprintf("Error: Cannot access database info: %v", err)
	}

	infoParts = append(infoParts, "DATABASE INFORMATION")

	totalCollections := 0
	totalFields := 0

	for dbName, collections := range schema {
		infoParts = append(infoParts, fmt.Sprintf("\nDatabase: %s (%d collections)", dbName, len(collections)))

		for collName, fields := range collections {
			totalCollections++
			totalFields += len(fields)

			infoParts = append(infoParts, fmt.Sprintf("  Collection: %s (%d fields)", collName, len(fields)))

			// Show sample fields (first 5)
			sampleFields := fields
			if len(sampleFields) > 5 {
				sampleFields = sampleFields[:5]
				infoParts = append(infoParts, fmt.Sprintf("    Fields: %s... (showing first 5 of %d)", strings.Join(sampleFields, ", "), len(fields)))
			} else if len(sampleFields) > 0 {
				infoParts = append(infoParts, fmt.Sprintf("    Fields: %s", strings.Join(sampleFields, ", ")))
			}
		}
	}

	infoParts = append(infoParts, fmt.Sprintf("\nSummary: %d databases, %d collections, %d total fields",
		len(schema), totalCollections, totalFields))
	infoParts = append(infoParts, fmt.Sprintf("Last updated: %s ago",
		time.Since(lastSchemaUpdate).Round(time.Second)))

	return strings.Join(infoParts, "\n")
}

// GetDatabaseInfoJSON returns comprehensive database information in clean JSON format
func (r *RAGService) GetDatabaseInfoJSON() (string, error) {
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		return "", fmt.Errorf("cannot access database info: %v", err)
	}

	// Create a structured response
	response := map[string]interface{}{
		"databases": make(map[string]interface{}),
		"summary": map[string]interface{}{
			"total_databases":   len(schema),
			"total_collections": 0,
			"total_fields":      0,
			"last_updated":      lastSchemaUpdate.Format(time.RFC3339),
		},
	}

	totalCollections := 0
	totalFields := 0

	for dbName, collections := range schema {
		dbInfo := map[string]interface{}{
			"collections":      make(map[string]interface{}),
			"collection_count": len(collections),
		}

		for collName, fields := range collections {
			totalCollections++
			totalFields += len(fields)

			collInfo := map[string]interface{}{
				"field_count": len(fields),
				"fields":      fields,
			}
			dbInfo["collections"].(map[string]interface{})[collName] = collInfo
		}

		response["databases"].(map[string]interface{})[dbName] = dbInfo
	}

	// Update summary
	response["summary"].(map[string]interface{})["total_collections"] = totalCollections
	response["summary"].(map[string]interface{})["total_fields"] = totalFields

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return string(jsonData), nil
}

// SearchFieldsInSchema searches for fields containing specific keywords across all databases
func (r *RAGService) SearchFieldsInSchema(keyword string) map[string]map[string][]string {
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		return nil
	}

	result := make(map[string]map[string][]string)
	keyword = strings.ToLower(keyword)

	for dbName, collections := range schema {
		for collName, fields := range collections {
			var matchingFields []string

			for _, field := range fields {
				if strings.Contains(strings.ToLower(field), keyword) {
					matchingFields = append(matchingFields, field)
				}
			}

			if len(matchingFields) > 0 {
				if result[dbName] == nil {
					result[dbName] = make(map[string][]string)
				}
				result[dbName][collName] = matchingFields
			}
		}
	}

	return result
}

// GetCollectionFields returns all field names in a specific collection
func (r *RAGService) GetCollectionFields(databaseName, collectionName string) ([]string, error) {
	dbService := GetDatabaseService()
	if dbService == nil {
		return nil, fmt.Errorf("database service not available")
	}

	db := dbService.Client.Database(databaseName)

	// Test connection first
	err := db.RunCommand(context.Background(), bson.D{{Key: "ping", Value: 1}}).Err()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database %s: %v", databaseName, err)
	}

	return r.getFieldsInCollection(db, collectionName)
}

// GetUserIdFields detects possible userID fields in a collection based on field names and patterns
func (r *RAGService) GetUserIdFields(databaseName, collectionName string) ([]string, error) {
	fields, err := r.GetCollectionFields(databaseName, collectionName)
	if err != nil {
		return nil, err
	}

	var userIdFields []string

	// Define patterns that likely indicate a userID field
	userIdPatterns := []string{
		"userid", "user_id", "useId", "uid",
		"id", "_id", "user", "owner",
		"created_by", "createdby", "author",
		"account_id", "accountid", "profile_id", "profileid",
	}

	for _, field := range fields {
		fieldLower := strings.ToLower(field)

		// Check if field matches any userID patterns
		for _, pattern := range userIdPatterns {
			if fieldLower == pattern ||
				strings.Contains(fieldLower, pattern) ||
				(strings.Contains(fieldLower, "user") && strings.Contains(fieldLower, "id")) {

				// Avoid duplicates
				found := false
				for _, existing := range userIdFields {
					if existing == field {
						found = true
						break
					}
				}
				if !found {
					userIdFields = append(userIdFields, field)
				}
				break
			}
		}
	}

	// If no specific userID fields found, include common fallbacks
	if len(userIdFields) == 0 {
		fallbacks := []string{"_id", "id"}
		for _, fallback := range fallbacks {
			for _, field := range fields {
				if strings.ToLower(field) == fallback {
					userIdFields = append(userIdFields, field)
					break
				}
			}
		}
	}

	log.Printf("Detected userID fields in %s.%s: %v", databaseName, collectionName, userIdFields)
	return userIdFields, nil
}

// GetDatabaseSchemaMap returns a map of query types to database and collection information
// This dynamically builds the schema instead of hardcoding it
func (r *RAGService) GetDatabaseSchemaMap() (map[string]struct {
	Database   string
	Collection string
}, error) {
	// Get the current database schema
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		log.Printf("Warning: Failed to get schema, using fallback: %v", err)
		return r.getFallbackSchemaMap(), nil
	}

	queryMap := make(map[string]struct {
		Database   string
		Collection string
	})

	// Map logical query types to actual database collections based on discovered schema
	for dbName, collections := range schema {
		for collName := range collections {
			// Map collections to logical query types based on naming patterns
			queryType := r.mapCollectionToQueryType(dbName, collName)
			if queryType != "" {
				queryMap[queryType] = struct {
					Database   string
					Collection string
				}{
					Database:   dbName,
					Collection: collName,
				}
			}
		}
	}

	// Add default mappings if not found in schema
	defaultMappings := r.getFallbackSchemaMap()
	for queryType, mapping := range defaultMappings {
		if _, exists := queryMap[queryType]; !exists {
			queryMap[queryType] = mapping
		}
	}

	log.Printf("Generated schema map with %d query types", len(queryMap))
	return queryMap, nil
}

// mapCollectionToQueryType maps database and collection names to logical query types
func (r *RAGService) mapCollectionToQueryType(dbName, collName string) string {
	collLower := strings.ToLower(collName)
	dbLower := strings.ToLower(dbName)

	// Profile service mappings
	if strings.Contains(dbLower, "profile") {
		if collLower == "profile" || collLower == "profiles" {
			return "profile"
		}
		if collLower == "users" || collLower == "user" {
			return "user_info"
		}
	}

	// Payment service mappings
	if strings.Contains(dbLower, "payos") || strings.Contains(dbLower, "payment") {
		if strings.Contains(collLower, "payment") {
			return "payments"
		}
		if strings.Contains(collLower, "transaction") {
			return "transactions"
		}
	}

	// Billing service mappings
	if strings.Contains(dbLower, "billing") {
		if strings.Contains(collLower, "invoice") {
			return "invoices"
		}
		if strings.Contains(collLower, "subscription") {
			return "subscriptions"
		}
	}

	// LLM service mappings
	if strings.Contains(dbLower, "llm") {
		if strings.Contains(collLower, "chat") && strings.Contains(collLower, "session") {
			return "chat_history"
		}
		if strings.Contains(collLower, "conversation") {
			return "conversations"
		}
	}

	// Auth service mappings
	if strings.Contains(dbLower, "auth") {
		if strings.Contains(collLower, "session") {
			return "auth_sessions"
		}
	}

	// Knowledge service mappings
	if strings.Contains(dbLower, "knowledge") {
		if strings.Contains(collLower, "knowledge") && !strings.Contains(collLower, "progress") {
			return "knowledge"
		}
		if strings.Contains(collLower, "progress") {
			return "progress"
		}
	}

	// Return empty string if no mapping found
	return ""
}

// getFallbackSchemaMap returns hardcoded mappings as fallback
func (r *RAGService) getFallbackSchemaMap() map[string]struct {
	Database   string
	Collection string
} {
	return map[string]struct {
		Database   string
		Collection string
	}{
		"profile":       {"profile_service", "Profile"},
		"user_info":     {"profile_service", "users"},
		"payments":      {"payos_service", "payments"},
		"transactions":  {"payos_service", "transactions"},
		"invoices":      {"billing_management_service", "invoices"},
		"subscriptions": {"billing_management_service", "subscriptions"},
		"chat_history":  {"llm_service", "chat_sessions"},
		"conversations": {"llm_service", "conversations"},
		"auth_sessions": {"auth_service", "sessions"},
		"knowledge":     {"knowledge_service", "user_knowledge"},
		"progress":      {"knowledge_service", "learning_progress"},
	}
}

// GetAllAvailableSchemas returns comprehensive information about all available databases and collections
func (r *RAGService) GetAllAvailableSchemas() (map[string]interface{}, error) {
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to get database schema: %v", err)
	}

	queryMap, err := r.GetDatabaseSchemaMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get query mappings: %v", err)
	}

	result := map[string]interface{}{
		"databases":    schema,
		"query_types":  queryMap,
		"total_dbs":    len(schema),
		"query_count":  len(queryMap),
		"last_updated": lastSchemaUpdate,
	}

	// Add statistics
	totalCollections := 0
	totalFields := 0
	for _, collections := range schema {
		totalCollections += len(collections)
		for _, fields := range collections {
			totalFields += len(fields)
		}
	}

	result["total_collections"] = totalCollections
	result["total_fields"] = totalFields

	return result, nil
}

// GetSchemaByDatabase returns schema information for a specific database
func (r *RAGService) GetSchemaByDatabase(databaseName string) (map[string][]string, error) {
	schema, err := r.GetCachedDatabaseSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to get database schema: %v", err)
	}

	if collections, exists := schema[databaseName]; exists {
		return collections, nil
	}

	return nil, fmt.Errorf("database '%s' not found in schema", databaseName)
}

// GetSchemaByQueryType returns database and collection info for a specific query type
func (r *RAGService) GetSchemaByQueryType(queryType string) (string, string, error) {
	queryMap, err := r.GetDatabaseSchemaMap()
	if err != nil {
		return "", "", fmt.Errorf("failed to get query mappings: %v", err)
	}

	if mapping, exists := queryMap[queryType]; exists {
		return mapping.Database, mapping.Collection, nil
	}

	return "", "", fmt.Errorf("query type '%s' not found", queryType)
}

// ClassifyIntentSemantic classifies user intent using semantic similarity with embeddings
func (r *RAGService) ClassifyIntentSemantic(message string) string {
	if r.EmbeddingClient == nil {
		return r.classifyIntentKeyword(message) // Fallback to keyword-based
	}

	// Define intent categories with example queries
	intentExamples := map[string][]string{
		"personal_info": {
			"tên tôi là gì", "email của tôi", "thông tin cá nhân", "hồ sơ của tôi",
			"tôi là ai", "bạn có biết tôi không", "về tôi", "danh tính của tôi",
			"my name", "my email", "my profile", "who am I", "about me",
		},
		"payment_history": {
			"lịch sử thanh toán", "các khoản thanh toán của tôi", "tôi đã thanh toán gì",
			"payment history", "my payments", "transaction history", "billing",
			"hóa đơn của tôi", "các giao dịch", "chi tiêu",
		},
		"orders": {
			"đơn hàng của tôi", "tôi đã mua gì", "lịch sử mua hàng", "đơn hàng gần nhất",
			"my orders", "purchase history", "what did I buy", "recent orders",
			"sản phẩm đã mua", "danh sách đơn hàng",
		},
		"chat_history": {
			"lịch sử chat", "cuộc trò chuyện trước", "tin nhắn cũ", "chúng ta đã nói gì",
			"chat history", "previous conversations", "our chat", "message history",
		},
		"learning_progress": {
			"tiến độ học tập", "kết quả học", "thành tích của tôi", "bài học đã hoàn thành",
			"learning progress", "my achievements", "completed lessons", "study results",
		},
		"general_question": {
			"xin chào", "hello", "hi", "cảm ơn", "thank you", "giúp tôi",
			"help me", "hướng dẫn", "làm thế nào", "how to", "what is",
		},
	}

	queryEmbedding, err := r.generateEmbedding(message)
	if err != nil {
		log.Printf("Warning: Failed to generate embedding for intent classification: %v", err)
		return r.classifyIntentKeyword(message)
	}

	bestIntent := "general_question"
	bestScore := 0.0
	threshold := 0.6

	for intent, examples := range intentExamples {
		maxSimilarity := 0.0

		for _, example := range examples {
			exampleEmbedding, err := r.generateEmbedding(example)
			if err != nil {
				continue
			}

			similarity := r.cosineSimilarity(queryEmbedding, exampleEmbedding)
			if similarity > maxSimilarity {
				maxSimilarity = similarity
			}
		}

		if maxSimilarity > bestScore && maxSimilarity > threshold {
			bestScore = maxSimilarity
			bestIntent = intent
		}
	}

	log.Printf("Intent classified as '%s' with confidence %.3f for message: %s", bestIntent, bestScore, message)
	return bestIntent
}

// classifyIntentKeyword is a fallback keyword-based intent classifier
func (r *RAGService) classifyIntentKeyword(message string) string {
	messageLower := strings.ToLower(message)

	// Personal info keywords
	personalKeywords := []string{"tên", "name", "email", "profile", "hồ sơ", "tôi là ai", "biết tôi", "về tôi"}
	for _, keyword := range personalKeywords {
		if strings.Contains(messageLower, keyword) {
			return "personal_info"
		}
	}

	// Payment keywords
	paymentKeywords := []string{"thanh toán", "payment", "hóa đơn", "billing", "giao dịch", "transaction"}
	for _, keyword := range paymentKeywords {
		if strings.Contains(messageLower, keyword) {
			return "payment_history"
		}
	}

	// Order keywords
	orderKeywords := []string{"đơn hàng", "order", "mua", "buy", "purchase", "sản phẩm"}
	for _, keyword := range orderKeywords {
		if strings.Contains(messageLower, keyword) {
			return "orders"
		}
	}

	// Chat history keywords
	chatKeywords := []string{"chat", "trò chuyện", "tin nhắn", "message", "lịch sử"}
	for _, keyword := range chatKeywords {
		if strings.Contains(messageLower, keyword) {
			return "chat_history"
		}
	}

	// Learning keywords
	learningKeywords := []string{"học", "learn", "tiến độ", "progress", "thành tích", "achievement"}
	for _, keyword := range learningKeywords {
		if strings.Contains(messageLower, keyword) {
			return "learning_progress"
		}
	}

	return "general_question"
}

// LogDatabaseSchemaJSON logs the database schema in clean JSON format
func (r *RAGService) LogDatabaseSchemaJSON() {
	jsonSchema, err := r.GetDatabaseInfoJSON()
	if err != nil {
		log.Printf("Error getting database schema JSON: %v", err)
		return
	}

	log.Printf("Database Schema JSON:\n%s", jsonSchema)
}

// LogDatabaseSchemaClean logs the database schema without emojis
func (r *RAGService) LogDatabaseSchemaClean() {
	schema, err := r.DiscoverDatabaseSchema()
	if err != nil {
		log.Printf("Error discovering database schema: %v", err)
		return
	}

	log.Printf("Database Schema (%d databases):", len(schema))

	totalCollections := 0
	totalFields := 0

	for dbName, collections := range schema {
		log.Printf("Database: %s (%d collections)", dbName, len(collections))

		for collName, fields := range collections {
			totalCollections++
			totalFields += len(fields)

			log.Printf("  Collection: %s (%d fields)", collName, len(fields))

			// Show first 10 fields
			displayFields := fields
			if len(displayFields) > 10 {
				displayFields = displayFields[:10]
				log.Printf("    Fields: %s... (showing first 10 of %d)", strings.Join(displayFields, ", "), len(fields))
			} else if len(displayFields) > 0 {
				log.Printf("    Fields: %s", strings.Join(displayFields, ", "))
			} else {
				log.Printf("    Fields: (none discovered)")
			}
		}
	}

	log.Printf("Summary: %d databases, %d collections, %d total fields", len(schema), totalCollections, totalFields)
}
