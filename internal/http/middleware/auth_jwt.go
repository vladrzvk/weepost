package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
)

// AuthJWTMiddleware valide le Bearer JWT et vérifie que la session est active (SM-17).
//
// Rejette si :
//   - Header Authorization absent ou malformé
//   - Signature JWT invalide ou token expiré
//   - Session introuvable en base
//   - session.Status ≠ active (pending_2fa, revoked, expired — SM-17 quality rule)
//
// Injecte : Locals(LocalsUserID uuid.UUID), Locals(LocalsSessionID uuid.UUID),
//
//	Locals(LocalsIPHash string).
func AuthJWTMiddleware(jwtSvc IJWTService, sessionRepo domain.ISessionRepo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extraire le Bearer token
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return respondError(c, 401, "AUTHENTICATION_FAILED", "Authorization header manquant ou invalide")
		}
		rawToken := strings.TrimPrefix(header, "Bearer ")

		// 2. Valider signature + expiration JWT
		claims, err := jwtSvc.Parse(rawToken)
		if err != nil {
			return respondError(c, 401, "INVALID_TOKEN", "Token JWT invalide ou expiré")
		}

		// 3. Extraire sub (userID) et sid (sessionID)
		subStr, _ := claims["sub"].(string)
		sidStr, _ := claims["sid"].(string)

		userID, err := uuid.Parse(subStr)
		if err != nil {
			return respondError(c, 401, "INVALID_TOKEN", "Token JWT invalide — claim sub manquant ou invalide")
		}
		sessionID, err := uuid.Parse(sidStr)
		if err != nil {
			return respondError(c, 401, "INVALID_TOKEN", "Token JWT invalide — claim sid manquant ou invalide")
		}

		// 4. SM-17 : vérifier session.Status = active
		// pending_2fa NE donne PAS accès aux ressources (quality rule SM-17)
		session, err := sessionRepo.GetByID(c.UserContext(), sessionID)
		if err != nil {
			return respondError(c, 401, "INVALID_TOKEN", "Session introuvable")
		}
		if session.Status != domain.SessionStatusActive {
			// Couvre : pending_2fa, revoked, expired
			return respondError(c, 401, "INVALID_TOKEN", "Session inactive — 2FA en attente, révoquée ou expirée")
		}

		// 5. Injecter dans le contexte Fiber
		c.Locals(LocalsUserID, userID)
		c.Locals(LocalsSessionID, sessionID)

		// 6. IP hash pour audit et rate limiting (RGPD : IP anonymisée)
		ip := c.Get("X-Forwarded-For")
		if ip == "" {
			ip = c.IP()
		}
		c.Locals(LocalsIPHash, hashIP(ip))

		return c.Next()
	}
}
