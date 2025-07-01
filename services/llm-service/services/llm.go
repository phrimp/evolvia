package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"llm-service/configs"
	"llm-service/models"
)

type LLMService struct {
	Client   *http.Client
	BaseURL  string
	APIKey   string
	Model    string
	Provider string
}

var llmService *LLMService

func InitLLMService() error {
	llmService = &LLMService{
		Client: &http.Client{
			Timeout: 120 * time.Second, // Increased timeout for LLM responses
		},
		BaseURL:  configs.AppConfig.LLMBaseURL,
		APIKey:   configs.AppConfig.LLMAPIKey,
		Model:    configs.AppConfig.LLMModel,
		Provider: configs.AppConfig.LLMProvider,
	}
	return nil
}

func GetLLMService() *LLMService {
	return llmService
}

func (l *LLMService) IsConnected() bool {
	// Test connection by making a simple request
	resp, err := http.Get(l.BaseURL + "/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

type ChatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []ChatCompletionMessage `json:"messages"`
	Stream   bool                    `json:"stream"`
}

type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatCompletionMessage `json:"message"`
	} `json:"choices"`
}

func (l *LLMService) ProcessChat(userMessage string, userID string) (*models.LLMResponse, error) {
	// Get RAG service
	rag := GetRAGService()
	if rag == nil {
		return nil, fmt.Errorf("RAG service not available")
	}

	// Load prompts from RAG files
	systemPrompt := rag.GetSystemPrompt()
	guardPrompt := rag.GetGuardPrompt()
	ragContext := rag.BuildRAGContext(userID, userMessage)

	// Combine prompts
	fullSystemPrompt := fmt.Sprintf("%s\n\n%s\n\n%s", guardPrompt, systemPrompt, ragContext)

	// Check if message should be processed
	if l.shouldRejectMessage(userMessage) {
		return &models.LLMResponse{
			Message:   "Xin lỗi, tôi chỉ có thể hỗ trợ bạn với các vấn đề liên quan đến:\n- Thông tin tài khoản cá nhân\n- Lịch sử đơn hàng và giao dịch\n- Sản phẩm và dịch vụ của chúng tôi\n- Hỗ trợ khách hàng\n\nBạn có câu hỏi nào khác về tài khoản của mình không?",
			Timestamp: time.Now(),
		}, nil
	}

	// Try to send request to LLM
	log.Printf("Sending LLM request for user: %s, message: %s", userID, userMessage)
	response, err := l.sendLLMRequest(userMessage, fullSystemPrompt)
	if err != nil {
		log.Printf("LLM service failed: %v", err)
		log.Printf("Falling back to default response")
		// Return fallback response based on RAG context
		return l.generateFallbackResponse(userMessage, ragContext), nil
	}

	log.Printf("LLM response received successfully")

	// Process response
	llmResponse := &models.LLMResponse{
		Message:   response.Choices[0].Message.Content,
		Timestamp: time.Now(),
	}

	return llmResponse, nil
}

func (l *LLMService) sendLLMRequest(userMessage, systemPrompt string) (*ChatCompletionResponse, error) {
	// Prepare chat request
	messages := []ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	request := ChatCompletionRequest{
		Model:    l.Model,
		Messages: messages,
		Stream:   false,
	}

	// Send request to LLM
	return l.sendChatRequest(request)
}

func (l *LLMService) generateFallbackResponse(userMessage, ragContext string) *models.LLMResponse {
	lowerMessage := strings.ToLower(userMessage)

	// Simple keyword-based responses
	if strings.Contains(lowerMessage, "tên") {
		if strings.Contains(ragContext, "Tên:") {
			// Extract name from RAG context
			lines := strings.Split(ragContext, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Tên:") {
					return &models.LLMResponse{
						Message:   "Dựa trên thông tin trong hệ thống: " + strings.TrimSpace(line),
						Timestamp: time.Now(),
					}
				}
			}
		}
		return &models.LLMResponse{
			Message:   "Xin lỗi, tôi không thể truy xuất thông tin tên của bạn lúc này. Vui lòng thử lại sau.",
			Timestamp: time.Now(),
		}
	}

	if strings.Contains(lowerMessage, "email") {
		return &models.LLMResponse{
			Message:   "Để xem thông tin email, bạn có thể kiểm tra trong phần cài đặt tài khoản.",
			Timestamp: time.Now(),
		}
	}

	return &models.LLMResponse{
		Message:   "Xin lỗi, dịch vụ AI đang gặp sự cố. Tôi có thể giúp bạn với:\n- Thông tin tài khoản\n- Lịch sử đơn hàng\n- Hỗ trợ kỹ thuật\n\nBạn muốn hỗ trợ gì?",
		Timestamp: time.Now(),
	}
}

func (l *LLMService) sendChatRequest(request ChatCompletionRequest) (*ChatCompletionResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	log.Printf("Making request to: %s", l.BaseURL+"/chat/completions")
	req, err := http.NewRequest("POST", l.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if l.APIKey != "" && l.APIKey != "none" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	log.Printf("Sending request to LLM API...")
	resp, err := l.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	log.Printf("Received response with status: %d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response ChatCompletionResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (l *LLMService) shouldRejectMessage(message string) bool {
	rejectKeywords := []string{
		"làm bài tập", "giải bài", "homework", "assignment",
		"viết code", "lập trình", "debug", "fix bug",
		"hack", "crack", "bypass", "exploit",
		"chứng khoán", "đầu tư", "bitcoin", "crypto",
		"thuốc", "bệnh", "chẩn đoán", "điều trị",
		"luật", "pháp lý", "kiện tụng",
	}

	lowerMessage := strings.ToLower(message)
	for _, keyword := range rejectKeywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}
