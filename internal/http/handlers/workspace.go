package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	wsUC "github.com/vladrzvk/weepost/internal/usecases/workspace"
	brandUC "github.com/vladrzvk/weepost/internal/usecases/brand"
)

// ─── Use case interfaces locales ──────────────────────────────────────────────

type (
	ICreateWorkspaceUC interface {
		Execute(ctx context.Context, cmd wsUC.CreateWorkspaceCommand) domain.Result[wsUC.CreateWorkspaceResult]
	}
	IUpdateWorkspaceUC interface {
		Execute(ctx context.Context, cmd wsUC.UpdateWorkspaceCommand) domain.Result[wsUC.UpdateWorkspaceResult]
	}
	IDeleteWorkspaceUC interface {
		Execute(ctx context.Context, cmd wsUC.DeleteWorkspaceCommand) domain.Result[struct{}]
	}
	IInviteMemberUC interface {
		Execute(ctx context.Context, cmd wsUC.InviteMemberCommand) domain.Result[wsUC.InviteMemberResult]
	}
	IAcceptInvitationUC interface {
		Execute(ctx context.Context, cmd wsUC.AcceptInvitationCommand) domain.Result[struct{}]
	}
	ICreateBrandUC interface {
		Execute(ctx context.Context, cmd brandUC.CreateBrandCommand) domain.Result[brandUC.CreateBrandResult]
	}
)

// ─── WorkspaceHandler ─────────────────────────────────────────────────────────

type WorkspaceHandler struct {
	createWorkspaceUC  ICreateWorkspaceUC
	updateWorkspaceUC  IUpdateWorkspaceUC
	deleteWorkspaceUC  IDeleteWorkspaceUC
	inviteMemberUC     IInviteMemberUC
	acceptInvitationUC IAcceptInvitationUC
	createBrandUC      ICreateBrandUC
}

func NewWorkspaceHandler(
	createWs ICreateWorkspaceUC,
	updateWs IUpdateWorkspaceUC,
	deleteWs IDeleteWorkspaceUC,
	inviteMember IInviteMemberUC,
	acceptInvitation IAcceptInvitationUC,
	createBrand ICreateBrandUC,
) *WorkspaceHandler {
	return &WorkspaceHandler{
		createWorkspaceUC:  createWs,
		updateWorkspaceUC:  updateWs,
		deleteWorkspaceUC:  deleteWs,
		inviteMemberUC:     inviteMember,
		acceptInvitationUC: acceptInvitation,
		createBrandUC:      createBrand,
	}
}

