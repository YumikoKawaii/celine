package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type Conversations struct {
	db *gorm.DB
}

func (r *Conversations) GetOrCreate(ctx context.Context, scope KataProsopon) (*Conversation, error) {
	c, err := r.Get(ctx, scope)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return r.Create(ctx, scope.ProsoponId)
}

func (r *Conversations) Get(ctx context.Context, scope Scope) (*Conversation, error) {
	c := &Conversation{}
	query := r.db.WithContext(ctx).Model(&Conversation{})
	if scope != nil {
		query = scope.Scope(query)
	}
	err := query.First(c).Error
	return c, err
}

func (r *Conversations) Create(ctx context.Context, prosoponId int64) (*Conversation, error) {
	c := &Conversation{ProsoponId: prosoponId}
	err := r.db.WithContext(ctx).Create(c).Error
	return c, err
}
