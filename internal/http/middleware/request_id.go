package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RequestIDMiddleware génère un UUID request ID si absent (X-Request-ID header).
// Injecte dans Locals(LocalsTraceID) et dans le header X-Request-ID de la réponse.
// Doit être le premier middleware dans la chaîne.
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := c.Get("X-Request-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Locals(LocalsTraceID, traceID)
		c.Set("X-Request-ID", traceID)
		return c.Next()
	}
}
