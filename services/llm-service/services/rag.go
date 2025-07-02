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
	"time"

	"llm-service/configs"

	"go.mongodb.org/mongo-driver/bson"
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

var ragService *RAGService

func InitRAGService() error {
	ragService = &RAGService{
		EmbeddingClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Load RAG prompts from files
	if err := ragService.loadRAGFiles(); err != nil {
		return fmt.Errorf("failed to load RAG files: %v", err)
	}

	// Initialize vector collection
	if err := ragService.initVectorCollection(); err != nil {
		log.Printf("Warning: Could not initialize vector collection: %v", err)
	}

	// Load and index knowledge base
	if err := ragService.loadKnowledgeBase(); err != nil {
		log.Printf("Warning: Could not load knowledge base: %v", err)
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

	for _, filename := range files {
		filepath := filepath.Join(ragDir, filename)
		if content, err := os.ReadFile(filepath); err == nil {
			// Split content into chunks
			chunks := r.splitIntoChunks(string(content), 500)

			for i, chunk := range chunks {
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

				// Generate embedding
				if vector, err := r.generateEmbedding(chunk); err == nil {
					doc.Vector = vector
					r.KnowledgeBase = append(r.KnowledgeBase, doc)

					// Store in MongoDB
					r.storeDocument(doc)
				} else {
					log.Printf("Warning: Could not generate embedding for %s chunk %d: %v", filename, i, err)
				}
			}
		}
	}

	log.Printf("Loaded %d document chunks into knowledge base", len(r.KnowledgeBase))
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

	reqBody := EmbeddingRequest{
		Model:  embeddingModel,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", embeddingURL+"/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.EmbeddingClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, err
	}

	return embResp.Embedding, nil
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
	// Generate embedding for query
	queryVector, err := r.generateEmbedding(query)
	if err != nil {
		log.Printf("Warning: Could not generate query embedding, using fallback: %v", err)
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

	// Fallback to in-memory search
	return r.vectorSearchInMemory(queryVector, topK), nil
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

// Enhanced BuildRAGContext with semantic search
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

	// Add user data if userID exists
	if userID != "" {
		if results, err := r.ExecuteCustomQuery(userID, "profile_service", "Profile", map[string]interface{}{}); err == nil && len(results) > 0 {
			contextParts = append(contextParts, fmt.Sprintf("\n=== DỮ LIỆU NGƯỜI DÙNG ===\n%+v", results[0]))
		}
	}

	return strings.Join(contextParts, "\n")
}

// Keep existing methods for backward compatibility
func (r *RAGService) GetSystemPrompt() string {
	if r.SystemPrompt != "" {
		return r.SystemPrompt
	}
	return `Bạn là trợ lý ảo thông minh của nền tảng Evolvia.`
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
	return `Bạn có thể truy vấn các database MongoDB sau:
- llm_service: Lưu session chat
- auth_service: Xác thực người dùng  
- profile_service: Thông tin hồ sơ (Collection: Profile)
- payos_service: Thanh toán
- billing_management_service: Hóa đơn
- knowledge_service: Cơ sở tri thức`
}

func (r *RAGService) ExecuteCustomQuery(userID string, databaseName string, collection string, query map[string]interface{}) ([]bson.M, error) {
	dbService := GetDatabaseService()
	if dbService == nil {
		return nil, fmt.Errorf("database service not available")
	}

	// Dynamic database connection using MongoDB client
	mongoClient := dbService.Client
	if mongoClient == nil {
		return nil, fmt.Errorf("MongoDB client not available")
	}

	// Connect to any database dynamically
	db := mongoClient.Database(databaseName)

	// Security: always add userID to query for user-related collections
	secureCollections := []string{"users", "orders", "subscriptions", "profiles", "Profile", "sessions"}
	for _, secureCol := range secureCollections {
		if collection == secureCol {
			query["userId"] = userID
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

// Helper function to format query for logging
func formatQueryForLog(query map[string]interface{}) string {
	if len(query) == 0 {
		return "{}"
	}

	// Simple JSON-like formatting for readability
	var parts []string
	for key, value := range query {
		parts = append(parts, fmt.Sprintf(`"%s": "%v"`, key, value))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
