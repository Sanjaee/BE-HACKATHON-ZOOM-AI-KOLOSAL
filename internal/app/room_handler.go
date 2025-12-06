package app

import (
	"net/http"
	"yourapp/internal/service"
	"yourapp/internal/util"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	roomService service.RoomService
}

func NewRoomHandler(roomService service.RoomService) *RoomHandler {
	return &RoomHandler{
		roomService: roomService,
	}
}

// CreateRoom handles room creation
// POST /api/v1/rooms
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	var req service.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	// Create room with creator ID
	room, err := h.roomService.CreateRoomWithUser(req, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusCreated, "Room created successfully", room)
}

// GetRoom handles getting room by ID
// GET /api/v1/rooms/:id
func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("id")
	if roomID == "" {
		util.BadRequest(c, "Room ID is required")
		return
	}

	room, err := h.roomService.GetRoomByID(roomID)
	if err != nil {
		util.ErrorResponse(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Room retrieved successfully", room)
}

// GetRooms handles getting all rooms
// GET /api/v1/rooms
func (h *RoomHandler) GetRooms(c *gin.Context) {
	rooms, err := h.roomService.GetAllRooms()
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Rooms retrieved successfully", rooms)
}

// GetMyRooms handles getting rooms created by current user
// GET /api/v1/rooms/my
func (h *RoomHandler) GetMyRooms(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		util.Unauthorized(c, "User not authenticated")
		return
	}

	rooms, err := h.roomService.GetRoomsByUser(userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Rooms retrieved successfully", rooms)
}

// JoinRoom handles joining a room and getting LiveKit token
// POST /api/v1/rooms/:id/join
func (h *RoomHandler) JoinRoom(c *gin.Context) {
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

	// Log for debugging
	c.Header("X-Debug-UserID", userID.(string))
	c.Header("X-Debug-RoomID", roomID)

	response, err := h.roomService.JoinRoom(roomID, userID.(string))
	if err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Joined room successfully", response)
}

// LeaveRoom handles leaving a room
// POST /api/v1/rooms/:id/leave
func (h *RoomHandler) LeaveRoom(c *gin.Context) {
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

	if err := h.roomService.LeaveRoom(roomID, userID.(string)); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Left room successfully", nil)
}

// DeleteRoom handles room deletion
// DELETE /api/v1/rooms/:id
func (h *RoomHandler) DeleteRoom(c *gin.Context) {
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

	if err := h.roomService.DeleteRoom(roomID, userID.(string)); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, http.StatusOK, "Room deleted successfully", nil)
}
