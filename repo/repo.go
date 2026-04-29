package repo

import (
	"context"
	"database/sql"
	"log/slog"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	slog.Info("creating repository instance")
	return &Repository{
		db: db,
	}
}
func (r *Repository) SaveMessage(ctx context.Context, user, message string) error {
	query := "INSERT INTO messages (username, message) VALUES ($1, $2)"
	slog.Debug("executing SaveMessage query", "user", user, "message_length", len(message))
	_, err := r.db.ExecContext(ctx, query, user, message)
	if err != nil {
		slog.Error("SaveMessage query failed", "user", user, "error", err)
		return err
	}
	slog.Info("SaveMessage query completed", "user", user)
	return err
}

func (r *Repository) GetMessage(ctx context.Context) (string, string, error) {
	query := "SELECT username, message FROM messages ORDER BY id DESC LIMIT 1"
	slog.Debug("executing GetMessage query")

	var Username string
	var Message string
	err := r.db.QueryRowContext(ctx, query).Scan(&Username, &Message)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Info("GetMessage query completed without rows")
			return "", "", err
		}
		slog.Error("GetMessage query failed", "error", err)
		return "", "", err
	}
	slog.Info("GetMessage query completed", "user", Username, "message_length", len(Message))
	return Username, Message, nil
}
