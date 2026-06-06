package mneme

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// IndexJob is the payload pushed onto IndexQueue by the RPC server and
// consumed by the graphe worker for async embedding.
type IndexJob struct {
	MessageID string `json:"message_id"`
	OwnerSub  string `json:"owner_sub"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

// MessageRepo provides persistence operations for the messages table.
// rdb is used for the async embedding queue; it is not part of any DB transaction.
type MessageRepo struct {
	db  *gorm.DB
	rdb *redis.Client
}

// Save persists a message and returns its generated ID.
func (r *MessageRepo) Save(ctx context.Context, convID, role, content string) (string, error) {
	m := Message{ID: newID("msg"), ConversationID: convID, Role: role, Content: content}
	return m.ID, r.db.WithContext(ctx).Create(&m).Error
}

// GetHistory returns stored messages for convID in ascending chronological order.
// Ownership is verified via a JOIN on conversations so a client cannot read
// another owner's history by guessing IDs.
func (r *MessageRepo) GetHistory(ctx context.Context, convID, ownerSub string) ([]Message, error) {
	var msgs []Message
	err := r.db.WithContext(ctx).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.conversation_id = ? AND conversations.owner_sub = ?", convID, ownerSub).
		Order("messages.created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// Enqueue pushes an index job onto the Redis queue for the graphe worker (§12).
func (r *MessageRepo) Enqueue(ctx context.Context, job IndexJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return r.rdb.LPush(ctx, IndexQueue, b).Err()
}
