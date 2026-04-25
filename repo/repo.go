package repo

import (
	"context"
	"database/sql"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db: db,
	}
}
func (r *Repository) SaveMessage(ctx context.Context, user, message string) error {
	query := "INSERT INTO messages (username, message) VALUES ($1, $2)"
	_, err := r.db.ExecContext(ctx, query, user, message)
	return err
}

func (r *Repository) GetMessage(ctx context.Context) (string, string, error) {
	query := "SELECT username, message FROM messages ORDER BY id DESC LIMIT 1"

	var Username string
	var Message string
	err := r.db.QueryRowContext(ctx, query).Scan(&Username, &Message)
	if err != nil {
		return "", "", err
	}
	return Username, Message, nil
}
