package security

import "github.com/google/uuid"

// Stubs pour les use cases 2FA, password-reset et admin security — Phase D.

// UnlockUserAccountCommand — SC-C-008.
type UnlockUserAccountCommand struct {
	UserID     uuid.UUID `json:"user_id"`
	UnlockMode string    `json:"unlock_mode"`
}

// RotateEncryptionKeysCommand — SC-C-014.
type RotateEncryptionKeysCommand struct {
	InitiatedByMemberID uuid.UUID `json:"initiated_by_member_id"`
	Notes               string    `json:"notes,omitempty"`
}

// RotateEncryptionKeysResult — SC-C-014.
type RotateEncryptionKeysResult struct {
	RotationID       string `json:"rotation_id"`
	ChannelsTotal    int    `json:"channels_total"`
	ChannelsRotated  int    `json:"channels_rotated"`
	ChannelsFailed   int    `json:"channels_failed"`
}

// Stubs pour les use cases 2FA et password-reset — implémentations complètes en Phase D.

// Enable2FACommand — SC-C-004.
type Enable2FACommand struct {
	UserID uuid.UUID `json:"user_id"`
}

// Enable2FAResult — retourne secret TOTP + backup codes (une seule fois).
type Enable2FAResult struct {
	TOTPSecret  string   `json:"totp_secret"`
	BackupCodes []string `json:"backup_codes"`
}

// Disable2FACommand — SC-C-005.
type Disable2FACommand struct {
	UserID   uuid.UUID `json:"user_id"`
	Password string    `json:"password" validate:"required"`
}

// SendPasswordResetCommand — SC-C-011.
type SendPasswordResetCommand struct {
	Email  string `json:"email"   validate:"required,email"`
	IPHash string `json:"ip_hash"`
}

// ValidatePasswordResetCommand — SC-C-012.
type ValidatePasswordResetCommand struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}
