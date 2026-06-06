package mneme

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Store is the top-level persistence factory.
//
// For single-repo operations use the accessor methods (Clients, Conversations,
// Messages, MemoryIndex). For operations that must be atomic across multiple
// repos, acquire a UnitOfWork via Begin.
type Store struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewStore wires the GORM DB and Redis client into a single root Store.
func NewStore(db *gorm.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
}

func (s *Store) Clients() *ClientRepo          { return &ClientRepo{db: s.db} }
func (s *Store) Conversations() *ConversationRepo { return &ConversationRepo{db: s.db} }
func (s *Store) Messages() *MessageRepo        { return &MessageRepo{db: s.db, rdb: s.rdb} }
func (s *Store) MemoryIndex() *MemoryIndexRepo { return &MemoryIndexRepo{db: s.db} }

// UnitOfWork groups all repos under a single DB transaction.
//
// Redis operations (Messages.Enqueue) bypass the transaction — they are
// fire-and-forget and cannot be rolled back.
//
// Usage:
//
//	uow, err := store.Begin(ctx)
//	if err != nil { ... }
//	defer uow.Rollback() // no-op after Commit
//	// ... use uow.Clients / uow.Conversations / uow.Messages ...
//	return uow.Commit()
type UnitOfWork struct {
	tx            *gorm.DB
	Clients       *ClientRepo
	Conversations *ConversationRepo
	Messages      *MessageRepo
}

// Begin opens a DB transaction and returns a UnitOfWork.
func (s *Store) Begin(ctx context.Context) (*UnitOfWork, error) {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &UnitOfWork{
		tx:            tx,
		Clients:       &ClientRepo{db: tx},
		Conversations: &ConversationRepo{db: tx},
		Messages:      &MessageRepo{db: tx, rdb: s.rdb},
	}, nil
}

// Commit commits the transaction. Call only on success.
func (u *UnitOfWork) Commit() error {
	return u.tx.Commit().Error
}

// Rollback aborts the transaction. Safe to call after Commit (becomes a no-op).
func (u *UnitOfWork) Rollback() {
	_ = u.tx.Rollback().Error
}
