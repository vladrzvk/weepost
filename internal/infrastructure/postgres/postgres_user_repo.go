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

type PostgresUserRepo struct{ db *pgxpool.Pool }

func NewPostgresUserRepo(db *pgxpool.Pool) *PostgresUserRepo { return &PostgresUserRepo{db: db} }

var _ domain.IUserRepo = (*PostgresUserRepo)(nil)

const userCols = `
	id, email, email_verified, status, password_hash,
	first_name, last_name, two_fa_enabled, totp_secret, two_fa_backup_codes,
	failed_login_attempts, locked_until, locked, last_login_at, last_login_ip,
	created_at, updated_at, deleted_at`

func scanUser(s scanner) (*domain.User, error) {
	var (
		id                  uuid.UUID
		email               string
		emailVerified       bool
		status              domain.UserStatus
		passwordHash        string
		firstName, lastName string
		twoFAEnabled        bool
		totpSecret          *string
		backupCodes         []string
		failedLogins        int
		lockedUntil         *time.Time
		locked              bool
		lastLoginAt         *time.Time
		lastLoginIP         *string
		createdAt, updatedAt time.Time
		deletedAt           *time.Time
	)
	err := s.Scan(
		&id, &email, &emailVerified, &status, &passwordHash,
		&firstName, &lastName, &twoFAEnabled, &totpSecret, &backupCodes,
		&failedLogins, &lockedUntil, &locked, &lastLoginAt, &lastLoginIP,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateUser(
		id, email, passwordHash, status, emailVerified,
		firstName, lastName, twoFAEnabled, totpSecret, backupCodes,
		failedLogins, lockedUntil, locked, lastLoginAt, lastLoginIP,
		createdAt, updatedAt, deletedAt,
	), nil
}

func (r *PostgresUserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (
			id, email, email_verified, status, password_hash,
			first_name, last_name, two_fa_enabled, totp_secret, two_fa_backup_codes,
			failed_login_attempts, locked_until, locked, last_login_at, last_login_ip,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		u.ID(), u.Email(), u.EmailVerified(), u.Status(), u.PasswordHash(),
		u.FirstName(), u.LastName(), u.TwoFactorEnabled(), u.TOTPSecret(), u.BackupCodes(),
		u.FailedLoginAttempts(), u.LockedUntil(), u.IsLocked(), u.LastLoginAt(), u.LastLoginIP(),
		u.CreatedAt(), u.UpdatedAt(),
	)
	return err
}

func (r *PostgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+userCols+` FROM users WHERE id=$1 AND deleted_at IS NULL`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *PostgresUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT`+userCols+` FROM users WHERE email=$1 AND deleted_at IS NULL`, email)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *PostgresUserRepo) Update(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			email=$2, email_verified=$3, status=$4, password_hash=$5,
			first_name=$6, last_name=$7, two_fa_enabled=$8, totp_secret=$9, two_fa_backup_codes=$10,
			failed_login_attempts=$11, locked_until=$12, locked=$13, last_login_at=$14, last_login_ip=$15,
			updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`,
		u.ID(), u.Email(), u.EmailVerified(), u.Status(), u.PasswordHash(),
		u.FirstName(), u.LastName(), u.TwoFactorEnabled(), u.TOTPSecret(), u.BackupCodes(),
		u.FailedLoginAttempts(), u.LockedUntil(), u.IsLocked(), u.LastLoginAt(), u.LastLoginIP(),
	)
	return err
}

func (r *PostgresUserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET deleted_at=NOW() AT TIME ZONE 'UTC', updated_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *PostgresUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND deleted_at IS NULL)`, email,
	).Scan(&exists)
	return exists, err
}
