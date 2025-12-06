package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// HistoryItem represents a history item for agent
type HistoryItem struct {
	Type      string      `json:"type"`
	Content   string      `json:"content,omitempty"`
	Name      string      `json:"name,omitempty"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// KolosalAgentRequest represents agent generate request
type KolosalAgentRequest struct {
	Input       string        `json:"input"`
	Model       string        `json:"model"`
	WorkspaceID string        `json:"workspace_id"`
	Tools       []string      `json:"tools,omitempty"`
	History     []HistoryItem `json:"history,omitempty"`
}

// KolosalAgentResponse represents agent generate response
type KolosalAgentResponse struct {
	Output  string                 `json:"output,omitempty"`
	History []HistoryItem          `json:"history,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type KolosalService interface {
	ChatCompletions(request *KolosalChatRequest) (*KolosalChatResponse, error)
	OCR(request *KolosalOCRRequest) (*KolosalOCRResponse, error)
	GetModels() (*ModelsResponse, error)
	AgentGenerate(request *KolosalAgentRequest) (*KolosalAgentResponse, error)
}

// CustomSchema represents custom extraction schema
type CustomSchema struct {
	Name   string      `json:"name"`
	Schema interface{} `json:"schema,omitempty"`
	Strict bool        `json:"strict,omitempty"`
}

// KolosalOCRRequest represents OCR request to Kolosal API
type KolosalOCRRequest struct {
	ImageData      string        `json:"image_data,omitempty"` // base64 image or URL or gs://bucket/file
	Language       string        `json:"language,omitempty"`
	AutoFix        bool          `json:"auto_fix,omitempty"`
	Invoice        bool          `json:"invoice,omitempty"`
	CustomSchema   *CustomSchema `json:"custom_schema,omitempty"` // Custom extraction schema
	GCSAccessToken string        `json:"gcs_access_token,omitempty"`
	GCSURL         string        `json:"gcs_url,omitempty"`
}

