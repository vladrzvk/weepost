package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresChannelRepo struct{ db *pgxpool.Pool }

func NewPostgresChannelRepo(db *pgxpool.Pool) *PostgresChannelRepo {
	return &PostgresChannelRepo{db: db}
}

var _ domain.IChannelRepo = (*PostgresChannelRepo)(nil)

// A071: external_id maps to platformAccountID; display_name maps to platformAccountName.
const channelCols = `
	id, brand_id, workspace_id, type, status,
	external_id, display_name, consecutive_failures,
	last_failure_at, token_expires_at,
	created_at, updated_at, deleted_at`

func scanChannel(s scanner) (*domain.Channel, error) {
	var (
		id, brandID, workspaceID uuid.UUID
		channelType              domain.ChannelType
		status                   domain.ChannelStatus
		externalID               string
		displayName              string
		consecutiveFailures      int
		lastFailureAt            *time.Time
		tokenExpiresAt           *time.Time
		createdAt, updatedAt     time.Time
		deletedAt                *time.Time
	)
	err := s.Scan(
		&id, &brandID, &workspaceID, &channelType, &status,
		&externalID, &displayName, &consecutiveFailures,
		&lastFailureAt, &tokenExpiresAt,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateChannel(
		id, brandID, workspaceID, channelType, status,
		externalID, displayName, consecutiveFailures,
		lastFailureAt, tokenExpiresAt,
		createdAt, updatedAt, deletedAt,
	), nil
}

func (r *PostgresChannelRepo) Create(ctx context.Context, c *domain.Channel) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO channels (
			id, brand_id, workspace_id, type, status,
			external_id, display_name, consecutive_failures,
			last_failure_at, token_expires_at,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		c.ID(), c.BrandID(), c.WorkspaceID(), c.ChannelType(), c.Status(),
		c.PlatformAccountID(), c.PlatformAccountName(), c.ConsecutiveFailures(),
		c.LastFailureAt(), c.TokenExpiresAt(),
		c.CreatedAt(), c.UpdatedAt(),
	)
	return err
}

func (r *PostgresChannelRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+channelCols+` FROM channels WHERE id=$1 AND deleted_at IS NULL`, id)
	c, err := scanChannel(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *PostgresChannelRepo) Update(ctx context.Context, c *domain.Channel) error {
	_, err := r.db.Exec(ctx, `
		UPDATE channels SET
			type=$2, status=$3, external_id=$4, display_name=$5,
			consecutive_failures=$6, last_failure_at=$7, token_expires_at=$8,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		c.ID(), c.ChannelType(), c.Status(),
		c.PlatformAccountID(), c.PlatformAccountName(),
		c.ConsecutiveFailures(), c.LastFailureAt(), c.TokenExpiresAt(),
	)
	return err
}

func (r *PostgresChannelRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE channels
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *PostgresChannelRepo) ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.Channel, error) {
	rows, err := r.db.Query(ctx,
		`SELECT`+channelCols+` FROM channels WHERE brand_id=$1 AND deleted_at IS NULL ORDER BY created_at ASC`,
		brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectChannels(rows)
}

func (r *PostgresChannelRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Channel, error) {
	rows, err := r.db.Query(ctx,
		`SELECT`+channelCols+` FROM channels WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at ASC`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectChannels(rows)
}

func (r *PostgresChannelRepo) GetByPlatformAccountID(ctx context.Context, platformAccountID string) (*domain.Channel, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+channelCols+` FROM channels WHERE external_id=$1 AND deleted_at IS NULL`, platformAccountID)
	c, err := scanChannel(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func collectChannels(rows pgx.Rows) ([]*domain.Channel, error) {
	var result []*domain.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
