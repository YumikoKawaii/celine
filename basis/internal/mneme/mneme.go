package mneme

import (
	"context"

	"gorm.io/gorm"
)

type Mneme struct {
	db *gorm.DB
}

func NewMneme(db *gorm.DB) *Mneme {
	return &Mneme{db: db}
}

func (s *Mneme) Prosopons() *Prosopons         { return &Prosopons{db: s.db} }
func (s *Mneme) Conversations() *Conversations { return &Conversations{db: s.db} }
func (s *Mneme) Messages() *Messages           { return &Messages{db: s.db} }
func (s *Mneme) Memories() *Memories           { return &Memories{db: s.db} }

// Tx runs fn inside a DB transaction. Commits on nil, rolls back on error or panic.
// Nested calls use savepoints automatically.
func (s *Mneme) Tx(ctx context.Context, fn func(*Mneme) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Mneme{db: tx})
	})
}
