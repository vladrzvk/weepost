package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 1. ChannelStatus — 7 valeurs canoniques (token_expired unifié — A001 résolu)
// ---------------------------------------------------------------------------

// ChannelStatus représente l'état de connexion d'un channel social.
// Valeur canonique "token_expired" uniformisée — SM-07 + Phase 1 §A.8 (A001 résolu).
type ChannelStatus string

const (
	ChannelStatusActive        ChannelStatus = "active"
	ChannelStatusTokenExpiring ChannelStatus = "token_expiring"
	ChannelStatusTokenExpired  ChannelStatus = "token_expired"
	ChannelStatusDisconnected  ChannelStatus = "disconnected"
	ChannelStatusError         ChannelStatus = "error"
	ChannelStatusRevoked       ChannelStatus = "revoked"
	ChannelStatusPendingReview ChannelStatus = "pending_review"
)

// CanPublish retourne true si ce statut autorise la publication (SM-07 D-01).
func (s ChannelStatus) CanPublish() bool {
	return s == ChannelStatusActive || s == ChannelStatusTokenExpiring
}

func (s ChannelStatus) IsValid() bool {
	switch s {
	case ChannelStatusActive, ChannelStatusTokenExpiring, ChannelStatusTokenExpired,
		ChannelStatusDisconnected, ChannelStatusError, ChannelStatusRevoked,
		ChannelStatusPendingReview:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 2. ChannelType — DDL chk_channels_type (Phase 1 P3d §A.8 — CamelCase canonique A013)
// ---------------------------------------------------------------------------

type ChannelType string

const (
	ChannelTypeFacebookPage      ChannelType = "facebook_page"
	ChannelTypeInstagramBusiness ChannelType = "instagram_business"
	ChannelTypeLinkedInPage      ChannelType = "linkedin_page"
	ChannelTypeTwitterAccount    ChannelType = "twitter_account" // reserved — not yet implemented
)

func (t ChannelType) IsValid() bool {
	switch t {
	case ChannelTypeFacebookPage, ChannelTypeInstagramBusiness,
		ChannelTypeLinkedInPage, ChannelTypeTwitterAccount:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 3. ChannelHealth — Value Object JSONB
// ---------------------------------------------------------------------------

// ChannelHealth représente la santé temps-réel d'un channel OAuth.
// Stocké en JSONB dans brand_channels.health_status.
type ChannelHealth struct {
	IsHealthy           bool       `json:"is_healthy"`
	TokenValid          bool       `json:"token_valid"`
	TokenExpiresAt      *time.Time `json:"token_expires_at"`
	LastError           *string    `json:"last_error"`
	LastErrorAt         *time.Time `json:"last_error_at"`
	LastErrorCode       *string    `json:"last_error_code"`
	PermissionsValid    bool       `json:"permissions_valid"`
	RateLimitStatus     string     `json:"rate_limit_status"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

func defaultChannelHealth() *ChannelHealth {
	return &ChannelHealth{
		IsHealthy:           true,
		TokenValid:          true,
		PermissionsValid:    true,
		RateLimitStatus:     "ok",
		ConsecutiveFailures: 0,
	}
}

// ---------------------------------------------------------------------------
// 3. Channel — aggregate root (entité brand_channels)
// ---------------------------------------------------------------------------

// Seuil canonique D-01 : 3 échecs consécutifs → status = error.
const ChannelErrorThreshold = 3

// Channel est l'aggregate root du bounded context Channel Management.
type Channel struct {
	id                      uuid.UUID
	workspaceID             uuid.UUID
	brandID                 uuid.UUID
	channelType             ChannelType
	platformAccountID       string
	platformAccountName     string
	platformAccountUsername *string
	status                  ChannelStatus
	healthStatus            *ChannelHealth
	connectedAt             time.Time
	lastHealthCheckAt       *time.Time
	lastPublishedAt         *time.Time
	disconnectedAt          *time.Time
	createdAt               time.Time
	updatedAt               time.Time
	deletedAt               *time.Time
}

// Getters

func (ch *Channel) ID() uuid.UUID                      { return ch.id }
func (ch *Channel) WorkspaceID() uuid.UUID             { return ch.workspaceID }
func (ch *Channel) BrandID() uuid.UUID                 { return ch.brandID }
func (ch *Channel) ChannelType() ChannelType           { return ch.channelType }
func (ch *Channel) PlatformAccountID() string          { return ch.platformAccountID }
func (ch *Channel) PlatformAccountName() string        { return ch.platformAccountName }
func (ch *Channel) PlatformAccountUsername() *string   { return ch.platformAccountUsername }
func (ch *Channel) Status() ChannelStatus              { return ch.status }
func (ch *Channel) HealthStatus() *ChannelHealth       { return ch.healthStatus }
func (ch *Channel) ConnectedAt() time.Time             { return ch.connectedAt }
func (ch *Channel) LastHealthCheckAt() *time.Time      { return ch.lastHealthCheckAt }
func (ch *Channel) LastPublishedAt() *time.Time        { return ch.lastPublishedAt }
func (ch *Channel) DisconnectedAt() *time.Time         { return ch.disconnectedAt }
func (ch *Channel) CreatedAt() time.Time               { return ch.createdAt }
func (ch *Channel) UpdatedAt() time.Time               { return ch.updatedAt }
func (ch *Channel) DeletedAt() *time.Time              { return ch.deletedAt }

func (ch *Channel) ConsecutiveFailures() int {
	if ch.healthStatus == nil {
		return 0
	}
	return ch.healthStatus.ConsecutiveFailures
}

func (ch *Channel) LastFailureAt() *time.Time {
	if ch.healthStatus == nil {
		return nil
	}
	return ch.healthStatus.LastErrorAt
}

func (ch *Channel) TokenExpiresAt() *time.Time {
	if ch.healthStatus == nil {
		return nil
	}
	return ch.healthStatus.TokenExpiresAt
}

// NewChannel crée un channel. Status initial : pending_review (validation de conformité).
func NewChannel(brandID uuid.UUID, channelType string) (*Channel, error) {
	ct := ChannelType(channelType)
	if !ct.IsValid() {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le type de channel n'est pas valide",
			map[string]interface{}{"channel_type": channelType},
			SeverityLOW,
			false,
		)
	}
	if brandID == uuid.Nil {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le brandID ne peut pas être nul",
			map[string]interface{}{"field": "brand_id"},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	return &Channel{
		id:           uuid.New(),
		brandID:      brandID,
		channelType:  ct,
		status:       ChannelStatusPendingReview,
		healthStatus: defaultChannelHealth(),
		connectedAt:  now,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// NewConnectedChannel crée un channel déjà connecté avec les données de la plateforme OAuth.
// Utilisé par ConnectChannelUseCase (A029 workaround : NewChannel ne prend pas platformAccountID).
func NewConnectedChannel(brandID, workspaceID uuid.UUID, channelType, platformAccountID, platformAccountName string) (*Channel, error) {
	ct := ChannelType(channelType)
	if !ct.IsValid() {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le type de channel n'est pas valide",
			map[string]interface{}{"channel_type": channelType},
			SeverityLOW,
			false,
		)
	}
	if brandID == uuid.Nil {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le brandID ne peut pas être nul",
			map[string]interface{}{"field": "brand_id"},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	return &Channel{
		id:                  uuid.New(),
		workspaceID:         workspaceID,
		brandID:             brandID,
		channelType:         ct,
		platformAccountID:   platformAccountID,
		platformAccountName: platformAccountName,
		status:              ChannelStatusPendingReview,
		healthStatus:        defaultChannelHealth(),
		connectedAt:         now,
		createdAt:           now,
		updatedAt:           now,
	}, nil
}

// CanPublish vérifie les invariants C-1b, C-1c, C-1, C-2 dans l'ordre canonique (Phase 2).
// Retourne nil si la publication est possible.
func (ch *Channel) CanPublish() error {
	// C-1b — Révocation côté plateforme externe (Facebook, Instagram…)
	if ch.status == ChannelStatusRevoked {
		return NewDomainError(
			ErrCodeCHANNEL_REVOKED,
			"L'accès à ce channel a été révoqué depuis la plateforme externe — reconnectez le compte",
			map[string]interface{}{
				"channel_id":   ch.id.String(),
				"channel_type": string(ch.channelType),
			},
			SeverityHIGH,
			false,
		)
	}
	// C-1c — Channel en état error (ConsecutiveFailures ≥ 3 — C-4)
	if ch.status == ChannelStatusError {
		failures := 0
		if ch.healthStatus != nil {
			failures = ch.healthStatus.ConsecutiveFailures
		}
		return NewDomainError(
			ErrCodeCHANNEL_ERROR,
			"Le channel est en état d'erreur suite à des échecs répétés — vérifiez la connexion",
			map[string]interface{}{
				"channel_id":           ch.id.String(),
				"channel_type":         string(ch.channelType),
				"consecutive_failures": failures,
			},
			SeverityHIGH,
			false,
		)
	}
	// C-1 — Déconnexion volontaire WeePost
	if ch.status == ChannelStatusDisconnected {
		return NewDomainError(
			ErrCodeCHANNEL_DISCONNECTED,
			"Le channel est déconnecté — reconnectez-le avant de publier",
			map[string]interface{}{
				"channel_id":      ch.id.String(),
				"channel_type":    string(ch.channelType),
				"disconnected_at": ch.disconnectedAt,
			},
			SeverityHIGH,
			false,
		)
	}
	// C-2 — Token OAuth invalide ou expiré
	if ch.healthStatus != nil && !ch.healthStatus.TokenValid {
		return NewDomainError(
			ErrCodeTOKEN_EXPIRED,
			"Le token OAuth a expiré — re-authentification requise",
			map[string]interface{}{
				"channel_id": ch.id.String(),
				"expires_at": ch.healthStatus.TokenExpiresAt,
			},
			SeverityHIGH,
			false,
		)
	}
	// Token OAuth expiré côté plateforme — re-authentification requise
	if ch.status == ChannelStatusTokenExpired {
		return NewDomainError(
			ErrCodeTOKEN_EXPIRED,
			"Le token OAuth est expiré — re-authentification requise",
			map[string]interface{}{
				"channel_id": ch.id.String(),
			},
			SeverityHIGH,
			false,
		)
	}
	// pending_review → accès non encore accordé
	if ch.status == ChannelStatusPendingReview {
		return NewDomainError(
			ErrCodeCHANNEL_ACCESS_DENIED,
			"Le channel est en attente de vérification de conformité",
			map[string]interface{}{
				"channel_id": ch.id.String(),
			},
			SeverityMEDIUM,
			false,
		)
	}
	return nil
}

// RecordPublicationFailure incrémente ConsecutiveFailures (C-4).
// Retourne true si le seuil 3 est atteint → le caller doit passer le status à error.
func (ch *Channel) RecordPublicationFailure() bool {
	if ch.healthStatus == nil {
		ch.healthStatus = defaultChannelHealth()
	}
	ch.healthStatus.ConsecutiveFailures++
	ch.healthStatus.IsHealthy = false
	ch.updatedAt = time.Now().UTC()
	return ch.healthStatus.ConsecutiveFailures >= ChannelErrorThreshold
}

// SetStatus met à jour le statut du channel (usage interne + use case layer).
func (ch *Channel) SetStatus(status ChannelStatus) {
	ch.status = status
	ch.updatedAt = time.Now().UTC()
	if status == ChannelStatusDisconnected {
		now := time.Now().UTC()
		ch.disconnectedAt = &now
	}
}

// ResetFailureCount remet le compteur à 0 après une publication réussie (D-01).
func (ch *Channel) ResetFailureCount() {
	if ch.healthStatus == nil {
		return
	}
	ch.healthStatus.ConsecutiveFailures = 0
	ch.healthStatus.IsHealthy = true
	now := time.Now().UTC()
	ch.lastPublishedAt = &now
	ch.updatedAt = now
}

// UpdateHealthCheck met à jour les données de santé après un check automatique.
func (ch *Channel) UpdateHealthCheck(health *ChannelHealth) {
	if health == nil {
		return
	}
	ch.healthStatus = health
	now := time.Now().UTC()
	ch.lastHealthCheckAt = &now
	ch.updatedAt = now
}
