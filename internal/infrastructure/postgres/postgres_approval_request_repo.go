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

type PostgresApprovalRequestRepo struct{ db *pgxpool.Pool }

func NewPostgresApprovalRequestRepo(db *pgxpool.Pool) *PostgresApprovalRequestRepo {
	return &PostgresApprovalRequestRepo{db: db}
}

var _ domain.IApprovalRequestRepo = (*PostgresApprovalRequestRepo)(nil)

const approvalCols = `
	id, post_id, type, status,
	requested_by_member_id, approver_member_id, approver_guest_id,
	reviewed_at, cancelled_at, rejection_reason, created_at, updated_at`

func scanApprovalRequest(s scanner) (*domain.ApprovalRequest, error) {
	var ar domain.ApprovalRequest
	err := s.Scan(
		&ar.ID, &ar.PostID, &ar.Type, &ar.Status,
		&ar.RequestedByMemberID, &ar.ApproverMemberID, &ar.ApproverGuestID,
		&ar.ReviewedAt, &ar.CancelledAt, &ar.RejectionReason, &ar.CreatedAt, &ar.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ar, nil
}

func (r *PostgresApprovalRequestRepo) Create(ctx context.Context, a *domain.ApprovalRequest) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO approval_requests (
			id, post_id, type, status,
			requested_by_member_id, approver_member_id, approver_guest_id,
			reviewed_at, cancelled_at, rejection_reason, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		a.ID, a.PostID, a.Type, a.Status,
		a.RequestedByMemberID, a.ApproverMemberID, a.ApproverGuestID,
		a.ReviewedAt, a.CancelledAt, a.RejectionReason, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *PostgresApprovalRequestRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ApprovalRequest, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+approvalCols+` FROM approval_requests WHERE id=$1`, id)
	a, err := scanApprovalRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// GetByGuestToken: guest token lives in brand_guests table; JOIN to find associated approval.
func (r *PostgresApprovalRequestRepo) GetByGuestToken(ctx context.Context, token string) (*domain.ApprovalRequest, error) {
	row := r.db.QueryRow(ctx, `
		SELECT ar.id, ar.post_id, ar.type, ar.status,
			ar.requested_by_member_id, ar.approver_member_id, ar.approver_guest_id,
			ar.reviewed_at, ar.cancelled_at, ar.rejection_reason, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN brand_guests bg ON bg.id=ar.approver_guest_id
		WHERE bg.token=$1`, token)
	a, err := scanApprovalRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (r *PostgresApprovalRequestRepo) Update(ctx context.Context, a *domain.ApprovalRequest) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approval_requests SET
			status=$2, approver_member_id=$3, approver_guest_id=$4,
			reviewed_at=$5, cancelled_at=$6, rejection_reason=$7,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1`,
		a.ID, a.Status, a.ApproverMemberID, a.ApproverGuestID,
		a.ReviewedAt, a.CancelledAt, a.RejectionReason,
	)
	return err
}

func (r *PostgresApprovalRequestRepo) ListByPost(ctx context.Context, postID uuid.UUID) ([]*domain.ApprovalRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+approvalCols+` FROM approval_requests WHERE post_id=$1 ORDER BY created_at ASC`,
		postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.ApprovalRequest
	for rows.Next() {
		a, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// ListPendingByBrand: approval_requests has no brand_id — JOIN via posts.
func (r *PostgresApprovalRequestRepo) ListPendingByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.ApprovalRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ar.id, ar.post_id, ar.type, ar.status,
			ar.requested_by_member_id, ar.approver_member_id, ar.approver_guest_id,
			ar.reviewed_at, ar.cancelled_at, ar.rejection_reason, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN posts p ON p.id=ar.post_id
		WHERE p.brand_id=$1 AND ar.status='pending' AND p.deleted_at IS NULL
		ORDER BY ar.created_at ASC`, brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.ApprovalRequest
	for rows.Next() {
		a, err := scanApprovalRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// Ensure time import used.
var _ = time.Now
