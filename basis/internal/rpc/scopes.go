package rpc

import "gorm.io/gorm"

// anamnesisMessages scopes a messages query to a single conversation for the
// Anamnesis RPC. Defined in the consumer package so any change to this query
// shape stays local to the handler and doesn't affect other callers of messages.List.
type anamnesisMessages struct {
	convID int64
}

func (s anamnesisMessages) Scope(q *gorm.DB) *gorm.DB {
	return q.Where("conversation_id = ?", s.convID).Order("created_at ASC")
}
