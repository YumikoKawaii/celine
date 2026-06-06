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

func (r *Messages) List(ctx context.Context, parameters MessageParameters) ([]Message, error) {
	var msgs []Message
	query := r.db.WithContext(ctx).Model(&Message{})
	query = query.Where("conversation_id = ?", parameters.ConversationID)
	if parameters.Pagination != nil {
		query = query.Limit(parameters.Pagination.PageSize).Offset(parameters.Pagination.Offset())
	}

	err := query.Find(&msgs).Error
	return msgs, err
}

type MessageParameters struct {
	ConversationID int64
	Pagination     *Pagination
}
