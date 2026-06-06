package mneme

import (
	"context"

	"gorm.io/gorm"
)

type Messages struct {
	db *gorm.DB
}

func (r *Messages) Create(ctx context.Context, message *Message) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *Messages) Get(ctx context.Context, scope Scope) (Message, error) {
	m := Message{}
	query := r.db.WithContext(ctx).Model(&Message{})
	if scope != nil {
		query = scope.Scope(query)
	}
	err := query.First(&m).Error
	return m, err
}

func (r *Messages) List(ctx context.Context, scope Scope, pagination *Pagination) ([]Message, error) {
	var msgs []Message
	query := r.db.WithContext(ctx).Model(&Message{})
	if scope != nil {
		query = scope.Scope(query)
	}
	if pagination != nil {
		query = query.Limit(pagination.PageSize).Offset(pagination.Offset())
	}

	err := query.Find(&msgs).Error
	return msgs, err
}
