package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 1. PostStatus — Phase 5 §SM-01 (A004 — draft→pending_validation uniquement)
// ---------------------------------------------------------------------------

type PostStatus string

const (
	PostStatusDraft             PostStatus = "draft"
	PostStatusPendingValidation PostStatus = "pending_validation"
	PostStatusValidated         PostStatus = "validated"
	PostStatusRejected          PostStatus = "rejected"
	PostStatusScheduled         PostStatus = "scheduled"
	PostStatusPublished         PostStatus = "published"
	PostStatusFailed            PostStatus = "failed"
	// cancelled : constante disponible, utilisée hors SM-01 standard (annulation admin).
	// La méthode Cancel() du domaine transite scheduled→draft (SM-01 §PB-C-008).
	PostStatusCancelled PostStatus = "cancelled"
)

// postStatusTransitions — SM-01 Phase 5 (exhaustif).
// Transitions INTERDITES notables :
//   draft → scheduled   (A004 — doit passer par pending_validation)
//   draft → published   (A004 — idem)
//   draft → validated   (doit passer par le workflow approbation)
//   published → *       (terminal absolu)
//   failed → scheduled  (doit repasser par draft — D-05)
var postStatusTransitions = map[PostStatus][]PostStatus{
	PostStatusDraft:             {PostStatusPendingValidation},
	PostStatusPendingValidation: {PostStatusValidated, PostStatusRejected, PostStatusDraft},
	PostStatusValidated:         {PostStatusScheduled, PostStatusPublished, PostStatusDraft},
	PostStatusRejected:          {PostStatusDraft},
	PostStatusScheduled:         {PostStatusPublished, PostStatusFailed, PostStatusDraft},
	PostStatusPublished:         {},
	PostStatusFailed:            {PostStatusDraft},
	PostStatusCancelled:         {},
}

func (s PostStatus) IsTerminal() bool { return s == PostStatusPublished }

