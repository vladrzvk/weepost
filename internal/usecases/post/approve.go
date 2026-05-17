package post

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// ============================================================
// RequestApprovalUseCase — SM-01 Phase 5 : soumettre un post à approbation
// ============================================================

// RequestApprovalCommand — CH-05 approval.request : Editor+ avec brand assignment.
// Seul un post au statut "validated" peut être soumis à approbation.
type RequestApprovalCommand struct {
	PostID  uuid.UUID `validate:"required"`
	ActorID uuid.UUID `validate:"required"`
}

type RequestApprovalResult struct {
	ApprovalRequestID string    `json:"approval_request_id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

type RequestApprovalUseCase struct {
	postRepo      domain.IPostRepo
	brandRepo     domain.IBrandRepo
	workspaceRepo domain.IWorkspaceRepo
	approvalRepo  domain.IApprovalRequestRepo
	eventBus      IEventBusPost
}

func NewRequestApprovalUseCase(
	postRepo domain.IPostRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	approvalRepo domain.IApprovalRequestRepo,
	eventBus IEventBusPost,
) *RequestApprovalUseCase {
	return &RequestApprovalUseCase{
		postRepo:      postRepo,
		brandRepo:     brandRepo,
		workspaceRepo: workspaceRepo,
		approvalRepo:  approvalRepo,
		eventBus:      eventBus,
	}
}

func (uc *RequestApprovalUseCase) Execute(ctx context.Context, cmd RequestApprovalCommand) domain.Result[RequestApprovalResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
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
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND,
			"Post introuvable",
			map[string]interface{}{"post_id": cmd.PostID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Vérifier statut = validated — SM-01 : seul un post validé peut être soumis
	if post.Status() != domain.PostStatusValidated {
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Seul un post validé peut être soumis à approbation",
			map[string]interface{}{
				"post_id":        cmd.PostID,
				"current_status": string(post.Status()),
				"required":       "validated",
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ④ Charger la brand pour obtenir workspaceID
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
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
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Acteur introuvable dans le workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑥ Vérification permission — Editor+ avec brand assignment ; BrandRoleViewer interdit
	if !actor.Role.BypassesBrandAssignment() {
		assignment, err := uc.brandRepo.GetAssignment(ctx, post.BrandID(), cmd.ActorID)
		if err != nil {
			return domain.Fail[RequestApprovalResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND,
				"Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": post.BrandID(), "actor_id": cmd.ActorID},
				domain.SeverityMEDIUM,
				false,
			))
		}
		if assignment.Role == domain.BrandRoleViewer {
			return domain.Fail[RequestApprovalResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED,
				"BrandRole insuffisant — BrandEditor minimum requis pour soumettre à approbation",
				map[string]interface{}{"actor_role": assignment.Role},
				domain.SeverityMEDIUM,
				false,
			))
		}
	}

	// ⑦ Vérifier qu'aucune approbation en attente n'existe déjà
	existing, err := uc.approvalRepo.ListByPost(ctx, cmd.PostID)
	if err == nil {
		for _, ar := range existing {
			if ar.Status == "pending" {
				return domain.Fail[RequestApprovalResult](domain.NewDomainError(
					domain.ErrCodeINVALID_INPUT,
					"Approbation déjà en attente",
					map[string]interface{}{
						"post_id":             cmd.PostID,
						"approval_request_id": ar.ID,
					},
					domain.SeverityLOW,
					false,
				))
			}
		}
	}

	// ⑧ Créer l'ApprovalRequest
	now := time.Now().UTC()
	ar := &domain.ApprovalRequest{
		ID:                  uuid.New(),
		PostID:              cmd.PostID,
		Type:                "internal",
		RequestedByMemberID: actor.ID,
		Status:              "pending",
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// ⑨ Persister
	if err := uc.approvalRepo.Create(ctx, ar); err != nil {
		return domain.Fail[RequestApprovalResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la création de la demande d'approbation",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑩ Publier événement domaine
	_ = uc.eventBus.Publish(ctx, events.NewApprovalRequestedEvent(
		ar.ID,
		brand.WorkspaceID(),
		post.ID(),
		post.BrandID(),
		cmd.ActorID.String(),
	))

	return domain.Ok[RequestApprovalResult](RequestApprovalResult{
		ApprovalRequestID: ar.ID.String(),
		Status:            ar.Status,
		CreatedAt:         ar.CreatedAt,
	})
}

// ============================================================
// ApprovePostUseCase — SM-01 Phase 5 : accorder l'approbation
// ============================================================

// ApprovePostCommand — Admin ou Owner du workspace requis (V0).
type ApprovePostCommand struct {
	ApprovalRequestID uuid.UUID `validate:"required"`
	ReviewerID        uuid.UUID `validate:"required"`
}

type ApprovePostResult struct {
	ApprovalRequestID string    `json:"approval_request_id"`
	PostID            string    `json:"post_id"`
	Status            string    `json:"status"`
	ReviewedAt        time.Time `json:"reviewed_at"`
}

type ApprovePostUseCase struct {
	approvalRepo  domain.IApprovalRequestRepo
	postRepo      domain.IPostRepo
	brandRepo     domain.IBrandRepo
	workspaceRepo domain.IWorkspaceRepo
	eventBus      IEventBusPost
}

func NewApprovePostUseCase(
	approvalRepo domain.IApprovalRequestRepo,
	postRepo domain.IPostRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	eventBus IEventBusPost,
) *ApprovePostUseCase {
	return &ApprovePostUseCase{
		approvalRepo:  approvalRepo,
		postRepo:      postRepo,
		brandRepo:     brandRepo,
		workspaceRepo: workspaceRepo,
		eventBus:      eventBus,
	}
}

func (uc *ApprovePostUseCase) Execute(ctx context.Context, cmd ApprovePostCommand) domain.Result[ApprovePostResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED,
			err.Error(),
			nil,
			domain.SeverityLOW,
			false,
		))
	}

	// ② Charger la demande d'approbation
	ar, err := uc.approvalRepo.GetByID(ctx, cmd.ApprovalRequestID)
	if err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeAPPROVAL_NOT_FOUND,
			"Demande d'approbation introuvable",
			map[string]interface{}{"approval_request_id": cmd.ApprovalRequestID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Idempotence : si déjà approuvée, retourner l'état courant
	if ar.Status == "approved" {
		reviewedAt := ar.UpdatedAt
		if ar.ReviewedAt != nil {
			reviewedAt = *ar.ReviewedAt
		}
		return domain.Ok[ApprovePostResult](ApprovePostResult{
			ApprovalRequestID: ar.ID.String(),
			PostID:            ar.PostID.String(),
			Status:            ar.Status,
			ReviewedAt:        reviewedAt,
		})
	}

	// ④ Vérifier statut == pending
	if ar.Status != "pending" {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Seule une demande en attente peut être approuvée",
			map[string]interface{}{
				"approval_request_id": cmd.ApprovalRequestID,
				"current_status":      ar.Status,
				"required":            "pending",
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑤ Charger le post pour obtenir brandID, puis la brand pour workspaceID
	post, err := uc.postRepo.GetByID(ctx, ar.PostID)
	if err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND,
			"Post de la demande introuvable",
			map[string]interface{}{"post_id": ar.PostID},
			domain.SeverityHIGH,
			false,
		))
	}
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND,
			"Brand de la demande introuvable",
			map[string]interface{}{"brand_id": post.BrandID()},
			domain.SeverityHIGH,
			false,
		))
	}

	// ⑥ Charger l'acteur et vérifier rôle Admin ou Owner
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ReviewerID)
	if err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Réviseur introuvable dans le workspace",
			map[string]interface{}{"reviewer_id": cmd.ReviewerID},
			domain.SeverityMEDIUM,
			false,
		))
	}
	if actor.Role != domain.MemberRoleAdmin && actor.Role != domain.MemberRoleOwner {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodePERMISSION_DENIED,
			"Admin ou Owner du workspace requis pour approuver un post",
			map[string]interface{}{
				"reviewer_id":   cmd.ReviewerID,
				"reviewer_role": actor.Role,
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑦ Mettre à jour la demande
	now := time.Now().UTC()
	ar.Status = "approved"
	ar.ApproverMemberID = &actor.ID
	ar.ReviewedAt = &now
	ar.UpdatedAt = now

	// ⑧ Persister
	if err := uc.approvalRepo.Update(ctx, ar); err != nil {
		return domain.Fail[ApprovePostResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la mise à jour de la demande d'approbation",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑨ Publier événement domaine
	_ = uc.eventBus.Publish(ctx, events.NewApprovalGrantedEvent(
		ar.ID,
		brand.WorkspaceID(),
		ar.PostID,
		post.BrandID(),
		cmd.ReviewerID.String(),
	))

	return domain.Ok[ApprovePostResult](ApprovePostResult{
		ApprovalRequestID: ar.ID.String(),
		PostID:            ar.PostID.String(),
		Status:            ar.Status,
		ReviewedAt:        now,
	})
}

// ============================================================
// RejectPostUseCase — SM-01 Phase 5 : rejeter la demande d'approbation
// ============================================================

// RejectPostCommand — Admin ou Owner du workspace requis (V0).
// Reason obligatoire (min=1, max=500).
type RejectPostCommand struct {
	PostID            uuid.UUID `json:"post_id"`
	ActorID           uuid.UUID `json:"actor_id"`
	ApprovalRequestID uuid.UUID `validate:"required"`
	ReviewerID        uuid.UUID `validate:"required"`
	Reason            string    `validate:"required,min=1,max=500"`
}

type RejectPostResult struct {
	ApprovalRequestID string    `json:"approval_request_id"`
	PostID            string    `json:"post_id"`
	Status            string    `json:"status"`
	ReviewedAt        time.Time `json:"reviewed_at"`
}

type RejectPostUseCase struct {
	approvalRepo  domain.IApprovalRequestRepo
	postRepo      domain.IPostRepo
	brandRepo     domain.IBrandRepo
	workspaceRepo domain.IWorkspaceRepo
	eventBus      IEventBusPost
}

func NewRejectPostUseCase(
	approvalRepo domain.IApprovalRequestRepo,
	postRepo domain.IPostRepo,
	brandRepo domain.IBrandRepo,
	workspaceRepo domain.IWorkspaceRepo,
	eventBus IEventBusPost,
) *RejectPostUseCase {
	return &RejectPostUseCase{
		approvalRepo:  approvalRepo,
		postRepo:      postRepo,
		brandRepo:     brandRepo,
		workspaceRepo: workspaceRepo,
		eventBus:      eventBus,
	}
}

func (uc *RejectPostUseCase) Execute(ctx context.Context, cmd RejectPostCommand) domain.Result[RejectPostResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeVALIDATION_FAILED,
			err.Error(),
			nil,
			domain.SeverityLOW,
			false,
		))
	}

	// ② Charger la demande d'approbation
	ar, err := uc.approvalRepo.GetByID(ctx, cmd.ApprovalRequestID)
	if err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeAPPROVAL_NOT_FOUND,
			"Demande d'approbation introuvable",
			map[string]interface{}{"approval_request_id": cmd.ApprovalRequestID},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ③ Idempotence : si déjà rejetée, retourner l'état courant
	if ar.Status == "rejected" {
		reviewedAt := ar.UpdatedAt
		if ar.ReviewedAt != nil {
			reviewedAt = *ar.ReviewedAt
		}
		return domain.Ok[RejectPostResult](RejectPostResult{
			ApprovalRequestID: ar.ID.String(),
			PostID:            ar.PostID.String(),
			Status:            ar.Status,
			ReviewedAt:        reviewedAt,
		})
	}

	// ④ Vérifier statut == pending
	if ar.Status != "pending" {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeINVALID_STATUS_TRANSITION,
			"Seule une demande en attente peut être rejetée",
			map[string]interface{}{
				"approval_request_id": cmd.ApprovalRequestID,
				"current_status":      ar.Status,
				"required":            "pending",
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑤ Charger le post pour obtenir brandID, puis la brand pour workspaceID
	post, err := uc.postRepo.GetByID(ctx, ar.PostID)
	if err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodePOST_NOT_FOUND,
			"Post de la demande introuvable",
			map[string]interface{}{"post_id": ar.PostID},
			domain.SeverityHIGH,
			false,
		))
	}
	brand, err := uc.brandRepo.GetByID(ctx, post.BrandID())
	if err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeBRAND_NOT_FOUND,
			"Brand de la demande introuvable",
			map[string]interface{}{"brand_id": post.BrandID()},
			domain.SeverityHIGH,
			false,
		))
	}

	// ⑥ Charger l'acteur et vérifier rôle Admin ou Owner
	actor, err := uc.workspaceRepo.GetMember(ctx, brand.WorkspaceID(), cmd.ReviewerID)
	if err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND,
			"Réviseur introuvable dans le workspace",
			map[string]interface{}{"reviewer_id": cmd.ReviewerID},
			domain.SeverityMEDIUM,
			false,
		))
	}
	if actor.Role != domain.MemberRoleAdmin && actor.Role != domain.MemberRoleOwner {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodePERMISSION_DENIED,
			"Admin ou Owner du workspace requis pour rejeter un post",
			map[string]interface{}{
				"reviewer_id":   cmd.ReviewerID,
				"reviewer_role": actor.Role,
			},
			domain.SeverityMEDIUM,
			false,
		))
	}

	// ⑦ Mettre à jour la demande
	now := time.Now().UTC()
	ar.Status = "rejected"
	ar.ApproverMemberID = &actor.ID
	ar.ReviewedAt = &now
	ar.RejectionReason = &cmd.Reason
	ar.UpdatedAt = now

	// ⑧ Persister
	if err := uc.approvalRepo.Update(ctx, ar); err != nil {
		return domain.Fail[RejectPostResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"Échec de la mise à jour de la demande d'approbation",
			nil,
			domain.SeverityHIGH,
			true,
		))
	}

	// ⑨ Publier événement domaine
	_ = uc.eventBus.Publish(ctx, events.NewApprovalRejectedEvent(
		ar.ID,
		brand.WorkspaceID(),
		ar.PostID,
		post.BrandID(),
		cmd.ReviewerID.String(),
		cmd.Reason,
	))

	return domain.Ok[RejectPostResult](RejectPostResult{
		ApprovalRequestID: ar.ID.String(),
		PostID:            ar.PostID.String(),
		Status:            ar.Status,
		ReviewedAt:        now,
	})
}
