package mneme

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Message struct {
	ID             string
	ConversationID string
	Role           string // "user" | "assistant"
	Content        string
	CreatedAtUnix  int64
}

// IndexJob is the payload pushed onto IndexQueue by the server and consumed
// by the graphe worker.
type IndexJob struct {
	MessageID string `json:"message_id"`
	OwnerSub  string `json:"owner_sub"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

type MessageStore struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewMessageStore(db *pgxpool.Pool, rdb *redis.Client) *MessageStore {
	return &MessageStore{db: db, rdb: rdb}
}

// Save persists the message and returns its generated ID.
func (s *MessageStore) Save(ctx context.Context, convID, role, content string) (string, error) {
	id := newID("msg")
	_, err := s.db.Exec(ctx,
		`INSERT INTO messages (id, conversation_id, role, content) VALUES ($1, $2, $3, $4)`,
		id, convID, role, content,
	)
	return id, err
}

// Enqueue pushes an index job onto the Redis queue for the graphe worker.
func (s *MessageStore) Enqueue(ctx context.Context, job IndexJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.rdb.LPush(ctx, IndexQueue, b).Err()
}

// GetHistory returns messages for convID in ascending order, verifying ownership via JOIN.
func (s *MessageStore) GetHistory(ctx context.Context, convID, ownerSub string) ([]Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT m.id, m.role, m.content, extract(epoch from m.created_at)::bigint
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 WHERE m.conversation_id = $1 AND c.owner_sub = $2
		 ORDER BY m.created_at ASC`,
		convID, ownerSub,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		m.ConversationID = convID
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAtUnix); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
