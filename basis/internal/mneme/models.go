package mneme

import (
	"encoding/json"
	"time"
)

type Prosopon struct {
	ID          int64           `gorm:"primaryKey;column:id"`
	Sub         string          `gorm:"uniqueIndex;column:sub"`
	Email       string          `gorm:"column:email"`
	DisplayName string          `gorm:"column:display_name"`
	AvatarURL   *string         `gorm:"column:avatar_url"`
	Preferences json.RawMessage `gorm:"column:preferences"`
	Persona     *string         `gorm:"column:persona"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

type Conversation struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	ProsoponID int64     `gorm:"column:prosopon_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

type Message struct {
	ID             int64     `gorm:"primaryKey;column:id"`
	ConversationID int64     `gorm:"column:conversation_id"`
	ProsoponID     int64     `gorm:"column:prosopon_id"`
	Content        string    `gorm:"column:content"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

// Memory — embedding column is vector(384), not mapped to a Go type.
// All writes go through raw SQL in MemoryRepo.
type Memory struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	MessageID int64     `gorm:"column:message_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Memory) TableName() string { return "memories" }

type Pagination struct {
	Page     int
	PageSize int
}

// Offset extract offset from page and page size
func (p Pagination) Offset() int {
	offset := 0
	if p.Page > 0 {
		offset = (p.Page - 1) * p.PageSize
	}
	return offset
}