func (s PostStatus) IsValid() bool {
	switch s {
	case PostStatusDraft, PostStatusPendingValidation, PostStatusValidated,
		PostStatusRejected, PostStatusScheduled, PostStatusPublished,
		PostStatusFailed, PostStatusCancelled:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 2. PublishType — DDL chk_posts_publish_type
// ---------------------------------------------------------------------------

type PublishType string

const (
	PublishTypeImmediate PublishType = "immediate"
	PublishTypeScheduled PublishType = "scheduled"
)

func (t PublishType) IsValid() bool {
	return t == PublishTypeImmediate || t == PublishTypeScheduled
}

// ---------------------------------------------------------------------------
// 3. Post — aggregate root
// ---------------------------------------------------------------------------

// Post est l'aggregate root du bounded context Content Publishing.
type Post struct {
	id                 uuid.UUID
	workspaceID        uuid.UUID
	brandID            uuid.UUID
	title              *string
	mainCaption        *string
	mediaURLs          []string
	status             PostStatus
	publishType        PublishType
	scheduledAt        *time.Time
	publishedAt        *time.Time
	createdByUserID    uuid.UUID
	validatedByUserID  *uuid.UUID
	approvedByGuestID  *uuid.UUID
	rejectionReason    *string
	version            int
	retryCount         int
	createdAt          time.Time
	updatedAt          time.Time
	deletedAt          *time.Time
	events             []DomainEvent
}

// Getters

func (p *Post) ID() uuid.UUID                   { return p.id }
func (p *Post) WorkspaceID() uuid.UUID          { return p.workspaceID }
func (p *Post) BrandID() uuid.UUID              { return p.brandID }
func (p *Post) Title() *string                  { return p.title }
func (p *Post) MainCaption() *string            { return p.mainCaption }
func (p *Post) MediaURLs() []string             { return p.mediaURLs }
func (p *Post) Status() PostStatus              { return p.status }
func (p *Post) PublishType() PublishType        { return p.publishType }
func (p *Post) ScheduledAt() *time.Time         { return p.scheduledAt }
func (p *Post) PublishedAt() *time.Time         { return p.publishedAt }
func (p *Post) CreatedByUserID() uuid.UUID      { return p.createdByUserID }
func (p *Post) ValidatedByUserID() *uuid.UUID   { return p.validatedByUserID }
func (p *Post) ApprovedByGuestID() *uuid.UUID   { return p.approvedByGuestID }
func (p *Post) RejectionReason() *string        { return p.rejectionReason }
func (p *Post) Version() int                    { return p.version }
func (p *Post) RetryCount() int                 { return p.retryCount }
func (p *Post) CreatedAt() time.Time            { return p.createdAt }
func (p *Post) UpdatedAt() time.Time            { return p.updatedAt }
func (p *Post) DeletedAt() *time.Time           { return p.deletedAt }
func (p *Post) Events() []DomainEvent           { return p.events }
func (p *Post) ClearEvents()                    { p.events = nil }

// NewPost crée un post en état draft. Brand + author obligatoires.
func NewPost(brandID, authorID uuid.UUID, title string) (*Post, error) {
	if brandID == uuid.Nil {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le brandID ne peut pas être nul",
			map[string]interface{}{"field": "brand_id"},
			SeverityLOW,
			false,
		)
	}
	if authorID == uuid.Nil {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"L'authorID ne peut pas être nul",
			map[string]interface{}{"field": "author_id"},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	p := &Post{
		id:              uuid.New(),
		brandID:         brandID,
		createdByUserID: authorID,
		status:          PostStatusDraft,
		publishType:     PublishTypeScheduled,
		version:         1,
		retryCount:      0,
		createdAt:       now,
		updatedAt:       now,
	}
	if title != "" {
		p.title = &title
	}
	return p, nil
}

// CanTransitionTo vérifie SM-01 exhaustivement (P-1).
func (p *Post) CanTransitionTo(target PostStatus) error {
	allowed, ok := postStatusTransitions[p.status]
	if !ok {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Statut source inconnu dans la state machine Post",
			map[string]interface{}{"current_status": string(p.status)},
			SeverityCRITICAL,
			false,
		)
	}
	for _, t := range allowed {
		if t == target {
			return nil
		}
	}
	return NewDomainError(
		ErrCodeINVALID_STATUS_TRANSITION,
		"Transition de statut post interdite",
		map[string]interface{}{
			"current_status": string(p.status),
			"target_status":  string(target),
			"allowed":        allowed,
		},
		SeverityMEDIUM,
		false,
	)
}

// SubmitForValidation — A004 : DRAFT → PENDING_VALIDATION uniquement.
// Transitions interdites depuis draft : scheduled, published.
func (p *Post) SubmitForValidation() error {
	if err := p.CanTransitionTo(PostStatusPendingValidation); err != nil {
		return err
	}
	p.status = PostStatusPendingValidation
	p.version++
	p.updatedAt = time.Now().UTC()
	return nil
}

// Validate approuve le post : pending_validation → validated.
func (p *Post) Validate(actorID uuid.UUID) error {
	if err := p.CanTransitionTo(PostStatusValidated); err != nil {
		return err
	}
	p.status = PostStatusValidated
	p.validatedByUserID = &actorID
	p.version++
	p.updatedAt = time.Now().UTC()
	return nil
}

// Reject refuse le post : pending_validation → rejected.
func (p *Post) Reject(actorID uuid.UUID, reason string) error {
	if err := p.CanTransitionTo(PostStatusRejected); err != nil {
		return err
	}
	if reason == "" {
		return NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Une raison de refus est obligatoire",
			map[string]interface{}{"field": "rejection_reason"},
			SeverityLOW,
			false,
		)
	}
	p.status = PostStatusRejected
	p.rejectionReason = &reason
	p.version++
	p.updatedAt = time.Now().UTC()
	return nil
}

