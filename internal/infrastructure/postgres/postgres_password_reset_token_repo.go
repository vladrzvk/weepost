package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

// A070: password_reset_tokens table absent from Phase 9. Requires migration 0043.
type PostgresPasswordResetTokenRepo struct{ db *pgxpool.Pool }

func NewPostgresPasswordResetTokenRepo(db *pgxpool.Pool) *PostgresPasswordResetTokenRepo {
	return &PostgresPasswordResetTokenRepo{db: db}
}

var _ domain.IPasswordResetTokenRepo = (*PostgresPasswordResetTokenRepo)(nil)

func scanPasswordResetToken(s scanner) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := s.Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PostgresPasswordResetTokenRepo) Create(ctx context.Context, t *domain.PasswordResetToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token, expires_at, used_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, t.Token, t.ExpiresAt, t.UsedAt, t.CreatedAt,
	)
	return err
}

func (r *PostgresPasswordResetTokenRepo) GetByToken(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, token, expires_at, used_at, created_at FROM password_reset_tokens WHERE token=$1`,
		token)
	t, err := scanPasswordResetToken(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (r *PostgresPasswordResetTokenRepo) Invalidate(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at=NOW() AT TIME ZONE 'UTC' WHERE id=$1`, id)
	return err
}

func (r *PostgresPasswordResetTokenRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE expires_at < NOW() AT TIME ZONE 'UTC'`)
	return err
}
