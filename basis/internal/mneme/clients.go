package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClientRepo provides persistence operations for the clients table.
type ClientRepo struct {
	db *gorm.DB
}

// Upsert inserts a new client or updates email, display_name, and avatar_url
// on conflict with an existing sub.
func (r *ClientRepo) Upsert(ctx context.Context, c Client) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sub"}},
			// Only update mutable profile fields; never overwrite memory_md or persona_note.
			DoUpdates: clause.AssignmentColumns([]string{
				"email", "display_name", "avatar_url", "updated_at",
			}),
		}).
		Create(&c).Error
}

// Get returns the client with the given sub.
// Returns gorm.ErrRecordNotFound (wrapped) if the sub does not exist.
func (r *ClientRepo) Get(ctx context.Context, sub string) (Client, error) {
	var c Client
	err := r.db.WithContext(ctx).Where("sub = ?", sub).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Client{}, err
	}
	return c, err
}
