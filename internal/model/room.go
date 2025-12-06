package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Room represents a video call room
type Room struct {
	ID              string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name            string         `gorm:"type:varchar(255);not null" json:"name"`
	Description     *string        `gorm:"type:text" json:"description,omitempty"`
	CreatedByID     string         `gorm:"type:uuid;not null" json:"created_by_id"`
	CreatedBy       User           `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	MaxParticipants *int           `gorm:"type:integer" json:"max_participants,omitempty"`
	Participants    []User         `gorm:"many2many:room_participants;" json:"participants,omitempty"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// RoomParticipant represents the many-to-many relationship between Room and User
type RoomParticipant struct {
	RoomID   string     `gorm:"type:uuid;primaryKey" json:"room_id"`
	UserID   string     `gorm:"type:uuid;primaryKey" json:"user_id"`
	JoinedAt time.Time  `gorm:"autoCreateTime" json:"joined_at"`
	LeftAt   *time.Time `gorm:"type:timestamp" json:"left_at,omitempty"`
	IsActive bool       `gorm:"default:true" json:"is_active"`
}

// TableName specifies the table name
func (Room) TableName() string {
	return "rooms"
}

// TableName specifies the table name for RoomParticipant
func (RoomParticipant) TableName() string {
	return "room_participants"
}

// BeforeCreate hook to generate UUID
func (r *Room) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
