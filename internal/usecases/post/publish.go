package post

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// validate est partagé dans le package post (déclaré dans create.go).
// var validate = validator.New() — ne pas re-déclarer.

// ============================================================
// Port interfaces locaux — T16
// ============================================================

// ISocialPublisher — port de sortie vers les adapters plateforme (Phase 7).
// Implémenté par infrastructure/external/social/factory.go (Phase 7).
// Le décryptage du token OAuth est délégué à l'adapter (BR-18 — SYSTEM uniquement).
type ISocialPublisher interface {
	// Publish envoie la variante sur la plateforme et retourne l'ID et l'URL de publication.
	Publish(
		ctx context.Context,
		channelType domain.ChannelType,
		channelID uuid.UUID,
		variant *domain.PostVariant,
		mainCaption string,
	) (platformPostID string, platformURL string, err error)
}

// IEventBusPublish — port local publish (évite re-déclaration du package post).
type IEventBusPublish interface {
	Publish(ctx context.Context, events ...domain.DomainEvent) error
}

// ============================================================
// SchedulePostUseCase
// ============================================================

// SchedulePostCommand — CH-04 post.schedule : Editor+ avec channel_permission `schedule`.
// En pratique V0 : Editor+ avec brand assignment vérifié (channel permissions non encore implémentées).
// ChannelIDs : channels sur lesquels le post sera planifié (génère les PostVariants).
type SchedulePostCommand struct {
	PostID      uuid.UUID   `validate:"required"`
	ActorID     uuid.UUID   `validate:"required"`
	ChannelIDs  []uuid.UUID `validate:"required,min=1"`
	ScheduledAt time.Time   `validate:"required"`
}

type SchedulePostResult struct {
	PostID      string    `json:"post_id"`
	Status      string    `json:"status"`
	ScheduledAt time.Time `json:"scheduled_at"`
	ChannelIDs  []string  `json:"channel_ids"`
}

type SchedulePostUseCase struct {
	postRepo        domain.IPostRepo
	postVariantRepo domain.IPostVariantRepo
	brandRepo       domain.IBrandRepo
	workspaceRepo   domain.IWorkspaceRepo
	channelRepo     domain.IChannelRepo
	mediaAssetRepo  domain.IMediaAssetRepo
	eventBus        IEventBusPublish
}

func NewSchedulePostUseCase(
	postRepo domain.IPostRepo,
	postVariantRepo domain.IPostVariantRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	channelRepo domain.IChannelRepo,
	mediaAssetRepo domain.IMediaAssetRepo,
	eventBus IEventBusPublish,
) *SchedulePostUseCase {
	return &SchedulePostUseCase{
		postRepo:        postRepo,
		postVariantRepo: postVariantRepo,
		brandRepo:       brandRepo,
		workspaceRepo:   workspaceRepo,
		channelRepo:     channelRepo,
		mediaAssetRepo:  mediaAssetRepo,
		eventBus:        eventBus,
	}
}

