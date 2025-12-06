package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type KolosalService interface {
	ChatCompletions(request *KolosalChatRequest) (*KolosalChatResponse, error)
}

type kolosalService struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewKolosalService(apiURL, apiKey string) KolosalService {
	return &kolosalService{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second, // 60 seconds timeout for AI responses
		},
	}
}

// KolosalChatRequest represents the request to Kolosal API
type KolosalChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// KolosalChatResponse represents the response from Kolosal API
type KolosalChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a choice in the response
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (s *kolosalService) ChatCompletions(request *KolosalChatRequest) (*KolosalChatResponse, error) {
	// Validate API key
	if s.apiKey == "" {
		return nil, fmt.Errorf("KOLOSAL_API_KEY is not set. Please configure KOLOSAL_API_KEY environment variable")
	}

	// Validate API URL
	if s.apiURL == "" {
		return nil, fmt.Errorf("KOLOSAL_API_URL is not set. Please configure KOLOSAL_API_URL environment variable")
	}

	// Prepare request body
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build full URL with endpoint
	apiURL := s.apiURL
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "v1/chat/completions"

	// Log request details (without sensitive data)
	fmt.Printf("[KolosalService] Making request to: %s\n", apiURL)
	fmt.Printf("[KolosalService] API Key present: %v (length: %d)\n", s.apiKey != "", len(s.apiKey))

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response KolosalChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}