// RegisterRoutes enregistre toutes les routes workspace sur le router Fiber.
// jwtMw      : AuthJWTMiddleware(jwtSvc, sessionRepo)
// permMw     : func(perm string) fiber.Handler = PermissionMiddleware(perm, checker)
// rateLimiter: IRateLimiter instance
func (h *WorkspaceHandler) RegisterRoutes(
	r fiber.Router,
	jwtMw fiber.Handler,
	permMw func(string) fiber.Handler,
) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/workspaces
	// Acteur : tout utilisateur authentifié (devient Owner)
	v1.Post("/workspaces", jwtMw, h.Create)

	// PATCH /api/v1/workspaces/:workspaceID
	// Permission WS-02 — A061 : mission disait "workspace.update"
	v1.Patch("/workspaces/:workspaceID", jwtMw, permMw("workspace.update_settings"), h.Update)

	// PATCH /api/v1/workspaces/:workspaceID/settings
	// Permission WS-02 — A062 : même UC et permission que PATCH /workspaces/:workspaceID
	v1.Patch("/workspaces/:workspaceID/settings", jwtMw, permMw("workspace.update_settings"), h.UpdateSettings)

	// DELETE /api/v1/workspaces/:workspaceID
	// Permission WS-03 — Owner uniquement
	v1.Delete("/workspaces/:workspaceID", jwtMw, permMw("workspace.delete"), h.Delete)

	// POST /api/v1/workspaces/:workspaceID/invitations
	// Permission WS-06
	v1.Post("/workspaces/:workspaceID/invitations", jwtMw, permMw("member.invite"), h.InviteMember)

	// POST /api/v1/workspaces/invitations/:token/accept
	// Pas de permission middleware — token dans l'URL fait foi (T10 AcceptInvitation)
	v1.Post("/workspaces/invitations/:token/accept", jwtMw, h.AcceptInvitation)

	// POST /api/v1/workspaces/:workspaceID/brands
	// Permission WS-15
	v1.Post("/workspaces/:workspaceID/brands", jwtMw, permMw("brand.create"), h.CreateBrand)
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// Create — POST /api/v1/workspaces
// Crée un nouveau workspace. L'appelant devient Owner (OwnerID = JWT sub).
func (h *WorkspaceHandler) Create(c *fiber.Ctx) error {
	var cmd wsUC.CreateWorkspaceCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	cmd.OwnerID = getUserID(c) // injecté depuis JWT — jamais du body

	result := h.createWorkspaceUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

// Update — PATCH /api/v1/workspaces/:workspaceID
// Met à jour nom/description/timezone/langue du workspace (WS-02).
func (h *WorkspaceHandler) Update(c *fiber.Ctx) error {
	var cmd wsUC.UpdateWorkspaceCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	workspaceID, err := uuid.Parse(c.Params("workspaceID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid workspace id"})
	}
	cmd.WorkspaceID = workspaceID
	cmd.ActorID = getUserID(c)

	result := h.updateWorkspaceUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// UpdateSettings — PATCH /api/v1/workspaces/:workspaceID/settings
// Même use case que Update (T8) — route séparée pour sémantique client.
// A062 : même permission workspace.update_settings.
func (h *WorkspaceHandler) UpdateSettings(c *fiber.Ctx) error {
	return h.Update(c) // délègue à Update — même UC, même logique
}

// Delete — DELETE /api/v1/workspaces/:workspaceID
// Soft delete via SM-02 active → pending_deletion (T9).
func (h *WorkspaceHandler) Delete(c *fiber.Ctx) error {
	workspaceIDDel, err := uuid.Parse(c.Params("workspaceID"))
	if err != nil {
		return respondBadRequest(c, "workspaceID invalide")
	}
	cmd := wsUC.DeleteWorkspaceCommand{
		WorkspaceID: workspaceIDDel,
		ActorID:     getUserID(c),
	}
	result := h.deleteWorkspaceUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.SendStatus(204)
}

// InviteMember — POST /api/v1/workspaces/:workspaceID/invitations
// Invite un nouveau membre au workspace (T10). TTL invitation 7 jours.
func (h *WorkspaceHandler) InviteMember(c *fiber.Ctx) error {
	var cmd wsUC.InviteMemberCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	workspaceIDInv, err := uuid.Parse(c.Params("workspaceID"))
	if err != nil {
		return respondBadRequest(c, "workspaceID invalide")
	}
	cmd.WorkspaceID = workspaceIDInv
	cmd.ActorID = getUserID(c)

	result := h.inviteMemberUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

// AcceptInvitation — POST /api/v1/workspaces/invitations/:token/accept
// Valide le token d'invitation et ajoute le membre au workspace (T10).
func (h *WorkspaceHandler) AcceptInvitation(c *fiber.Ctx) error {
	cmd := wsUC.AcceptInvitationCommand{
		Token:   c.Params("token"),
		ActorID: getUserID(c),
	}
	result := h.acceptInvitationUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "accepted"})
}

// CreateBrand — POST /api/v1/workspaces/:workspaceID/brands
// Crée une brand dans le workspace (T11, WS-15).
func (h *WorkspaceHandler) CreateBrand(c *fiber.Ctx) error {
	var cmd brandUC.CreateBrandCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	workspaceIDBrand, err := uuid.Parse(c.Params("workspaceID"))
	if err != nil {
		return respondBadRequest(c, "workspaceID invalide")
	}
	cmd.WorkspaceID = workspaceIDBrand
	cmd.ActorID = getUserID(c)

	result := h.createBrandUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

