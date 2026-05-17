package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// ─── Context locals keys ────────────────────────────────────────────────────

const (
	LocalsUserID    = "user_id"    // uuid.UUID
	LocalsSessionID = "session_id" // uuid.UUID
	LocalsTraceID   = "trace_id"   // string
	LocalsIPHash    = "ip_hash"    // string (SHA-256 hex)
)

// ─── ErrorResponse — format uniforme Phase 0 §2.2.3 ────────────────────────

// ErrorResponse est la réponse d'erreur uniforme retournée par tous les handlers.
// TraceID = valeur injectée par RequestIDMiddleware.
type ErrorResponse struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	TraceID string                 `json:"trace_id"`
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func respondError(c *fiber.Ctx, status int, code, message string) error {
	traceID, _ := c.Locals(LocalsTraceID).(string)
	return c.Status(status).JSON(ErrorResponse{
		Code:    code,
		Message: message,
		TraceID: traceID,
	})
}

func respondErrorDetails(c *fiber.Ctx, status int, code, message string, details map[string]interface{}) error {
	traceID, _ := c.Locals(LocalsTraceID).(string)
	return c.Status(status).JSON(ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
		TraceID: traceID,
	})
}

func getTraceID(c *fiber.Ctx) string {
	id, _ := c.Locals(LocalsTraceID).(string)
	return id
}

// hashIP retourne le SHA-256 hex d'une adresse IP (anonymisation RGPD).
func hashIP(ip string) string {
	// X-Forwarded-For peut contenir plusieurs IPs — prendre la première
	if idx := strings.Index(ip, ","); idx != -1 {
		ip = strings.TrimSpace(ip[:idx])
	}
	h := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(h[:])
}

// ─── IJWTService — port vers l'adapteur JWT (infrastructure) ────────────────

// IJWTService est utilisé par AuthJWTMiddleware.
// Implémentation : internal/infrastructure/jwt/jwt_service.go
type IJWTService interface {
	Parse(token string) (jwt.MapClaims, error)
}