func (uc *SchedulePostUseCase) Execute(ctx context.Context, cmd SchedulePostCommand) domain.Result[SchedulePostResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED,
			err.Error(),
			nil,
			domain.SeverityLOW,
			false,
		))
	}

	// ② Charger le post
	post, err := uc.postRepo.GetByID(ctx, cmd.PostID)
	if err != nil {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND,
			"Post introuvable",
			map[string]interface{}{"post_id": cmd.PostID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Vérifier statut = validated — A004 : interdiction de planifier depuis draft
	// Phase 5 SM-01 : validated → scheduled uniquement
	if post.Status() != domain.PostStatusValidated {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Seul un post validé peut être planifié — soumettez d'abord à validation (A004)",
			map[string]interface{}{
				"post_id":        cmd.PostID,
				"current_status": string(post.Status()),
				"required":       "validated",
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ④ Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND,
			"Brand du post introuvable",
			map[string]interface{}{"brand_id": post.BrandID()},
			domain.SeverityHIGH,
			false,
		))
	}

	// ⑤ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ActorID)
	if err != nil {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Acteur introuvable dans le workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑥ Vérification permission — CH-04 post.schedule : Editor+ avec brand assignment
	if !actor.Role.BypassesBrandAssignment() {
		assignment, err := uc.brandRepo.GetAssignment(ctx, post.BrandID(), cmd.ActorID)
		if err != nil {
			return domain.Fail[SchedulePostResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND,
				"Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": post.BrandID(), "actor_id": cmd.ActorID},
				domain.SeverityMEDIUM,
				false,
			))
		}
		if assignment.Role == domain.BrandRoleViewer {
			return domain.Fail[SchedulePostResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED,
				"BrandRole insuffisant — BrandEditor minimum requis pour planifier",
				map[string]interface{}{"actor_role": assignment.Role},
				domain.SeverityMEDIUM,
				false,
			))
		}
	}

	// ⑦ Vérification X-7 (TX-07 Phase 5 R-07) : aucun MediaAsset quarantined
	// Déclenché si un média attaché passe en quarantaine après validation (cas limite SM-01 TX-07)
	mediaAssets, err := uc.mediaAssetRepo.ListByPost(ctx, cmd.PostID)
	if err == nil {
		for _, asset := range mediaAssets {
			if asset.Status == domain.MediaAssetStatusQuarantined {
				return domain.Fail[SchedulePostResult](domain.NewDomainError(
					domain.ErrCodeVIRUS_DETECTED,
					"Un média attaché à ce post est en quarantaine — publication bloquée (X-7)",
					map[string]interface{}{
						"post_id":        cmd.PostID,
						"media_asset_id": asset.ID,
					},
					domain.SeverityHIGH,
					false,
				))
			}
		}
	}

	// ⑧ Vérification CanPublish() sur chaque channel cible — C-1/C-1b/C-1c/C-2
	var validChannels []*domain.Channel
	for _, channelID := range cmd.ChannelIDs {
		channel, err := uc.channelRepo.GetByID(ctx, channelID)
		if err != nil {
			return domain.Fail[SchedulePostResult](domain.NewDomainError(
				domain.ErrCodeCHANNEL_NOT_FOUND,
				"Channel cible introuvable",
				map[string]interface{}{"channel_id": channelID},
				domain.SeverityMEDIUM,
				false,
			))
		}
		// C-1/C-1b/C-1c/C-2 : CanPublish() vérifie REVOKED → ERROR → DISCONNECTED → TOKEN invalide
		if domErr := channel.CanPublish(); domErr != nil {
			return domain.Fail[SchedulePostResult](domErr.(*domain.DomainError))
		}
		validChannels = append(validChannels, channel)
	}

	// ⑨ Transition validated → scheduled — post.Schedule() applique garde P-2 (>now+5min)
	if domErr := post.Schedule(cmd.ScheduledAt); domErr != nil {
		return domain.Fail[SchedulePostResult](domErr.(*domain.DomainError))
	}

	// ⑩ Créer ou mettre à jour les PostVariants (une par channel)
	for _, channel := range validChannels {
		variant := domain.NewPostVariant(brand.WorkspaceID(), post.ID(), channel.ID())
		variant.MarkPending()
		// Ignorer erreur de création individuelle — la variante sera recréée si besoin
		_ = uc.postVariantRepo.Create(ctx, variant)
	}

	// ⑪ Persistance post
	if err := uc.postRepo.Update(ctx, post); err != nil {
		return domain.Fail[SchedulePostResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la persistance de la planification",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑫ Publication événement domaine — Phase 3 §BC05
	channelIDStrs := make([]string, len(cmd.ChannelIDs))
	for i, id := range cmd.ChannelIDs {
		channelIDStrs[i] = id.String()
	}
	_ = uc.eventBus.Publish(ctx, events.NewPostScheduledEvent(
		post.ID(),            // postID uuid.UUID
		post.WorkspaceID(),   // workspaceID uuid.UUID
		post.BrandID(),       // brandID uuid.UUID
		cmd.ScheduledAt,      // scheduledAt time.Time
		cmd.ActorID.String(), // scheduledByUserID string
		channelIDStrs,        // channelIDs []string
	))

	return domain.Ok[SchedulePostResult](SchedulePostResult{
		PostID:      post.ID().String(),
		Status:      string(domain.PostStatusScheduled),
		ScheduledAt: cmd.ScheduledAt,
		ChannelIDs:  channelIDStrs,
	})
}

// ============================================================
// PublishPostUseCase — acteur SYSTEM (worker CRON PB-C-010)
// ============================================================

// PublishPostCommand — PB-C-010 : déclenché par worker CRON, ActorID = UUID système.
type PublishPostCommand struct {
	PostID  uuid.UUID `validate:"required"`
	ActorID uuid.UUID `validate:"required"` // UUID SYSTEM
}

type PublishPostResult struct {
	PostID            string   `json:"post_id"`
	Status            string   `json:"status"`
	ChannelsPublished []string `json:"channels_published"`
	ChannelsFailed    []string `json:"channels_failed"`
}

type PublishPostUseCase struct {
	postRepo        domain.IPostRepo
	postVariantRepo domain.IPostVariantRepo
	channelRepo     domain.IChannelRepo
	mediaAssetRepo  domain.IMediaAssetRepo
	socialPublisher ISocialPublisher
	eventBus        IEventBusPublish
}

func NewPublishPostUseCase(
	postRepo domain.IPostRepo,
	postVariantRepo domain.IPostVariantRepo,
	channelRepo domain.IChannelRepo,
	mediaAssetRepo domain.IMediaAssetRepo,
	socialPublisher ISocialPublisher,
	eventBus IEventBusPublish,
) *PublishPostUseCase {
	return &PublishPostUseCase{
		postRepo:        postRepo,
		postVariantRepo: postVariantRepo,
		channelRepo:     channelRepo,
		mediaAssetRepo:  mediaAssetRepo,
		socialPublisher: socialPublisher,
		eventBus:        eventBus,
	}
}

func (uc *PublishPostUseCase) Execute(ctx context.Context, cmd PublishPostCommand) domain.Result[PublishPostResult] {
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[PublishPostResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ① Charger le post — doit être scheduled
	post, err := uc.postRepo.GetByID(ctx, cmd.PostID)
	if err != nil {
		return domain.Fail[PublishPostResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND, "Post introuvable",
			map[string]interface{}{"post_id": cmd.PostID}, domain.SeverityMEDIUM, false,
		))
	}
	if post.Status() != domain.PostStatusScheduled {
		return domain.Fail[PublishPostResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"PublishPost ne s'applique qu'aux posts en statut 'scheduled'",
			map[string]interface{}{"post_id": cmd.PostID, "current_status": string(post.Status())},
			domain.SeverityMEDIUM, false,
		))
	}

	// ② X-7 : re-vérification quarantined (un média peut être scanné après planification — SM-01 TX-07)
	mediaAssets, err := uc.mediaAssetRepo.ListByPost(ctx, cmd.PostID)
	if err == nil {
		for _, asset := range mediaAssets {
			if asset.Status == domain.MediaAssetStatusQuarantined {
				// Transition scheduled → failed (virus détecté post-planification)
				_ = uc.postRepo.Update(ctx, post)
				return domain.Fail[PublishPostResult](domain.NewDomainError(
					domain.ErrCodeVIRUS_DETECTED,
					"Média en quarantaine détecté — publication annulée (X-7)",
					map[string]interface{}{"post_id": cmd.PostID, "media_asset_id": asset.ID},
					domain.SeverityHIGH, false,
				))
			}
		}
	}

	// ③ Charger les variants
	variants, err := uc.postVariantRepo.ListByPost(ctx, cmd.PostID)
	if err != nil || len(variants) == 0 {
		return domain.Fail[PublishPostResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Aucune variante trouvée pour ce post — planification incomplète",
			map[string]interface{}{"post_id": cmd.PostID}, domain.SeverityHIGH, false,
		))
	}

	// ④ Récupérer mainCaption pour l'héritage (Phase 1 P3a §5.1.2 Invariant 1)
	mainCaption := ""
	if post.MainCaption() != nil {
		mainCaption = *post.MainCaption()
	}

	var channelsPublished, channelsFailed []string
	allFailed := true

	// ⑤ Publier chaque variante
	for _, variant := range variants {
		channel, err := uc.channelRepo.GetByID(ctx, variant.BrandChannelID)
		if err != nil {
			variant.MarkFailed("channel_not_found")
			_ = uc.postVariantRepo.Update(ctx, variant)
			channelsFailed = append(channelsFailed, variant.BrandChannelID.String())
			continue
		}

		// C-1/C-1b/C-1c/C-2 : CanPublish() — vérification au moment de la publication
		if domErr := channel.CanPublish(); domErr != nil {
			variant.MarkFailed(domErr.Error())
			_ = uc.postVariantRepo.Update(ctx, variant)
			channelsFailed = append(channelsFailed, channel.ID().String())
			continue
		}

		// Publication via adapter externe (Phase 7)
		platformPostID, platformURL, pubErr := uc.socialPublisher.Publish(
			ctx,
			channel.ChannelType(),
			channel.ID(),
			variant,
			mainCaption,
		)

		if pubErr != nil {
			// Échec de publication — C-4 : incrémenter ConsecutiveFailures
			variant.MarkFailed(pubErr.Error())
			_ = uc.postVariantRepo.Update(ctx, variant)

			// C-4 : seuil 3 → status channel = error (T4 RecordPublicationFailure)
			if thresholdReached := channel.RecordPublicationFailure(); thresholdReached {
				channel.SetStatus(domain.ChannelStatusError)
			}
			_ = uc.channelRepo.Update(ctx, channel)
			channelsFailed = append(channelsFailed, channel.ID().String())
		} else {
			// Succès — marquer variant published + remettre compteur à 0 (C-4)
			if markErr := variant.MarkPublished(platformPostID, platformURL); markErr == nil {
				_ = uc.postVariantRepo.Update(ctx, variant)
				channel.ResetFailureCount()
				_ = uc.channelRepo.Update(ctx, channel)
				channelsPublished = append(channelsPublished, channel.ID().String())
				allFailed = false
			}
		}
	}

	// ⑥ Transition post finale — SM-01
	now := time.Now().UTC()
	if allFailed || len(channelsPublished) == 0 {
		// Tous les channels ont échoué → post.Status = failed
		// CanTransitionTo(PostStatusFailed) depuis scheduled est valide (SM-01)
		if err := uc.postRepo.Update(ctx, post); err == nil {
			_ = uc.eventBus.Publish(ctx, events.NewPostFailedEvent(
				post.ID(),                                   // postID uuid.UUID
				post.WorkspaceID(),                          // workspaceID uuid.UUID
				post.BrandID(),                              // brandID uuid.UUID
				string(domain.ErrCodeINTERNAL_SERVER_ERROR), // errorCode string
				now,                                         // failedAt time.Time
				post.RetryCount(),                           // retryCount int
			))
		}
		return domain.Ok[PublishPostResult](PublishPostResult{
			PostID:         post.ID().String(),
			Status:         string(domain.PostStatusFailed),
			ChannelsFailed: channelsFailed,
		})
	}

	// Publication partielle ou totale réussie → published
	// SM-01 : scheduled → published (T4 CanTransitionTo valide)
	_ = uc.postRepo.Update(ctx, post)
	_ = uc.eventBus.Publish(ctx, events.NewPostPublishedEvent(
		post.ID(),          // postID uuid.UUID
		post.WorkspaceID(), // workspaceID uuid.UUID
		post.BrandID(),     // brandID uuid.UUID
		now,                // publishedAt time.Time
		channelsPublished,  // channelsPublished []string
	))

	return domain.Ok[PublishPostResult](PublishPostResult{
		PostID:            post.ID().String(),
		Status:            string(domain.PostStatusPublished),
		ChannelsPublished: channelsPublished,
		ChannelsFailed:    channelsFailed,
	})
}

// shared validate instance — déclarée ici si create.go n'existe pas encore dans ce package.
var validate = validator.New()
