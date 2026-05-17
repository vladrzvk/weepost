package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresSessionRepo struct{ db *pgxpool.Pool }

func NewPostgresSessionRepo(db *pgxpool.Pool) *PostgresSessionRepo {
	return &PostgresSessionRepo{db: db}
}

var _ domain.ISessionRepo = (*PostgresSessionRepo)(nil)

func scanSession(s scanner) (*domain.UserSession, error) {
	var sess domain.UserSession
	// Phase 9 user_sessions DDL: id, user_id, status, expires_at, created_at
	// RevokedReason not in DB — kept as zero value on struct.
	err := s.Scan(&sess.ID, &sess.UserID, &sess.Status, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (r *PostgresSessionRepo) Create(ctx context.Context, s *domain.UserSession) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, status, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		s.ID, s.UserID, s.Status, s.ExpiresAt, s.CreatedAt,
	)
	return err
}

func (r *PostgresSessionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.UserSession, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, status, expires_at, created_at FROM user_sessions WHERE id=$1`, id)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return sess, err
}

func (r *PostgresSessionRepo) GetByJTI(ctx context.Context, jti string) (*domain.UserSession, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, user_id, status, expires_at, created_at FROM user_sessions WHERE jti=$1`, jti)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return sess, err
}

func (r *PostgresSessionRepo) Update(ctx context.Context, s *domain.UserSession) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_sessions SET status=$2, expires_at=$3 WHERE id=$1`,
		s.ID, s.Status, s.ExpiresAt,
	)
	return err
}

func (r *PostgresSessionRepo) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_sessions SET status='revoked'
		WHERE user_id=$1 AND status IN ('active','pending_2fa')`, userID)
	return err
}

func (r *PostgresSessionRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM user_sessions WHERE expires_at < NOW() AT TIME ZONE 'UTC'`)
	return err
}

func (r *PostgresSessionRepo) RevokeByJTI(ctx context.Context, jti string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_sessions SET status='revoked'
		WHERE jti=$1 AND status IN ('active','pending_2fa')`, jti)
	return err
}
