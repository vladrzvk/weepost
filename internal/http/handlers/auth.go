package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/vladrzvk/weepost/internal/domain"
	secUC "github.com/vladrzvk/weepost/internal/usecases/security"
)

// ─── Use case interfaces locales ──────────────────────────────────────────────

type (
	IValidateCredentialsUC interface {
		Execute(ctx context.Context, cmd secUC.ValidateCredentialsCommand) domain.Result[secUC.ValidateCredentialsResult]
	}
	IValidate2FAUC interface {
		Execute(ctx context.Context, cmd secUC.Validate2FACodeCommand) domain.Result[secUC.Validate2FAResult]
	}
	IRevokeSessionUC interface {
		Execute(ctx context.Context, cmd secUC.RevokeJWTTokenCommand) domain.Result[struct{}]
	}
	IEnable2FAUC interface {
		Execute(ctx context.Context, cmd secUC.Enable2FACommand) domain.Result[secUC.Enable2FAResult]
	}
	IDisable2FAUC interface {
		Execute(ctx context.Context, cmd secUC.Disable2FACommand) domain.Result[struct{}]
	}
	ISendPasswordResetUC interface {
		Execute(ctx context.Context, cmd secUC.SendPasswordResetCommand) domain.Result[struct{}]
	}
	IValidatePasswordResetUC interface {
		Execute(ctx context.Context, cmd secUC.ValidatePasswordResetCommand) domain.Result[struct{}]
	}
)

// ─── AuthHandler ──────────────────────────────────────────────────────────────

type AuthHandler struct {
	loginUC                IValidateCredentialsUC
	validate2FAUC          IValidate2FAUC
	revokeSessionUC        IRevokeSessionUC
	enable2FAUC            IEnable2FAUC
	disable2FAUC           IDisable2FAUC
	sendPasswordResetUC    ISendPasswordResetUC
	confirmPasswordResetUC IValidatePasswordResetUC
}

func NewAuthHandler(
	login IValidateCredentialsUC,
	validate2FA IValidate2FAUC,
	revokeSession IRevokeSessionUC,
	enable2FA IEnable2FAUC,
	disable2FA IDisable2FAUC,
	sendReset ISendPasswordResetUC,
	confirmReset IValidatePasswordResetUC,
) *AuthHandler {
	return &AuthHandler{
		loginUC:                login,
		validate2FAUC:          validate2FA,
		revokeSessionUC:        revokeSession,
		enable2FAUC:            enable2FA,
		disable2FAUC:           disable2FA,
		sendPasswordResetUC:    sendReset,
		confirmPasswordResetUC: confirmReset,
	}
}

// RegisterRoutes enregistre les routes auth.
// rateLimiter : une instance IRateLimiter partagée (ou dédiée par route).
// A060 : security.2fa → security.manage (WS-17).
func (h *AuthHandler) RegisterRoutes(
	r fiber.Router,
	jwtMw fiber.Handler,
	permMw func(string) fiber.Handler,
	rateMw func(int, time.Duration) fiber.Handler,
) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/auth/login — RateLimit 10/1min
	v1.Post("/auth/login", rateMw(10, time.Minute), h.Login)

	// POST /api/v1/auth/2fa/validate — RateLimit 5/1min
	v1.Post("/auth/2fa/validate", rateMw(5, time.Minute), h.Validate2FA)

	// DELETE /api/v1/auth/session — AuthJWT requis (logout)
	v1.Delete("/auth/session", jwtMw, h.Logout)

	// POST /api/v1/auth/2fa/enable — AuthJWT + Permission WS-17
	// A060 : mission disait "security.2fa" — Phase 6 WS-17 = "security.manage"
	v1.Post("/auth/2fa/enable", jwtMw, permMw("security.manage"), h.Enable2FA)

	// DELETE /api/v1/auth/2fa — AuthJWT + Permission WS-17
	// A060 : même correction
	v1.Delete("/auth/2fa", jwtMw, permMw("security.manage"), h.Disable2FA)

	// POST /api/v1/auth/password-reset — RateLimit 3/5min (anti-bruteforce)
	v1.Post("/auth/password-reset", rateMw(3, 5*time.Minute), h.RequestPasswordReset)

	// POST /api/v1/auth/password-reset/confirm — RateLimit 5/1min
	v1.Post("/auth/password-reset/confirm", rateMw(5, time.Minute), h.ConfirmPasswordReset)
}

// Login — POST /api/v1/auth/login
// SC-C-001 ValidateCredentials. Anti-énumération : même message si email inconnu ou mdp incorrect.
// Retourne { user_id, requires_2fa, session_token }.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var cmd secUC.ValidateCredentialsCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	// IPHash injecté par RateLimitMiddleware via Locals
	cmd.IPHash = getIPHash(c)

	result := h.loginUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// Validate2FA — POST /api/v1/auth/2fa/validate
// SC-C-006 : valide le code TOTP, révoque la session pending_2fa, retourne un JWT actif.
func (h *AuthHandler) Validate2FA(c *fiber.Ctx) error {
	var cmd secUC.Validate2FACodeCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	result := h.validate2FAUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// Logout — DELETE /api/v1/auth/session
// SC-C-003 : révoque la session courante (SM-17 active → revoked).
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	cmd := secUC.RevokeJWTTokenCommand{
		SessionID: getSessionID(c),
		UserID:    getUserID(c),
		Reason:    "logout",
	}
	result := h.revokeSessionUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.SendStatus(204)
}

// Enable2FA — POST /api/v1/auth/2fa/enable
// SC-C-004 : active le TOTP. Retourne les backup codes en clair (UNE SEULE FOIS).
// Feature gate Starter+ vérifiée dans le UC.
func (h *AuthHandler) Enable2FA(c *fiber.Ctx) error {
	var cmd secUC.Enable2FACommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	cmd.UserID = getUserID(c)

	result := h.enable2FAUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// Disable2FA — DELETE /api/v1/auth/2fa
// SC-C-005 : désactive le TOTP. Re-auth password obligatoire (dans le UC).
func (h *AuthHandler) Disable2FA(c *fiber.Ctx) error {
	var cmd secUC.Disable2FACommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	cmd.UserID = getUserID(c)

	result := h.disable2FAUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.SendStatus(204)
}

// RequestPasswordReset — POST /api/v1/auth/password-reset
// SC-C-011 : anti-énumération OBLIGATOIRE — toujours 202 Accepted quel que soit l'état de l'email.
func (h *AuthHandler) RequestPasswordReset(c *fiber.Ctx) error {
	var cmd secUC.SendPasswordResetCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	cmd.IPHash = getIPHash(c)

	// Anti-énumération : ignorer l'erreur du UC — toujours 202 (quality rule SC-C-011)
	h.sendPasswordResetUC.Execute(c.UserContext(), cmd)
	return c.Status(202).JSON(fiber.Map{
		"message": "Si cet email existe et est vérifié, un lien de réinitialisation a été envoyé.",
	})
}

// ConfirmPasswordReset — POST /api/v1/auth/password-reset/confirm
// SC-C-012 : valide le token, applique NewPassword VO (U-3), invalide le token (usage unique).
func (h *AuthHandler) ConfirmPasswordReset(c *fiber.Ctx) error {
	var cmd secUC.ValidatePasswordResetCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	result := h.confirmPasswordResetUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "password_changed"})
}
