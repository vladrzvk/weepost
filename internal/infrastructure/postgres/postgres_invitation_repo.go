package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vladrzvk/weepost/internal/domain"
)

type PostgresInvitationRepo struct{ db *pgxpool.Pool }

func NewPostgresInvitationRepo(db *pgxpool.Pool) *PostgresInvitationRepo {
	return &PostgresInvitationRepo{db: db}
}

var _ domain.IInvitationRepo = (*PostgresInvitationRepo)(nil)

const invitationCols = `
	id, workspace_id, inviter_member_id, email, role, status,
	token, expires_at, accepted_at, created_at, updated_at`

func scanInvitation(s scanner) (*domain.WorkspaceInvitation, error) {
	var inv domain.WorkspaceInvitation
	err := s.Scan(
		&inv.ID, &inv.WorkspaceID, &inv.InviterMemberID,
		&inv.Email, &inv.Role, &inv.Status,
		&inv.Token, &inv.ExpiresAt, &inv.AcceptedAt,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *PostgresInvitationRepo) Create(ctx context.Context, inv *domain.WorkspaceInvitation) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO workspace_invitations (
			id, workspace_id, inviter_member_id, email, role, status,
			token, expires_at, accepted_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		inv.ID, inv.WorkspaceID, inv.InviterMemberID,
		inv.Email, inv.Role, inv.Status,
		inv.Token, inv.ExpiresAt, inv.AcceptedAt,
		inv.CreatedAt, inv.UpdatedAt,
	)
	return err
}

func (r *PostgresInvitationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.WorkspaceInvitation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+invitationCols+` FROM workspace_invitations WHERE id=$1`, id)
	inv, err := scanInvitation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return inv, err
}

func (r *PostgresInvitationRepo) GetByToken(ctx context.Context, token string) (*domain.WorkspaceInvitation, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+invitationCols+` FROM workspace_invitations WHERE token=$1`, token)
	inv, err := scanInvitation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return inv, err
}

func (r *PostgresInvitationRepo) Update(ctx context.Context, inv *domain.WorkspaceInvitation) error {
	_, err := r.db.Exec(ctx, `
		UPDATE workspace_invitations SET
			status=$2, accepted_at=$3, updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1`,
		inv.ID, inv.Status, inv.AcceptedAt,
	)
	return err
}

func (r *PostgresInvitationRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.WorkspaceInvitation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+invitationCols+` FROM workspace_invitations WHERE workspace_id=$1 ORDER BY created_at DESC`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.WorkspaceInvitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, inv)
	}
	return result, rows.Err()
}

func (r *PostgresInvitationRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM workspace_invitations
		WHERE expires_at < NOW() AT TIME ZONE 'UTC' AND status='pending'`)
	return err
}
