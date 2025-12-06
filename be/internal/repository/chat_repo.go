package repository

import (
	"yourapp/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	Create(message *model.ChatMessage) error
	FindByRoomID(roomID string, limit, offset int) ([]model.ChatMessage, error)
	FindByID(id string) (*model.ChatMessage, error)
	GetMessageCount(roomID string) (int64, error)
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) Create(message *model.ChatMessage) error {
	return r.db.Create(message).Error
}

func (r *chatRepository) FindByRoomID(roomID string, limit, offset int) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	err := r.db.Preload("User").Preload("Room").
		Where("room_id = ?", roomID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	
	// Reverse order to show oldest first
	if len(messages) > 0 {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	
	return messages, err
}

func (r *chatRepository) FindByID(id string) (*model.ChatMessage, error) {
	var message model.ChatMessage
	err := r.db.Preload("User").Preload("Room").
		Where("id = ?", id).
		First(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *chatRepository) GetMessageCount(roomID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.ChatMessage{}).
		Where("room_id = ?", roomID).
		Count(&count).Error
	return count, err
}

