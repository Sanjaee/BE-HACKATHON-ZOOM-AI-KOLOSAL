package app

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

// TestKolosalAPI handles testing Kolosal API
// POST /api/v1/rooms/:id/test-kolosal
// This handler follows the EXACT same pattern as CreateMessage
func (h *ChatHandler) TestKolosalAPI(c *gin.Context) {
	log.Printf("[TestKolosalAPI] ===== HANDLER CALLED =====")
	log.Printf("[TestKolosalAPI] Method: %s, Path: %s", c.Request.Method, c.Request.URL.Path)
	log.Printf("[TestKolosalAPI] FullPath: %s", c.FullPath())

	// Get user ID from context first (same order as CreateMessage)
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("[TestKolosalAPI] User not authenticated")
		util.Unauthorized(c, "User not authenticated")
		return
	}
	log.Printf("[TestKolosalAPI] User ID: %s", userID)

	// Get room ID from param (same as CreateMessage)
	roomID := c.Param("id")
	log.Printf("[TestKolosalAPI] Room ID from param: %s", roomID)
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
		log.Printf("[TestKolosalAPI] Error calling Kolosal API: %v", err)
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

	log.Printf("[TestKolosalAPI] Success: user=%s, room=%s, prompt=%s, response_length=%d",
		userID, roomID, req.Prompt, len(aiResponse))

	util.SuccessResponse(c, http.StatusOK, "Kolosal API test successful", gin.H{
		"prompt":   req.Prompt,
		"response": aiResponse,
		"model":    response.Model,
		"usage":    response.Usage,
	})
}