// Schedule programme la publication : validated → scheduled.
// Garde P-2 : scheduledAt > now + 5min.
func (p *Post) Schedule(scheduledAt time.Time) error {
	if err := p.CanTransitionTo(PostStatusScheduled); err != nil {
		return err
	}
	minScheduleTime := time.Now().UTC().Add(5 * time.Minute)
	if scheduledAt.Before(minScheduleTime) {
		return NewDomainError(
			ErrCodeSCHEDULE_DATE_IN_PAST,
			"La date de programmation doit être dans au moins 5 minutes",
			map[string]interface{}{
				"scheduled_at": scheduledAt,
				"min_allowed":  minScheduleTime,
			},
			SeverityLOW,
			false,
		)
	}
	p.status = PostStatusScheduled
	p.scheduledAt = &scheduledAt
	p.publishType = PublishTypeScheduled
	p.version++
	p.updatedAt = time.Now().UTC()
	return nil
}

// Cancel annule un post programmé : scheduled → draft (PB-C-008).
func (p *Post) Cancel() error {
	if err := p.CanTransitionTo(PostStatusDraft); err != nil {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Seul un post 'scheduled' peut être annulé",
			map[string]interface{}{
				"current_status": string(p.status),
				"post_id":        p.id.String(),
			},
			SeverityLOW,
			false,
		)
	}
	if p.status != PostStatusScheduled {
		return NewDomainError(
			ErrCodeINVALID_STATUS_TRANSITION,
			"Seul un post 'scheduled' peut être annulé",
			map[string]interface{}{
				"current_status": string(p.status),
				"post_id":        p.id.String(),
			},
			SeverityLOW,
			false,
		)
	}
	p.status = PostStatusDraft
	p.scheduledAt = nil
	p.version++
	p.updatedAt = time.Now().UTC()
	return nil
}

// CanRetry vérifie si le post peut être remis en draft pour retry (D-05).
// retry_count < 3 obligatoire.
func (p *Post) CanRetry() bool {
	return p.status == PostStatusFailed && p.retryCount < 3
}

// IncrementRetryCount incrémente le compteur avant remise en draft (D-05).
func (p *Post) IncrementRetryCount() {
	p.retryCount++
	p.updatedAt = time.Now().UTC()
}

// ---------------------------------------------------------------------------
// 3. MediaAssetStatus — Phase 1 P3d §A.13
// ---------------------------------------------------------------------------

type MediaAssetStatus string

const (
	// pending_scan : défaut à la création (A016).
	// Phase 1 P3a indique 'processing' comme défaut DB — A016 corrige en 'pending_scan'.
	MediaAssetStatusPendingScan MediaAssetStatus = "pending_scan"
	MediaAssetStatusProcessing  MediaAssetStatus = "processing"
	MediaAssetStatusReady       MediaAssetStatus = "ready"
	MediaAssetStatusFailed      MediaAssetStatus = "failed"
	MediaAssetStatusQuarantined MediaAssetStatus = "quarantined"
	MediaAssetStatusDeleted     MediaAssetStatus = "deleted"
)

func (s MediaAssetStatus) IsUsable() bool { return s == MediaAssetStatusReady }

// ---------------------------------------------------------------------------
// 4. AntivirusStatus — Phase 1 P3d §A.14
// ---------------------------------------------------------------------------

type AntivirusStatus string

const (
	AntivirusStatusPending     AntivirusStatus = "pending"
	AntivirusStatusScanning    AntivirusStatus = "scanning"
	AntivirusStatusClean       AntivirusStatus = "clean"
	AntivirusStatusQuarantined AntivirusStatus = "quarantined"
	AntivirusStatusError       AntivirusStatus = "error"
)

// ---------------------------------------------------------------------------
// 5. MediaAsset — aggregate root
// ---------------------------------------------------------------------------

