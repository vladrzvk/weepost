package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresPostRepo struct{ db *pgxpool.Pool }

func NewPostgresPostRepo(db *pgxpool.Pool) *PostgresPostRepo { return &PostgresPostRepo{db: db} }

var _ domain.IPostRepo = (*PostgresPostRepo)(nil)

const postCols = `
	id, brand_id, workspace_id, created_by_user_id,
	title, main_caption, status, retry_count,
	scheduled_at, published_at, publish_type, rejection_reason,
	created_at, updated_at, deleted_at, version`

func scanPost(s scanner) (*domain.Post, error) {
	var (
		id, brandID, workspaceID, createdByUserID uuid.UUID
		title, mainCaption                        *string
		status                                    domain.PostStatus
		retryCount                                int
		scheduledAt, publishedAt                  *time.Time
		publishType                               domain.PublishType
		rejectionReason                           *string
		createdAt, updatedAt                      time.Time
		deletedAt                                 *time.Time
		version                                   int
	)
	err := s.Scan(
		&id, &brandID, &workspaceID, &createdByUserID,
		&title, &mainCaption, &status, &retryCount,
		&scheduledAt, &publishedAt, &publishType, &rejectionReason,
		&createdAt, &updatedAt, &deletedAt, &version,
	)
	if err != nil {
		return nil, err
	}
	return domain.RehydratePost(
		id, brandID, workspaceID, createdByUserID,
		title, mainCaption, status, retryCount,
		scheduledAt, publishedAt, publishType, rejectionReason,
		createdAt, updatedAt, deletedAt, version,
	), nil
}

func (r *PostgresPostRepo) Create(ctx context.Context, p *domain.Post) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO posts (
			id, brand_id, workspace_id, created_by_user_id,
			title, main_caption, status, retry_count,
			scheduled_at, published_at, publish_type, rejection_reason,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		p.ID(), p.BrandID(), p.WorkspaceID(), p.CreatedByUserID(),
		p.Title(), p.MainCaption(), p.Status(), p.RetryCount(),
		p.ScheduledAt(), p.PublishedAt(), p.PublishType(), p.RejectionReason(),
		p.CreatedAt(), p.UpdatedAt(),
	)
	return err
}

func (r *PostgresPostRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Post, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+postCols+` FROM posts WHERE id=$1 AND deleted_at IS NULL`, id)
	p, err := scanPost(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *PostgresPostRepo) Update(ctx context.Context, p *domain.Post) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE posts SET
			title=$2, main_caption=$3, status=$4, retry_count=$5,
			scheduled_at=$6, published_at=$7, publish_type=$8, rejection_reason=$9,
			version=version+1,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND version=$10 AND deleted_at IS NULL`,
		p.ID(), p.Title(), p.MainCaption(), p.Status(), p.RetryCount(),
		p.ScheduledAt(), p.PublishedAt(), p.PublishType(), p.RejectionReason(),
		p.Version(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrCodeCONCURRENCY_CONFLICT, "post modified concurrently", nil, domain.SeverityMEDIUM, false)
	}
	return nil
}

func (r *PostgresPostRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE posts
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *PostgresPostRepo) ListByBrand(ctx context.Context, brandID uuid.UUID, filter domain.PostFilter, page domain.PageRequest) ([]*domain.Post, domain.PageResult, error) {
	args := []any{brandID}
	where := []string{"brand_id=$1", "deleted_at IS NULL"}
	args, where = applyPostFilter(args, where, filter)
	return r.listPaginated(ctx, strings.Join(where, " AND "), args, page)
}

func (r *PostgresPostRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, filter domain.PostFilter, page domain.PageRequest) ([]*domain.Post, domain.PageResult, error) {
	args := []any{workspaceID}
	where := []string{"workspace_id=$1", "deleted_at IS NULL"}
	args, where = applyPostFilter(args, where, filter)
	return r.listPaginated(ctx, strings.Join(where, " AND "), args, page)
}

func applyPostFilter(args []any, where []string, f domain.PostFilter) ([]any, []string) {
	if len(f.Status) > 0 {
		placeholders := make([]string, len(f.Status))
		for i, s := range f.Status {
			args = append(args, s)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		where = append(where, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}
	if f.AuthorID != nil {
		args = append(args, *f.AuthorID)
		where = append(where, fmt.Sprintf("created_by_user_id=$%d", len(args)))
	}
	if f.ScheduledBefore != nil {
		args = append(args, *f.ScheduledBefore)
		where = append(where, fmt.Sprintf("scheduled_at<$%d", len(args)))
	}
	return args, where
}

func (r *PostgresPostRepo) listPaginated(ctx context.Context, where string, args []any, page domain.PageRequest) ([]*domain.Post, domain.PageResult, error) {
	var total int64
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM posts WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, domain.PageResult{}, err
	}
	offset := (page.Page - 1) * page.PageSize
	paginatedArgs := append(append([]any{}, args...), page.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT`+postCols+` FROM posts WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(paginatedArgs)-1, len(paginatedArgs),
	)
	rows, err := r.db.Query(ctx, query, paginatedArgs...)
	if err != nil {
		return nil, domain.PageResult{}, err
	}
	defer rows.Close()
	var posts []*domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, domain.PageResult{}, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageResult{}, err
	}
	return posts, domain.PageResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.PageSize,
		HasNext:  int64(offset+page.PageSize) < total,
		HasPrev:  page.Page > 1,
	}, nil
}

func (r *PostgresPostRepo) ListScheduledBefore(ctx context.Context, before time.Time) ([]*domain.Post, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+postCols+`
		FROM posts
		WHERE status='scheduled' AND scheduled_at<=$1 AND deleted_at IS NULL
		ORDER BY scheduled_at ASC`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *PostgresPostRepo) ListFailed(ctx context.Context) ([]*domain.Post, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+postCols+`
		FROM posts
		WHERE status='failed' AND deleted_at IS NULL
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
