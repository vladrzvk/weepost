package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	secUC "github.com/vladrzvk/weepost/internal/usecases/security"
)

type (
	IUnlockUserUC interface {
		Execute(ctx context.Context, cmd secUC.UnlockUserAccountCommand) domain.Result[struct{}]
	}
	IRotateEncryptionKeysUC interface {
		Execute(ctx context.Context, cmd secUC.RotateEncryptionKeysCommand) domain.Result[secUC.RotateEncryptionKeysResult]
	}
)

type SecurityHandler struct {
	unlockUserUC IUnlockUserUC
	rotateKeysUC IRotateEncryptionKeysUC
}

func NewSecurityHandler(
	unlockUser IUnlockUserUC,
	rotateKeys IRotateEncryptionKeysUC,
) *SecurityHandler {
	return &SecurityHandler{
		unlockUserUC: unlockUser,
		rotateKeysUC: rotateKeys,
	}
}

// RegisterRoutes enregistre les routes admin security.
// Permission WS-17 security.manage (Owner+Admin).
// NOTE : ces routes sont workspace-scoped — passer ?workspace_id=... si permMw requis.
func (h *SecurityHandler) RegisterRoutes(
	r fiber.Router,
	jwtMw fiber.Handler,
	permMw func(string) fiber.Handler,
) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/admin/users/:userID/unlock?workspace_id=...
	// Permission WS-17 security.manage
	v1.Post("/admin/users/:userID/unlock", jwtMw, permMw("security.manage"), h.UnlockUser)

	// POST /api/v1/admin/security/rotate-keys?workspace_id=...
	// Permission WS-17 security.manage
	// NOTE SCX-009 : opère sur TOUS les credentials actifs — scope global documenté dans T24
	v1.Post("/admin/security/rotate-keys", jwtMw, permMw("security.manage"), h.RotateKeys)
}

// UnlockUser — POST /api/v1/admin/users/:userID/unlock
// SC-C-008 : déverrouillage manuel par Admin/Owner (UnlockMode = admin_manual).
// SM-17b : locked → active.
func (h *SecurityHandler) UnlockUser(c *fiber.Ctx) error {
	targetUserID, err := uuid.Parse(c.Params("userID"))
	if err != nil {
		return respondBadRequest(c, "userID invalide")
	}
	cmd := secUC.UnlockUserAccountCommand{
		UserID:     targetUserID,
		UnlockMode: "admin_manual", // forcé — ce handler est admin-only
	}
	result := h.unlockUserUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "unlocked"})
}

// RotateKeys — POST /api/v1/admin/security/rotate-keys
// SC-C-014 : rotation AES-256-GCM des credentials channel par batch de 50.
// Retourne { rotation_id, channels_total, channels_rotated, channels_failed }.
func (h *SecurityHandler) RotateKeys(c *fiber.Ctx) error {
	var body struct {
		Notes string `json:"notes"`
	}
	_ = c.BodyParser(&body) // Notes optionnel

	cmd := secUC.RotateEncryptionKeysCommand{
		InitiatedByMemberID: getUserID(c),
		Notes:               body.Notes,
	}
	result := h.rotateKeysUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}
