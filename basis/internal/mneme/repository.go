package mneme

import "context"

// ClientRepository is the persistence contract for the clients table.
type ClientRepository interface {
	Upsert(ctx context.Context, c Client) error
	Get(ctx context.Context, sub string) (Client, error)
}

// ConversationRepository is the persistence contract for the conversations table.
type ConversationRepository interface {
	GetOrCreate(ctx context.Context, ownerSub, convID string) (string, error)
	List(ctx context.Context, ownerSub string) ([]Conversation, error)
}

// MessageRepository is the persistence contract for the messages table.
// Enqueue is a Redis side-effect: it is not transactional and cannot be rolled back.
type MessageRepository interface {
	Save(ctx context.Context, convID, role, content string) (string, error)
	GetHistory(ctx context.Context, convID, ownerSub string) ([]Message, error)
	Enqueue(ctx context.Context, job IndexJob) error
}

// MemoryIndexRepository is the persistence contract for the memory_index table.
type MemoryIndexRepository interface {
	Insert(ctx context.Context, job IndexJob, embedding []float32) error
}
