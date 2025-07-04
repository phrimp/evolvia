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
	// Debug logging
	log.Printf("=== ProcessChat Debug ===")
	log.Printf("UserID: %s", userID)
	log.Printf("Message: %s", userMessage)

	// Get RAG service
	rag := GetRAGService()
	if rag == nil {
		return nil, fmt.Errorf("RAG service not available")
	}

	// Load prompts from RAG files
	systemPrompt := rag.GetSystemPrompt()
	guardPrompt := rag.GetGuardPrompt()
	ragContext := rag.BuildRAGContext(userID, userMessage)

	// Debug RAG context
	log.Printf("RAG Context length: %d chars", len(ragContext))
	log.Printf("RAG Context preview: %.300s...", ragContext)

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

	// Debug log để xem ragContext có dữ liệu không
	log.Printf("RAG Context preview: %.200s...", ragContext)

	// Check if we have user data in RAG context
	if strings.Contains(ragContext, "THÔNG TIN NGƯỜI DÙNG") {
		// Có dữ liệu người dùng - AI có thể trả lời chi tiết

		if strings.Contains(lowerMessage, "tài khoản") || strings.Contains(lowerMessage, "thông tin") {
			return &models.LLMResponse{
				Message: fmt.Sprintf(`Xin chào! Tôi đã tìm thấy thông tin tài khoản của bạn trong hệ thống:

%s

Bạn có muốn biết thêm chi tiết về phần nào không? Tôi có thể giúp bạn với:
• Thông tin cá nhân (tên, email, số điện thoại)
• Lịch sử giao dịch và thanh toán
• Cài đặt tài khoản
• Bảo mật tài khoản

Vui lòng cho tôi biết bạn cần hỗ trợ gì!`, ragContext),
				Timestamp: time.Now(),
			}
		}

		if strings.Contains(lowerMessage, "tên") {
			nameInfo := l.extractNameFromContext(ragContext)
			if nameInfo != "" {
				return &models.LLMResponse{
					Message:   nameInfo,
					Timestamp: time.Now(),
				}
			}
		}

		if strings.Contains(lowerMessage, "email") {
			emailInfo := l.extractEmailFromContext(ragContext)
			if emailInfo != "" {
				return &models.LLMResponse{
					Message:   emailInfo,
					Timestamp: time.Now(),
				}
			}
			return &models.LLMResponse{
				Message:   "Tôi đã tìm thấy thông tin email của bạn trong hệ thống. Để bảo mật, bạn có muốn tôi hiển thị một phần thông tin email không?",
				Timestamp: time.Now(),
			}
		}
	}

	// Không có user context hoặc userID = anonymous
	if strings.Contains(lowerMessage, "tài khoản") || strings.Contains(lowerMessage, "thông tin") {
		return &models.LLMResponse{
			Message: `Để truy cập thông tin tài khoản, bạn cần đăng nhập trước. 

🔐 **Vui lòng:**
1. Đăng nhập với tài khoản của bạn
2. Cung cấp JWT token hợp lệ trong header Authorization

Sau khi đăng nhập, tôi sẽ có thể truy xuất và hiển thị đầy đủ thông tin tài khoản của bạn một cách an toàn.

💡 **Tôi có thể giúp bạn với:**
• Hướng dẫn đăng nhập
• Hỗ trợ kỹ thuật
• Thông tin dịch vụ

Bạn cần hỗ trợ gì?`,
			Timestamp: time.Now(),
		}
	}

	return &models.LLMResponse{
		Message:   "Xin lỗi, dịch vụ AI đang gặp sự cố. Tôi có thể giúp bạn với:\n- Thông tin tài khoản (cần đăng nhập)\n- Hỗ trợ kỹ thuật\n- Hướng dẫn sử dụng\n\nBạn muốn hỗ trợ gì?",
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

// extractNameFromContext extracts name information from RAG context
func (l *LLMService) extractNameFromContext(ragContext string) string {
	// Check for various name fields that might exist in the database
	nameFields := []string{"name", "fullName", "full_name", "firstName", "first_name", "lastName", "last_name", "displayName", "display_name"}

	// First try to find structured data (JSON-like format)
	lines := strings.Split(ragContext, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for various name formats in the data
		for _, field := range nameFields {
			// Check for key:value format (like "name: John Doe")
			if strings.Contains(strings.ToLower(line), field+":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[1])
					if name != "" && name != "null" && name != "undefined" {
						return fmt.Sprintf("Tên của bạn là: %s", name)
					}
				}
			}

			// Check for JSON-like format
			if strings.Contains(strings.ToLower(line), `"`+field+`"`) {
				// Try to extract value after the field name
				fieldPattern := `"` + field + `"`
				if idx := strings.Index(strings.ToLower(line), fieldPattern); idx != -1 {
					remaining := line[idx+len(fieldPattern):]
					if colonIdx := strings.Index(remaining, ":"); colonIdx != -1 {
						valueStart := colonIdx + 1
						value := strings.TrimSpace(remaining[valueStart:])
						// Remove quotes and trailing comma if present
						value = strings.Trim(value, `"`)
						if commaIdx := strings.Index(value, ","); commaIdx != -1 {
							value = value[:commaIdx]
						}
						value = strings.TrimSpace(value)
						if value != "" && value != "null" && value != "undefined" {
							return fmt.Sprintf("Tên của bạn là: %s", value)
						}
					}
				}
			}
		}

		// Also check for Vietnamese format
		if strings.Contains(strings.ToLower(line), "tên:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				if name != "" && name != "null" && name != "undefined" {
					return fmt.Sprintf("Tên của bạn là: %s", name)
				}
			}
		}
	}

	return ""
}

// extractEmailFromContext extracts email information from RAG context
func (l *LLMService) extractEmailFromContext(ragContext string) string {
	// Check for various email fields that might exist in the database
	emailFields := []string{"email", "emailAddress", "email_address", "userEmail", "user_email", "contactEmail"}

	lines := strings.Split(ragContext, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for various email formats in the data
		for _, field := range emailFields {
			// Check for key:value format (like "email: user@example.com")
			if strings.Contains(strings.ToLower(line), field+":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					email := strings.TrimSpace(parts[1])
					if email != "" && email != "null" && email != "undefined" && strings.Contains(email, "@") {
						return fmt.Sprintf("Email của bạn là: %s", email)
					}
				}
			}

			// Check for JSON-like format
			if strings.Contains(strings.ToLower(line), `"`+field+`"`) {
				fieldPattern := `"` + field + `"`
				if idx := strings.Index(strings.ToLower(line), fieldPattern); idx != -1 {
					remaining := line[idx+len(fieldPattern):]
					if colonIdx := strings.Index(remaining, ":"); colonIdx != -1 {
						valueStart := colonIdx + 1
						value := strings.TrimSpace(remaining[valueStart:])
						value = strings.Trim(value, `"`)
						if commaIdx := strings.Index(value, ","); commaIdx != -1 {
							value = value[:commaIdx]
						}
						value = strings.TrimSpace(value)
						if value != "" && value != "null" && value != "undefined" && strings.Contains(value, "@") {
							return fmt.Sprintf("Email của bạn là: %s", value)
						}
					}
				}
			}
		}
	}

	return ""
}
