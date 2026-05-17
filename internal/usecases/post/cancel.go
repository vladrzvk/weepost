package post

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// ============================================================
// PostDTO — type de retour partagé dans le package post
// ============================================================

type PostDTO struct {
	PostID      string     `json:"post_id"`
	BrandID     string     `json:"brand_id"`
	Title       string     `json:"title,omitempty"`
	Status      string     `json:"status"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toPostDTO(p *domain.Post) PostDTO {
	dto := PostDTO{
		PostID:    p.ID().String(),
		BrandID:   p.BrandID().String(),
		Status:    string(p.Status()),
		UpdatedAt: p.UpdatedAt(),
	}
	if p.Title() != nil {
		dto.Title = *p.Title()
	}
	if p.ScheduledAt() != nil {
		dto.ScheduledAt = p.ScheduledAt()
	}
	return dto
}

// ============================================================
// CancelScheduledPostUseCase — pattern canonique post-S007
// ============================================================

// CancelScheduledPostCommand — SM-01 : scheduled → draft.
// Phase 6 §TX-01 (pattern) : auteur du post OU Manager+ avec brand assignment.
type CancelScheduledPostCommand struct {
	PostID  uuid.UUID `validate:"required"`
	ActorID uuid.UUID `validate:"required"`
}

type CancelScheduledPostUseCase struct {
	postRepo      domain.IPostRepo
	brandRepo     domain.IBrandRepo
	workspaceRepo domain.IWorkspaceRepo
	eventBus      IEventBusPost
}

func NewCancelScheduledPostUseCase(
	postRepo domain.IPostRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	eventBus IEventBusPost,
) *CancelScheduledPostUseCase {
	return &CancelScheduledPostUseCase{
		postRepo:      postRepo,
		brandRepo:     brandRepo,
		workspaceRepo: workspaceRepo,
		eventBus:      eventBus,
	}
}

// Execute — pattern canonique post-S007 (domain.Fail/Ok strict, helper checkPermission).
func (uc *CancelScheduledPostUseCase) Execute(
	ctx context.Context,
	cmd CancelScheduledPostCommand,
) domain.Result[PostDTO] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[PostDTO](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Charger le post — T5 GetByID (mission utilise FindByID : ANOMALIE A035-note)
	post, err := uc.postRepo.GetByID(ctx, cmd.PostID)
	if err != nil {
		return domain.Fail[PostDTO](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND, "Post introuvable",
			map[string]interface{}{"post_id": cmd.PostID}, domain.SeverityMEDIUM, false,
		))
	}

	// ③ Vérification de permission — helper checkPermission
	if err := uc.checkPermission(ctx, cmd.ActorID, post); err != nil {
		return domain.Fail[PostDTO](domain.NewDomainError(
			domain.ErrCodePERMISSION_DENIED,
			"Action non autorisée",
			map[string]interface{}{
				"actor_id": cmd.ActorID,
				"post_id":  cmd.PostID,
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ④ Transition scheduled → draft — T4 post.Cancel()
	if err := post.Cancel(); err != nil {
		return domain.Fail[PostDTO](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Cette transition d'état n'est pas autorisée",
			map[string]interface{}{
				"post_id":        cmd.PostID,
				"current_status": string(post.Status()),
				"target_status":  "draft",
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑤ Persistance
	if err := uc.postRepo.Update(ctx, post); err != nil {
		return domain.Fail[PostDTO](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "Échec de la persistance de l'annulation",
			nil, domain.SeverityHIGH, true,
		))
	}

	// ⑥ Publication événement domaine
	// ANOMALIE A035 : NewPostCancelledEvent absent de T6 constructors.go.
	// Workaround : NewPostStatusChangedEvent (même pattern que T14/T15).
	var wsID uuid.UUID
	if brand, bErr := uc.brandRepo.GetByID(ctx, post.BrandID()); bErr == nil && brand != nil {
		wsID = brand.WorkspaceID()
	}
	_ = uc.eventBus.Publish(ctx, events.NewPostStatusChangedEvent(
		post.ID(),
		wsID,
		post.BrandID(),
		string(domain.PostStatusScheduled),
		string(domain.PostStatusDraft),
	))

	return domain.Ok[PostDTO](toPostDTO(post))
}

// checkPermission — auteur du post OU Manager+ avec brand assignment.
// Phase 6 pattern : TX-01 (post.view_draft) — auteur ou Manager+.
// Owner bypass via BypassesBrandAssignment().
func (uc *CancelScheduledPostUseCase) checkPermission(ctx context.Context, actorID uuid.UUID, post *domain.Post) error {
	// Auteur du post peut toujours annuler sa propre planification
	if post.CreatedByUserID() == actorID {
		return nil
	}

	// Charger la brand pour obtenir le workspaceID
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND, "Brand introuvable",
			map[string]interface{}{"brand_id": post.BrandID()}, domain.SeverityHIGH, false,
		)
	}

	// Charger le membre workspace
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), actorID)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND, "Acteur introuvable",
			map[string]interface{}{"actor_id": actorID}, domain.SeverityMEDIUM, false,
		)
	}

	// Owner bypass
	if actor.Role.BypassesBrandAssignment() {
		return nil
	}

	// Manager+ avec brand assignment peut annuler n'importe quel post planifié
	assignment, err := uc.brandRepo.GetAssignment(ctx, post.BrandID(), actorID)
	if err != nil {
		return domain.NewDomainError(
			domain.ErrCodeNO_ASSIGNMENT_TO_BRAND, "Aucune affectation à cette brand",
			map[string]interface{}{"brand_id": post.BrandID()}, domain.SeverityMEDIUM, false,
		)
	}
	if assignment.Role != domain.BrandRoleOwner && assignment.Role != domain.BrandRoleManager {
		return domain.NewDomainError(
			domain.ErrCodeBRAND_ACCESS_DENIED,
			"BrandManager minimum requis pour annuler le post d'un autre membre",
			map[string]interface{}{"actor_role": assignment.Role}, domain.SeverityMEDIUM, false,
		)
	}
	return nil
}

// IEventBusPost — port local cancel/post (évite re-déclaration dans le package post).
type IEventBusPost interface {
	Publish(ctx context.Context, events ...domain.DomainEvent) error
}
