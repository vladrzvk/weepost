package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Utility types
// ============================================================

type PageRequest struct {
	Page     int
	PageSize int
}

type PageResult struct {
	Total    int64
	Page     int
	PageSize int
	HasNext  bool
	HasPrev  bool
}

type PostFilter struct {
	Status          []PostStatus
	BrandID         *uuid.UUID
	AuthorID        *uuid.UUID
	ScheduledBefore *time.Time
}

type AuditFilter struct {
	Action       string
	ActorID      *uuid.UUID
	ResourceType *ResourceType
	From         *time.Time
	To           *time.Time
}

// ============================================================
// Supporting domain structs (not persisted as aggregates)
// ============================================================

// InvitationStatus — SM-06 InvitationStatus (DDL chk_invitations_status)
type InvitationStatus string

const (
	InvitationStatusPending   InvitationStatus = "pending"
	InvitationStatusAccepted  InvitationStatus = "accepted"
	InvitationStatusExpired   InvitationStatus = "expired"
	InvitationStatusCancelled InvitationStatus = "cancelled"
)

// WorkspaceInvitation — pending invite token sent by email (Phase 2 P2a §UC-W-06)
type WorkspaceInvitation struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	InviterMemberID uuid.UUID  // FK workspace_members(id)
	Email           string
	Role            MemberRole
	Status          InvitationStatus
	Token           string
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	CancelledAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ApprovalRequest — internal or guest approval request for a post (Phase 5 §D-03, SM-10)
type ApprovalRequest struct {
	ID                  uuid.UUID
	PostID              uuid.UUID
	Type                string     // "internal" | "external"
	Status              string     // "pending" | "approved" | "rejected" | "cancelled"
	RequestedByMemberID uuid.UUID  // FK workspace_members(id)
	ApproverMemberID    *uuid.UUID // FK workspace_members(id) — nil si external
	ApproverGuestID     *uuid.UUID // FK brand_guests(id) — nil si internal
	ReviewedAt          *time.Time
	CancelledAt         *time.Time
	RejectionReason     *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// CanReceiveDecision retourne true si la demande est encore en attente de décision.
func (a *ApprovalRequest) CanReceiveDecision() bool {
	return a.Status == "pending"
}

// KeyRotation — encryption key rotation record (Phase 9 §A9-8, Phase 2 P4c §UC-S-04)
type KeyRotation struct {
	ID                  uuid.UUID
	KeyVersion          int
	InitiatedByMemberID uuid.UUID
	ChannelsTotal       int
	ChannelsRotated     int
	ChannelsFailed      int
	Status              string // pending | in_progress | completed | failed
	Notes               *string
	StartedAt           time.Time
	CompletedAt         *time.Time
}

// PasswordResetToken — one-time token for password reset flow (Phase 2 P2b §UC-U-05)
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// AuditEntry — immutable event log entry for compliance (Phase 1 P3c §audit_logs)
type AuditEntry struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	ActorID      uuid.UUID
	ActorType    string
	Action       string
	ResourceType ResourceType
	ResourceID   uuid.UUID
	Metadata     map[string]interface{}
	IPHash       string
	CreatedAt    time.Time
}

// ============================================================
// Repository interfaces
// ============================================================

// IWorkspaceRepo — aggregate root Workspace (Phase 2 P2a)
type IWorkspaceRepo interface {
	Create(ctx context.Context, w *Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*Workspace, error)
	Update(ctx context.Context, w *Workspace) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Workspace, error)
	GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error)
	AddMember(ctx context.Context, m *WorkspaceMember) error
	UpdateMember(ctx context.Context, m *WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID, memberID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*WorkspaceMember, error)
}

// IBrandRepo — aggregate root Brand (Phase 2 P2c)
type IBrandRepo interface {
	Create(ctx context.Context, b *Brand) error
	GetByID(ctx context.Context, id uuid.UUID) (*Brand, error)
	Update(ctx context.Context, b *Brand) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*Brand, error)
	ExistsBySlugInWorkspace(ctx context.Context, slug string, workspaceID uuid.UUID) (bool, error) // audit2
	AddAssignment(ctx context.Context, a *BrandAssignment) error
	UpdateAssignment(ctx context.Context, a *BrandAssignment) error
	RemoveAssignment(ctx context.Context, brandID, memberID uuid.UUID) error
	GetAssignment(ctx context.Context, brandID, memberID uuid.UUID) (*BrandAssignment, error)
	ListAssignments(ctx context.Context, brandID uuid.UUID) ([]*BrandAssignment, error)
}

