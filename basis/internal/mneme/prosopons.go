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

func (r *Prosopons) Get(ctx context.Context, filter ProsoponFilter) (Prosopon, error) {
	var c Prosopon
	err := r.db.WithContext(ctx).Where("sub = ?", filter.Sub).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Prosopon{}, err
	}
	return c, err
}

type ProsoponFilter struct {
	Sub string
}
