package mneme

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Memories struct {
	db *gorm.DB
}

// Insert vector(384) is not a native GORM type — writes use raw SQL with a ::vector cast.
func (r *Memories) Insert(ctx context.Context, memory Memory, embedding []float32) error {
	return r.db.WithContext(ctx).Exec(
		"insert into memories (message_id, embedding) "+
			"VALUES (?, ?::vector) ON CONFLICT (message_id) DO NOTHING",
		memory.MessageId, vecLiteral(embedding),
	).Error
}

// vecLiteral formats []float32 as a Postgres vector literal: '[x,y,...]'
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