// IUserRepo — aggregate root User (Phase 2 P2b)
type IUserRepo interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// IChannelRepo — aggregate root Channel (Phase 2 P3a)
type IChannelRepo interface {
	Create(ctx context.Context, c *Channel) error
	GetByID(ctx context.Context, id uuid.UUID) (*Channel, error)
	Update(ctx context.Context, c *Channel) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*Channel, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*Channel, error)
	GetByPlatformAccountID(ctx context.Context, platformAccountID string) (*Channel, error)
}

// IPostRepo — aggregate root Post (Phase 2 P3b)
type IPostRepo interface {
	Create(ctx context.Context, p *Post) error
	GetByID(ctx context.Context, id uuid.UUID) (*Post, error)
	Update(ctx context.Context, p *Post) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByBrand(ctx context.Context, brandID uuid.UUID, filter PostFilter, page PageRequest) ([]*Post, PageResult, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, filter PostFilter, page PageRequest) ([]*Post, PageResult, error)
	ListScheduledBefore(ctx context.Context, before time.Time) ([]*Post, error)
	ListFailed(ctx context.Context) ([]*Post, error)
}

// IMediaAssetRepo — aggregate root MediaAsset (Phase 2 P3c)
type IMediaAssetRepo interface {
	Create(ctx context.Context, a *MediaAsset) error
	GetByID(ctx context.Context, id uuid.UUID) (*MediaAsset, error)
	Update(ctx context.Context, a *MediaAsset) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*MediaAsset, error)
	ListByPost(ctx context.Context, postID uuid.UUID) ([]*MediaAsset, error)
	ListPendingScan(ctx context.Context) ([]*MediaAsset, error)
}

// ISessionRepo — UserSession (Phase 2 P2b §UC-U-01, SM-17)
type ISessionRepo interface {
	Create(ctx context.Context, s *UserSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*UserSession, error)
	GetByJTI(ctx context.Context, jti string) (*UserSession, error)         // audit2 — SC-C-006
	Update(ctx context.Context, s *UserSession) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
	RevokeByJTI(ctx context.Context, jti string) error                      // audit2 — SC-C-006
	DeleteExpired(ctx context.Context) error
}

// IApprovalRequestRepo — ApprovalRequest (Phase 2 P4b)
type IApprovalRequestRepo interface {
	Create(ctx context.Context, a *ApprovalRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error)
	GetByGuestToken(ctx context.Context, token string) (*ApprovalRequest, error)
	Update(ctx context.Context, a *ApprovalRequest) error
	ListByPost(ctx context.Context, postID uuid.UUID) ([]*ApprovalRequest, error)
	ListPendingByBrand(ctx context.Context, brandID uuid.UUID) ([]*ApprovalRequest, error)
}

// IKeyRotationRepo — KeyRotation (Phase 2 P4c §UC-S-04, Phase 9 §A9-8)
type IKeyRotationRepo interface {
	Create(ctx context.Context, kr *KeyRotation) error
	GetByID(ctx context.Context, id uuid.UUID) (*KeyRotation, error)
	GetLatest(ctx context.Context) (*KeyRotation, error)
	List(ctx context.Context) ([]*KeyRotation, error)
	Update(ctx context.Context, kr *KeyRotation) error
}

// IPasswordResetTokenRepo — PasswordResetToken (Phase 2 P2b §UC-U-05)
type IPasswordResetTokenRepo interface {
	Create(ctx context.Context, t *PasswordResetToken) error
	GetByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	Invalidate(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// IAuditRepo — AuditEntry write-once (Phase 1 P3c §audit_logs)
type IAuditRepo interface {
	Create(ctx context.Context, e *AuditEntry) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, filter AuditFilter, page PageRequest) ([]*AuditEntry, PageResult, error)
	ListByUser(ctx context.Context, userID uuid.UUID, filter AuditFilter, page PageRequest) ([]*AuditEntry, PageResult, error)
}

// IInvitationRepo — WorkspaceInvitation (Phase 2 P2a §UC-W-06)
type IInvitationRepo interface {
	Create(ctx context.Context, inv *WorkspaceInvitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*WorkspaceInvitation, error)
	GetByToken(ctx context.Context, token string) (*WorkspaceInvitation, error)
	Update(ctx context.Context, inv *WorkspaceInvitation) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*WorkspaceInvitation, error)
	DeleteExpired(ctx context.Context) error
}

