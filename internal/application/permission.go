package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
)

// PermissionResult — résultat du check de permission Phase 6.
// BillingWarning : subscription past_due → accès maintenu, header X-Billing-Warning émis.
type PermissionResult struct {
	Allowed        bool
	BillingWarning bool
}

// PermissionChecker — service applicatif Phase 6 §1.
// Vérifie qu'un acteur a l'action autorisée sur la resource cible.
// Règles : Phase 6 permission matrix + subscription status.
type PermissionChecker struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
}

// NewPermissionChecker construit le checker avec ses dépendances.
func NewPermissionChecker(
	workspaceRepo domain.IWorkspaceRepo,
	brandRepo domain.IBrandRepo,
) *PermissionChecker {
	return &PermissionChecker{
		workspaceRepo: workspaceRepo,
		brandRepo:     brandRepo,
	}
}

// CheckPermission vérifie la permission d'un acteur sur une resource.
// Retourne domain.Result[PermissionResult] — IsFail() si accès refusé.
func (pc *PermissionChecker) CheckPermission(
	ctx context.Context,
	userID uuid.UUID,
	action domain.Action,
	resource domain.Resource,
) domain.Result[PermissionResult] {
	// ① Charger le workspace
	ws, err := pc.workspaceRepo.GetByID(ctx, resource.WorkspaceID)
	if err != nil || ws == nil {
		return domain.Fail[PermissionResult](domain.NewDomainError(
			domain.ErrCodeWORKSPACE_NOT_FOUND, "Workspace introuvable",
			map[string]interface{}{"workspace_id": resource.WorkspaceID},
			domain.SeverityMEDIUM, false,
		))
	}

	// ② Workspace suspendu → accès refusé sauf billing.read
	if ws.Status() == domain.WorkspaceStatusSuspended && string(action) != "workspace.billing.read" {
		return domain.Fail[PermissionResult](domain.NewDomainError(
			domain.ErrCodeWORKSPACE_SUSPENDED, "Workspace suspendu",
			nil, domain.SeverityMEDIUM, false,
		))
	}

	// ③ Charger le membre
	member, err := pc.workspaceRepo.GetMember(ctx, resource.WorkspaceID, userID)
	if err != nil || member == nil {
		return domain.Fail[PermissionResult](domain.NewDomainError(
			domain.ErrCodeMEMBER_NOT_FOUND, "Membre introuvable dans ce workspace",
			map[string]interface{}{"user_id": userID},
			domain.SeverityMEDIUM, false,
		))
	}

	// ④ Membre inactif → accès refusé
	if member.Status != domain.MemberStatusActive {
		return domain.Fail[PermissionResult](domain.NewDomainError(
			domain.ErrCodePERMISSION_DENIED, "Membre inactif",
			nil, domain.SeverityMEDIUM, false,
		))
	}

	// ⑤ Owner bypass — accès total
	if member.Role == domain.MemberRoleOwner {
		return domain.Ok[PermissionResult](PermissionResult{Allowed: true})
	}

	// ⑥ Admin bypass — accès total sauf ownership.transfer
	if member.Role == domain.MemberRoleAdmin && string(action) != "ownership.transfer" {
		return domain.Ok[PermissionResult](PermissionResult{Allowed: true})
	}

	// ⑦ Pour les actions brand-scoped : vérifier assignment + rôle brand
	if resource.Type == domain.ResourceTypeBrand && resource.BrandID != uuid.Nil {
		assignment, err := pc.brandRepo.GetAssignment(ctx, resource.BrandID, member.ID)
		if err != nil || assignment == nil {
			return domain.Fail[PermissionResult](domain.NewDomainError(
				domain.ErrCodeNO_ASSIGNMENT_TO_BRAND, "Aucune affectation à cette brand",
				map[string]interface{}{"brand_id": resource.BrandID},
				domain.SeverityMEDIUM, false,
			))
		}
		if assignment.Role == domain.BrandRoleViewer {
			return domain.Fail[PermissionResult](domain.NewDomainError(
				domain.ErrCodeBRAND_ACCESS_DENIED, "Permission insuffisante sur cette brand",
				map[string]interface{}{"required": "BrandEditor+", "actual": assignment.Role},
				domain.SeverityMEDIUM, false,
			))
		}
	}

	return domain.Ok[PermissionResult](PermissionResult{Allowed: true})
}
