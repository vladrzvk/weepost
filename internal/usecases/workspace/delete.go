// internal/usecases/workspace/delete.go
package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// DeleteWorkspaceCommand — WS-C-003.
// ActorID injecté depuis JWT.
// Confirm doit être true — protection contre suppression accidentelle.
type DeleteWorkspaceCommand struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ActorID     uuid.UUID `json:"-"`
	Confirm     bool      `json:"confirm" validate:"required"`
}

// DeleteWorkspaceUseCase — WS-C-003.
// Transition SM-02 : active → pending_deletion (pas directement → deleted).
// La transition pending_deletion → deleted est gérée par WorkspaceDeletionSaga (CRON J+7).
type DeleteWorkspaceUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
	sessionRepo   domain.ISessionRepo
	eventBus      domain.IEventBus
}

func NewDeleteWorkspaceUseCase(
	wsRepo      domain.IWorkspaceRepo,
	brandRepo   domain.IBrandRepo,
	sessionRepo domain.ISessionRepo,
	eventBus    domain.IEventBus,
) *DeleteWorkspaceUseCase {
	return &DeleteWorkspaceUseCase{
		workspaceRepo: wsRepo,
		brandRepo:     brandRepo,
		sessionRepo:   sessionRepo,
		eventBus:      eventBus,
	}
}

func (uc *DeleteWorkspaceUseCase) Execute(
	ctx context.Context,
	cmd DeleteWorkspaceCommand,
) domain.Result[struct{}] {

	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Vérification confirmation explicite (garde anti-accidentelle)
	if !cmd.Confirm {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT,
			"la suppression doit être explicitement confirmée",
			map[string]interface{}{"workspace_id": cmd.WorkspaceID},
			domain.SeverityMEDIUM, false,
		))
	}

	// ③ Charger le workspace
	ws, err := uc.workspaceRepo.GetByID(ctx, cmd.WorkspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Fail[struct{}](domain.NewDomainError(
				domain.ErrCodeNOT_FOUND, "workspace not found",
				map[string]interface{}{"workspace_id": cmd.WorkspaceID},
				domain.SeverityMEDIUM, false,
			))
		}
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to load workspace",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ④ Vérification acteur = Owner (W-5 — NOT_WORKSPACE_OWNER)
	member, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil || member == nil {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeNOT_WORKSPACE_OWNER,
			"seul l'Owner peut supprimer un workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityHIGH, false,
		))
	}
	if member.Role != domain.MemberRoleOwner {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeNOT_WORKSPACE_OWNER,
			"seul l'Owner peut supprimer un workspace",
			map[string]interface{}{
				"actor_id":   cmd.ActorID,
				"actor_role": member.Role,
			},
			domain.SeverityHIGH, false,
		))
	}

	// ⑤ Transition SM-02 : active → pending_deletion (ANOMALIE A023 : MarkPendingDeletion() absent de T3)
	// T3 doit exposer MarkPendingDeletion() error qui transite active → pending_deletion.
	if err := ws.Delete(cmd.ActorID); err != nil {
		return domain.Fail[struct{}](err.(*domain.DomainError))
	}

	// ⑥ Cascade immédiate (sécurité — accès bloqué dès pending_deletion)
	// Étape 6a : Révoquer toutes les sessions des membres (ANOMALIE A024 : RevokeAllByUserID, pas RevokeAllForMember)
	members, err := uc.workspaceRepo.ListMembers(ctx, cmd.WorkspaceID)
	if err != nil {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to list members for session revocation",
			nil, domain.SeverityCRITICAL, true,
		))
	}
	for _, m := range members {
		if revokeErr := uc.sessionRepo.RevokeAllByUserID(ctx, m.UserID); revokeErr != nil {
			// Log mais non bloquant — l'expiration naturelle des sessions prend le relais
			_ = revokeErr
		}
	}

	// Étape 6b : Archiver toutes les brands (X-1 cascade — brands → archived)
	brands, err := uc.brandRepo.ListByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to list brands for archival",
			nil, domain.SeverityCRITICAL, true,
		))
	}
	for _, b := range brands {
		if err := b.Archive(cmd.ActorID); err == nil {
			_ = uc.brandRepo.Update(ctx, b)
		}
	}

	// ⑦ Persister l'état pending_deletion
	if err := uc.workspaceRepo.Update(ctx, ws); err != nil {
		return domain.Fail[struct{}](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to update workspace status",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ⑧ Événement domaine (T6 — workspace.deleted émis à l'initiation, pas à la suppression physique)
	event := events.NewWorkspaceDeletedEvent(
		cmd.WorkspaceID,
		cmd.ActorID.String(),
		time.Now().UTC(),
	)
	_ = uc.eventBus.Publish(ctx, event)

	return domain.Ok(struct{}{})
}
