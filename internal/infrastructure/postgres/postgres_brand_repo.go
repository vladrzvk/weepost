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

type PostgresBrandRepo struct{ db *pgxpool.Pool }

func NewPostgresBrandRepo(db *pgxpool.Pool) *PostgresBrandRepo { return &PostgresBrandRepo{db: db} }

var _ domain.IBrandRepo = (*PostgresBrandRepo)(nil)

const brandCols = `
	id, workspace_id, name, slug, status, industry,
	identity_logo_url, identity_primary_color, identity_secondary_color,
	tone_formality, tone_humor_level, tone_emojis_allowed,
	created_at, updated_at, deleted_at`

func scanBrand(s scanner) (*domain.Brand, error) {
	var (
		id, workspaceID                         uuid.UUID
		name, slug                              string
		status                                  domain.BrandStatus
		industry                                *string
		logoURL, primaryColor, secondaryColor   *string
		toneFormality, toneHumor                string
		toneEmojis                              bool
		createdAt, updatedAt                    time.Time
		deletedAt                               *time.Time
	)
	err := s.Scan(
		&id, &workspaceID, &name, &slug, &status, &industry,
		&logoURL, &primaryColor, &secondaryColor,
		&toneFormality, &toneHumor, &toneEmojis,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateBrand(
		id, workspaceID, name, slug, status, industry,
		domain.BrandIdentity{LogoURL: logoURL, PrimaryColor: primaryColor, SecondaryColor: secondaryColor},
		domain.ToneOfVoice{Formality: toneFormality, HumorLevel: toneHumor, EmojisAllowed: toneEmojis},
		createdAt, updatedAt, deletedAt,
	), nil
}

func (r *PostgresBrandRepo) Create(ctx context.Context, b *domain.Brand) error {
	id, t := b.Identity(), b.Tone()
	_, err := r.db.Exec(ctx, `
		INSERT INTO brands (
			id, workspace_id, name, slug, status, industry,
			identity_logo_url, identity_primary_color, identity_secondary_color,
			tone_formality, tone_humor_level, tone_emojis_allowed,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		b.ID(), b.WorkspaceID(), b.Name(), b.Slug(), b.Status(), b.Industry(),
		id.LogoURL, id.PrimaryColor, id.SecondaryColor,
		t.Formality, t.HumorLevel, t.EmojisAllowed,
		b.CreatedAt(), b.UpdatedAt(),
	)
	return err
}

func (r *PostgresBrandRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+brandCols+` FROM brands WHERE id=$1 AND deleted_at IS NULL`, id)
	b, err := scanBrand(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return b, err
}

func (r *PostgresBrandRepo) Update(ctx context.Context, b *domain.Brand) error {
	id, t := b.Identity(), b.Tone()
	_, err := r.db.Exec(ctx, `
		UPDATE brands SET
			name=$2, slug=$3, status=$4, industry=$5,
			identity_logo_url=$6, identity_primary_color=$7, identity_secondary_color=$8,
			tone_formality=$9, tone_humor_level=$10, tone_emojis_allowed=$11,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		b.ID(), b.Name(), b.Slug(), b.Status(), b.Industry(),
		id.LogoURL, id.PrimaryColor, id.SecondaryColor,
		t.Formality, t.HumorLevel, t.EmojisAllowed,
	)
	return err
}

func (r *PostgresBrandRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE brands
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *PostgresBrandRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Brand, error) {
	rows, err := r.db.Query(ctx,
		`SELECT`+brandCols+` FROM brands WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at ASC`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Brand
	for rows.Next() {
		b, err := scanBrand(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (r *PostgresBrandRepo) ExistsBySlugInWorkspace(ctx context.Context, slug string, workspaceID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM brands WHERE workspace_id=$1 AND slug=$2 AND deleted_at IS NULL)`,
		workspaceID, slug).Scan(&exists)
	return exists, err
}

const assignmentCols = `id, brand_id, member_id, role, assigned_by_member_id, created_at, updated_at`

func scanBrandAssignment(s scanner) (*domain.BrandAssignment, error) {
	var (
		id, brandID, memberID uuid.UUID
		role                  domain.BrandRole
		assignedBy            *uuid.UUID
		createdAt, updatedAt  time.Time
	)
	err := s.Scan(&id, &brandID, &memberID, &role, &assignedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateBrandAssignment(id, brandID, memberID, role, assignedBy, createdAt, updatedAt), nil
}

func (r *PostgresBrandRepo) AddAssignment(ctx context.Context, a *domain.BrandAssignment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO brand_assignments (id, brand_id, member_id, role, assigned_by_member_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		a.ID, a.BrandID, a.MemberID, a.Role, a.AssignedByMemberID, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *PostgresBrandRepo) UpdateAssignment(ctx context.Context, a *domain.BrandAssignment) error {
	_, err := r.db.Exec(ctx, `
		UPDATE brand_assignments SET role=$2, updated_at=NOW() AT TIME ZONE 'UTC' WHERE id=$1`,
		a.ID, a.Role,
	)
	return err
}

func (r *PostgresBrandRepo) RemoveAssignment(ctx context.Context, brandID, memberID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM brand_assignments WHERE brand_id=$1 AND member_id=$2`, brandID, memberID)
	return err
}

func (r *PostgresBrandRepo) GetAssignment(ctx context.Context, brandID, memberID uuid.UUID) (*domain.BrandAssignment, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+assignmentCols+` FROM brand_assignments WHERE brand_id=$1 AND member_id=$2`,
		brandID, memberID)
	a, err := scanBrandAssignment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (r *PostgresBrandRepo) ListAssignments(ctx context.Context, brandID uuid.UUID) ([]*domain.BrandAssignment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+assignmentCols+` FROM brand_assignments WHERE brand_id=$1 ORDER BY created_at ASC`,
		brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.BrandAssignment
	for rows.Next() {
		a, err := scanBrandAssignment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
