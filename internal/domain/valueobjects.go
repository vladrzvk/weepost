package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// ---------------------------------------------------------------------------
// 1. ResourceType — Phase 0 §6.1
// ---------------------------------------------------------------------------

type ResourceType string

const (
	ResourceTypeWorkspace ResourceType = "workspace"
	ResourceTypeBrand     ResourceType = "brand"
	ResourceTypeChannel   ResourceType = "channel"
	ResourceTypePost      ResourceType = "post"
	ResourceTypeMember    ResourceType = "member"
	ResourceTypeMedia     ResourceType = "media"
	ResourceTypeComment   ResourceType = "comment"
)

func (r ResourceType) IsValid() bool {
	switch r {
	case ResourceTypeWorkspace, ResourceTypeBrand, ResourceTypeChannel,
		ResourceTypePost, ResourceTypeMember, ResourceTypeMedia, ResourceTypeComment:
		return true
	}
	return false
}

// Action — Phase 6 canonical permission string (ex: "workspace.update_settings", "brand.update")
type Action string

// Resource — Phase 6 §2 permission target
type Resource struct {
	Type        ResourceType
	WorkspaceID uuid.UUID
	BrandID     uuid.UUID
	ResourceID  uuid.UUID
}

// ---------------------------------------------------------------------------
// 2. WorkspaceSettings — Phase 1 P3d §B.1
// ---------------------------------------------------------------------------

