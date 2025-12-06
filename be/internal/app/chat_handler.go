package app

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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
		Prompt    string `json:"prompt" binding:"required"`
		Model     string `json:"model,omitempty"`
		MaxTokens int    `json:"max_tokens,omitempty"`
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

	// Call Kolosal API
	response, err := h.kolosalService.ChatCompletions(kolosalRequest)
	if err != nil {
		log.Printf("[KolosalAPI] Error calling Kolosal API: %v", err)

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
	var aiResponse string
	if len(response.Choices) > 0 && len(response.Choices[0].Message.Content) > 0 {
		aiResponse = response.Choices[0].Message.Content
	} else {
		aiResponse = "No response from AI"
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

	util.SuccessResponse(c, http.StatusOK, "Kolosal API request successful", gin.H{
		"prompt":   req.Prompt,
		"response": aiResponse,
		"model":    response.Model,
		"usage":    response.Usage,
		"message":  aiMessage, // Include saved message in response
	})
}
