package mneme

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Store exposes every repository bound to one GORM session — either the base
// connection (returned by UnitOfWork.Store) or a transaction (passed into Do).
// Fields are concrete types; consumers define their own narrow interfaces and
// receive these by value, satisfied implicitly by Go's structural typing.
type Store struct {
	Clients       *ClientRepo
	Conversations *ConversationRepo
	Messages      *MessageRepo
	MemoryIndex   *MemoryIndexRepo
}

func newStore(db *gorm.DB, rdb *redis.Client) *Store {
	return &Store{
		Clients:       &ClientRepo{db: db},
		Conversations: &ConversationRepo{db: db},
		Messages:      &MessageRepo{db: db, rdb: rdb},
		MemoryIndex:   &MemoryIndexRepo{db: db},
	}
}

// UnitOfWork hands out repositories either directly (Store, base connection)
// or atomically within a transaction (Do).
type UnitOfWork interface {
	// Store returns repositories bound to the base connection — no transaction.
	// Use for reads and single-table writes that don't need atomicity.
	Store() *Store
	// Do runs fn inside a transaction, committing on nil and rolling back on
	// error or panic. Nested Do calls use savepoints automatically.
	// Keep side-effects that must not be undone (Redis writes, post-commit reads)
	// AFTER Do returns nil.
	Do(ctx context.Context, fn func(s *Store) error) error
}

type unitOfWork struct {
	db    *gorm.DB
	rdb   *redis.Client
	store *Store
}

// New returns a UnitOfWork backed by db and rdb.
func New(db *gorm.DB, rdb *redis.Client) UnitOfWork {
	return &unitOfWork{db: db, rdb: rdb, store: newStore(db, rdb)}
}

func (u *unitOfWork) Store() *Store { return u.store }

func (u *unitOfWork) Do(ctx context.Context, fn func(s *Store) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(newStore(tx, u.rdb))
	})
}
