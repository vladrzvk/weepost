package brand

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

var validate = validator.New()
var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// PlanLimitsChecker port local pour les limites de plan (M-5, B-4).
type PlanLimitsChecker interface {
	CheckMemberLimit(ctx context.Context, workspaceID uuid.UUID) error
	CheckBrandLimit(ctx context.Context, workspaceID uuid.UUID) error
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRegex.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ─────────────────────────────────────────────────────────
// CreateBrandUseCase
// ─────────────────────────────────────────────────────────

type CreateBrandCommand struct {
	ActorID     uuid.UUID `validate:"required"`
	WorkspaceID uuid.UUID `validate:"required"`
	Name        string    `validate:"required,min=1,max=100"`
}

type CreateBrandResult struct {
	BrandID     string    `json:"brand_id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateBrandUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
	userRepo      domain.IUserRepo
	eventBus      domain.IEventBus
	planChecker   PlanLimitsChecker
}

func NewCreateBrandUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	brandRepo domain.IBrandRepo,
	userRepo domain.IUserRepo,
	eventBus domain.IEventBus,
	planChecker PlanLimitsChecker,
) *CreateBrandUseCase {
	return &CreateBrandUseCase{
		workspaceRepo: workspaceRepo,
		brandRepo:     brandRepo,
		userRepo:      userRepo,
		eventBus:      eventBus,
		planChecker:   planChecker,
	}
}

func (uc *CreateBrandUseCase) Execute(ctx context.Context, cmd CreateBrandCommand) domain.Result[CreateBrandResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger le workspace
	_, err := uc.workspaceRepo.GetByID(ctx, cmd.WorkspaceID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeWORKSPACE_NOT_FOUND, "workspace not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ③ Vérifier que l'acteur est membre
	actor, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "actor is not a workspace member", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// WS-15 brand.create : Owner, Admin, Manager
	if !actor.Role.IsAtLeast(domain.MemberRoleManager) {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "insufficient role to create a brand", nil, domain.SeverityMEDIUM, false))
	}

	// U-4 : email de l'acteur vérifié
	actorUser, err := uc.userRepo.GetByID(ctx, cmd.ActorID)
	if err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}
	if !actorUser.EmailVerified() {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeEMAIL_NOT_VERIFIED, "actor email must be verified to create a brand", nil, domain.SeverityMEDIUM, false))
	}

	// B-4 : limite de brands du plan
	if err := uc.planChecker.CheckBrandLimit(ctx, cmd.WorkspaceID); err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_LIMIT_REACHED, err.Error(), nil, domain.SeverityMEDIUM, false))
	}

	// B-1 : slug unique dans le workspace (ANOMALIE A026 : pas d'ExistsBySlug sur IBrandRepo)
	targetSlug := slugify(cmd.Name)
	existingBrands, err := uc.brandRepo.ListByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}
	for _, b := range existingBrands {
		if b.Slug() == targetSlug {
			return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_SLUG_ALREADY_EXISTS, "a brand with this name already exists in the workspace", nil, domain.SeverityMEDIUM, false))
		}
	}

	// ④ Créer l'agrégat domain
	brand, err := domain.NewBrand(cmd.WorkspaceID, cmd.ActorID, cmd.Name)
	if err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑤ Persister
	if err := uc.brandRepo.Create(ctx, brand); err != nil {
		return domain.Fail[CreateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement
	_ = uc.eventBus.Publish(ctx, events.NewBrandCreatedEvent(
		brand.ID(),
		cmd.WorkspaceID,
		brand.Name(),
		brand.Slug(),
		cmd.ActorID.String(),
	))

	// ⑦ DTO
	return domain.Ok(CreateBrandResult{
		BrandID:     brand.ID().String(),
		WorkspaceID: cmd.WorkspaceID.String(),
		Name:        brand.Name(),
		Slug:        brand.Slug(),
		CreatedAt:   brand.CreatedAt(),
	})
}

// ─────────────────────────────────────────────────────────
// UpdateBrandUseCase
// ─────────────────────────────────────────────────────────

type UpdateBrandCommand struct {
	ActorID     uuid.UUID `validate:"required"`
	WorkspaceID uuid.UUID `validate:"required"`
	BrandID     uuid.UUID `validate:"required"`
	Name        *string `validate:"omitempty,min=1,max=100"`
	// Status : "active" | "archived" — transitions SM-06
	Status *string `validate:"omitempty,oneof=active archived"`
}

type UpdateBrandResult struct {
	BrandID   string    `json:"brand_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateBrandUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	brandRepo     domain.IBrandRepo
	eventBus      domain.IEventBus
}

func NewUpdateBrandUseCase(
	workspaceRepo domain.IWorkspaceRepo,
	brandRepo domain.IBrandRepo,
	eventBus domain.IEventBus,
) *UpdateBrandUseCase {
	return &UpdateBrandUseCase{
		workspaceRepo: workspaceRepo,
		brandRepo:     brandRepo,
		eventBus:      eventBus,
	}
}

func (uc *UpdateBrandUseCase) Execute(ctx context.Context, cmd UpdateBrandCommand) domain.Result[UpdateBrandResult] {
	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeVALIDATION, err.Error(), nil, domain.SeverityLOW, false))
	}

	// ② Charger la brand
	brand, err := uc.brandRepo.GetByID(ctx, cmd.BrandID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Vérifier que la brand appartient au workspace
	if brand.WorkspaceID() != cmd.WorkspaceID {
		return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_NOT_FOUND, "brand not found in this workspace", nil, domain.SeverityMEDIUM, false))
	}

	// ③ Charger l'acteur
	actor, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "actor is not a workspace member", nil, domain.SeverityMEDIUM, false))
		}
		return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// Permission BR-01 brand.update
	// A003 : BypassesBrandAssignment() = true uniquement pour Owner (pas Admin)
	if !actor.Role.BypassesBrandAssignment() {
		actorAssignment, assignErr := uc.brandRepo.GetAssignment(ctx, cmd.BrandID, actor.ID)
		if assignErr != nil {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "no brand assignment found", nil, domain.SeverityMEDIUM, false))
		}
		if actorAssignment.Role != domain.BrandRoleOwner && actorAssignment.Role != domain.BrandRoleManager {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeFORBIDDEN, "brand owner or manager role required to update brand", nil, domain.SeverityMEDIUM, false))
		}
	}

	// ④ Appliquer les changements
	changedFields := map[string]interface{}{}

	if cmd.Name != nil && *cmd.Name != brand.Name() {
		// B-1 : vérifier l'unicité du nouveau slug (ANOMALIE A026)
		newSlug := slugify(*cmd.Name)
		existingBrands, listErr := uc.brandRepo.ListByWorkspace(ctx, cmd.WorkspaceID)
		if listErr != nil {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, listErr.Error(), nil, domain.SeverityHIGH, true))
		}
		for _, b := range existingBrands {
			if b.ID() != cmd.BrandID && b.Slug() == newSlug {
				return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeBRAND_SLUG_ALREADY_EXISTS, "a brand with this name already exists in the workspace", nil, domain.SeverityMEDIUM, false))
			}
		}
		if err := brand.Rename(*cmd.Name); err != nil {
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
		}
		changedFields["name"] = brand.Name()
		changedFields["slug"] = brand.Slug()
	}

	if cmd.Status != nil {
		requestedStatus := domain.BrandStatus(*cmd.Status)
		switch {
		case requestedStatus == domain.BrandStatusArchived && brand.Status() == domain.BrandStatusActive:
			// SM-06 : active → archived
			if err := brand.Archive(cmd.ActorID); err != nil {
				return domain.Fail[UpdateBrandResult](err.(*domain.DomainError))
			}
			changedFields["status"] = string(domain.BrandStatusArchived)

		case requestedStatus == domain.BrandStatusActive && brand.Status() == domain.BrandStatusArchived:
			if err := brand.Unarchive(); err != nil {
				return domain.Fail[UpdateBrandResult](err.(*domain.DomainError))
			}
			changedFields["status"] = string(domain.BrandStatusActive)

		case requestedStatus == brand.Status():
			// aucun changement de statut, pas d'erreur

		default:
			return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINVALID_STATUS_TRANSITION,
				"invalid brand status transition from "+string(brand.Status())+" to "+string(requestedStatus), nil, domain.SeverityMEDIUM, false))
		}
	}

	if len(changedFields) == 0 {
		return domain.Ok(UpdateBrandResult{
			BrandID:   brand.ID().String(),
			Name:      brand.Name(),
			Slug:      brand.Slug(),
			Status:    string(brand.Status()),
			UpdatedAt: time.Now().UTC(),
		})
	}

	// ⑤ Persister
	if err := uc.brandRepo.Update(ctx, brand); err != nil {
		return domain.Fail[UpdateBrandResult](domain.NewDomainError(domain.ErrCodeINTERNAL, err.Error(), nil, domain.SeverityHIGH, true))
	}

	// ⑥ Émettre l'événement
	_ = uc.eventBus.Publish(ctx, events.NewBrandUpdatedEvent(cmd.BrandID, cmd.WorkspaceID, changedFields))

	// ⑦ DTO
	return domain.Ok(UpdateBrandResult{
		BrandID:   brand.ID().String(),
		Name:      brand.Name(),
		Slug:      brand.Slug(),
		Status:    string(brand.Status()),
		UpdatedAt: time.Now().UTC(),
	})
}
