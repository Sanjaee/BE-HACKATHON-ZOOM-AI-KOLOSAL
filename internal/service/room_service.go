package service

import (
	"errors"
	"time"
	"yourapp/internal/config"
	"yourapp/internal/model"
	"yourapp/internal/repository"

	"github.com/livekit/protocol/auth"
)

type RoomService interface {
	CreateRoom(req CreateRoomRequest) (*RoomResponse, error)
	CreateRoomWithUser(req CreateRoomRequest, userID string) (*RoomResponse, error)
	GetRoomByID(roomID string) (*RoomResponse, error)
	GetRoomsByUser(userID string) ([]RoomResponse, error)
	GetAllRooms() ([]RoomResponse, error)
	JoinRoom(roomID, userID string) (*JoinRoomResponse, error)
	LeaveRoom(roomID, userID string) error
	DeleteRoom(roomID, userID string) error
}

type roomService struct {
	roomRepo repository.RoomRepository
	userRepo repository.UserRepository
	cfg      *config.Config
}

func NewRoomService(roomRepo repository.RoomRepository, userRepo repository.UserRepository, cfg *config.Config) RoomService {
	return &roomService{
		roomRepo: roomRepo,
		userRepo: userRepo,
		cfg:      cfg,
	}
}

type CreateRoomRequest struct {
	Name            string  `json:"name" binding:"required"`
	Description     *string `json:"description"`
	MaxParticipants *int    `json:"max_participants"`
}

type RoomResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	CreatedByID      string    `json:"created_by_id"`
	CreatedByName    string    `json:"created_by_name"`
	IsActive         bool      `json:"is_active"`
	MaxParticipants  *int      `json:"max_participants,omitempty"`
	ParticipantCount int64     `json:"participant_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type JoinRoomResponse struct {
	Token string       `json:"token"`
	URL   string       `json:"url"`
	Room  RoomResponse `json:"room"`
}

func (s *roomService) CreateRoom(req CreateRoomRequest) (*RoomResponse, error) {
	return nil, errors.New("use CreateRoomWithUser instead")
}

func (s *roomService) CreateRoomWithUser(req CreateRoomRequest, userID string) (*RoomResponse, error) {
	room := &model.Room{
		Name:            req.Name,
		Description:     req.Description,
		MaxParticipants: req.MaxParticipants,
		CreatedByID:     userID,
		IsActive:        true,
	}

	if err := s.roomRepo.Create(room); err != nil {
		return nil, errors.New("failed to create room")
	}

	return s.roomToResponse(room), nil
}

func (s *roomService) GetRoomByID(roomID string) (*RoomResponse, error) {
	room, err := s.roomRepo.FindByIDWithParticipants(roomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	participantCount, _ := s.roomRepo.GetParticipantCount(roomID)
	response := s.roomToResponse(room)
	response.ParticipantCount = participantCount

	return response, nil
}

func (s *roomService) GetRoomsByUser(userID string) ([]RoomResponse, error) {
	rooms, err := s.roomRepo.FindByCreatedBy(userID)
	if err != nil {
		return nil, errors.New("failed to fetch rooms")
	}

	responses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		participantCount, _ := s.roomRepo.GetParticipantCount(room.ID)
		responses[i] = *s.roomToResponse(&room)
		responses[i].ParticipantCount = participantCount
	}

	return responses, nil
}

func (s *roomService) GetAllRooms() ([]RoomResponse, error) {
	rooms, err := s.roomRepo.FindAll()
	if err != nil {
		return nil, errors.New("failed to fetch rooms")
	}

	responses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		participantCount, _ := s.roomRepo.GetParticipantCount(room.ID)
		responses[i] = *s.roomToResponse(&room)
		responses[i].ParticipantCount = participantCount
	}

	return responses, nil
}

func (s *roomService) JoinRoom(roomID, userID string) (*JoinRoomResponse, error) {
	// Verify room exists
	room, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	if !room.IsActive {
		return nil, errors.New("room is not active")
	}

	// Check max participants if set
	if room.MaxParticipants != nil {
		count, _ := s.roomRepo.GetParticipantCount(roomID)
		if count >= int64(*room.MaxParticipants) {
			return nil, errors.New("room is full")
		}
	}

	// Get user for identity
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Add participant to room
	if err := s.roomRepo.AddParticipant(roomID, userID); err != nil {
		return nil, errors.New("failed to join room")
	}

	// Generate LiveKit token
	at := auth.NewAccessToken(s.cfg.LiveKitAPIKey, s.cfg.LiveKitAPISecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomID,
	}

	// Use user email or ID as identity
	identity := user.Email
	if user.Username != nil && *user.Username != "" {
		identity = *user.Username
	}

	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(time.Hour * 24)

	token, err := at.ToJWT()
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	// Get room response
	roomResponse, _ := s.GetRoomByID(roomID)

	return &JoinRoomResponse{
		Token: token,
		URL:   s.cfg.LiveKitURL,
		Room:  *roomResponse,
	}, nil
}

func (s *roomService) LeaveRoom(roomID, userID string) error {
	// Verify room exists
	_, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	// Remove participant from room
	if err := s.roomRepo.RemoveParticipant(roomID, userID); err != nil {
		return errors.New("failed to leave room")
	}

	// Check if room has no active participants left, then auto-delete
	count, err := s.roomRepo.GetParticipantCount(roomID)
	if err != nil {
		// Continue even if count check fails
		return nil
	}

	// Auto-delete room if no participants left
	if count == 0 {
		if err := s.roomRepo.Delete(roomID); err != nil {
			// Log error but don't fail the leave operation
			// The room will be cleaned up later
			return nil
		}
	}

	return nil
}

func (s *roomService) DeleteRoom(roomID, userID string) error {
	room, err := s.roomRepo.FindByID(roomID)
	if err != nil {
		return errors.New("room not found")
	}

	// Only creator can delete
	if room.CreatedByID != userID {
		return errors.New("unauthorized to delete this room")
	}

	return s.roomRepo.Delete(roomID)
}

func (s *roomService) roomToResponse(room *model.Room) *RoomResponse {
	response := &RoomResponse{
		ID:              room.ID,
		Name:            room.Name,
		Description:     room.Description,
		CreatedByID:     room.CreatedByID,
		CreatedByName:   room.CreatedBy.FullName,
		IsActive:        room.IsActive,
		MaxParticipants: room.MaxParticipants,
		CreatedAt:       room.CreatedAt,
	}

	if room.CreatedBy.FullName == "" {
		response.CreatedByName = room.CreatedBy.Email
	}

	return response
}
