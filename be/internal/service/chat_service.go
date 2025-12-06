package service

import (
	"errors"
	"time"
	"yourapp/internal/model"
	"yourapp/internal/repository"
)

type ChatService interface {
	CreateMessage(roomID, userID, message string) (*ChatMessageResponse, error)
	GetMessages(roomID string, limit, offset int) ([]ChatMessageResponse, error)
	GetMessageCount(roomID string) (int64, error)
}

type chatService struct {
	chatRepo repository.ChatRepository
	roomRepo repository.RoomRepository
	userRepo repository.UserRepository
}

func NewChatService(chatRepo repository.ChatRepository, roomRepo repository.RoomRepository, userRepo repository.UserRepository) ChatService {
	return &chatService{
		chatRepo: chatRepo,
		roomRepo: roomRepo,
		userRepo: userRepo,
	}
}

type ChatMessageResponse struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	UserEmail string    `json:"user_email"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateChatMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func (s *chatService) CreateMessage(roomID, userID, message string) (*ChatMessageResponse, error) {
	// Verify room exists
	_, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	// Verify user exists
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Validate message
	if message == "" {
		return nil, errors.New("message cannot be empty")
	}

	// Create message
	chatMessage := &model.ChatMessage{
		RoomID:  roomID,
		UserID:  userID,
		Message: message,
	}

	if err := s.chatRepo.Create(chatMessage); err != nil {
		return nil, errors.New("failed to create message")
	}

	// Load with relations for response
	createdMessage, err := s.chatRepo.FindByID(chatMessage.ID)
	if err != nil {
		return nil, errors.New("failed to fetch created message")
	}

	return s.chatMessageToResponse(createdMessage, user), nil
}

func (s *chatService) GetMessages(roomID string, limit, offset int) ([]ChatMessageResponse, error) {
	// Verify room exists
	_, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	// Get messages
	messages, err := s.chatRepo.FindByRoomID(roomID, limit, offset)
	if err != nil {
		return nil, errors.New("failed to fetch messages")
	}

	// Convert to response
	responses := make([]ChatMessageResponse, len(messages))
	for i, msg := range messages {
		userName := msg.User.FullName
		if msg.User.Username != nil && *msg.User.Username != "" {
			userName = *msg.User.Username
		}
		
		responses[i] = ChatMessageResponse{
			ID:        msg.ID,
			RoomID:    msg.RoomID,
			UserID:    msg.UserID,
			UserName:  userName,
			UserEmail: msg.User.Email,
			Message:   msg.Message,
			CreatedAt: msg.CreatedAt,
		}
	}

	return responses, nil
}

func (s *chatService) GetMessageCount(roomID string) (int64, error) {
	return s.chatRepo.GetMessageCount(roomID)
}

func (s *chatService) chatMessageToResponse(msg *model.ChatMessage, user *model.User) *ChatMessageResponse {
	userName := user.FullName
	if user.Username != nil && *user.Username != "" {
		userName = *user.Username
	}

	return &ChatMessageResponse{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		UserName:  userName,
		UserEmail: user.Email,
		Message:   msg.Message,
		CreatedAt: msg.CreatedAt,
	}
}

