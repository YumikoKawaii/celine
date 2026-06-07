package agent

import "gorm.io/gorm"

// historyMessages scopes the message list to a conversation's full history in
// chronological order, ready to feed directly to Claude as context.
type historyMessages struct {
	convID int64
}

func (s historyMessages) Scope(q *gorm.DB) *gorm.DB {
	return q.Where("conversation_id = ?", s.convID).Order("created_at DESC")
}
