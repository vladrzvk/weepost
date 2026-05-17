// internal/usecases/workspace/update.go
package workspace

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// UpdateWorkspaceCommand — WS-C-002 (nom, description).
// ActorID injecté depuis JWT.
// ANOMALIE A021 : Mode ignoré — W-3 le calcule automatiquement.
type UpdateWorkspaceCommand struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ActorID     uuid.UUID `json:"-"`
	Name        *string   `json:"name,omitempty"        validate:"omitempty,min=2,max=100"`
	Description *string   `json:"description,omitempty"`
	Mode        *string   `json:"mode,omitempty"        validate:"omitempty,oneof=simple team agency"` // ignoré — W-3
}

// UpdateWorkspaceSettingsCommand — WS-C-002 (settings).
type UpdateWorkspaceSettingsCommand struct {
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	ActorID      uuid.UUID `json:"-"`
	Timezone     *string   `json:"timezone,omitempty"`
	Language     *string   `json:"language,omitempty"     validate:"omitempty,oneof=fr en es"`
	DateFormat   *string   `json:"date_format,omitempty"`
	TimeFormat   *string   `json:"time_format,omitempty"  validate:"omitempty,oneof=24h 12h"`
	WeekStartDay *int      `json:"week_start_day,omitempty" validate:"omitempty,min=0,max=6"`
}

// UpdateWorkspaceResult — DTO commun aux deux use cases.
type UpdateWorkspaceResult struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	Mode        string    `json:"mode"`
	UpdatedAt   string    `json:"updated_at"`
}

// UpdateWorkspaceUseCase — WS-C-002 (nom, description).
type UpdateWorkspaceUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	eventBus      domain.IEventBus
}

func NewUpdateWorkspaceUseCase(
	repo     domain.IWorkspaceRepo,
	eventBus domain.IEventBus,
) *UpdateWorkspaceUseCase {
	return &UpdateWorkspaceUseCase{workspaceRepo: repo, eventBus: eventBus}
}

func (uc *UpdateWorkspaceUseCase) Execute(
	ctx context.Context,
	cmd UpdateWorkspaceCommand,
) domain.Result[UpdateWorkspaceResult] {

	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Charger le workspace
	ws, err := uc.workspaceRepo.GetByID(ctx, cmd.WorkspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
				domain.ErrCodeNOT_FOUND, "workspace not found",
				map[string]interface{}{"workspace_id": cmd.WorkspaceID},
				domain.SeverityMEDIUM, false,
			))
		}
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to load workspace",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ③ Vérification permission (Phase 6 WS-02 : Owner + Admin)
	member, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil || member == nil {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINSUFFICIENT_PERMISSIONS,
			"acteur non membre de ce workspace",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityHIGH, false,
		))
	}
	if !member.Role.IsAtLeast(domain.MemberRoleAdmin) {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINSUFFICIENT_PERMISSIONS,
			"workspace.update_settings requiert Owner ou Admin",
			map[string]interface{}{
				"actor_role":   member.Role,
				"required_min": domain.MemberRoleAdmin,
			},
			domain.SeverityHIGH, false,
		))
	}

	// ④ Appliquer les modifications
	// ANOMALIE A022 : Rename() et SetDescription() doivent être ajoutés à T3 (Workspace aggregate).
	// W-2 garanti : le slug n'est PAS modifié — seul le nom d'affichage change.
	changedFields := map[string]interface{}{}
	if cmd.Name != nil {
		if err := ws.Rename(*cmd.Name); err != nil {
			return domain.Fail[UpdateWorkspaceResult](err.(*domain.DomainError))
		}
		changedFields["name"] = *cmd.Name
	}
	if cmd.Description != nil {
		ws.SetDescription(*cmd.Description)
		changedFields["description"] = *cmd.Description
	}
	// cmd.Mode ignoré — W-3 auto-calcule via recomputeMode() (ANOMALIE A021)

	if len(changedFields) == 0 {
		// Aucun champ modifié — retourner succès sans écriture DB
		return domain.Ok(UpdateWorkspaceResult{
			WorkspaceID: ws.ID(),
			Name:        ws.Name(),
			Mode:        string(ws.Mode()),
			UpdatedAt:   ws.UpdatedAt().Format("2006-01-02T15:04:05Z"),
		})
	}

	// ⑤ Persistance
	if err := uc.workspaceRepo.Update(ctx, ws); err != nil {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to update workspace",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ⑥ Événement domaine (T6 — workspace.updated)
	event := events.NewWorkspaceUpdatedEvent(cmd.WorkspaceID, changedFields)
	_ = uc.eventBus.Publish(ctx, event)

	return domain.Ok(UpdateWorkspaceResult{
		WorkspaceID: ws.ID(),
		Name:        ws.Name(),
		Mode:        string(ws.Mode()),
		UpdatedAt:   ws.UpdatedAt().Format("2006-01-02T15:04:05Z"),
	})
}

