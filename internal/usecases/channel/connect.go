package channel

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
// Port interfaces locaux (non définis dans T5 — domaine externe)
// ============================================================

// IChannelCredentialRepo — ANOMALIE A030 : absent de T5 repositories.go
// Correspond à la table brand_channel_credentials (Phase 1 P2 §4.1.3).
// À ajouter dans domain/repositories.go T5.
type IChannelCredentialRepo interface {
	Create(ctx context.Context, channelID uuid.UUID, accessTokenEnc, refreshTokenEnc string, expiresAt *time.Time) error
	Update(ctx context.Context, channelID uuid.UUID, accessTokenEnc, refreshTokenEnc string, expiresAt *time.Time) error
	DeleteByChannelID(ctx context.Context, channelID uuid.UUID) error
}

// IPlanLimitsCheckerChannel — port local pour vérification quota channels (Phase 6 §5)
type IPlanLimitsCheckerChannel interface {
	CheckChannelLimit(ctx context.Context, workspaceID, brandID uuid.UUID) error
}

// IEventBusChannel — port local isolé pour publication événements
type IEventBusChannel interface {
	Publish(ctx context.Context, events ...domain.DomainEvent) error
}

// ============================================================
// ConnectChannelUseCase
// ============================================================

// ConnectChannelCommand — BR-10 : acteur doit avoir BrandRole ∈ {BrandOwner, BrandManager}
// ou être Owner workspace (bypass). Platform : A015 — pas twitter en V0.
type ConnectChannelCommand struct {
	BrandID      uuid.UUID `validate:"required"`
	ActorID      uuid.UUID `validate:"required"`
	Platform     string    `validate:"required,oneof=facebook_page instagram_business linkedin_page"`
	AccessToken  string    `validate:"required"`
	RefreshToken string    // optionnel — certaines plateformes n'en ont pas
	ExternalID   string    `validate:"required"`
	DisplayName  string    `validate:"required"`
	TokenExpiresAt *time.Time // nil = pas d'expiration connue
}

type ConnectChannelResult struct {
	ChannelID   string    `json:"channel_id"`
	BrandID     string    `json:"brand_id"`
	Platform    string    `json:"platform"`
	DisplayName string    `json:"display_name"`
	ExternalID  string    `json:"external_id"`
	Status      string    `json:"status"`
	ConnectedAt time.Time `json:"connected_at"`
}

type ConnectChannelUseCase struct {
	channelRepo     domain.IChannelRepo
	channelCredRepo IChannelCredentialRepo
	brandRepo       domain.IBrandRepo
	workspaceRepo   domain.IWorkspaceRepo
	cryptoSvc       domain.ICryptoService
	planChecker     IPlanLimitsCheckerChannel
	eventBus        IEventBusChannel
}

func NewConnectChannelUseCase(
	channelRepo domain.IChannelRepo,
	channelCredRepo IChannelCredentialRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	cryptoSvc domain.ICryptoService,
	planChecker IPlanLimitsCheckerChannel,
	eventBus IEventBusChannel,
) *ConnectChannelUseCase {
	return &ConnectChannelUseCase{
		channelRepo:     channelRepo,
		channelCredRepo: channelCredRepo,
		brandRepo:       brandRepo,
		workspaceRepo:   workspaceRepo,
		cryptoSvc:       cryptoSvc,
		planChecker:     planChecker,
		eventBus:        eventBus,
	}
}

