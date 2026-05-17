package domain

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// ---------------------------------------------------------------------------
// 1. UserStatus — Phase 5 §17b SM-UserStatus
// ---------------------------------------------------------------------------

type UserStatus string

const (
	UserStatusPendingVerification UserStatus = "pending_verification"
	UserStatusActive              UserStatus = "active"
	UserStatusLocked              UserStatus = "locked"
	UserStatusSuspended           UserStatus = "suspended"
	UserStatusDeleted             UserStatus = "deleted"
)

var userStatusTransitions = map[UserStatus][]UserStatus{
	UserStatusPendingVerification: {UserStatusActive},
	UserStatusActive:              {UserStatusLocked, UserStatusSuspended, UserStatusDeleted},
	UserStatusLocked:              {UserStatusActive},
	UserStatusSuspended:           {UserStatusActive},
	UserStatusDeleted:             {},
}

func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusPendingVerification, UserStatusActive, UserStatusLocked,
		UserStatusSuspended, UserStatusDeleted:
		return true
	}
	return false
}

func (s UserStatus) IsTerminal() bool { return s == UserStatusDeleted }

func (s UserStatus) CanTransitionTo(target UserStatus) error {
	allowed := userStatusTransitions[s]
	for _, t := range allowed {
		if t == target {
			return nil
		}
	}
	return NewDomainError(
		ErrCodeINVALID_STATUS_TRANSITION,
		"Transition de statut utilisateur interdite",
		map[string]interface{}{
			"current_status": string(s),
			"target_status":  string(target),
		},
		SeverityMEDIUM,
		false,
	)
}

// ---------------------------------------------------------------------------
// 2. SessionStatus — Phase 5 §SM-17 (avec pending_2fa)
// ---------------------------------------------------------------------------

type SessionStatus string

const (
	SessionStatusActive     SessionStatus = "active"
	SessionStatusExpired    SessionStatus = "expired"
	SessionStatusRevoked    SessionStatus = "revoked"
	SessionStatusPending2FA SessionStatus = "pending_2fa"
)

var sessionStatusTransitions = map[SessionStatus][]SessionStatus{
	SessionStatusActive:     {SessionStatusExpired, SessionStatusRevoked},
	SessionStatusPending2FA: {SessionStatusActive, SessionStatusRevoked},
	SessionStatusExpired:    {},
	SessionStatusRevoked:    {},
}

func (s SessionStatus) IsTerminal() bool {
	return s == SessionStatusExpired || s == SessionStatusRevoked
}

// ---------------------------------------------------------------------------
// 3. User — aggregate root
// ---------------------------------------------------------------------------

const userFailedLoginThreshold = 5

// User est l'aggregate root de l'identité. Champs privés, accès via getters/méthodes.
type User struct {
	id                  uuid.UUID
	email               string
	passwordHash        string
	firstName           string
	lastName            string
	avatarURL           *string
	status              UserStatus
	emailVerified       bool
	emailVerifiedAt     *time.Time
	twoFactorEnabled    bool
	totpSecret          *string
	backupCodes         []string
	failedLoginAttempts int
	lockedUntil         *time.Time
	lastLoginAt         *time.Time
	lastLoginIP         *string
	createdAt           time.Time
	updatedAt           time.Time
	deletedAt           *time.Time
	events              []DomainEvent
}

// Getters

func (u *User) ID() uuid.UUID             { return u.id }
func (u *User) Email() string             { return u.email }
func (u *User) PasswordHash() string      { return u.passwordHash }
func (u *User) FirstName() string         { return u.firstName }
func (u *User) LastName() string          { return u.lastName }
func (u *User) AvatarURL() *string        { return u.avatarURL }
func (u *User) Status() UserStatus        { return u.status }
func (u *User) EmailVerified() bool       { return u.emailVerified }
func (u *User) EmailVerifiedAt() *time.Time { return u.emailVerifiedAt }
func (u *User) TwoFactorEnabled() bool    { return u.twoFactorEnabled }
func (u *User) TOTPSecret() *string       { return u.totpSecret }
func (u *User) BackupCodes() []string     { return u.backupCodes }
func (u *User) FailedLoginAttempts() int  { return u.failedLoginAttempts }
func (u *User) LockedUntil() *time.Time   { return u.lockedUntil }
func (u *User) LastLoginAt() *time.Time   { return u.lastLoginAt }
func (u *User) LastLoginIP() *string      { return u.lastLoginIP }
func (u *User) CreatedAt() time.Time      { return u.createdAt }
func (u *User) UpdatedAt() time.Time      { return u.updatedAt }
func (u *User) DeletedAt() *time.Time     { return u.deletedAt }
func (u *User) Events() []DomainEvent     { return u.events }
func (u *User) ClearEvents()              { u.events = nil }

