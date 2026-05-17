package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	brandUC "github.com/vladrzvk/weepost/internal/usecases/brand"
)

// ─── Use case interfaces locales ──────────────────────────────────────────────

type (
	IUpdateBrandUC interface {
		Execute(ctx context.Context, cmd brandUC.UpdateBrandCommand) domain.Result[brandUC.UpdateBrandResult]
	}
	IAssignBrandMemberUC interface {
		Execute(ctx context.Context, cmd brandUC.AssignBrandMemberCommand) domain.Result[brandUC.AssignBrandMemberResult]
	}
	IRevokeBrandMemberUC interface {
		Execute(ctx context.Context, cmd brandUC.RevokeBrandAccessCommand) domain.Result[struct{}]
	}
)

// ─── BrandHandler ─────────────────────────────────────────────────────────────

type BrandHandler struct {
	updateBrandUC       IUpdateBrandUC
	assignBrandMemberUC IAssignBrandMemberUC
	revokeBrandMemberUC IRevokeBrandMemberUC
}

func NewBrandHandler(
	updateBrand IUpdateBrandUC,
	assignMember IAssignBrandMemberUC,
	revokeMember IRevokeBrandMemberUC,
) *BrandHandler {
	return &BrandHandler{
		updateBrandUC:       updateBrand,
		assignBrandMemberUC: assignMember,
		revokeBrandMemberUC: revokeMember,
	}
}

// RegisterRoutes enregistre les routes brand.
// NOTE brand-scoped : passer ?workspace_id={id} sur toutes ces routes.
// A067/A068 : permission strings corrigés (brand.assignment.create/remove).
func (h *BrandHandler) RegisterRoutes(r fiber.Router, jwtMw fiber.Handler, permMw func(string) fiber.Handler) {
	v1 := r.Group("/api/v1")

	// PATCH /api/v1/brands/:brandID?workspace_id=...
	// Permission BR-02
	v1.Patch("/brands/:brandID", jwtMw, permMw("brand.update"), h.Update)

	// POST /api/v1/brands/:brandID/members?workspace_id=...
	// Permission BR-06 — A067 : mission disait "brand.member.assign"
	v1.Post("/brands/:brandID/members", jwtMw, permMw("brand.assignment.create"), h.AssignMember)

	// DELETE /api/v1/brands/:brandID/members/:memberID?workspace_id=...
	// Permission BR-08 — A068 : mission disait "brand.member.revoke"
	v1.Delete("/brands/:brandID/members/:memberID", jwtMw, permMw("brand.assignment.remove"), h.RemoveMember)
}

// Update — PATCH /api/v1/brands/:brandID
func (h *BrandHandler) Update(c *fiber.Ctx) error {
	var cmd brandUC.UpdateBrandCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	brandID, err := uuid.Parse(c.Params("brandID"))
	if err != nil {
		return respondBadRequest(c, "brandID invalide")
	}
	cmd.BrandID = brandID
	cmd.ActorID = getUserID(c)

	result := h.updateBrandUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// AssignMember — POST /api/v1/brands/:brandID/members
// Affecte un membre du workspace à la brand avec un BrandRole (T12).
func (h *BrandHandler) AssignMember(c *fiber.Ctx) error {
	var cmd brandUC.AssignBrandMemberCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	brandID, err := uuid.Parse(c.Params("brandID"))
	if err != nil {
		return respondBadRequest(c, "brandID invalide")
	}
	cmd.BrandID = brandID
	cmd.ActorID = getUserID(c)

	result := h.assignBrandMemberUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

// RemoveMember — DELETE /api/v1/brands/:brandID/members/:memberID
// Révoque l'accès d'un membre à la brand (T12).
func (h *BrandHandler) RemoveMember(c *fiber.Ctx) error {
	brandID, err := uuid.Parse(c.Params("brandID"))
	if err != nil {
		return respondBadRequest(c, "brandID invalide")
	}
	memberID, err := uuid.Parse(c.Params("memberID"))
	if err != nil {
		return respondBadRequest(c, "memberID invalide")
	}
	cmd := brandUC.RevokeBrandAccessCommand{
		BrandID:  brandID,
		MemberID: memberID,
		ActorID:  getUserID(c),
	}
	result := h.revokeBrandMemberUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.SendStatus(204)
}
