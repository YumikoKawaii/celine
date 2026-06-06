package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ConversationRepo provides persistence operations for the conversations table.
type ConversationRepo struct {
	db *gorm.DB
}

// GetOrCreate returns convID if it exists and belongs to ownerSub.
// If convID is empty or not found, a new conversation is created and its ID returned.
func (r *ConversationRepo) GetOrCreate(ctx context.Context, ownerSub, convID string) (string, error) {
	if convID != "" {
		var c Conversation
		err := r.db.WithContext(ctx).
			Where("id = ? AND owner_sub = ?", convID, ownerSub).
			First(&c).Error
		if err == nil {
			return c.ID, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}

	c := Conversation{ID: newID("conv"), OwnerSub: ownerSub}
	return c.ID, r.db.WithContext(ctx).Create(&c).Error
}

// List returns all conversations for ownerSub, newest first.
func (r *ConversationRepo) List(ctx context.Context, ownerSub string) ([]Conversation, error) {
	var cs []Conversation
	err := r.db.WithContext(ctx).
		Where("owner_sub = ?", ownerSub).
		Order("created_at DESC").
		Find(&cs).Error
	return cs, err
}