// WorkspaceSettings stocké en JSONB dans workspaces.settings
type WorkspaceSettings struct {
	Timezone             string `json:"timezone"`
	Language             string `json:"language"`
	DateFormat           string `json:"date_format"`
	TimeFormat           string `json:"time_format"`
	WeekStartDay         int    `json:"week_start_day"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
}

// NewWorkspaceSettings construit des settings validés. Exactement 5 paramètres (NotificationsEnabled défaut: true).
func NewWorkspaceSettings(timezone, language, dateFormat, timeFormat string, weekStartDay int) (*WorkspaceSettings, error) {
	if !isValidIANATimezone(timezone) {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le fuseau horaire fourni n'est pas valide",
			map[string]interface{}{"field": "timezone", "timezone": timezone},
			SeverityLOW,
			false,
		)
	}
	if !isValidISO639Language(language) {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le code de langue fourni n'est pas valide",
			map[string]interface{}{"field": "language", "language": language},
			SeverityLOW,
			false,
		)
	}
	validDateFormats := map[string]bool{
		"DD/MM/YYYY": true,
		"MM/DD/YYYY": true,
		"YYYY-MM-DD": true,
	}
	if !validDateFormats[dateFormat] {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"date_format doit être 'DD/MM/YYYY', 'MM/DD/YYYY' ou 'YYYY-MM-DD'",
			map[string]interface{}{"field": "date_format", "date_format": dateFormat},
			SeverityLOW,
			false,
		)
	}
	if timeFormat != "24h" && timeFormat != "12h" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"time_format doit être '24h' ou '12h'",
			map[string]interface{}{"field": "time_format", "time_format": timeFormat},
			SeverityLOW,
			false,
		)
	}
	if weekStartDay < 0 || weekStartDay > 6 {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"week_start_day doit être compris entre 0 (dimanche) et 6 (samedi)",
			map[string]interface{}{"field": "week_start_day", "week_start_day": weekStartDay},
			SeverityLOW,
			false,
		)
	}
	return &WorkspaceSettings{
		Timezone:             timezone,
		Language:             language,
		DateFormat:           dateFormat,
		TimeFormat:           timeFormat,
		WeekStartDay:         weekStartDay,
		NotificationsEnabled: true,
	}, nil
}

// DefaultWorkspaceSettings retourne les settings par défaut (timezone Paris, langue fr).
func DefaultWorkspaceSettings() *WorkspaceSettings {
	return &WorkspaceSettings{
		Timezone:             "Europe/Paris",
		Language:             "fr",
		DateFormat:           "DD/MM/YYYY",
		TimeFormat:           "24h",
		WeekStartDay:         1,
		NotificationsEnabled: true,
	}
}

func (s *WorkspaceSettings) GetTimezone() string   { return s.Timezone }
func (s *WorkspaceSettings) GetLanguage() string   { return s.Language }
func (s *WorkspaceSettings) GetDateFormat() string { return s.DateFormat }
func (s *WorkspaceSettings) GetTimeFormat() string { return s.TimeFormat }
func (s *WorkspaceSettings) GetWeekStartDay() int  { return s.WeekStartDay }

// isValidIANATimezone vérifie via time.LoadLocation (source IANA authoritative).
func isValidIANATimezone(tz string) bool {
	if tz == "" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// isValidISO639Language valide les codes ISO 639-1 supportés en V0.
func isValidISO639Language(lang string) bool {
	supported := map[string]bool{
		"fr": true,
		"en": true,
		"es": true,
		"de": true,
		"it": true,
		"pt": true,
	}
	return supported[strings.ToLower(lang)]
}

// ---------------------------------------------------------------------------
// 3. Password — Phase 2 §U-3 (ISO 27001 A.9.4.3)
// ---------------------------------------------------------------------------

// argon2idParams définit les paramètres de hachage Argon2id
// conformément aux recommandations OWASP 2024.
type argon2idParams struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultArgon2idParams = &argon2idParams{
	memory:      64 * 1024, // 64 MiB
	iterations:  1,
	parallelism: 4,
	saltLength:  16,
	keyLength:   32,
}

// Password encapsule un hash Argon2id encodé. Jamais le mot de passe en clair.
type Password struct {
	hash string
}

// NewPassword hache le mot de passe brut selon la politique U-3 (DA-2 — Argon2id OWASP 2024).
// Règles : min 12 chars, 1 majuscule, 1 minuscule, 1 chiffre, 1 caractère spécial.
func NewPassword(raw string) (*Password, error) {
	if err := validatePasswordPolicy(raw); err != nil {
		return nil, err
	}
	salt := make([]byte, defaultArgon2idParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, NewDomainError(
			ErrCodeINTERNAL_SERVER_ERROR,
			"Erreur lors de la génération du salt",
			map[string]interface{}{"error": err.Error()},
			SeverityCRITICAL,
			false,
		)
	}
	hash := argon2.IDKey(
		[]byte(raw),
		salt,
		defaultArgon2idParams.iterations,
		defaultArgon2idParams.memory,
		defaultArgon2idParams.parallelism,
		defaultArgon2idParams.keyLength,
	)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		defaultArgon2idParams.memory,
		defaultArgon2idParams.iterations,
		defaultArgon2idParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return &Password{hash: encoded}, nil
}

// NewPasswordFromHash reconstruit un Password depuis un hash Argon2id stocké (lecture DB).
func NewPasswordFromHash(hash string) (*Password, error) {
	if hash == "" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le hash du mot de passe ne peut pas être vide",
			nil,
			SeverityMEDIUM,
			false,
		)
	}
	return &Password{hash: hash}, nil
}

// Hash retourne le hash Argon2id encodé (pour persistance).
func (p *Password) Hash() string { return p.hash }

// Verify retourne true si le mot de passe en clair correspond au hash.
// Décode le format $argon2id$..., re-calcule avec le même salt,
// compare en temps constant pour éviter les timing attacks.
func (p *Password) Verify(raw string) bool {
	params, salt, expectedHash, err := decodeArgon2idHash(p.hash)
	if err != nil {
		return false
	}
	computedHash := argon2.IDKey(
		[]byte(raw),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	)
	return subtle.ConstantTimeCompare(expectedHash, computedHash) == 1
}

// decodeArgon2idHash parse le format $argon2id$v=...$m=...,t=...,p=...$salt$hash
func decodeArgon2idHash(encoded string) (*argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, nil, fmt.Errorf("format de hash invalide")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, fmt.Errorf("version invalide: %w", err)
	}
	if version != argon2.Version {
		return nil, nil, nil, fmt.Errorf("version Argon2 incompatible: %d", version)
	}
	params := &argon2idParams{}
	if _, err := fmt.Sscanf(
		parts[3], "m=%d,t=%d,p=%d",
		&params.memory, &params.iterations, &params.parallelism,
	); err != nil {
		return nil, nil, nil, fmt.Errorf("paramètres invalides: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt invalide: %w", err)
	}
	params.saltLength = uint32(len(salt))
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hash invalide: %w", err)
	}
	params.keyLength = uint32(len(hash))
	return params, salt, hash, nil
}

// validatePasswordPolicy applique la politique U-3 complète.
func validatePasswordPolicy(raw string) error {
	if len([]rune(raw)) < 12 {
		return NewDomainError(
			ErrCodePASSWORD_TOO_WEAK,
			"Le mot de passe doit contenir au moins 12 caractères",
			map[string]interface{}{"field": "password", "min_length": 12},
			SeverityMEDIUM,
			false,
		)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range raw {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	missing := []string{}
	if !hasUpper {
		missing = append(missing, "une lettre majuscule")
	}
	if !hasLower {
		missing = append(missing, "une lettre minuscule")
	}
	if !hasDigit {
		missing = append(missing, "un chiffre")
	}
	if !hasSpecial {
		missing = append(missing, "un caractère spécial")
	}
	if len(missing) > 0 {
		return NewDomainError(
			ErrCodePASSWORD_TOO_WEAK,
			fmt.Sprintf("Le mot de passe doit contenir : %s", strings.Join(missing, ", ")),
			map[string]interface{}{
				"field":            "password",
				"missing_criteria": missing,
			},
			SeverityMEDIUM,
			false,
		)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 4. EmailAddress — RFC 5321
// ---------------------------------------------------------------------------

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// EmailAddress est un value object représentant une adresse email validée.
type EmailAddress struct {
	value string
}

// NewEmailAddress valide et crée une EmailAddress.
func NewEmailAddress(raw string) (*EmailAddress, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'adresse email ne peut pas être vide",
			map[string]interface{}{"field": "email"},
			SeverityLOW,
			false,
		)
	}
	if len(normalized) > 254 {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'adresse email dépasse la longueur maximale RFC 5321 (254 caractères)",
			map[string]interface{}{"field": "email", "max_length": 254},
			SeverityLOW,
			false,
		)
	}
	if !emailRegex.MatchString(normalized) {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'adresse email n'est pas valide",
			map[string]interface{}{"field": "email", "value": normalized},
			SeverityLOW,
			false,
		)
	}
	return &EmailAddress{value: normalized}, nil
}

func (e *EmailAddress) String() string { return e.value }
func (e *EmailAddress) Value() string  { return e.value }
func (e *EmailAddress) Domain() string {
	parts := strings.Split(e.value, "@")
	return parts[1]
}

// ---------------------------------------------------------------------------
// 5. IPHash — RGPD SHA-256 (Phase 0 §RGPD)
// ---------------------------------------------------------------------------

// IPHash est un hash SHA-256 d'une adresse IP (anonymisation RGPD).
type IPHash struct {
	value string
}

// NewIPHash anonymise une adresse IP via SHA-256.
func NewIPHash(rawIP string) (*IPHash, error) {
	if rawIP == "" {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'adresse IP ne peut pas être vide",
			map[string]interface{}{"field": "ip"},
			SeverityLOW,
			false,
		)
	}
	sum := sha256.Sum256([]byte(rawIP))
	return &IPHash{value: fmt.Sprintf("%x", sum)}, nil
}

// NewIPHashFromStored reconstruit un IPHash depuis un hash stocké.
func NewIPHashFromStored(hash string) (*IPHash, error) {
	if len(hash) != 64 {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le hash IP doit être un hash SHA-256 hexadécimal (64 caractères)",
			map[string]interface{}{"field": "ip_hash", "length": len(hash)},
			SeverityLOW,
			false,
		)
	}
	return &IPHash{value: hash}, nil
}

func (h *IPHash) String() string { return h.value }
func (h *IPHash) Value() string  { return h.value }
