package media

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

var validate = validator.New()

// ============================================================
// Port interfaces locaux — T17
// ============================================================

type IEventBusMedia interface {
	Publish(ctx context.Context, events ...domain.DomainEvent) error
	PublishSystem(ctx context.Context, events ...domain.DomainEvent) error
}

// ============================================================
// UploadMediaAssetUseCase
// ============================================================

// UploadMediaAssetCommand — TX-08 media.upload : Editor+ avec brand assignment.
// StorageKey : clé objet dans le bucket de stockage (S3/GCS) — upload déjà effectué par le client.
// Le use case ne gère pas l'upload réseau — il enregistre le MediaAsset post-upload.
type UploadMediaAssetCommand struct {
	PostID      uuid.UUID `validate:"required"`
	ActorID     uuid.UUID `validate:"required"`
	Filename    string    `validate:"required,min=1,max=512"`
	ContentType string    `validate:"required"`
	SizeBytes   int64     `validate:"required,min=1"`
	StorageKey  string    `validate:"required"`
}

type UploadMediaAssetResult struct {
	MediaAssetID string    `json:"media_asset_id"`
	Status       string    `json:"status"` // pending_scan — A016
	CreatedAt    time.Time `json:"created_at"`
}

type UploadMediaAssetUseCase struct {
	mediaAssetRepo domain.IMediaAssetRepo
	postRepo       domain.IPostRepo
	brandRepo      domain.IBrandRepo
	workspaceRepo  domain.IWorkspaceRepo
	eventBus       IEventBusMedia
}

func NewUploadMediaAssetUseCase(
	mediaAssetRepo domain.IMediaAssetRepo,
	postRepo domain.IPostRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	eventBus IEventBusMedia,
) *UploadMediaAssetUseCase {
	return &UploadMediaAssetUseCase{
		mediaAssetRepo: mediaAssetRepo,
		postRepo:       postRepo,
		brandRepo:      brandRepo,
		workspaceRepo:  workspaceRepo,
		eventBus:       eventBus,
	}
}

func (uc *UploadMediaAssetUseCase) Execute(ctx context.Context, cmd UploadMediaAssetCommand) domain.Result[UploadMediaAssetResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Charger le post associé (pour récupérer brandID + workspaceID)
	post, err := uc.postRepo.GetByID(ctx, cmd.PostID)
	if err != nil {
		return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND, "Post introuvable",
			map[string]interface{}{"post_id": cmd.PostID}, domain.SeverityMEDIUM, false,
		))
	}

	// ③ Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND, "Brand du post introuvable",
			map[string]interface{}{"brand_id": post.BrandID()}, domain.SeverityHIGH, false,
		))
	}

	// ④ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ActorID)
	if err != nil {
		return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND, "Acteur introuvable",
			map[string]interface{}{"actor_id": cmd.ActorID}, domain.SeverityMEDIUM, false,
		))
	}

	// ⑤ Vérification permission — TX-08 media.upload : Editor+ avec brand assignment
	if !actor.Role.BypassesBrandAssignment() {
		assignment, err := uc.brandRepo.GetAssignment(ctx, post.BrandID(), cmd.ActorID)
		if err != nil {
			return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND, "Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": post.BrandID()}, domain.SeverityMEDIUM, false,
			))
		}
		if assignment.Role == domain.BrandRoleViewer {
			return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED, "BrandEditor minimum requis pour upload de média",
				map[string]interface{}{"actor_role": assignment.Role}, domain.SeverityMEDIUM, false,
			))
		}
	}

	// ⑥ Vérification type MIME (NewMediaAsset valide déjà — T4)
	// Les types autorisés : image/jpeg, image/png, image/gif, image/webp, video/mp4
	// T4 NewMediaAsset retourne ErrCodeINVALID_MEDIA_TYPE si ContentType non supporté.

	// ⑦ Création MediaAsset — M-1 : status initial = pending_scan (A016)
	// ANOMALIE A033-note : NewMediaAsset (T4) ne prend pas BrandID.
	// Workaround : utiliser NewMediaAsset + setter BrandID (champ public sur MediaAsset T4).
	brandID := post.BrandID()
	storageBucket := "weepost-media" // convention V0 — en prod depuis config
	asset, domErr := domain.NewMediaAsset(
		brand.WorkspaceID(),
		cmd.ActorID,
		cmd.Filename,
		cmd.Filename,   // filenameStored = filenameOriginal en V0 (storage key géré séparément)
		cmd.StorageKey, // storagePath = storage key
		storageBucket,
		cmd.ContentType,
		cmd.SizeBytes,
	)
	if domErr != nil {
		return domain.Fail[UploadMediaAssetResult](domErr.(*domain.DomainError))
	}
	// ANOMALIE A033 : BrandID absent de NewMediaAsset — setter direct (champ public)
	asset.BrandID = &brandID
	// Status confirmé : pending_scan (A016 — T4 NewMediaAsset initialise pending_scan)

	// ⑧ Persistance
	if err := uc.mediaAssetRepo.Create(ctx, asset); err != nil {
		return domain.Fail[UploadMediaAssetResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "Échec de la persistance du média",
			nil, domain.SeverityHIGH, true,
		))
	}

	// ⑨ Publication événement domaine — Phase 3 §BC05
	// NewMediaUploadedEvent déclenche le scan antivirus async (consommateur : AntivirusWorker)
	_ = uc.eventBus.Publish(ctx, events.NewMediaUploadedEvent(
		asset.ID,
		brand.WorkspaceID(),
		brandID,
		cmd.ActorID.String(),
		cmd.ContentType,
		cmd.SizeBytes,
	))

	return domain.Ok[UploadMediaAssetResult](UploadMediaAssetResult{
		MediaAssetID: asset.ID.String(),
		Status:       string(domain.MediaAssetStatusPendingScan), // M-1 : pending_scan
		CreatedAt:    asset.CreatedAt,
	})
}

