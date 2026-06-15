package mneme

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Prosopon struct {
	Id          int64           `gorm:"primaryKey;column:id"`
	Sub         string          `gorm:"uniqueIndex;column:sub"`
	Email       string          `gorm:"column:email"`
	DisplayName string          `gorm:"column:display_name"`
	AvatarURL   *string         `gorm:"column:avatar_url"`
	Preferences json.RawMessage `gorm:"column:preferences;default:'{}'"`
	Persona     *string         `gorm:"column:persona"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

type Conversation struct {
	Id         int64     `gorm:"primaryKey;column:id"`
	ProsoponId int64     `gorm:"column:prosopon_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

type Message struct {
	Id             int64     `gorm:"primaryKey;column:id"`
	ConversationId int64     `gorm:"column:conversation_id"`
	ProsoponId     int64     `gorm:"column:prosopon_id"`
	Content        string    `gorm:"column:content"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

// Memory — embedding column is vector(384), not mapped to a Go type.
// All writes go through raw SQL in MemoryRepo.
type Memory struct {
	Id        int64     `gorm:"primaryKey;column:id"`
	MessageId int64     `gorm:"column:message_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Memory) TableName() string { return "memories" }

type Scope interface {
	Scope(q *gorm.DB) *gorm.DB
}

// Primordial Scope implementations — cover the common lookup patterns.
// Custom query logic (ordering, joins, etc.) belongs in the consumer package.

type KataSub struct {
	Sub string
}

func (f KataSub) Scope(q *gorm.DB) *gorm.DB {
	return q.Where("sub = ?", f.Sub)
}

type KataProsopon struct {
	ProsoponId int64
}

func (f KataProsopon) Scope(q *gorm.DB) *gorm.DB {
	return q.Where("prosopon_id = ?", f.ProsoponId)
}

type KataMessage struct {
	Id int64
}

func (f KataMessage) Scope(q *gorm.DB) *gorm.DB {
	return q.Where("id = ?", f.Id)
}

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
