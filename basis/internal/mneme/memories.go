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

// MemoryHit is one row returned by a recall search: the remembered message
// text plus its cosine similarity to the query (1.0 = identical direction).
type MemoryHit struct {
	MessageId  int64
	ProsoponId int64
	Content    string
	Similarity float64
}

// Search returns the top-k memories most similar to embedding, scoped to a
// single client's conversations and ordered by cosine similarity (highest
// first). Brute-force exact scan — no ANN index — per §11; fine at this scale.
//
// ownerProsoponId is the client whose memories to search: the join walks
// memories → messages → conversations and filters on conversations.prosopon_id,
// so both the client's own messages and Celine's replies in that thread are
// recallable (both are indexed, §12).
func (r *Memories) Search(ctx context.Context, ownerProsoponId int64, embedding []float32, k int) ([]MemoryHit, error) {
	if k <= 0 {
		k = 5
	}
	var hits []MemoryHit
	err := r.db.WithContext(ctx).Raw(
		`SELECT m.id           AS message_id,
		        m.prosopon_id  AS prosopon_id,
		        m.content      AS content,
		        1 - (mem.embedding <=> ?::vector) AS similarity
		   FROM memories mem
		   JOIN messages m      ON m.id = mem.message_id
		   JOIN conversations c ON c.id = m.conversation_id
		  WHERE c.prosopon_id = ?
		  ORDER BY mem.embedding <=> ?::vector
		  LIMIT ?`,
		vecLiteral(embedding), ownerProsoponId, vecLiteral(embedding), k,
	).Scan(&hits).Error
	return hits, err
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