// UpdateWorkspaceSettingsUseCase — WS-C-002 (timezone, language, dateFormat, timeFormat, weekStartDay).
type UpdateWorkspaceSettingsUseCase struct {
	workspaceRepo domain.IWorkspaceRepo
	eventBus      domain.IEventBus
}

func NewUpdateWorkspaceSettingsUseCase(
	repo     domain.IWorkspaceRepo,
	eventBus domain.IEventBus,
) *UpdateWorkspaceSettingsUseCase {
	return &UpdateWorkspaceSettingsUseCase{workspaceRepo: repo, eventBus: eventBus}
}

func (uc *UpdateWorkspaceSettingsUseCase) Execute(
	ctx context.Context,
	cmd UpdateWorkspaceSettingsCommand,
) domain.Result[UpdateWorkspaceResult] {

	// ① Validation syntaxique
	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINVALID_INPUT, err.Error(), nil, domain.SeverityLOW, false,
		))
	}

	// ② Charger le workspace
	ws, err := uc.workspaceRepo.GetByID(ctx, cmd.WorkspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
				domain.ErrCodeNOT_FOUND, "workspace not found",
				map[string]interface{}{"workspace_id": cmd.WorkspaceID},
				domain.SeverityMEDIUM, false,
			))
		}
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to load workspace",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ③ Vérification permission (WS-02 : Owner + Admin)
	member, err := uc.workspaceRepo.GetMember(ctx, cmd.WorkspaceID, cmd.ActorID)
	if err != nil || member == nil || !member.Role.IsAtLeast(domain.MemberRoleAdmin) {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeINSUFFICIENT_PERMISSIONS,
			"workspace.update_settings requiert Owner ou Admin",
			map[string]interface{}{"actor_id": cmd.ActorID},
			domain.SeverityHIGH, false,
		))
	}

	// ④ Construire les nouveaux settings par fusion (pointer fields = patch partiel)
	current := ws.Settings()
	timezone := current.Timezone
	language := current.Language
	dateFormat := current.DateFormat
	timeFormat := current.TimeFormat
	weekStartDay := current.WeekStartDay

	changedFields := map[string]interface{}{}
	if cmd.Timezone != nil     { timezone = *cmd.Timezone;         changedFields["timezone"] = *cmd.Timezone }
	if cmd.Language != nil     { language = *cmd.Language;         changedFields["language"] = *cmd.Language }
	if cmd.DateFormat != nil   { dateFormat = *cmd.DateFormat;     changedFields["date_format"] = *cmd.DateFormat }
	if cmd.TimeFormat != nil   { timeFormat = *cmd.TimeFormat;     changedFields["time_format"] = *cmd.TimeFormat }
	if cmd.WeekStartDay != nil { weekStartDay = *cmd.WeekStartDay; changedFields["week_start_day"] = *cmd.WeekStartDay }

	if len(changedFields) == 0 {
		return domain.Ok(UpdateWorkspaceResult{
			WorkspaceID: ws.ID(),
			Name:        ws.Name(),
			Mode:        string(ws.Mode()),
			UpdatedAt:   ws.UpdatedAt().Format("2006-01-02T15:04:05Z"),
		})
	}

	// ⑤ Valider et appliquer les nouveaux settings (T2 NewWorkspaceSettings)
	newSettings, settingsErr := domain.NewWorkspaceSettings(
		timezone, language, dateFormat, timeFormat, weekStartDay,
	)
	if settingsErr != nil {
		return domain.Fail[UpdateWorkspaceResult](settingsErr.(*domain.DomainError))
	}
	if err := ws.UpdateSettings(newSettings, cmd.ActorID); err != nil {
		return domain.Fail[UpdateWorkspaceResult](err.(*domain.DomainError))
	}

	// ⑥ Persistance
	if err := uc.workspaceRepo.Update(ctx, ws); err != nil {
		return domain.Fail[UpdateWorkspaceResult](domain.NewDomainError(
			domain.ErrCodeDATABASE_CONNECTION, "failed to update workspace settings",
			nil, domain.SeverityCRITICAL, true,
		))
	}

	// ⑦ Événement domaine (T6 — workspace.updated)
	event := events.NewWorkspaceUpdatedEvent(cmd.WorkspaceID, changedFields)
	_ = uc.eventBus.Publish(ctx, event)

	return domain.Ok(UpdateWorkspaceResult{
		WorkspaceID: ws.ID(),
		Name:        ws.Name(),
		Mode:        string(ws.Mode()),
		UpdatedAt:   ws.UpdatedAt().Format("2006-01-02T15:04:05Z"),
	})
}
