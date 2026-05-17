package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresAuditRepo struct{ db *pgxpool.Pool }

func NewPostgresAuditRepo(db *pgxpool.Pool) *PostgresAuditRepo {
	return &PostgresAuditRepo{db: db}
}

var _ domain.IAuditRepo = (*PostgresAuditRepo)(nil)

func scanAuditEntry(s scanner) (*domain.AuditEntry, error) {
	var (
		ae       domain.AuditEntry
		metaRaw  []byte
	)
	err := s.Scan(
		&ae.ID, &ae.WorkspaceID, &ae.ActorID, &ae.ActorType,
		&ae.Action, &ae.ResourceType, &ae.ResourceID,
		&metaRaw, &ae.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if metaRaw != nil {
		_ = json.Unmarshal(metaRaw, &ae.Metadata)
	}
	return &ae, nil
}

func (r *PostgresAuditRepo) Create(ctx context.Context, e *domain.AuditEntry) error {
	metaBytes, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_logs (
			id, workspace_id, actor_id, actor_type,
			action, resource_type, resource_id, metadata, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, e.WorkspaceID, e.ActorID, e.ActorType,
		e.Action, e.ResourceType, e.ResourceID, metaBytes, e.CreatedAt,
	)
	return err
}

func (r *PostgresAuditRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, filter domain.AuditFilter, page domain.PageRequest) ([]*domain.AuditEntry, domain.PageResult, error) {
	args := []any{workspaceID}
	where := []string{"workspace_id=$1"}
	args, where = applyAuditFilter(args, where, filter)
	return r.listAuditPaginated(ctx, strings.Join(where, " AND "), args, page)
}

func (r *PostgresAuditRepo) ListByUser(ctx context.Context, userID uuid.UUID, filter domain.AuditFilter, page domain.PageRequest) ([]*domain.AuditEntry, domain.PageResult, error) {
	args := []any{userID}
	where := []string{"actor_id=$1"}
	args, where = applyAuditFilter(args, where, filter)
	return r.listAuditPaginated(ctx, strings.Join(where, " AND "), args, page)
}

func applyAuditFilter(args []any, where []string, f domain.AuditFilter) ([]any, []string) {
	if f.Action != "" {
		args = append(args, f.Action)
		where = append(where, fmt.Sprintf("action=$%d", len(args)))
	}
	if f.ActorID != nil {
		args = append(args, *f.ActorID)
		where = append(where, fmt.Sprintf("actor_id=$%d", len(args)))
	}
	if f.ResourceType != nil {
		args = append(args, *f.ResourceType)
		where = append(where, fmt.Sprintf("resource_type=$%d", len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where = append(where, fmt.Sprintf("created_at>=$%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("created_at<=$%d", len(args)))
	}
	return args, where
}

func (r *PostgresAuditRepo) listAuditPaginated(ctx context.Context, where string, args []any, page domain.PageRequest) ([]*domain.AuditEntry, domain.PageResult, error) {
	var total int64
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, domain.PageResult{}, err
	}
	offset := (page.Page - 1) * page.PageSize
	paginatedArgs := append(append([]any{}, args...), page.PageSize, offset)
	query := fmt.Sprintf(
		`SELECT id, workspace_id, actor_id, actor_type, action, resource_type, resource_id, metadata, created_at
		FROM audit_logs WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(paginatedArgs)-1, len(paginatedArgs),
	)
	rows, err := r.db.Query(ctx, query, paginatedArgs...)
	if err != nil {
		return nil, domain.PageResult{}, err
	}
	defer rows.Close()
	var entries []*domain.AuditEntry
	for rows.Next() {
		e, err := scanAuditEntry(rows)
		if err != nil {
			return nil, domain.PageResult{}, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageResult{}, err
	}
	return entries, domain.PageResult{
		Total:    total,
		Page:     page.Page,
		PageSize: page.PageSize,
		HasNext:  int64(offset+page.PageSize) < total,
		HasPrev:  page.Page > 1,
	}, nil
}

// CountFailedLoginsByIP: A073 — extra method, not in IAuditRepo interface.
// Required by SC-C-001 rate limiting.
func (r *PostgresAuditRepo) CountFailedLoginsByIP(ctx context.Context, ipHash string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE metadata->>'ip_hash'=$1
		  AND action='login_failed'
		  AND created_at>=$2`,
		ipHash, since,
	).Scan(&count)
	return count, err
}
