package mneme

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Conversation struct {
	ID            string
	CreatedAtUnix int64
}

type ConversationStore struct {
	db *pgxpool.Pool
}

func NewConversationStore(db *pgxpool.Pool) *ConversationStore {
	return &ConversationStore{db: db}
}

// GetOrCreate returns convID if it exists and belongs to ownerSub.
// If convID is empty or not found, a new conversation is created.
func (s *ConversationStore) GetOrCreate(ctx context.Context, ownerSub, convID string) (string, error) {
	if convID != "" {
		var exists bool
		err := s.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1 AND owner_sub = $2)`,
			convID, ownerSub,
		).Scan(&exists)
		if err != nil {
			return "", err
		}
		if exists {
			return convID, nil
		}
	}
	return s.create(ctx, ownerSub)
}

func (s *ConversationStore) List(ctx context.Context, ownerSub string) ([]Conversation, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, extract(epoch from created_at)::bigint
		 FROM conversations
		 WHERE owner_sub = $1
		 ORDER BY created_at DESC`,
		ownerSub,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.CreatedAtUnix); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *ConversationStore) create(ctx context.Context, ownerSub string) (string, error) {
	id := newID("conv")
	_, err := s.db.Exec(ctx,
		`INSERT INTO conversations (id, owner_sub) VALUES ($1, $2)`,
		id, ownerSub,
	)
	return id, err
}

func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}