// NewUser crée un compte utilisateur. Status initial : pending_verification (SM-17b).
// passwordHash doit être pré-calculé via Password VO (U-3).
func NewUser(email, passwordHash string) (*User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'email ne peut pas être vide",
			map[string]interface{}{"field": "email"},
			SeverityLOW,
			false,
		)
	}
	if passwordHash == "" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le hash du mot de passe ne peut pas être vide",
			map[string]interface{}{"field": "password_hash"},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	return &User{
		id:           uuid.New(),
		email:        normalized,
		passwordHash: passwordHash,
		status:       UserStatusPendingVerification,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// RecordFailedLogin incrémente le compteur d'échecs (U-2).
// Retourne true si le seuil 5 est atteint → le caller doit déclencher Lock().
func (u *User) RecordFailedLogin() bool {
	u.failedLoginAttempts++
	u.updatedAt = time.Now().UTC()
	return u.failedLoginAttempts >= userFailedLoginThreshold
}

// IsLocked retourne true si le compte est verrouillé et que le TTL n'est pas expiré.
func (u *User) IsLocked() bool {
	return u.status == UserStatusLocked &&
		u.lockedUntil != nil &&
		time.Now().UTC().Before(*u.lockedUntil)
}

// Lock verrouille le compte pour une durée donnée (U-2 canonique : 30min).
// Transition SM-17b : active → locked.
func (u *User) Lock(duration time.Duration) error {
	if err := u.status.CanTransitionTo(UserStatusLocked); err != nil {
		return err
	}
	now := time.Now().UTC()
	lockUntil := now.Add(duration)
	u.status = UserStatusLocked
	u.lockedUntil = &lockUntil
	u.updatedAt = now
	return nil
}

// Unlock déverrouille le compte et remet le compteur à zéro.
// Transition SM-17b : locked → active.
func (u *User) Unlock() error {
	if err := u.status.CanTransitionTo(UserStatusActive); err != nil {
		return err
	}
	u.status = UserStatusActive
	u.failedLoginAttempts = 0
	u.lockedUntil = nil
	u.updatedAt = time.Now().UTC()
	return nil
}

// VerifyEmail confirme l'email et fait transiter pending_verification → active.
// Invariant U-4.
func (u *User) VerifyEmail() error {
	if err := u.status.CanTransitionTo(UserStatusActive); err != nil {
		return NewDomainError(
			ErrCodeEMAIL_NOT_VERIFIED,
			"Le compte ne peut pas être activé depuis l'état actuel",
			map[string]interface{}{
				"user_id":        u.id.String(),
				"current_status": string(u.status),
			},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	u.emailVerified = true
	u.emailVerifiedAt = &now
	u.status = UserStatusActive
	u.updatedAt = now
	return nil
}

// RequireEmailVerified vérifie U-4 : email vérifié obligatoire pour actions critiques.
func (u *User) RequireEmailVerified() error {
	if !u.emailVerified {
		return NewDomainError(
			ErrCodeEMAIL_NOT_VERIFIED,
			"L'email doit être vérifié avant d'effectuer cette action",
			map[string]interface{}{
				"user_id": u.id.String(),
				"email":   u.email,
			},
			SeverityMEDIUM,
			false,
		)
	}
	return nil
}

// ChangePassword remplace le hash. La politique U-3 doit être validée
// en amont via le Password VO avant d'appeler cette méthode.
func (u *User) ChangePassword(newHash string) error {
	if newHash == "" {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le hash du nouveau mot de passe ne peut pas être vide",
			map[string]interface{}{"field": "password_hash"},
			SeverityMEDIUM,
			false,
		)
	}
	u.passwordHash = newHash
	u.updatedAt = time.Now().UTC()
	return nil
}

// RecordSuccessfulLogin réinitialise le compteur d'échecs et enregistre la connexion.
func (u *User) RecordSuccessfulLogin(ipAddress string) {
	now := time.Now().UTC()
	u.failedLoginAttempts = 0
	u.lockedUntil = nil
	u.lastLoginAt = &now
	u.lastLoginIP = &ipAddress
	u.updatedAt = now
}

// Enable2FA active la 2FA pour l'utilisateur.
// IMPORTANT : les arguments doivent être pré-traités par le Use Case :
//   - encryptedSecret : secret TOTP chiffré AES-256-GCM via ICryptoService (invariant C-3)
//   - hashedBackupCodes : codes de récupération hashés via Argon2id (chaque code usage-unique)
// L'agrégat ne chiffre/hash pas lui-même — stocke des données déjà transformées.
func (u *User) Enable2FA(encryptedSecret string, hashedBackupCodes []string) error {
	if u.status != UserStatusActive {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"L'utilisateur doit être actif pour activer la 2FA",
			map[string]interface{}{
				"user_id":        u.id.String(),
				"current_status": string(u.status),
			},
			SeverityMEDIUM,
			false,
		)
	}
	if u.twoFactorEnabled {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"La 2FA est déjà activée pour cet utilisateur",
			map[string]interface{}{
				"user_id": u.id.String(),
				"field":   "two_fa_enabled",
			},
			SeverityLOW,
			false,
		)
	}
	if encryptedSecret == "" {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le secret TOTP chiffré est obligatoire",
			map[string]interface{}{"field": "encrypted_secret"},
			SeverityLOW,
			false,
		)
	}
	if len(hashedBackupCodes) == 0 {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Au moins un code de récupération est obligatoire",
			map[string]interface{}{
				"field": "hashed_backup_codes",
				"count": 0,
			},
			SeverityLOW,
			false,
		)
	}
	u.twoFactorEnabled = true
	u.totpSecret = &encryptedSecret
	u.backupCodes = hashedBackupCodes
	u.updatedAt = time.Now().UTC()
	return nil
}

