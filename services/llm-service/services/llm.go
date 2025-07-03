package services

import (
	"bufio"
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
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Stream      bool                    `json:"stream"`
	Temperature *float64                `json:"temperature,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
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

	// Let LLM decide based on guard prompt - don't pre-filter with keywords
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

// ProcessChatStream processes chat message and returns streaming response
func (l *LLMService) ProcessChatStream(userMessage string, userID string, responseChan chan models.StreamChunk) {
	defer close(responseChan)

	// Get RAG service
	rag := GetRAGService()
	if rag == nil {
		responseChan <- models.StreamChunk{
			Content:   "",
			IsEnd:     true,
			Error:     "RAG service not available",
			Timestamp: time.Now(),
		}
		return
	}

	// Load prompts from RAG files
	systemPrompt := rag.GetSystemPrompt()
	guardPrompt := rag.GetGuardPrompt()
	ragContext := rag.BuildRAGContext(userID, userMessage)

	// Combine prompts
	fullSystemPrompt := fmt.Sprintf("%s\n\n%s\n\n%s", guardPrompt, systemPrompt, ragContext)

	log.Printf("Sending streaming LLM request for user: %s, message: %s", userID, userMessage)

	// Try to send streaming request to LLM
	err := l.sendStreamingLLMRequest(userMessage, fullSystemPrompt, responseChan)
	if err != nil {
		log.Printf("LLM streaming service failed: %v", err)
		log.Printf("Falling back to non-streaming response")

		// Fallback to regular response and simulate streaming
		fallbackResponse := l.generateFallbackResponse(userMessage, ragContext)
		l.simulateStreaming(fallbackResponse.Message, responseChan)
	}
}

func (l *LLMService) sendStreamingLLMRequest(userMessage, systemPrompt string, responseChan chan models.StreamChunk) error {
	// Prepare chat request with streaming enabled
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
		Stream:   true, // Enable streaming
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	log.Printf("Making streaming request to: %s", l.BaseURL+"/chat/completions")
	req, err := http.NewRequest("POST", l.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if l.APIKey != "" && l.APIKey != "none" {
		req.Header.Set("Authorization", "Bearer "+l.APIKey)
	}

	resp, err := l.Client.Do(req)
	if err != nil {
		return fmt.Errorf("streaming request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("Started receiving streaming response...")

	// Use a scanner to read line by line more reliably
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if line == "data: [DONE]" {
			responseChan <- models.StreamChunk{
				Content:   "",
				IsEnd:     true,
				Timestamp: time.Now(),
			}
			return nil
		}

		if strings.HasPrefix(line, "data: ") {
			data := line[6:] // Remove "data: " prefix
			if data == "[DONE]" {
				responseChan <- models.StreamChunk{
					Content:   "",
					IsEnd:     true,
					Timestamp: time.Now(),
				}
				return nil
			}

			var streamResp models.StreamingResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				log.Printf("Failed to parse streaming response: %v", err)
				continue
			}

			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					responseChan <- models.StreamChunk{
						Content:   content,
						IsEnd:     false,
						Timestamp: time.Now(),
					}
				}

				if streamResp.Choices[0].FinishReason != nil {
					responseChan <- models.StreamChunk{
						Content:   "",
						IsEnd:     true,
						Timestamp: time.Now(),
					}
					return nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading streaming response: %v", err)
	}

	// If we reach here without seeing [DONE], send end signal
	responseChan <- models.StreamChunk{
		Content:   "",
		IsEnd:     true,
		Timestamp: time.Now(),
	}

	return nil
}

func (l *LLMService) simulateStreaming(message string, responseChan chan models.StreamChunk) {
	words := strings.Fields(message)

	for i, word := range words {
		// Add space before word (except first word)
		content := word
		if i > 0 {
			content = " " + word
		}

		responseChan <- models.StreamChunk{
			Content:   content,
			IsEnd:     false,
			Timestamp: time.Now(),
		}

		// Small delay to simulate typing
		time.Sleep(50 * time.Millisecond)
	}

	// Send end signal
	responseChan <- models.StreamChunk{
		Content:   "",
		IsEnd:     true,
		Timestamp: time.Now(),
	}
}
