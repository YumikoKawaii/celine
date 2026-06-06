package mneme

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Client maps to the clients table — one row per Google OIDC subject (sub claim).
type Client struct {
	Sub         string    `gorm:"primaryKey"`
	Email       string    `gorm:"not null"`
	DisplayName string    `gorm:"not null;default:''"`
	AvatarURL   *string   // nullable TEXT
	MemoryMD    *string   // nullable TEXT — curated profile injected into the cached system prefix
	PersonaNote *string   // nullable TEXT — admin-level annotation, not shown to the client
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// NewClient builds a Client from the fields available at OAuth sign-in.
// avatarURL is promoted to a pointer so callers don't manage *string directly.
func NewClient(sub, email, displayName, avatarURL string) Client {
	url := avatarURL
	return Client{Sub: sub, Email: email, DisplayName: displayName, AvatarURL: &url}
}

// Conversation is a named chat thread owned by one Client.
type Conversation struct {
	ID        string    `gorm:"primaryKey"`
	OwnerSub  string    `gorm:"not null;index:idx_conv_owner_created,priority:1"`
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_conv_owner_created,priority:2,sort:desc"`
}

// Message is one stored turn (user or assistant) inside a Conversation.
type Message struct {
	ID             string    `gorm:"primaryKey"`
	ConversationID string    `gorm:"not null;index:idx_msg_conv_created,priority:1"`
	Role           string    `gorm:"not null"` // "user" | "assistant"
	Content        string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"autoCreateTime;index:idx_msg_conv_created,priority:2"`
}

// MemoryIndex stores per-owner message embeddings for RAG recall (§12).
// The embedding column is vector(384) — not mapped to a Go type because GORM
// has no native pgvector support. All reads/writes use raw SQL via MemoryIndexRepo.
type MemoryIndex struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	OwnerSub  string    `gorm:"not null;index"`
	MessageID string    `gorm:"not null;uniqueIndex"`
	Role      string    `gorm:"not null"` // "user" | "assistant"
	Content   string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName overrides GORM's default pluralisation ("memory_indices").
func (MemoryIndex) TableName() string { return "memory_index" }

// newID generates a random prefixed ID: "<prefix>-<24 hex chars>".
func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}
