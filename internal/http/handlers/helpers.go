package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/http/middleware"
)

// respondDomainError convertit une *domain.DomainError en réponse HTTP uniforme.
func respondDomainError(c *fiber.Ctx, domErr *domain.DomainError) error {
	return c.Status(domain.GetHTTPStatus(domErr.Code)).JSON(middleware.ErrorResponse{
		Code:    string(domErr.Code),
		Message: domErr.Message,
		Details: domErr.Details,
		TraceID: c.Locals(middleware.LocalsTraceID).(string),
	})
}

// respondErr cast une error en *DomainError et répond.
func respondErr(c *fiber.Ctx, err error) error {
	if domErr, ok := err.(*domain.DomainError); ok {
		return respondDomainError(c, domErr)
	}
	return c.Status(500).JSON(middleware.ErrorResponse{
		Code:    "INTERNAL_SERVER_ERROR",
		Message: "Erreur interne",
		TraceID: c.Locals(middleware.LocalsTraceID).(string),
	})
}

// respondBadRequest retourne une réponse 400 avec un message d'erreur.
func respondBadRequest(c *fiber.Ctx, msg string) error {
	traceID, _ := c.Locals(middleware.LocalsTraceID).(string)
	return c.Status(400).JSON(middleware.ErrorResponse{
		Code:    "INVALID_REQUEST",
		Message: msg,
		TraceID: traceID,
	})
}

// getUserID extrait l'UUID utilisateur depuis les Locals JWT.
func getUserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(middleware.LocalsUserID).(uuid.UUID)
	return id
}

// getSessionID extrait l'UUID session depuis les Locals JWT.
func getSessionID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(middleware.LocalsSessionID).(uuid.UUID)
	return id
}

// getIPHash extrait le hash IP depuis les Locals (injecté par RateLimit ou AuthJWT).
func getIPHash(c *fiber.Ctx) string {
	h, _ := c.Locals(middleware.LocalsIPHash).(string)
	return h
}
