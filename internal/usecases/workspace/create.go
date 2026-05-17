// internal/usecases/workspace/create.go
package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

var validate = validator.New()

// ErrNotFound sentinel — utilisé pour distinguer "absent" de "erreur technique".
// Doit être retourné par les implementations IWorkspaceRepo quand l'entité est absente.
var ErrNotFound = errors.New("not found")

// CreateWorkspaceCommand — inputs WS-C-001.
// OwnerID injecté depuis le middleware JWT — JAMAIS du body HTTP.
type CreateWorkspaceCommand struct {
	OwnerID      uuid.UUID `json:"-"` // depuis contexte JWT
	Name         string    `json:"name"          validate:"required,min=2,max=100"`
	Description  string    `json:"description"`
	Timezone     string    `json:"timezone"      validate:"required"`
	Language     string    `json:"language"      validate:"required,oneof=fr en es"`
	DateFormat   string    `json:"date_format"   validate:"omitempty"`
	TimeFormat   string    `json:"time_format"   validate:"omitempty,oneof=24h 12h"`
	WeekStartDay int       `json:"week_start_day" validate:"min=0,max=6"`
}

// CreateWorkspaceResult — DTO de sortie (A8-3 : jamais l'aggregate complet).
type CreateWorkspaceResult struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateWorkspaceUseCase — WS-C-001.
type CreateWorkspaceUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	eventBus      domain.IEventBus
}

func NewCreateWorkspaceUseCase(
	repo     domain.IWorkspaceRepo,
	eventBus domain.IEventBus,
) *CreateWorkspaceUseCase {
	return &CreateWorkspaceUseCase{
		workspaceRepo: repo,
		eventBus:      eventBus,
	}
}

// Execute — (ctx, cmd) → Result[CreateWorkspaceResult] (A8-3).
func (uc *CreateWorkspaceUseCase) Execute(
	ctx context.Context,
	cmd CreateWorkspaceCommand,
) domain.Result[CreateWorkspaceResult] {

	// ① Validation syntaxique (A8-7 — go-playground/validator/v10)
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[CreateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT,
			err.Error(),
			nil, domain.SeverityLOW, false,
		))
	}

	// ② Création de l'aggregate en mémoire (slug dérivé du nom par T3 generateSlug)
	// W-1 garanti : NewWorkspace ajoute l'OwnerID comme unique membre actif Owner.
	ws, err := domain.NewWorkspace(cmd.OwnerID, cmd.Name, cmd.Timezone, cmd.Language)
	if err != nil {
		return domain.Fail[CreateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT,
			err.Error(),
			nil, domain.SeverityHIGH, false,
		))
	}

	// ③ Vérification unicité du slug (W-2 — SLUG_ALREADY_EXISTS)
	// ANOMALIE A020 : mission dit ExistsByName — T5 fournit GetBySlug.
	// Le slug est dérivé du nom → unicité du slug ↔ unicité effective du nom.
	existing, repoErr := uc.workspaceRepo.GetBySlug(ctx, ws.Slug())
	if repoErr != nil && !errors.Is(repoErr, ErrNotFound) {
		return domain.Fail[CreateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"error checking slug uniqueness",
			nil, domain.SeverityCRITICAL, true,
		))
	}
	if existing != nil {
		return domain.Fail[CreateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeSLUG_ALREADY_EXISTS,
			"un workspace avec ce nom existe déjà",
			map[string]interface{}{
				"name": cmd.Name,
				"slug": ws.Slug(),
			},
			domain.SeverityMEDIUM, false,
		))
	}

	// ④ Créer et appliquer les settings complets (T2 NewWorkspaceSettings)
	// NewWorkspace initialise Timezone+Language ; UpdateSettings applique dateFormat/timeFormat/weekStartDay.
	settings, settingsErr := domain.NewWorkspaceSettings(
		cmd.Timezone, cmd.Language, cmd.DateFormat, cmd.TimeFormat, cmd.WeekStartDay,
	)
	if settingsErr != nil {
		return domain.Fail[CreateWorkspaceResult](settingsErr.(*domain.DomainError))
	}
	if err := ws.UpdateSettings(settings, cmd.OwnerID); err != nil {
		return domain.Fail[CreateWorkspaceResult](err.(*domain.DomainError))
	}

	// ⑤ Persistance
	if err := uc.workspaceRepo.Create(ctx, ws); err != nil {
		return domain.Fail[CreateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION,
			"failed to persist workspace",
			map[string]interface{}{"name": cmd.Name},
			domain.SeverityCRITICAL, true,
		))
	}

	// ⑥ Événement domaine (T6 — Phase 3 §BC01 workspace.created)
	event := events.NewWorkspaceCreatedEvent(
		ws.ID(),
		ws.Name(),
		ws.Slug(),
		cmd.OwnerID.String(),
		"", // plan_id : placeholder V0 (billing non intégré dans ce UC)
	)
	_ = uc.eventBus.Publish(ctx, event) // échec non bloquant en V0 in-process

	// ⑦ DTO (A8-3)
	return domain.Ok(CreateWorkspaceResult{
		WorkspaceID: ws.ID(),
		Name:        ws.Name(),
		Slug:        ws.Slug(),
		Status:      string(ws.Status()),
		CreatedAt:   ws.CreatedAt(),
	})
}