func (uc *ConnectChannelUseCase) Execute(ctx context.Context, cmd ConnectChannelCommand) domain.Result[ConnectChannelResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED,
			err.Error(),
			nil,
			domain.SeverityLOW,
			false,
		))
	}

	// ② Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, cmd.BrandID)
	if err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND,
			"Brand introuvable",
			map[string]interface{}{"brand_id": cmd.BrandID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// Brand archivée bloque la connexion (Phase 6 §3.2 note ⁵ — BR-10 implicite)
	// ANOMALIE A030-note : Phase 6 utilise ErrCodeBRAND_ARCHIVED, T1a n'a que ErrCodeBRAND_ALREADY_ARCHIVED
	if brand.Status() == domain.BrandStatusArchived {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeBRAND_ALREADY_ARCHIVED,
			"La brand est archivée — impossible de connecter un channel",
			map[string]interface{}{"brand_id": cmd.BrandID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Charger l'acteur (WorkspaceMember)
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ActorID)
	if err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Acteur introuvable dans le workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ④ Vérification de permission — BR-10 : channel.connect
	// Phase 6 §3.2 : Owner bypass, sinon BrandOwner|BrandManager requis
	if !actor.Role.BypassesBrandAssignment() {
		assignment, err := uc.brandRepo.GetAssignment(ctx, cmd.BrandID, cmd.ActorID)
		if err != nil {
			return domain.Fail[ConnectChannelResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND,
				"Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": cmd.BrandID, "actor_id": cmd.ActorID},
				domain.SeverityMEDIUM,
				false,
			))
		}
		if assignment.Role != domain.BrandRoleOwner && assignment.Role != domain.BrandRoleManager {
			return domain.Fail[ConnectChannelResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED,
				"BrandRole insuffisant — BrandOwner ou BrandManager requis pour connecter un channel",
				map[string]interface{}{"actor_role": assignment.Role},
				domain.SeverityMEDIUM,
				false,
			))
		}
	}

	// ⑤ Déduplication — C-5 implicite : un même compte externe ne peut être connecté deux fois
	existing, err := uc.channelRepo.GetByPlatformAccountID(ctx, cmd.ExternalID)
	if err == nil && existing != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeCHANNEL_ALREADY_CONNECTED,
			"Ce compte de plateforme est déjà connecté",
			map[string]interface{}{
				"external_id":  cmd.ExternalID,
				"platform":     cmd.Platform,
				"channel_id":   existing.ID().String(),
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑥ Vérification quota plan (Phase 6 §5 : MAX_CHANNELS_REACHED)
	if err := uc.planChecker.CheckChannelLimit(ctx, brand.WorkspaceID(), cmd.BrandID); err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(domain.ErrCodeMAX_CHANNELS_REACHED, err.Error(), nil, domain.SeverityMEDIUM, false))
	}

	// ⑦ Chiffrement du token OAuth — Phase 2 §C-3 (AES-256-GCM)
	accessTokenEnc, err := uc.cryptoSvc.EncryptToken(ctx, cmd.AccessToken)
	if err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeINTERNAL_SERVER_ERROR,
			"Échec du chiffrement du token OAuth",
			nil,
			domain.SeverityHIGH,
			false,
		))
	}
	var refreshTokenEnc string
	if cmd.RefreshToken != "" {
		refreshTokenEnc, err = uc.cryptoSvc.EncryptToken(ctx, cmd.RefreshToken)
		if err != nil {
			return domain.Fail[ConnectChannelResult](domain.NewDomainError(
				domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Échec du chiffrement du refresh token",
				nil,
				domain.SeverityHIGH,
				false,
			))
		}
	}

	// ⑧ Création de l'aggregate Channel
	// ANOMALIE A029 : domain.NewChannel (T4) ne prend pas platformAccountID, platformAccountName
	// ni workspaceID. La factory NewConnectedChannel ci-dessous est le workaround.
	// Ces champs doivent être ajoutés à NewChannel ou via une factory dédiée dans domain/channel.go.
	channel, domErr := domain.NewConnectedChannel(
		cmd.BrandID,
		brand.WorkspaceID(),
		cmd.Platform,
		cmd.ExternalID,
		cmd.DisplayName,
	)
	if domErr != nil {
		return domain.Fail[ConnectChannelResult](domErr.(*domain.DomainError))
	}

	// ⑨ Persistance channel + credentials (deux writes atomiques sous transaction repo)
	if err := uc.channelRepo.Create(ctx, channel); err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la persistance du channel",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}
	if err := uc.channelCredRepo.Create(ctx, channel.ID(), accessTokenEnc, refreshTokenEnc, cmd.TokenExpiresAt); err != nil {
		return domain.Fail[ConnectChannelResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la persistance des credentials",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑩ Publication événement domaine — Phase 3 §BC04
	_ = uc.eventBus.Publish(ctx, events.NewChannelConnectedEvent(
		channel.ID(),          // channelID uuid.UUID
		brand.WorkspaceID(),   // workspaceID uuid.UUID
		cmd.BrandID,           // brandID uuid.UUID
		cmd.Platform,          // channelType string
		cmd.ExternalID,        // platformAccountID string
		channel.ConnectedAt(), // connectedAt time.Time
	))

	return domain.Ok[ConnectChannelResult](ConnectChannelResult{
		ChannelID:   channel.ID().String(),
		BrandID:     cmd.BrandID.String(),
		Platform:    cmd.Platform,
		DisplayName: cmd.DisplayName,
		ExternalID:  cmd.ExternalID,
		Status:      string(domain.ChannelStatusActive),
		ConnectedAt: channel.ConnectedAt(),
	})
}

// ============================================================
// DisconnectChannelUseCase
// ============================================================

type DisconnectChannelCommand struct {
	ChannelID uuid.UUID `validate:"required"`
	ActorID   uuid.UUID `validate:"required"`
	Reason    string    // optionnel — raison de la déconnexion
}

type DisconnectChannelResult struct {
	ChannelID      string    `json:"channel_id"`
	Status         string    `json:"status"`
	DisconnectedAt time.Time `json:"disconnected_at"`
	PostsCancelled int       `json:"posts_cancelled"` // A017 — nombre de posts scheduled → draft
}

type DisconnectChannelUseCase struct {
	channelRepo domain.IChannelRepo
	brandRepo   domain.IBrandRepo
	workspaceRepo domain.IWorkspaceRepo
	postRepo    domain.IPostRepo
	eventBus    IEventBusChannel
}

func NewDisconnectChannelUseCase(
	channelRepo domain.IChannelRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	postRepo domain.IPostRepo,
	eventBus IEventBusChannel,
) *DisconnectChannelUseCase {
	return &DisconnectChannelUseCase{
		channelRepo:   channelRepo,
		brandRepo:     brandRepo,
		workspaceRepo: workspaceRepo,
		postRepo:      postRepo,
		eventBus:      eventBus,
	}
}

func (uc *DisconnectChannelUseCase) Execute(ctx context.Context, cmd DisconnectChannelCommand) domain.Result[DisconnectChannelResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED,
			err.Error(),
			nil,
			domain.SeverityLOW,
			false,
		))
	}

	// ② Charger le channel
	channel, err := uc.channelRepo.GetByID(ctx, cmd.ChannelID)
	if err != nil {
		return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
			domain.ErrCodeCHANNEL_NOT_FOUND,
			"Channel introuvable",
			map[string]interface{}{"channel_id": cmd.ChannelID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Charger la brand associée
	brand, err := uc.brandRepo.GetByID(ctx, channel.BrandID())
	if err != nil {
		return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND,
			"Brand associée au channel introuvable",
			map[string]interface{}{"brand_id": channel.BrandID()},
			domain.SeverityHIGH,
			false,
		))
	}

	// ④ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ActorID)
	if err != nil {
		return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Acteur introuvable dans le workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑤ Vérification de permission — BR-11 : channel.disconnect
	if !actor.Role.BypassesBrandAssignment() {
		assignment, err := uc.brandRepo.GetAssignment(ctx, channel.BrandID(), cmd.ActorID)
		if err != nil {
			return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND,
				"Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": channel.BrandID(), "actor_id": cmd.ActorID},
				domain.SeverityMEDIUM,
				false,
			))
		}
		if assignment.Role != domain.BrandRoleOwner && assignment.Role != domain.BrandRoleManager {
			return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED,
				"BrandRole insuffisant — BrandOwner ou BrandManager requis pour déconnecter un channel",
				map[string]interface{}{"actor_role": assignment.Role},
				domain.SeverityMEDIUM,
				false,
			))
		}
	}

	// ⑥ Garde idempotence — channel déjà déconnecté
	if channel.Status() == domain.ChannelStatusDisconnected {
		return domain.Ok[DisconnectChannelResult](DisconnectChannelResult{
			ChannelID:      cmd.ChannelID.String(),
			Status:         string(domain.ChannelStatusDisconnected),
			DisconnectedAt: *channel.DisconnectedAt(),
			PostsCancelled: 0,
		})
	}

	// ⑦ Transition de statut — channel.SetStatus (T4)
	channel.SetStatus(domain.ChannelStatusDisconnected)

	// ⑧ A017 — Annuler les posts SCHEDULED pour cette brand → DRAFT
	// ANOMALIE A031 : IPostVariantRepo absent de T5. Seul IPostRepo disponible.
	// La cancellation se fait au niveau Post (scheduled→draft via Cancel()), pas variant.
	// Impact : les post_variants liées à d'autres channels non déconnectés sont aussi annulées.
	// Mitigation V1 : acceptable en V0 — DisconnectChannel implique revue manuelle des posts.
	scheduledFilter := domain.PostFilter{
		Status:  []domain.PostStatus{domain.PostStatusScheduled},
		BrandID: func() *uuid.UUID { id := channel.BrandID(); return &id }(),
	}
	scheduledPosts, _, err := uc.postRepo.ListByBrand(ctx, channel.BrandID(), scheduledFilter, domain.PageRequest{Page: 1, PageSize: 1000})
	postsCancelled := 0
	if err == nil {
		for _, post := range scheduledPosts {
			if cancelErr := post.Cancel(); cancelErr == nil {
				if updateErr := uc.postRepo.Update(ctx, post); updateErr == nil {
					postsCancelled++
				}
			}
		}
	}

	// ⑨ Persistance channel
	if err := uc.channelRepo.Update(ctx, channel); err != nil {
		return domain.Fail[DisconnectChannelResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la persistance de la déconnexion",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑩ Publication événement domaine — Phase 3 §BC04
	reason := cmd.Reason
	if reason == "" {
		reason = "manual_disconnect"
	}
	_ = uc.eventBus.Publish(ctx, events.NewChannelDisconnectedEvent(
		channel.ID(),           // channelID uuid.UUID
		channel.WorkspaceID(),  // workspaceID uuid.UUID
		channel.BrandID(),      // brandID uuid.UUID
		cmd.ActorID.String(),   // disconnectedByUserID string
		reason,                 // reason string
	))

	disconnectedAt := time.Now().UTC()
	if channel.DisconnectedAt() != nil {
		disconnectedAt = *channel.DisconnectedAt()
	}

	return domain.Ok[DisconnectChannelResult](DisconnectChannelResult{
		ChannelID:      cmd.ChannelID.String(),
		Status:         string(domain.ChannelStatusDisconnected),
		DisconnectedAt: disconnectedAt,
		PostsCancelled: postsCancelled,
	})
}
