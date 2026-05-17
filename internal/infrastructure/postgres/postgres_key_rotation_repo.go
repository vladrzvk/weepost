package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresKeyRotationRepo struct{ db *pgxpool.Pool }

func NewPostgresKeyRotationRepo(db *pgxpool.Pool) *PostgresKeyRotationRepo {
	return &PostgresKeyRotationRepo{db: db}
}

var _ domain.IKeyRotationRepo = (*PostgresKeyRotationRepo)(nil)

const keyRotCols = `
	id, key_version, status, initiated_by_member_id,
	channels_total, channels_rotated, channels_failed,
	started_at, completed_at, notes`

func scanKeyRotation(s scanner) (*domain.KeyRotation, error) {
	var kr domain.KeyRotation
	err := s.Scan(
		&kr.ID, &kr.KeyVersion, &kr.Status, &kr.InitiatedByMemberID,
		&kr.ChannelsTotal, &kr.ChannelsRotated, &kr.ChannelsFailed,
		&kr.StartedAt, &kr.CompletedAt, &kr.Notes,
	)
	if err != nil {
		return nil, err
	}
	return &kr, nil
}

func (r *PostgresKeyRotationRepo) Create(ctx context.Context, kr *domain.KeyRotation) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO encryption_key_rotations (
			id, key_version, status, initiated_by_member_id,
			channels_total, channels_rotated, channels_failed,
			started_at, completed_at, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		kr.ID, kr.KeyVersion, kr.Status, kr.InitiatedByMemberID,
		kr.ChannelsTotal, kr.ChannelsRotated, kr.ChannelsFailed,
		kr.StartedAt, kr.CompletedAt, kr.Notes,
	)
	return err
}

func (r *PostgresKeyRotationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.KeyRotation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+keyRotCols+` FROM encryption_key_rotations WHERE id=$1`, id)
	kr, err := scanKeyRotation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return kr, err
}

func (r *PostgresKeyRotationRepo) GetLatest(ctx context.Context) (*domain.KeyRotation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+keyRotCols+` FROM encryption_key_rotations ORDER BY key_version DESC LIMIT 1`)
	kr, err := scanKeyRotation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return kr, err
}

func (r *PostgresKeyRotationRepo) List(ctx context.Context) ([]*domain.KeyRotation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+keyRotCols+` FROM encryption_key_rotations ORDER BY key_version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.KeyRotation
	for rows.Next() {
		kr, err := scanKeyRotation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, kr)
	}
	return result, rows.Err()
}

func (r *PostgresKeyRotationRepo) Update(ctx context.Context, kr *domain.KeyRotation) error {
	_, err := r.db.Exec(ctx, `
		UPDATE encryption_key_rotations SET
			status=$2, channels_total=$3, channels_rotated=$4, channels_failed=$5,
			completed_at=$6, notes=$7
		WHERE id=$1`,
		kr.ID, kr.Status, kr.ChannelsTotal, kr.ChannelsRotated, kr.ChannelsFailed,
		kr.CompletedAt, kr.Notes,
	)
	return err
}
