package mneme

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Client struct {
	Sub         string    `gorm:"primaryKey"`
	Email       string    `gorm:"not null"`
	DisplayName string    `gorm:"not null;default:''"`
	AvatarURL   *string   // nullable
	MemoryMD    *string   // nullable — curated profile injected into the cached system prefix (§13)
	PersonaNote *string   // nullable — admin annotation, never shown to the client
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// NewClient promotes avatarURL to *string so callers don't manage the pointer themselves.
func NewClient(sub, email, displayName, avatarURL string) Client {
	url := avatarURL
	return Client{Sub: sub, Email: email, DisplayName: displayName, AvatarURL: &url}
}

type Conversation struct {
	ID        string    `gorm:"primaryKey"`
	OwnerSub  string    `gorm:"not null;index:idx_conv_owner_created,priority:1"`
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_conv_owner_created,priority:2,sort:desc"`
}

type Message struct {
	ID             string    `gorm:"primaryKey"`
	ConversationID string    `gorm:"not null;index:idx_msg_conv_created,priority:1"`
	Role           string    `gorm:"not null"` // "user" | "assistant"
	Content        string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"autoCreateTime;index:idx_msg_conv_created,priority:2"`
}

// MemoryIndex — embedding column is vector(384), not mapped to a Go type because
// GORM has no native pgvector support; all writes go through raw SQL in MemoryIndexRepo.
type MemoryIndex struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	OwnerSub  string    `gorm:"not null;index"`
	MessageID string    `gorm:"not null;uniqueIndex"`
	Role      string    `gorm:"not null"` // "user" | "assistant"
	Content   string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName prevents GORM from pluralising to "memory_indices".
func (MemoryIndex) TableName() string { return "memory_index" }

func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}
