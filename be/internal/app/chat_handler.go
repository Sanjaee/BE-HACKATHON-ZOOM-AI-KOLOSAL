package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"yourapp/internal/service"
	"yourapp/internal/util"
	"yourapp/internal/websocket"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService    service.ChatService
	kolosalService service.KolosalService
	roomService    service.RoomService
	hub            *websocket.Hub
	jwtSecret      string
	// Store agent history per room (roomID -> []HistoryItem)
	agentHistory sync.Map // map[string][]service.HistoryItem
}

func NewChatHandler(chatService service.ChatService, kolosalService service.KolosalService, roomService service.RoomService, hub *websocket.Hub, jwtSecret string) *ChatHandler {
	return &ChatHandler{
		chatService:    chatService,
		kolosalService: kolosalService,
		roomService:    roomService,
		hub:            hub,
		jwtSecret:      jwtSecret,
	}
}

// CreateMessage handles creating a new chat message
// POST /api/v1/rooms/:id/messages
func (h *ChatHandler) CreateMessage(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	roomID := c.Param("id")
	if roomID == "" {
		util.BadRequest(c, "Room ID is required")
		return
	}

	var req service.CreateChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	message, err := h.chatService.CreateMessage(roomID, userID.(string), req.Message)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Broadcast message via WebSocket
	h.hub.BroadcastMessage(roomID, &websocket.Message{
		RoomID:  roomID,
		UserID:  userID.(string),
		Type:    "message",
		Payload: message,
	})

	util.SuccessResponse(c, http.StatusCreated, "Message created successfully", message)
}

// GetMessages handles getting messages for a room
// GET /api/v1/rooms/:id/messages
func (h *ChatHandler) GetMessages(c *gin.Context) {
	roomID := c.Param("id")
	if roomID == "" {
		util.BadRequest(c, "Room ID is required")
		return
	}

	limit := 50 // default limit
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	messages, err := h.chatService.GetMessages(roomID, limit, offset)
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Messages retrieved successfully", messages)
}

