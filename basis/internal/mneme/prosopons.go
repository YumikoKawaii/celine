package mneme

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Prosopons struct {
	db *gorm.DB
}

func (r *Prosopons) Upsert(ctx context.Context, c *Prosopon) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sub"}},
			UpdateAll: true,
		}).
		Create(c).Error
}

func (r *Prosopons) Get(ctx context.Context, scope Scope) (Prosopon, error) {
	var c Prosopon
	query := r.db.WithContext(ctx).Model(&Prosopon{})
	if scope != nil {
		query = scope.Scope(query)
	}
	err := query.First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Prosopon{}, err
	}
	return c, err
}