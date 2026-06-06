package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type Conversations struct {
	db *gorm.DB
}

func (r *Conversations) GetOrCreate(ctx context.Context, prosoponId int64) (*Conversation, error) {
	c, err := r.Get(ctx, prosoponId)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return r.Create(ctx, prosoponId)
}

func (r *Conversations) Get(ctx context.Context, prosoponId int64) (*Conversation, error) {
	c := &Conversation{}
	err := r.db.WithContext(ctx).Model(&Conversation{}).Where("prosopon_id = ?", prosoponId).First(c).Error
	return c, err
}

func (r *Conversations) Create(ctx context.Context, prosoponId int64) (*Conversation, error) {
	c := &Conversation{
		ProsoponID: prosoponId,
	}
	err := r.db.WithContext(ctx).Model(&Conversation{}).Create(c).Error
	return c, err
}
