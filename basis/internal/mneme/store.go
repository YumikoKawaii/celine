package mneme

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Store is the top-level persistence entry point.
// For single-repo operations call the accessor methods directly.
// For multi-repo atomic operations use Tx.
type Store struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewStore(db *gorm.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
}

func (s *Store) Clients() *ClientRepo          { return &ClientRepo{db: s.db} }
func (s *Store) Conversations() *ConversationRepo { return &ConversationRepo{db: s.db} }
func (s *Store) Messages() *MessageRepo        { return &MessageRepo{db: s.db, rdb: s.rdb} }
func (s *Store) MemoryIndex() *MemoryIndexRepo { return &MemoryIndexRepo{db: s.db} }

// Tx runs fn inside a DB transaction, passing a tx-scoped *Store.
// Commits on nil return, rolls back on error or panic.
// Nested Tx calls use savepoints automatically (GORM behaviour).
func (s *Store) Tx(ctx context.Context, fn func(*Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{db: tx, rdb: s.rdb})
	})
}