// ============================================================
// QuarantineMediaUseCase — acteur SYSTEM (worker antivirus)
// ============================================================

// QuarantineMediaCommand — déclenché par le worker d'antivirus (SYSTEM actor).
// Phase 5 R-07 : media quarantined → X-7 bloque publication des posts liés.
// AntivirusEngine : nom du moteur antivirus (ex: "ClamAV 1.0")
// ThreatName : nom de la menace détectée (ex: "Win.Malware.Eicar")
type QuarantineMediaCommand struct {
	MediaAssetID    uuid.UUID `validate:"required"`
	Reason          string    `validate:"required,min=1,max=1000"`
	AntivirusEngine string    `validate:"required"`
	ThreatName      string    `validate:"required"`
}

type QuarantineMediaResult struct {
	MediaAssetID  string    `json:"media_asset_id"`
	Status        string    `json:"status"`
	QuarantinedAt time.Time `json:"quarantined_at"`
}

type QuarantineMediaUseCase struct {
	mediaAssetRepo domain.IMediaAssetRepo
	eventBus       IEventBusMedia
}

func NewQuarantineMediaUseCase(
	mediaAssetRepo domain.IMediaAssetRepo,
	eventBus IEventBusMedia,
) *QuarantineMediaUseCase {
	return &QuarantineMediaUseCase{
		mediaAssetRepo: mediaAssetRepo,
		eventBus:       eventBus,
	}
}

func (uc *QuarantineMediaUseCase) Execute(ctx context.Context, cmd QuarantineMediaCommand) domain.Result[QuarantineMediaResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[QuarantineMediaResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Charger le média
	asset, err := uc.mediaAssetRepo.GetByID(ctx, cmd.MediaAssetID)
	if err != nil {
		return domain.Fail[QuarantineMediaResult](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT, "MediaAsset introuvable",
			map[string]interface{}{"media_asset_id": cmd.MediaAssetID}, domain.SeverityMEDIUM, false,
		))
	}

	// ③ Garde idempotence — déjà quarantined
	if asset.Status == domain.MediaAssetStatusQuarantined {
		return domain.Ok[QuarantineMediaResult](QuarantineMediaResult{
			MediaAssetID:  asset.ID.String(),
			Status:        string(domain.MediaAssetStatusQuarantined),
			QuarantinedAt: asset.UpdatedAt,
		})
	}

	// ④ Transition → quarantined (T4 MediaAsset.Quarantine)
	asset.Quarantine(cmd.AntivirusEngine)

	// ⑤ Persistance
	if err := uc.mediaAssetRepo.Update(ctx, asset); err != nil {
		return domain.Fail[QuarantineMediaResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "Échec de la persistance de la quarantaine",
			nil, domain.SeverityHIGH, true,
		))
	}

	// ⑥ Publication événement domaine — Phase 3 §BC05 + X-7 propagation
	// Consommateurs : ScheduledPostChecker (bloque publication) + Notifications (alerte équipe)
	var brandIDForEvent uuid.UUID
	if asset.BrandID != nil {
		brandIDForEvent = *asset.BrandID
	}
	_ = uc.eventBus.PublishSystem(ctx, events.NewMediaQuarantinedEvent(
		asset.ID,
		uuid.Nil,
		brandIDForEvent,
		cmd.AntivirusEngine,
		cmd.ThreatName,
	))

	return domain.Ok[QuarantineMediaResult](QuarantineMediaResult{
		MediaAssetID:  asset.ID.String(),
		Status:        string(domain.MediaAssetStatusQuarantined),
		QuarantinedAt: asset.UpdatedAt,
	})
}
