package mneme

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	Sub         string
	Email       string
	DisplayName string
	AvatarURL   string
}

type ClientStore struct {
	db *pgxpool.Pool
}

func NewClientStore(db *pgxpool.Pool) *ClientStore {
	return &ClientStore{db: db}
}

func (s *ClientStore) Upsert(ctx context.Context, c Client) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO clients (sub, email, display_name, avatar_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (sub) DO UPDATE SET
		   email        = excluded.email,
		   display_name = excluded.display_name,
		   avatar_url   = excluded.avatar_url,
		   updated_at   = now()`,
		c.Sub, c.Email, c.DisplayName, c.AvatarURL,
	)
	return err
}

func (s *ClientStore) Get(ctx context.Context, sub string) (Client, error) {
	var c Client
	err := s.db.QueryRow(ctx,
		`SELECT sub, email, display_name, coalesce(avatar_url, '')
		 FROM clients WHERE sub = $1`,
		sub,
	).Scan(&c.Sub, &c.Email, &c.DisplayName, &c.AvatarURL)
	return c, err
}
