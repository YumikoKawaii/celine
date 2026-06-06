package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type ConversationRepo struct {
	db *gorm.DB
}

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

func (r *ConversationRepo) List(ctx context.Context, ownerSub string) ([]Conversation, error) {
	var cs []Conversation
	err := r.db.WithContext(ctx).
		Where("owner_sub = ?", ownerSub).
		Order("created_at DESC").
		Find(&cs).Error
	return cs, err
}
