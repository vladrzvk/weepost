package brand

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// validate et slugRegex déclarés dans brand/brand.go (même package).

// ─────────────────────────────────────────────────────────
// AssignMemberToBrandUseCase
// ─────────────────────────────────────────────────────────

type AssignMemberToBrandCommand struct {
	ActorID     uuid.UUID `validate:"required"`
	WorkspaceID uuid.UUID `validate:"required"`
	BrandID     uuid.UUID `validate:"required"`
	MemberID    uuid.UUID `validate:"required"`
	// Role — A025 : valeurs correctes = owner/manager/editor/viewer (pas brand_owner…)
	Role string `validate:"required,oneof=owner manager editor viewer"`
}

type AssignMemberToBrandResult struct {
	AssignmentID string `json:"assignment_id"`
	BrandID      string `json:"brand_id"`
	MemberID     string `json:"member_id"`
	Role         string `json:"role"`
}

type AssignMemberToBrandUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
	eventBus      domain.IEventBus
}

func NewAssignMemberToBrandUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	brandRepo domain.IBrandRepo,
	eventBus domain.IEventBus,
) *AssignMemberToBrandUseCase {
	return &AssignMemberToBrandUseCase{
		workspaceRepo: workspaceRepo,
		brandRepo:     brandRepo,
		eventBus:      eventBus,
	}
}