// ServeWebSocket handles WebSocket connections for chat
// WS /api/v1/rooms/:id/chat/ws?token=...
func (h *ChatHandler) ServeWebSocket(c *gin.Context) {
	roomID := c.Param("id")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}

	// Get token from query parameter (WebSocket can't send custom headers before upgrade)
	token := c.Query("token")
	if token == "" {
		// Try to get from Authorization header as fallback
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		log.Printf("[WS] WebSocket connection rejected: No token provided for room %s", roomID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
		return
	}

	// Validate token
	claims, err := util.ValidateToken(token, h.jwtSecret)
	if err != nil {
		log.Printf("[WS] WebSocket connection rejected: Invalid token for room %s, error: %v", roomID, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	log.Printf("[WS] WebSocket connection accepted: room=%s, user=%s", roomID, claims.UserID)

	// Serve WebSocket connection
	h.hub.ServeWS(c.Writer, c.Request, roomID, claims.UserID)
}

// KolosalAPI handles Kolosal AI API requests
// POST /api/v1/rooms/:id/kolosal
// This handler follows the EXACT same pattern as CreateMessage
func (h *ChatHandler) KolosalAPI(c *gin.Context) {
	log.Printf("[KolosalAPI] ===== HANDLER CALLED =====")
	log.Printf("[KolosalAPI] Method: %s, Path: %s", c.Request.Method, c.Request.URL.Path)
	log.Printf("[KolosalAPI] FullPath: %s", c.FullPath())

	// Get user ID from context first (same order as CreateMessage)
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("[KolosalAPI] User not authenticated")
		util.Unauthorized(c, "User not authenticated")
		return
	}
	log.Printf("[KolosalAPI] User ID: %s", userID)

	// Get room ID from param (same as CreateMessage)
	roomID := c.Param("id")
	log.Printf("[KolosalAPI] Room ID from param: %s", roomID)
	if roomID == "" {
		util.BadRequest(c, "Room ID is required")
		return
	}

	// Verify room exists (same pattern as CreateMessage - it verifies in service)
	// But we verify here first to match the pattern
	_, err := h.roomService.GetRoomByID(roomID)
	if err != nil {
		util.ErrorResponse(c, http.StatusNotFound, "Room not found", nil)
		return
	}

	var req struct {
		Prompt          string                 `json:"prompt" binding:"required"`
		Model           string                 `json:"model,omitempty"`
		MaxTokens       int                    `json:"max_tokens,omitempty"`
		Cache           *bool                  `json:"cache,omitempty"`             // Enable/disable response caching
		ImageData       string                 `json:"image_data,omitempty"`        // Base64 image for OCR
		UseOCR          bool                   `json:"use_ocr,omitempty"`           // Flag to use OCR instead of chat
		OCRLanguage     string                 `json:"ocr_language,omitempty"`      // OCR language
		OCRAutoFix      bool                   `json:"ocr_auto_fix,omitempty"`      // OCR auto fix
		OCRInvoice      bool                   `json:"ocr_invoice,omitempty"`       // OCR invoice mode
		OCRCustomSchema map[string]interface{} `json:"ocr_custom_schema,omitempty"` // OCR custom schema
		UseAgent        bool                   `json:"use_agent,omitempty"`         // Flag to use agent mode
		WorkspaceID     string                 `json:"workspace_id,omitempty"`      // Workspace ID for agent
		Tools           []string               `json:"tools,omitempty"`             // Tools for agent
		History         []service.HistoryItem  `json:"history,omitempty"`           // History from frontend
		ResetHistory    bool                   `json:"reset_history,omitempty"`     // Reset chat history
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	// Default values
	model := req.Model
	if model == "" {
		model = "meta-llama/llama-4-maverick-17b-128e-instruct"
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	// Broadcast AI typing status to all users in room (disable input for all)
	h.hub.BroadcastMessage(roomID, &websocket.Message{
		RoomID: roomID,
		UserID: userID.(string),
		Type:   "ai_typing",
		Payload: map[string]interface{}{
			"user_id": userID.(string),
			"status":  "processing",
		},
	})

	var aiResponse string
	var response *service.KolosalChatResponse

	// Handle reset history if requested
	if req.ResetHistory {
		h.agentHistory.Delete(roomID)
		log.Printf("[KolosalAPI] History reset for room: %s", roomID)
	}

	// Check if Agent mode is requested
	if req.UseAgent && req.WorkspaceID != "" {
		// Handle Agent request
		log.Printf("[KolosalAPI] Processing Agent request with workspace: %s", req.WorkspaceID)

		// Get history - prefer from request, fallback to memory
		var history []service.HistoryItem
		if len(req.History) > 0 {
			// Use history from frontend request
			history = req.History
			log.Printf("[KolosalAPI] Using history from request: %d items", len(history))
		} else if hist, ok := h.agentHistory.Load(roomID); ok {
			// Fallback to memory history
			history = hist.([]service.HistoryItem)
			log.Printf("[KolosalAPI] Using history from memory: %d items", len(history))
		}

		// Build agent request
		agentRequest := &service.KolosalAgentRequest{
			Input:       req.Prompt,
			Model:       model,
			WorkspaceID: req.WorkspaceID,
			Tools:       req.Tools,
			History:     history,
		}

		agentResponse, err := h.kolosalService.AgentGenerate(agentRequest)
		if err != nil {
			log.Printf("[KolosalAPI] Error calling Kolosal Agent API: %v", err)

			// Broadcast error to all users
			h.hub.BroadcastMessage(roomID, &websocket.Message{
				RoomID: roomID,
				UserID: userID.(string),
				Type:   "ai_error",
				Payload: map[string]interface{}{
					"user_id": userID.(string),
					"error":   fmt.Sprintf("Failed to call Kolosal Agent API: %v", err),
				},
			})

			util.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to call Kolosal Agent API: %v", err), nil)
			return
		}

		// Extract agent response
		if agentResponse.Output != "" {
			aiResponse = agentResponse.Output
		} else {
			aiResponse = "No response from Agent"
		}

		// Update history with new response
		if agentResponse.History != nil && len(agentResponse.History) > 0 {
			h.agentHistory.Store(roomID, agentResponse.History)
			log.Printf("[KolosalAPI] Updated agent history for room: %s, history length: %d", roomID, len(agentResponse.History))
		} else {
			// Manually update history if not returned
			newHistory := append(history, service.HistoryItem{
				Type:    "user",
				Content: req.Prompt,
			}, service.HistoryItem{
				Type:    "assistant",
				Content: aiResponse,
			})
			h.agentHistory.Store(roomID, newHistory)
		}

		// Create a mock response structure for consistency
		response = &service.KolosalChatResponse{
			Model: "agent",
			Choices: []service.Choice{
				{
					Message: service.Message{
						Role:    "assistant",
						Content: aiResponse,
					},
				},
			},
		}
	} else if req.UseOCR && req.ImageData != "" {
		// Handle OCR request
		log.Printf("[KolosalAPI] Processing OCR request")

		ocrRequest := &service.KolosalOCRRequest{
			ImageData: req.ImageData,
			Language:  req.OCRLanguage,
			AutoFix:   true,
			Invoice:   false,
		}
		if req.OCRLanguage == "" {
			ocrRequest.Language = "auto"
		}

		ocrResponse, err := h.kolosalService.OCR(ocrRequest)
		if err != nil {
			log.Printf("[KolosalAPI] Error calling Kolosal OCR API: %v", err)

			// Broadcast error to all users
			h.hub.BroadcastMessage(roomID, &websocket.Message{
				RoomID: roomID,
				UserID: userID.(string),
				Type:   "ai_error",
				Payload: map[string]interface{}{
					"user_id": userID.(string),
					"error":   fmt.Sprintf("Failed to call Kolosal OCR API: %v", err),
				},
			})

			util.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to call Kolosal OCR API: %v", err), nil)
			return
		}

		// Helper function to extract string from interface{} (can be string, array, or other)
		extractStringFromInterface := func(val interface{}) string {
			if val == nil {
				return ""
			}
			switch v := val.(type) {
			case string:
				return v
			case []interface{}:
				// If it's an array, join all string elements
				var parts []string
				for _, item := range v {
					if str, ok := item.(string); ok {
						parts = append(parts, str)
					} else if itemMap, ok := item.(map[string]interface{}); ok {
						// Try to extract text from object
						if text, ok := itemMap["text"].(string); ok {
							parts = append(parts, text)
						} else if content, ok := itemMap["content"].(string); ok {
							parts = append(parts, content)
						}
					}
				}
				return strings.Join(parts, "\n")
			case []string:
				return strings.Join(v, "\n")
			default:
				// Try to convert to string
				return fmt.Sprintf("%v", v)
			}
		}

		// Log full OCR response for debugging
		log.Printf("[KolosalAPI] OCR Response - Text: %T, ExtractedText: %T, Result: %T, Content: %T, Data: %v",
			ocrResponse.Text, ocrResponse.ExtractedText, ocrResponse.Result, ocrResponse.Content, ocrResponse.Data != nil)

		// Extract OCR text - check multiple possible fields (similar to frontend handler)
		if text := extractStringFromInterface(ocrResponse.Text); text != "" {
			aiResponse = text
			log.Printf("[KolosalAPI] Using Text field: %d chars", len(aiResponse))
		} else if extractedText := extractStringFromInterface(ocrResponse.ExtractedText); extractedText != "" {
			aiResponse = extractedText
			log.Printf("[KolosalAPI] Using ExtractedText field: %d chars", len(aiResponse))
		} else if result := extractStringFromInterface(ocrResponse.Result); result != "" {
			aiResponse = result
			log.Printf("[KolosalAPI] Using Result field: %d chars", len(aiResponse))
		} else if content := extractStringFromInterface(ocrResponse.Content); content != "" {
			aiResponse = content
			log.Printf("[KolosalAPI] Using Content field: %d chars", len(aiResponse))
		} else if ocrText := extractStringFromInterface(ocrResponse.OcrText); ocrText != "" {
			aiResponse = ocrText
			log.Printf("[KolosalAPI] Using OcrText field: %d chars", len(aiResponse))
		} else if len(ocrResponse.Blocks) > 0 {
			// Extract text from blocks
			var textParts []string
			for _, block := range ocrResponse.Blocks {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok && text != "" {
						textParts = append(textParts, text)
					} else if content, ok := blockMap["content"].(string); ok && content != "" {
						textParts = append(textParts, content)
					} else if value, ok := blockMap["value"].(string); ok && value != "" {
						textParts = append(textParts, value)
					}
				}
			}
			if len(textParts) > 0 {
				aiResponse = strings.Join(textParts, "\n")
				log.Printf("[KolosalAPI] Using Blocks field: %d blocks, %d chars", len(ocrResponse.Blocks), len(aiResponse))
			}
		} else if len(ocrResponse.Lines) > 0 {
			// Extract text from lines
			var textParts []string
			for _, line := range ocrResponse.Lines {
				if lineMap, ok := line.(map[string]interface{}); ok {
					if text, ok := lineMap["text"].(string); ok && text != "" {
						textParts = append(textParts, text)
					} else if content, ok := lineMap["content"].(string); ok && content != "" {
						textParts = append(textParts, content)
					}
				}
			}
			if len(textParts) > 0 {
				aiResponse = strings.Join(textParts, "\n")
				log.Printf("[KolosalAPI] Using Lines field: %d lines, %d chars", len(ocrResponse.Lines), len(aiResponse))
			}
		} else if len(ocrResponse.Paragraphs) > 0 {
			// Extract text from paragraphs
			var textParts []string
			for _, para := range ocrResponse.Paragraphs {
				if paraMap, ok := para.(map[string]interface{}); ok {
					if text, ok := paraMap["text"].(string); ok && text != "" {
						textParts = append(textParts, text)
					} else if content, ok := paraMap["content"].(string); ok && content != "" {
						textParts = append(textParts, content)
					}
				}
			}
			if len(textParts) > 0 {
				aiResponse = strings.Join(textParts, "\n")
				log.Printf("[KolosalAPI] Using Paragraphs field: %d paragraphs, %d chars", len(ocrResponse.Paragraphs), len(aiResponse))
			}
		} else if ocrResponse.Data != nil {
			// Check if Data contains text fields
			if text, ok := ocrResponse.Data["text"].(string); ok && text != "" {
				aiResponse = text
				log.Printf("[KolosalAPI] Using Data.text field: %d chars", len(aiResponse))
			} else if extractedText, ok := ocrResponse.Data["extracted_text"].(string); ok && extractedText != "" {
				aiResponse = extractedText
				log.Printf("[KolosalAPI] Using Data.extracted_text field: %d chars", len(aiResponse))
			} else {
				// If structured data, convert to string
				if dataBytes, err := json.Marshal(ocrResponse.Data); err == nil {
					aiResponse = fmt.Sprintf("📝 **OCR Result:**\n\n```json\n%s\n```", string(dataBytes))
					log.Printf("[KolosalAPI] Using Data as JSON: %d chars", len(aiResponse))
				} else {
					log.Printf("[KolosalAPI] WARNING: Could not extract text from OCR response")
					aiResponse = "OCR completed but no text extracted. Response data format not recognized."
				}
			}
		} else {
			// Log full response for debugging
			responseBytes, _ := json.Marshal(ocrResponse)
			log.Printf("[KolosalAPI] WARNING: No text found in OCR response. Full response: %s", string(responseBytes))
			aiResponse = "OCR completed but no text extracted. Please check the image or try again."
		}

		// Create a mock response structure for consistency
		response = &service.KolosalChatResponse{
			Model: "ocr",
			Choices: []service.Choice{
				{
					Message: service.Message{
						Role:    "assistant",
						Content: aiResponse,
					},
				},
			},
		}
	} else {
		// Handle regular chat completion
		// Prepare Kolosal request
		kolosalRequest := &service.KolosalChatRequest{
			Model: model,
			Messages: []service.Message{
				{
					Role:    "user",
					Content: req.Prompt,
				},
			},
			MaxTokens:   maxTokens,
			Temperature: 0.7,
			Stream:      false,
		}

		// Set cache if provided
		if req.Cache != nil {
			kolosalRequest.Cache = req.Cache
		}

		// Call Kolosal API
		var err error
		response, err = h.kolosalService.ChatCompletions(kolosalRequest)
		if err != nil {
			log.Printf("[KolosalAPI] Error calling Kolosal API: %v", err)
			log.Printf("[KolosalAPI] Error details - Type: %T, Message: %s", err, err.Error())

			// Check if it's a configuration error (missing API key/URL)
			errorMsg := err.Error()
			if strings.Contains(errorMsg, "KOLOSAL_API_KEY") || strings.Contains(errorMsg, "KOLOSAL_API_URL") {
				log.Printf("[KolosalAPI] Configuration error detected - check environment variables")
				util.ErrorResponse(c, http.StatusInternalServerError, "Kolosal AI is not properly configured. Please check server environment variables.", gin.H{
					"error":   errorMsg,
					"details": "Missing KOLOSAL_API_KEY or KOLOSAL_API_URL environment variable",
				})
				return
			}

			// Broadcast error to all users
			h.hub.BroadcastMessage(roomID, &websocket.Message{
				RoomID: roomID,
				UserID: userID.(string),
				Type:   "ai_error",
				Payload: map[string]interface{}{
					"user_id": userID.(string),
					"error":   fmt.Sprintf("Failed to call Kolosal API: %v", err),
				},
			})

			util.ErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to call Kolosal API: %v", err), nil)
			return
		}

		// Extract response message
		if len(response.Choices) > 0 && len(response.Choices[0].Message.Content) > 0 {
			aiResponse = response.Choices[0].Message.Content
		} else {
			aiResponse = "No response from AI"
		}
	}

	// Generate temporary ID for streaming message
	tempID := fmt.Sprintf("ai-temp-%d", time.Now().UnixNano())

	// Simulate streaming by sending chunks (for realtime effect)
	chunkSize := 20 // characters per chunk
	aiResponseRunes := []rune(aiResponse)
	totalChunks := (len(aiResponseRunes) + chunkSize - 1) / chunkSize

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(aiResponseRunes) {
			end = len(aiResponseRunes)
		}

		chunk := string(aiResponseRunes[start:end])
		accumulatedContent := string(aiResponseRunes[0:end])

		// Broadcast streaming chunk to all users
		h.hub.BroadcastMessage(roomID, &websocket.Message{
			RoomID: roomID,
			UserID: userID.(string),
			Type:   "ai_stream",
			Payload: map[string]interface{}{
				"id":         tempID,
				"content":    accumulatedContent,
				"chunk":      chunk,
				"user_id":    userID.(string),
				"user_name":  "AI Agent",
				"user_email": "ai@agent.com",
			},
		})

		// Small delay to simulate streaming
		time.Sleep(50 * time.Millisecond)
	}

	// Save AI message to database (persistent storage)
	// Note: AI messages are saved with userID "ai-agent" - this user should exist in DB
	// If user doesn't exist, CreateMessage will fail but we'll still broadcast via WebSocket
	var aiMessage *service.ChatMessageResponse
	aiUserID := "ai-agent" // Fixed user ID for AI agent
	aiMessage, err = h.chatService.CreateMessage(roomID, aiUserID, aiResponse)
	if err != nil {
		log.Printf("[KolosalAPI] Error saving AI message (user 'ai-agent' may not exist): %v", err)
		log.Printf("[KolosalAPI] Message will still be broadcast via WebSocket but not persisted")
		// Create a temporary message structure for WebSocket if DB save fails
		aiMessage = &service.ChatMessageResponse{
			ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
			RoomID:    roomID,
			UserID:    aiUserID,
			UserName:  "AI Agent",
			UserEmail: "ai@agent.com",
			Message:   aiResponse,
			CreatedAt: time.Now(),
		}
	} else {
		log.Printf("[KolosalAPI] AI message saved to database: %s", aiMessage.ID)
	}

	// Broadcast AI complete with final message
	h.hub.BroadcastMessage(roomID, &websocket.Message{
		RoomID: roomID,
		UserID: userID.(string),
		Type:   "ai_complete",
		Payload: map[string]interface{}{
			"temp_id": tempID,
			"user_id": userID.(string),
			"message": aiMessage,
		},
	})

	log.Printf("[KolosalAPI] Success: user=%s, room=%s, prompt=%s, response_length=%d",
		userID, roomID, req.Prompt, len(aiResponse))

	// Prepare response with history if agent mode
	responseData := gin.H{
		"prompt":   req.Prompt,
		"response": aiResponse,
		"model":    response.Model,
		"usage":    response.Usage,
		"message":  aiMessage, // Include saved message in response
	}

	// Include history in response if agent mode
	if req.UseAgent {
		var currentHistory []service.HistoryItem
		if hist, ok := h.agentHistory.Load(roomID); ok {
			currentHistory = hist.([]service.HistoryItem)
		}
		responseData["history"] = currentHistory
	}

	util.SuccessResponse(c, http.StatusOK, "Kolosal API request successful", responseData)
}
