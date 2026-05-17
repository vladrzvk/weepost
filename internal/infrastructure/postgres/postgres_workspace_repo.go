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

type PostgresWorkspaceRepo struct{ db *pgxpool.Pool }

func NewPostgresWorkspaceRepo(db *pgxpool.Pool) *PostgresWorkspaceRepo {
	return &PostgresWorkspaceRepo{db: db}
}

var _ domain.IWorkspaceRepo = (*PostgresWorkspaceRepo)(nil)

const workspaceCols = `
	id, slug, name, owner_user_id, status, current_mode, plan_id,
	settings_language, settings_timezone, settings_date_format, settings_time_format, settings_week_start_day, settings_notifications_enabled,
	limits_max_members, limits_max_brands, limits_max_channels,
	created_at, updated_at, deleted_at`

func scanWorkspace(s scanner) (*domain.Workspace, error) {
	var (
		id                           uuid.UUID
		slug, name                   string
		ownerUserID                  uuid.UUID
		status                       domain.WorkspaceStatus
		mode                         domain.WorkspaceMode
		planID                       string
		sLang, sTZ, sDateFmt, sTimeFmt string
		sWeekStart                   int
		sNotif                       bool
		lMembers, lBrands, lChannels int
		createdAt, updatedAt         time.Time
		deletedAt                    *time.Time
	)
	err := s.Scan(
		&id, &slug, &name, &ownerUserID, &status, &mode, &planID,
		&sLang, &sTZ, &sDateFmt, &sTimeFmt, &sWeekStart, &sNotif,
		&lMembers, &lBrands, &lChannels,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateWorkspace(
		id, slug, name, ownerUserID, status, mode, planID,
		domain.WorkspaceSettings{
			Timezone:             sTZ,
			Language:             sLang,
			DateFormat:           sDateFmt,
			TimeFormat:           sTimeFmt,
			WeekStartDay:         sWeekStart,
			NotificationsEnabled: sNotif,
		},
		domain.WorkspaceLimits{MaxMembers: lMembers, MaxBrands: lBrands, MaxChannels: lChannels},
		createdAt, updatedAt, deletedAt,
	), nil
}

func (r *PostgresWorkspaceRepo) Create(ctx context.Context, w *domain.Workspace) error {
	s, l := w.Settings(), w.Limits()
	_, err := r.db.Exec(ctx, `
		INSERT INTO workspaces (
			id, slug, name, owner_user_id, status, current_mode, plan_id,
			settings_language, settings_timezone, settings_date_format, settings_time_format, settings_week_start_day, settings_notifications_enabled,
			limits_max_members, limits_max_brands, limits_max_channels,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		w.ID(), w.Slug(), w.Name(), w.OwnerUserID(), w.Status(), w.Mode(), w.PlanID(),
		s.Language, s.Timezone, s.DateFormat, s.TimeFormat, s.WeekStartDay, s.NotificationsEnabled,
		l.MaxMembers, l.MaxBrands, l.MaxChannels,
		w.CreatedAt(), w.UpdatedAt(),
	)
	return err
}

func (r *PostgresWorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+workspaceCols+` FROM workspaces WHERE id=$1 AND deleted_at IS NULL`, id)
	w, err := scanWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

func (r *PostgresWorkspaceRepo) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+workspaceCols+` FROM workspaces WHERE slug=$1 AND deleted_at IS NULL`, slug)
	w, err := scanWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

func (r *PostgresWorkspaceRepo) Update(ctx context.Context, w *domain.Workspace) error {
	s, l := w.Settings(), w.Limits()
	_, err := r.db.Exec(ctx, `
		UPDATE workspaces SET
			slug=$2, name=$3, status=$4, current_mode=$5, plan_id=$6,
			settings_language=$7, settings_timezone=$8, settings_date_format=$9, settings_time_format=$10, settings_week_start_day=$11, settings_notifications_enabled=$12,
			limits_max_members=$13, limits_max_brands=$14, limits_max_channels=$15,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		w.ID(), w.Slug(), w.Name(), w.Status(), w.Mode(), w.PlanID(),
		s.Language, s.Timezone, s.DateFormat, s.TimeFormat, s.WeekStartDay, s.NotificationsEnabled,
		l.MaxMembers, l.MaxBrands, l.MaxChannels,
	)
	return err
}

func (r *PostgresWorkspaceRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE workspaces
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *PostgresWorkspaceRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Workspace, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+workspaceCols+`
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id=w.id AND wm.user_id=$1 AND wm.deleted_at IS NULL
		WHERE w.deleted_at IS NULL
		ORDER BY w.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

const memberCols = `id, workspace_id, user_id, role, status, invited_by_member_id, created_at, updated_at, deleted_at`

func scanWorkspaceMember(s scanner) (*domain.WorkspaceMember, error) {
	var (
		id, workspaceID, userID uuid.UUID
		role                    domain.MemberRole
		status                  domain.MemberStatus
		invitedBy               *uuid.UUID
		createdAt, updatedAt    time.Time
		deletedAt               *time.Time
	)
	err := s.Scan(&id, &workspaceID, &userID, &role, &status, &invitedBy, &createdAt, &updatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateWorkspaceMember(id, workspaceID, userID, role, status, invitedBy, createdAt, updatedAt, deletedAt), nil
}

func (r *PostgresWorkspaceRepo) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*domain.WorkspaceMember, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+memberCols+` FROM workspace_members WHERE workspace_id=$1 AND user_id=$2 AND deleted_at IS NULL`,
		workspaceID, userID)
	m, err := scanWorkspaceMember(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func (r *PostgresWorkspaceRepo) AddMember(ctx context.Context, m *domain.WorkspaceMember) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO workspace_members (id, workspace_id, user_id, role, status, invited_by_member_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		m.ID, m.WorkspaceID, m.UserID, m.Role, m.Status, m.InvitedByMemberID, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *PostgresWorkspaceRepo) UpdateMember(ctx context.Context, m *domain.WorkspaceMember) error {
	_, err := r.db.Exec(ctx, `
		UPDATE workspace_members SET role=$2, status=$3, updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		m.ID, m.Role, m.Status,
	)
	return err
}

func (r *PostgresWorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, memberID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE workspace_members
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE workspace_id=$1 AND id=$2 AND deleted_at IS NULL`,
		workspaceID, memberID,
	)
	return err
}

func (r *PostgresWorkspaceRepo) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*domain.WorkspaceMember, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+memberCols+` FROM workspace_members WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at ASC`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.WorkspaceMember
	for rows.Next() {
		m, err := scanWorkspaceMember(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
