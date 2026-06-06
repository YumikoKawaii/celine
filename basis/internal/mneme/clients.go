package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ClientRepo struct {
	db *gorm.DB
}

func (r *ClientRepo) Upsert(ctx context.Context, c Client) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sub"}},
			// Never overwrite memory_md or persona_note on login — those are managed separately.
			DoUpdates: clause.AssignmentColumns([]string{
				"email", "display_name", "avatar_url", "updated_at",
			}),
		}).
		Create(&c).Error
}

func (r *ClientRepo) Get(ctx context.Context, sub string) (Client, error) {
	var c Client
	err := r.db.WithContext(ctx).Where("sub = ?", sub).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Client{}, err
	}
	return c, err
}