// KolosalOCRResponse represents OCR response from Kolosal API
// Using interface{} for flexible field types (can be string, array, or object)
type KolosalOCRResponse struct {
	Text          interface{}            `json:"text,omitempty"`           // Can be string or array
	ExtractedText interface{}            `json:"extracted_text,omitempty"` // Can be string or array
	Result        interface{}            `json:"result,omitempty"`         // Can be string or array
	Content       interface{}            `json:"content,omitempty"`        // Can be string or array
	OcrText       interface{}            `json:"ocr_text,omitempty"`       // Can be string or array
	Blocks        []interface{}          `json:"blocks,omitempty"`
	Lines         []interface{}          `json:"lines,omitempty"`
	Paragraphs    []interface{}          `json:"paragraphs,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

type kolosalService struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewKolosalService(apiURL, apiKey string) KolosalService {
	// Trim whitespace from API key to prevent format issues
	apiKey = strings.TrimSpace(apiKey)

	return &kolosalService{
		apiURL: strings.TrimSpace(apiURL),
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
	Cache       *bool     `json:"cache,omitempty"` // Enable/disable response caching
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

	// Trim any remaining whitespace (safety check)
	s.apiKey = strings.TrimSpace(s.apiKey)

	// Validate API key format - should start with "kol_"
	if !strings.HasPrefix(s.apiKey, "kol_") {
		return nil, fmt.Errorf("KOLOSAL_API_KEY format is invalid. Token should start with 'kol_'. Current prefix: '%s' (length: %d)",
			func() string {
				if len(s.apiKey) >= 4 {
					return s.apiKey[:4]
				}
				return "N/A"
			}(), len(s.apiKey))
	}

	// Trim any remaining whitespace (safety check)
	s.apiKey = strings.TrimSpace(s.apiKey)

	// Validate API key format - should start with "kol_"
	if !strings.HasPrefix(s.apiKey, "kol_") {
		return nil, fmt.Errorf("KOLOSAL_API_KEY format is invalid. Token should start with 'kol_'. Current prefix: '%s' (length: %d)",
			func() string {
				if len(s.apiKey) >= 4 {
					return s.apiKey[:4]
				}
				return "N/A"
			}(), len(s.apiKey))
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
	fmt.Printf("[KolosalService] API Key present: %v (length: %d, prefix: %s)\n",
		s.apiKey != "",
		len(s.apiKey),
		func() string {
			if len(s.apiKey) >= 4 {
				return s.apiKey[:4]
			}
			return "N/A"
		}())

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
		errorMsg := string(body)
		log.Printf("[KolosalService] API Error - Status: %d, Response: %s", resp.StatusCode, errorMsg)

		// Provide more helpful error messages for common errors
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("Kolosal API authentication failed (401). Please check KOLOSAL_API_KEY format. Token should start with 'kol_'. Response: %s", errorMsg)
		}

		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var response KolosalChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// OCR performs OCR on an image using Kolosal API
func (s *kolosalService) OCR(request *KolosalOCRRequest) (*KolosalOCRResponse, error) {
	// Validate API key
	if s.apiKey == "" {
		return nil, fmt.Errorf("KOLOSAL_API_KEY is not set. Please configure KOLOSAL_API_KEY environment variable")
	}

	// Trim any remaining whitespace (safety check)
	s.apiKey = strings.TrimSpace(s.apiKey)

	// Validate API key format - should start with "kol_"
	if !strings.HasPrefix(s.apiKey, "kol_") {
		return nil, fmt.Errorf("KOLOSAL_API_KEY format is invalid. Token should start with 'kol_'. Current prefix: '%s' (length: %d)",
			func() string {
				if len(s.apiKey) >= 4 {
					return s.apiKey[:4]
				}
				return "N/A"
			}(), len(s.apiKey))
	}

	// Validate API URL
	if s.apiURL == "" {
		return nil, fmt.Errorf("KOLOSAL_API_URL is not set. Please configure KOLOSAL_API_URL environment variable")
	}

	// Build OCR URL
	ocrURL := s.apiURL
	if !strings.HasSuffix(ocrURL, "/") {
		ocrURL += "/"
	}
	ocrURL += "ocr"

	// Prepare request body
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[KolosalService] Making OCR request to: %s", ocrURL)
	log.Printf("[KolosalService] Image data length: %d", len(request.ImageData))

	// Create HTTP request
	req, err := http.NewRequest("POST", ocrURL, bytes.NewBuffer(requestBody))
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
		errorMsg := string(body)
		log.Printf("[KolosalService] OCR API Error - Status: %d, Response: %s", resp.StatusCode, errorMsg)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Log raw response for debugging
	log.Printf("[KolosalService] OCR API Response (first 500 chars): %s", func() string {
		if len(body) > 500 {
			return string(body[:500]) + "..."
		}
		return string(body)
	}())

	// Parse response
	var response KolosalOCRResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("[KolosalService] Failed to parse OCR response: %v, Raw body: %s", err, string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Log parsed response structure
	log.Printf("[KolosalService] OCR Response parsed - Has Text: %v, Has ExtractedText: %v, Has Result: %v, Has Content: %v, Has Data: %v, Blocks: %d, Lines: %d",
		response.Text != "", response.ExtractedText != "", response.Result != "", response.Content != "",
		response.Data != nil, len(response.Blocks), len(response.Lines))

	return &response, nil
}

// Model represents a Kolosal AI model
type Model struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Pricing     Pricing `json:"pricing"`
	ContextSize int     `json:"contextSize"`
	LastUpdated string  `json:"lastUpdated"`
}

// Pricing represents model pricing
type Pricing struct {
	Input    float64 `json:"input"`
	Output   float64 `json:"output"`
	Currency string  `json:"currency"`
	Unit     string  `json:"unit"`
}

// ModelsResponse represents the response from /v1/models endpoint
type ModelsResponse struct {
	Count  int     `json:"count"`
	Models []Model `json:"models"`
}

// GetModels fetches available models from Kolosal API
func (s *kolosalService) GetModels() (*ModelsResponse, error) {
	// Validate API key
	if s.apiKey == "" {
		return nil, fmt.Errorf("KOLOSAL_API_KEY is not set. Please configure KOLOSAL_API_KEY environment variable")
	}

	// Trim any remaining whitespace (safety check)
	s.apiKey = strings.TrimSpace(s.apiKey)

	// Validate API URL
	if s.apiURL == "" {
		return nil, fmt.Errorf("KOLOSAL_API_URL is not set. Please configure KOLOSAL_API_URL environment variable")
	}

	// Build models URL
	modelsURL := s.apiURL
	if !strings.HasSuffix(modelsURL, "/") {
		modelsURL += "/"
	}
	modelsURL += "v1/models"

	log.Printf("[KolosalService] Fetching models from: %s", modelsURL)

	// Create HTTP request
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
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
		errorMsg := string(body)
		log.Printf("[KolosalService] GetModels API Error - Status: %d, Response: %s", resp.StatusCode, errorMsg)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var response ModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// AgentGenerate generates agent response using Kolosal API
func (s *kolosalService) AgentGenerate(request *KolosalAgentRequest) (*KolosalAgentResponse, error) {
	// Validate API key
	if s.apiKey == "" {
		return nil, fmt.Errorf("KOLOSAL_API_KEY is not set. Please configure KOLOSAL_API_KEY environment variable")
	}

	// Trim any remaining whitespace (safety check)
	s.apiKey = strings.TrimSpace(s.apiKey)

	// Validate API key format - should start with "kol_"
	if !strings.HasPrefix(s.apiKey, "kol_") {
		return nil, fmt.Errorf("KOLOSAL_API_KEY format is invalid. Token should start with 'kol_'")
	}

	// Validate API URL
	if s.apiURL == "" {
		return nil, fmt.Errorf("KOLOSAL_API_URL is not set. Please configure KOLOSAL_API_URL environment variable")
	}

	// Build agent URL
	agentURL := s.apiURL
	if !strings.HasSuffix(agentURL, "/") {
		agentURL += "/"
	}
	agentURL += "v1/agent/generate"

	// Prepare request body
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[KolosalService] Making agent request to: %s", agentURL)
	log.Printf("[KolosalService] Workspace ID: %s, Tools: %v, History length: %d",
		request.WorkspaceID, request.Tools, len(request.History))

	// Create HTTP request
	req, err := http.NewRequest("POST", agentURL, bytes.NewBuffer(requestBody))
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
		errorMsg := string(body)
		log.Printf("[KolosalService] Agent API Error - Status: %d, Response: %s", resp.StatusCode, errorMsg)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, errorMsg)
	}

	// Parse response
	var response KolosalAgentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}
