package app

import (
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
	chatService service.ChatService
	hub         *websocket.Hub
	jwtSecret   string
}

func NewChatHandler(chatService service.ChatService, hub *websocket.Hub, jwtSecret string) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		hub:         hub,
		jwtSecret:   jwtSecret,
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
