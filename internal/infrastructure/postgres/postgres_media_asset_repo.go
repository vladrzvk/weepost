package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresMediaAssetRepo struct{ db *pgxpool.Pool }

func NewPostgresMediaAssetRepo(db *pgxpool.Pool) *PostgresMediaAssetRepo {
	return &PostgresMediaAssetRepo{db: db}
}

var _ domain.IMediaAssetRepo = (*PostgresMediaAssetRepo)(nil)

const mediaCols = `
	id, workspace_id, uploaded_by_member_id,
	type, status, original_filename, storage_key,
	mime_type, size_bytes, width_px, height_px,
	duration_seconds, alt_text,
	created_at, updated_at, deleted_at`

func scanMediaAsset(s scanner) (*domain.MediaAsset, error) {
	var ma domain.MediaAsset
	err := s.Scan(
		&ma.ID, &ma.WorkspaceID, &ma.UploadedByUserID,
		&ma.AssetType, &ma.Status, &ma.FilenameOriginal, &ma.StoragePath,
		&ma.MimeType, &ma.FileSizeBytes, &ma.WidthPx, &ma.HeightPx,
		&ma.DurationSeconds, &ma.AltText,
		&ma.CreatedAt, &ma.UpdatedAt, &ma.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ma, nil
}

func (r *PostgresMediaAssetRepo) Create(ctx context.Context, a *domain.MediaAsset) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO media_assets (
			id, workspace_id, uploaded_by_member_id,
			type, status, original_filename, storage_key,
			mime_type, size_bytes, width_px, height_px,
			duration_seconds, alt_text, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		a.ID, a.WorkspaceID, a.UploadedByUserID,
		a.AssetType, a.Status, a.FilenameOriginal, a.StoragePath,
		a.MimeType, a.FileSizeBytes, a.WidthPx, a.HeightPx,
		a.DurationSeconds, a.AltText, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *PostgresMediaAssetRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.MediaAsset, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+mediaCols+` FROM media_assets WHERE id=$1 AND deleted_at IS NULL`, id)
	a, err := scanMediaAsset(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (r *PostgresMediaAssetRepo) Update(ctx context.Context, a *domain.MediaAsset) error {
	_, err := r.db.Exec(ctx, `
		UPDATE media_assets SET
			type=$2, status=$3, original_filename=$4, storage_key=$5,
			mime_type=$6, size_bytes=$7, width_px=$8, height_px=$9,
			duration_seconds=$10, alt_text=$11,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		a.ID, a.AssetType, a.Status, a.FilenameOriginal, a.StoragePath,
		a.MimeType, a.FileSizeBytes, a.WidthPx, a.HeightPx,
		a.DurationSeconds, a.AltText,
	)
	return err
}

func (r *PostgresMediaAssetRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE media_assets
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

// ListByBrand: A074 — media_assets has no brand_id. JOIN via post_media_assets → posts.
func (r *PostgresMediaAssetRepo) ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.MediaAsset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT
			ma.id, ma.workspace_id, ma.uploaded_by_member_id,
			ma.type, ma.status, ma.original_filename, ma.storage_key,
			ma.mime_type, ma.size_bytes, ma.width_px, ma.height_px,
			ma.duration_seconds, ma.alt_text,
			ma.created_at, ma.updated_at, ma.deleted_at
		FROM media_assets ma
		JOIN post_media_assets pma ON pma.media_asset_id=ma.id
		JOIN posts p ON p.id=pma.post_id
		WHERE p.brand_id=$1 AND ma.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY ma.created_at DESC`, brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectMediaAssets(rows)
}

func (r *PostgresMediaAssetRepo) ListByPost(ctx context.Context, postID uuid.UUID) ([]*domain.MediaAsset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			ma.id, ma.workspace_id, ma.uploaded_by_member_id,
			ma.type, ma.status, ma.original_filename, ma.storage_key,
			ma.mime_type, ma.size_bytes, ma.width_px, ma.height_px,
			ma.duration_seconds, ma.alt_text,
			ma.created_at, ma.updated_at, ma.deleted_at
		FROM media_assets ma
		JOIN post_media_assets pma ON pma.media_asset_id=ma.id
		WHERE pma.post_id=$1 AND ma.deleted_at IS NULL
		ORDER BY pma.display_order ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectMediaAssets(rows)
}

func (r *PostgresMediaAssetRepo) ListPendingScan(ctx context.Context) ([]*domain.MediaAsset, error) {
	rows, err := r.db.Query(ctx,
		`SELECT`+mediaCols+` FROM media_assets WHERE status='pending_scan' AND deleted_at IS NULL ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectMediaAssets(rows)
}

func collectMediaAssets(rows pgx.Rows) ([]*domain.MediaAsset, error) {
	var result []*domain.MediaAsset
	for rows.Next() {
		a, err := scanMediaAsset(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
