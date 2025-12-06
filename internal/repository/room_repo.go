package repository

import (
	"time"
	"yourapp/internal/model"

	"gorm.io/gorm"
)

type RoomRepository interface {
	Create(room *model.Room) error
	FindByID(id string) (*model.Room, error)
	FindByIDWithParticipants(id string) (*model.Room, error)
	FindByCreatedBy(userID string) ([]model.Room, error)
	FindAll() ([]model.Room, error)
	Update(room *model.Room) error
	Delete(id string) error
	AddParticipant(roomID, userID string) error
	RemoveParticipant(roomID, userID string) error
	IsParticipant(roomID, userID string) (bool, error)
	GetParticipantCount(roomID string) (int64, error)
}

type roomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) RoomRepository {
	return &roomRepository{db: db}
}

func (r *roomRepository) Create(room *model.Room) error {
	return r.db.Create(room).Error
}

func (r *roomRepository) FindByID(id string) (*model.Room, error) {
	var room model.Room
	err := r.db.Where("id = ?", id).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) FindByIDWithParticipants(id string) (*model.Room, error) {
	var room model.Room
	err := r.db.Preload("CreatedBy").Preload("Participants").Where("id = ?", id).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) FindByCreatedBy(userID string) ([]model.Room, error) {
	var rooms []model.Room
	err := r.db.Where("created_by_id = ?", userID).Order("created_at DESC").Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) FindAll() ([]model.Room, error) {
	var rooms []model.Room
	err := r.db.Preload("CreatedBy").Order("created_at DESC").Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) Update(room *model.Room) error {
	return r.db.Save(room).Error
}

func (r *roomRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.Room{}).Error
}

func (r *roomRepository) AddParticipant(roomID, userID string) error {
	// Check if participant already exists
	var existing model.RoomParticipant
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&existing).Error

	if err == nil {
		// Participant exists, just update to active
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"is_active": true,
			"left_at":   nil,
		}).Error
	}

	// Create new participant
	participant := model.RoomParticipant{
		RoomID:   roomID,
		UserID:   userID,
		IsActive: true,
	}
	return r.db.Create(&participant).Error
}

func (r *roomRepository) RemoveParticipant(roomID, userID string) error {
	now := time.Now()
	return r.db.Model(&model.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(map[string]interface{}{
			"is_active": false,
			"left_at":   &now,
		}).Error
}

func (r *roomRepository) IsParticipant(roomID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.RoomParticipant{}).
		Where("room_id = ? AND user_id = ? AND is_active = ?", roomID, userID, true).
		Count(&count).Error
	return count > 0, err
}

func (r *roomRepository) GetParticipantCount(roomID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.RoomParticipant{}).
		Where("room_id = ? AND is_active = ?", roomID, true).
		Count(&count).Error
	return count, err
}