func (uc *AssignMemberToBrandUseCase) Execute(ctx context.Context, cmd AssignMemberToBrandCommand) domain.Result[AssignMemberToBrandResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, cmd.BrandID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Vérifier appartenance au workspace
	if brand.WorkspaceID() != cmd.WorkspaceID {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found in this workspace", nil, domain.SeverityMEDIUM, false))
	}

	// BR-06 note ⁵ : brand ARCHIVED bloque l'assignation, même pour Owner
	if brand.Status() == domain.BrandStatusArchived {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_ARCHIVED, "cannot assign members to an archived brand", nil, domain.SeverityMEDIUM, false))
	}

	// ③ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "actor is not a workspace member", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// BR-06 permission brand.assignment.create
	// B-5 + A003 : BypassesBrandAssignment() = true uniquement pour MemberRoleOwner
	if !actor.Role.BypassesBrandAssignment() {
		actorAssignment, assignErr := uc.brandRepo.GetAssignment(ctx, cmd.BrandID, actor.ID)
		if assignErr != nil {
			return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "no brand assignment found for actor", nil, domain.SeverityMEDIUM, false))
		}
		if actorAssignment.Role != domain.BrandRoleOwner {
			return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeNOT_BRAND_OWNER, "brand owner role required to assign members", nil, domain.SeverityMEDIUM, false))
		}
	}

	// Vérifier que la cible est bien membre du workspace
	_, err = uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.MemberID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeMEMBER_NOT_FOUND, "target member not found in workspace", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Vérifier qu'une assignation n'existe pas déjà
	_, existingErr := uc.brandRepo.GetAssignment(ctx, cmd.BrandID, cmd.MemberID)
	if existingErr == nil {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_ASSIGNMENT_ALREADY_EXISTS, "member already has a brand assignment", nil, domain.SeverityMEDIUM, false))
	}

	// ④ Construire l'assignation
	assignmentUUID := uuid.New()
	actorID := cmd.ActorID
	assignment := &domain.BrandAssignment{
		ID:                 assignmentUUID,
		BrandID:            cmd.BrandID,
		MemberID:           cmd.MemberID,
		Role:               domain.BrandRole(cmd.Role),
		AssignedByMemberID: &actorID,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	// Notifier l'agrégat domain (T3 Brand.AssignMember enregistre l'invariant)
	if err := brand.AssignMember(cmd.MemberID, domain.BrandRole(cmd.Role), cmd.ActorID); err != nil {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑤ Persister
	if err := uc.brandRepo.AddAssignment(ctx, assignment); err != nil {
		return domain.Fail[AssignMemberToBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement
	_ = uc.eventBus.Publish(ctx, events.NewBrandMemberAssignedEvent(
		assignmentUUID,
		cmd.BrandID,
		cmd.WorkspaceID,
		cmd.MemberID.String(),
		cmd.Role,
		cmd.ActorID.String(),
	))

	// ⑦ DTO
	return domain.Ok(AssignMemberToBrandResult{
		AssignmentID: assignmentUUID.String(),
		BrandID:      cmd.BrandID.String(),
		MemberID:     cmd.MemberID.String(),
		Role:         cmd.Role,
	})
}

// ─────────────────────────────────────────────────────────
// RevokeBrandAccessUseCase
// ─────────────────────────────────────────────────────────

type RevokeBrandAccessCommand struct {
	ActorID     uuid.UUID `validate:"required"`
	WorkspaceID uuid.UUID `validate:"required"`
	BrandID     uuid.UUID `validate:"required"`
	MemberID    uuid.UUID `validate:"required"`
}

type RevokeBrandAccessResult struct {
	BrandID  string `json:"brand_id"`
	MemberID string `json:"member_id"`
}

type RevokeBrandAccessUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
	eventBus      domain.IEventBus
}

func NewRevokeBrandAccessUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	brandRepo domain.IBrandRepo,
	eventBus domain.IEventBus,
) *RevokeBrandAccessUseCase {
	return &RevokeBrandAccessUseCase{
		workspaceRepo: workspaceRepo,
		brandRepo:     brandRepo,
		eventBus:      eventBus,
	}
}

func (uc *RevokeBrandAccessUseCase) Execute(ctx context.Context, cmd RevokeBrandAccessCommand) domain.Result[RevokeBrandAccessResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, cmd.BrandID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	if brand.WorkspaceID() != cmd.WorkspaceID {
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found in this workspace", nil, domain.SeverityMEDIUM, false))
	}

	// note ⁵ : brand ARCHIVED bloque la révocation (même politique que l'assignation)
	if brand.Status() == domain.BrandStatusArchived {
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeBRAND_ARCHIVED, "cannot revoke access on an archived brand", nil, domain.SeverityMEDIUM, false))
	}

	// ③ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "actor is not a workspace member", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Permission : même règle que brand.assignment.create (BR-06 / B-5 / A003)
	if !actor.Role.BypassesBrandAssignment() {
		actorAssignment, assignErr := uc.brandRepo.GetAssignment(ctx, cmd.BrandID, actor.ID)
		if assignErr != nil {
			return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "no brand assignment found for actor", nil, domain.SeverityMEDIUM, false))
		}
		if actorAssignment.Role != domain.BrandRoleOwner {
			return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeNOT_BRAND_OWNER, "brand owner role required to revoke access", nil, domain.SeverityMEDIUM, false))
		}
	}

	// Vérifier que l'assignation cible existe
	_, err = uc.brandRepo.GetAssignment(ctx, cmd.BrandID, cmd.MemberID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeBRAND_ASSIGNMENT_NOT_FOUND, "brand assignment not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ④ Notifier l'agrégat domain
	if err := brand.RemoveMember(cmd.MemberID, cmd.ActorID); err != nil {
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑤ Persister
	if err := uc.brandRepo.RemoveAssignment(ctx, cmd.BrandID, cmd.MemberID); err != nil {
		return domain.Fail[RevokeBrandAccessResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement — P14 (AUD2-035) : construction inline remplacée par events.NewBrandMemberRevokedEvent.
	// ANOMALIE P14 : RevokeBrandAccessCommand n'a pas de champ Reason.
	// Passer "" en attendant l'ajout du champ Reason à la commande (correctif P15 ou futur).
	revokedEvent := events.NewBrandMemberRevokedEvent(
		cmd.BrandID,         // aggregateID = brandID
		brand.WorkspaceID(), // workspaceID depuis l'agrégat chargé
		cmd.MemberID,
		cmd.ActorID,
		"", // reason — ANOMALIE P14 : absent de RevokeBrandAccessCommand
		time.Now().UTC(),
	)
	_ = uc.eventBus.Publish(ctx, revokedEvent)

	// ⑦ DTO
	return domain.Ok(RevokeBrandAccessResult{
		BrandID:  cmd.BrandID.String(),
		MemberID: cmd.MemberID.String(),
	})
}