// Disable2FA désactive l'authentification à deux facteurs et efface les secrets.
func (u *User) Disable2FA() {
	u.twoFactorEnabled = false
	u.totpSecret = nil
	u.backupCodes = nil
	u.updatedAt = time.Now().UTC()
}

// ResetPassword remplace le hash dans le flux de réinitialisation par token (UC-U-05).
// La vérification du token est faite par le use case avant d'appeler cette méthode.
// Le newHash doit être produit par Argon2id (DA-2) avant l'appel.
func (u *User) ResetPassword(newHash string) error {
	return u.ChangePassword(newHash)
}

// VerifyPassword vérifie un mot de passe en clair contre le hash Argon2id stocké (DA-2).
// Format du hash : $argon2id$v=19$m=<m>,t=<t>,p=<p>$<base64-salt>$<base64-hash>
// OWASP 2024 : time=1, memory=64*1024, threads=4, keyLen=32.
// Retourne (false, nil) si le mot de passe est incorrect (pas d'erreur — distinction volontaire).
// Retourne (false, DomainError) uniquement si le format du hash stocké est invalide.
func (u *User) VerifyPassword(plain string) (bool, error) {
	parts := strings.Split(u.passwordHash, "$")
	// Format : ["", "argon2id", "v=19", "m=65536,t=1,p=4", "<salt-b64>", "<hash-b64>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, NewDomainError(
			ErrCodeINVARIANT_VIOLATION,
			"Format de hash Argon2id invalide",
			map[string]interface{}{"field": "password_hash"},
			SeverityCRITICAL,
			false,
		)
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != 19 {
		return false, NewDomainError(
			ErrCodeINVARIANT_VIOLATION,
			"Version Argon2id non supportée",
			map[string]interface{}{"version": parts[2]},
			SeverityCRITICAL,
			false,
		)
	}
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, NewDomainError(
			ErrCodeINVARIANT_VIOLATION,
			"Paramètres Argon2id invalides",
			map[string]interface{}{"params": parts[3]},
			SeverityCRITICAL,
			false,
		)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, NewDomainError(
			ErrCodeINVARIANT_VIOLATION,
			"Salt Argon2id invalide (base64 malformé)",
			nil,
			SeverityCRITICAL,
			false,
		)
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, NewDomainError(
			ErrCodeINVARIANT_VIOLATION,
			"Hash Argon2id invalide (base64 malformé)",
			nil,
			SeverityCRITICAL,
			false,
		)
	}
	keyLen := uint32(len(storedHash))
	computedHash := argon2.IDKey([]byte(plain), salt, timeCost, memory, threads, keyLen)
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1, nil
}

// ---------------------------------------------------------------------------
// 4. UserSession — entité liée à User
// ---------------------------------------------------------------------------

// UserSession représente une session JWT active. Champs publics (lecture directe par repo).
type UserSession struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	JTI           string        // JWT ID — unique token identifier for RevokeByJTI (audit2)
	Status        SessionStatus
	ExpiresAt     time.Time
	RevokedReason string
	CreatedAt     time.Time
}

// NewUserSession crée une session. Status initial déterminé par le caller (active ou pending_2fa).
func NewUserSession(userID uuid.UUID, status SessionStatus, expiresAt time.Time) (*UserSession, error) {
	if status != SessionStatusActive && status != SessionStatusPending2FA {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le statut initial d'une session doit être 'active' ou 'pending_2fa'",
			map[string]interface{}{"status": string(status)},
			SeverityLOW,
			false,
		)
	}
	if expiresAt.Before(time.Now().UTC()) {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"La date d'expiration de la session doit être dans le futur",
			map[string]interface{}{"expires_at": expiresAt},
			SeverityLOW,
			false,
		)
	}
	return &UserSession{
		ID:        uuid.New(),
		UserID:    userID,
		Status:    status,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// CanTransitionTo vérifie SM-17 exhaustivement.
// pending_2fa → active (SC-C-006 Validate2FACode)
// pending_2fa → revoked (timeout 5min)
// active → expired (CRON JWT TTL)
// active → revoked (logout / suspension / pwd_changed)
func (s *UserSession) CanTransitionTo(target SessionStatus) error {
	allowed := sessionStatusTransitions[s.Status]
	for _, t := range allowed {
		if t == target {
			return nil
		}
	}
	return NewDomainError(
		ErrCodeINVALID_STATUS_TRANSITION,
		"Transition de statut session interdite",
		map[string]interface{}{
			"session_id":     s.ID.String(),
			"current_status": string(s.Status),
			"target_status":  string(target),
		},
		SeverityMEDIUM,
		false,
	)
}

// IsValid retourne true si la session est active et non expirée.
func (s *UserSession) IsValid() bool {
	return s.Status == SessionStatusActive && time.Now().UTC().Before(s.ExpiresAt)
}