// MediaAsset est l'aggregate root du bounded context Media Management.
type MediaAsset struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	BrandID            *uuid.UUID
	AssetType          string
	UploadedByUserID   uuid.UUID
	FilenameOriginal   string
	FilenameStored     string
	StoragePath        string
	StorageBucket      string
	MimeType           string
	FileSizeBytes      int64
	WidthPx            *int
	HeightPx           *int
	DurationSeconds    *int
	AltText            *string
	AntivirusStatus    AntivirusStatus
	AntivirusScannedAt *time.Time
	AntivirusEngine    *string
	IsOptimized        bool
	OptimizedAt        *time.Time
	// Status initial : pending_scan (A016). Corrige la valeur DB par défaut 'processing'.
	Status    MediaAssetStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// NewMediaAsset crée un media asset. Status initial : pending_scan (A016).
func NewMediaAsset(
	workspaceID uuid.UUID,
	uploadedByUserID uuid.UUID,
	filenameOriginal, filenameStored, storagePath, storageBucket, mimeType string,
	fileSizeBytes int64,
) (*MediaAsset, error) {
	if workspaceID == uuid.Nil {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"Le workspaceID ne peut pas être nul",
			map[string]interface{}{"field": "workspace_id"},
			SeverityLOW,
			false,
		)
	}
	if fileSizeBytes <= 0 {
		return nil, NewDomainError(
			ErrCodeVALIDATION_FAILED,
			"La taille du fichier doit être supérieure à 0",
			map[string]interface{}{"field": "file_size_bytes", "value": fileSizeBytes},
			SeverityLOW,
			false,
		)
	}
	allowedMimeTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
		"video/mp4":  true,
	}
	if !allowedMimeTypes[mimeType] {
		return nil, NewDomainError(
			ErrCodeINVALID_MEDIA_TYPE,
			"Le type MIME n'est pas autorisé",
			map[string]interface{}{
				"mime_type": mimeType,
				"allowed":   []string{"image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4"},
			},
			SeverityLOW,
			false,
		)
	}
	now := time.Now().UTC()
	return &MediaAsset{
		ID:               uuid.New(),
		WorkspaceID:      workspaceID,
		UploadedByUserID: uploadedByUserID,
		FilenameOriginal: filenameOriginal,
		FilenameStored:   filenameStored,
		StoragePath:      storagePath,
		StorageBucket:    storageBucket,
		MimeType:         mimeType,
		FileSizeBytes:    fileSizeBytes,
		AntivirusStatus:  AntivirusStatusPending,
		IsOptimized:      false,
		Status:           MediaAssetStatusPendingScan, // A016
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// IsUsable vérifie que le média est prêt pour utilisation dans un post.
// Invariant MA-2 : status=ready + antivirus=clean + non supprimé.
func (m *MediaAsset) IsUsable() bool {
	return m.Status == MediaAssetStatusReady &&
		m.AntivirusStatus == AntivirusStatusClean &&
		m.DeletedAt == nil
}

// MarkReady passe le média en état 'ready' après scan antivirus clean.
func (m *MediaAsset) MarkReady(engine string) {
	now := time.Now().UTC()
	m.Status = MediaAssetStatusReady
	m.AntivirusStatus = AntivirusStatusClean
	m.AntivirusEngine = &engine
	m.AntivirusScannedAt = &now
	m.UpdatedAt = now
}

// Quarantine met le média en quarantaine après détection d'une menace.
func (m *MediaAsset) Quarantine(engine string) {
	now := time.Now().UTC()
	m.Status = MediaAssetStatusQuarantined
	m.AntivirusStatus = AntivirusStatusQuarantined
	m.AntivirusEngine = &engine
	m.AntivirusScannedAt = &now
	m.UpdatedAt = now
}

// SoftDelete marque le média comme supprimé (RGPD — purge physique après 30j).
func (m *MediaAsset) SoftDelete() {
	now := time.Now().UTC()
	m.Status = MediaAssetStatusDeleted
	m.DeletedAt = &now
	m.UpdatedAt = now
}