// ============================================================
// New interfaces — audit2 corrections
// ============================================================

// ICryptoService fournit les opérations de chiffrement symétrique
// pour les données sensibles du domaine.
//
// Utilisations :
//   - T13 ConnectChannel : chiffrement tokens OAuth
//   - T23 Enable2FA : chiffrement secret TOTP (invariant C-3)
//   - T24 RotateEncryptionKeys : rotation des clés AES
//
// Note : la gestion du keyID et de la rotation des clés est une
// responsabilité de l'implémentation infrastructure, transparente
// pour le domaine.
type ICryptoService interface {
	// EncryptToken chiffre une valeur sensible (token OAuth, secret TOTP).
	// Retourne la valeur chiffrée encodée (format opaque à l'appelant).
	EncryptToken(ctx context.Context, plaintext string) (string, error)

	// DecryptToken déchiffre une valeur précédemment chiffrée par EncryptToken.
	// Retourne ErrCodeINTERNAL_SERVER_ERROR si le déchiffrement échoue.
	DecryptToken(ctx context.Context, ciphertext string) (string, error)
}

// ChannelCredential — encrypted OAuth credentials for a channel (Phase 9 §A9-8)
type ChannelCredential struct {
	ID              uuid.UUID
	ChannelID       uuid.UUID
	AccessTokenEnc  string     // encrypted via ICryptoService
	RefreshTokenEnc *string    // encrypted, nullable (not all platforms provide refresh tokens)
	KeyID           string     // identifies which encryption key was used (for rotation)
	ExpiresAt       *time.Time // OAuth token expiry
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IChannelCredentialRepo — ChannelCredential persistence (Phase 9 §A9-8, Phase 2 P4c §UC-S-03)
type IChannelCredentialRepo interface {
	Upsert(ctx context.Context, cred *ChannelCredential) error
	GetByChannelID(ctx context.Context, channelID uuid.UUID) (*ChannelCredential, error)
	ListByKeyID(ctx context.Context, keyID string) ([]*ChannelCredential, error) // for key rotation
	Delete(ctx context.Context, channelID uuid.UUID) error
}

// VariantStatus représente l'état d'un PostVariant (Phase 1 P3a §5.1.2).
// P13 ANOMALIE : Phase 9 CHECK inclut 'scheduled' et 'cancelled' ; la valeur 'pending' ici
// n'existe pas en base. Alignement Phase 9 à traiter dans un correctif dédié.
type VariantStatus string

const (
	VariantStatusDraft      VariantStatus = "draft"
	VariantStatusPending    VariantStatus = "pending"
	VariantStatusScheduled  VariantStatus = "scheduled"
	VariantStatusPublished  VariantStatus = "published"
	VariantStatusFailed     VariantStatus = "failed"
	VariantStatusCancelled  VariantStatus = "cancelled"
)

// PostVariant — entité satellite de Post, une par channel cible.
// Champs publics (pas d'aggregate root — hydration directe par le repo).
// UNIQUE(post_id, brand_channel_id) — Phase 1 P3a §5.1.2.
// P13 : déplacé depuis T16 partie-3b (AUD2-034). Voir ANOMALIES P13 pour divergences Phase 9.
type PostVariant struct {
	ID                   uuid.UUID
	WorkspaceID          uuid.UUID
	PostID               uuid.UUID
	BrandChannelID       uuid.UUID              // Phase 9 SQL : channel_id (nom divergent — ANOMALIE P13)
	Caption              *string                // Phase 9 SQL : content (nom divergent — ANOMALIE P13)
	MediaURLs            []string               // absent de la table post_variants Phase 9 — ANOMALIE P13
	PlatformSpecificData map[string]interface{} // Phase 9 SQL : platform_data JSONB
	Status               VariantStatus
	PlatformPostID       *string
	PlatformURL          *string    // absent de la table post_variants Phase 9 — ANOMALIE P13
	PublishedAt          *time.Time
	FailedReason         *string    // absent de la table post_variants Phase 9 — ANOMALIE P13
	RetryCount           int        // absent de la table post_variants Phase 9 — ANOMALIE P13
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// EffectiveCaption — Phase 1 P3a §5.1.2 Invariant 1 : hérite de main_caption si nil.
func (v *PostVariant) EffectiveCaption(mainCaption string) string {
	if v.Caption != nil && *v.Caption != "" {
		return *v.Caption
	}
	return mainCaption
}

// captionLimits — Phase 1 P3a §5.1.2 Invariant 4.
// ANOMALIE A013-note : P3a utilise ChannelTypeFACEBOOK_PAGE (ALL_CAPS) ;
// P3d §A.8 + T4 utilisent ChannelTypeFacebookPage (CamelCase). Valeur retenue : CamelCase (P3d canonique).
var captionLimits = map[ChannelType]int{
	ChannelTypeFacebookPage:      63206,
	ChannelTypeInstagramBusiness: 2200,
	ChannelTypeLinkedInPage:      3000,
}

// ValidateCaptionLength vérifie la limite par plateforme (Phase 1 P3a §5.1.2 Invariant 4).
func (v *PostVariant) ValidateCaptionLength(ct ChannelType) error {
	if v.Caption == nil {
		return nil
	}
	limit, ok := captionLimits[ct]
	if !ok {
		return nil
	}
	if len([]rune(*v.Caption)) > limit {
		return NewDomainError(
			ErrCodeCAPTION_TOO_LONG,
			"La légende dépasse la limite autorisée pour cette plateforme",
			map[string]interface{}{
				"channel_type": string(ct),
				"limit":        limit,
				"actual":       len([]rune(*v.Caption)),
			},
			SeverityLOW,
			false,
		)
	}
	return nil
}

// MarkPublished — transition → published ; exige platform_post_id (Invariant 2).
func (v *PostVariant) MarkPublished(platformPostID, platformURL string) error {
	if platformPostID == "" {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"platform_post_id obligatoire pour marquer une variante publiée",
			map[string]interface{}{"variant_id": v.ID},
			SeverityMEDIUM,
			false,
		)
	}
	now := time.Now().UTC()
	v.Status = VariantStatusPublished
	v.PlatformPostID = &platformPostID
	v.PlatformURL = &platformURL
	v.PublishedAt = &now
	v.UpdatedAt = now
	return nil
}

// MarkFailed — transition → failed ; enregistre la raison.
func (v *PostVariant) MarkFailed(reason string) {
	v.Status = VariantStatusFailed
	v.FailedReason = &reason
	v.RetryCount++
	v.UpdatedAt = time.Now().UTC()
}

// MarkPending — draft → pending au moment de la planification.
func (v *PostVariant) MarkPending() {
	v.Status = VariantStatusPending
	v.UpdatedAt = time.Now().UTC()
}

// NewPostVariant crée une variante pour un channel cible (status initial : draft).
func NewPostVariant(workspaceID, postID, brandChannelID uuid.UUID) *PostVariant {
	now := time.Now().UTC()
	return &PostVariant{
		ID:             uuid.New(),
		WorkspaceID:    workspaceID,
		PostID:         postID,
		BrandChannelID: brandChannelID,
		Status:         VariantStatusDraft,
		MediaURLs:      []string{},
		RetryCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// IPostVariantRepo — gère la persistance des déclinaisons de posts par plateforme (Phase 2 P3b).
// P13 : déplacé depuis T16 partie-3b (AUD2-034).
type IPostVariantRepo interface {
	Create(ctx context.Context, v *PostVariant) error
	ListByPost(ctx context.Context, postID uuid.UUID) ([]*PostVariant, error)
	Update(ctx context.Context, v *PostVariant) error
	DeleteByPost(ctx context.Context, postID uuid.UUID) error
}

// SecurityEvent — immutable record of a security-relevant event (Phase 2 P4c, Phase 3 §BC14)
type SecurityEvent struct {
	ID          uuid.UUID
	UserID      *uuid.UUID             // nullable — system events may have no user
	WorkspaceID *uuid.UUID             // nullable — cross-tenant events
	EventType   string                 // mirrors DomainEvent.EventType for security BCs
	IPHash      string                 // SHA-256 of IP address (RGPD)
	Details     map[string]interface{}
	RiskLevel   string     // low | medium | high | critical
	OccurredAt  time.Time
	CreatedAt   time.Time
}

// ISecurityEventRepo — SecurityEvent write-once log (Phase 2 P4c §UC-S-05)
type ISecurityEventRepo interface {
	Create(ctx context.Context, e *SecurityEvent) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*SecurityEvent, error)
	ListHighRisk(ctx context.Context, limit int) ([]*SecurityEvent, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*SecurityEvent, error)
}
