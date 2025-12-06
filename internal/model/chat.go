package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatMessage represents a message in a room chat
type ChatMessage struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RoomID    string    `gorm:"type:uuid;not null;index" json:"room_id"`
	Room      Room      `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	UserID    string    `gorm:"type:uuid;not null;index" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// BeforeCreate hook to generate UUID
func (c *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

