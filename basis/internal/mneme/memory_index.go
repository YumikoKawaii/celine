package mneme

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// MemoryIndexRepo provides persistence operations for the memory_index table (§12).
// The embedding column is vector(384) — not supported by GORM natively, so all
// writes go through raw SQL with a ::vector cast.
type MemoryIndexRepo struct {
	db *gorm.DB
}

func NewMemoryIndexRepo(db *gorm.DB) *MemoryIndexRepo {
	return &MemoryIndexRepo{db: db}
}

// Insert embeds a message into memory_index, skipping duplicates (ON CONFLICT DO NOTHING).
func (r *MemoryIndexRepo) Insert(ctx context.Context, job IndexJob, embedding []float32) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO memory_index (owner_sub, message_id, role, content, embedding) "+
			"VALUES (?, ?, ?, ?, ?::vector) ON CONFLICT (message_id) DO NOTHING",
		job.OwnerSub, job.MessageID, job.Role, job.Content, vecLiteral(embedding),
	).Error
}

// vecLiteral serialises a float32 slice as a Postgres vector literal: '[x,y,...]'.
// Used as the ?::vector argument for pgvector casts.
func vecLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
