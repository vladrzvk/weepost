package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// IRateLimiter — port vers le store de rate limiting (in-memory V0 ou Redis production).
type IRateLimiter interface {
	Allow(key string, maxReqs int, window time.Duration) bool
}

// InMemoryRateLimiter — sliding window rate limiter en mémoire.
// Thread-safe. Usage V0 ; remplacer par Redis en production (multi-instance).
type InMemoryRateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
}

func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	return &InMemoryRateLimiter{buckets: make(map[string][]time.Time)}
}

// Allow retourne true si la clé peut effectuer une nouvelle requête dans la fenêtre glissante.
func (rl *InMemoryRateLimiter) Allow(key string, maxReqs int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// Purger les timestamps expirés
	existing := rl.buckets[key]
	valid := existing[:0]
	for _, t := range existing {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= maxReqs {
		rl.buckets[key] = valid
		return false
	}

	rl.buckets[key] = append(valid, now)
	return true
}

// RateLimitMiddleware limite les requêtes par IP (SHA-256 hash de X-Forwarded-For).
// Retourne 429 si maxReqs dépassé dans window.
// Injecte LocalsIPHash dans le contexte (utilisé par SC-C-001 pour l'audit).
func RateLimitMiddleware(maxReqs int, window time.Duration, limiter IRateLimiter) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.Get("X-Forwarded-For")
		if ip == "" {
			ip = c.IP()
		}
		ipHash := hashIP(ip)

		if !limiter.Allow(ipHash, maxReqs, window) {
			return c.Status(429).JSON(ErrorResponse{
				Code:    "RATE_LIMIT_EXCEEDED",
				Message: "Trop de requêtes — réessayez plus tard",
				TraceID: getTraceID(c),
			})
		}

		// Injecter IPHash pour les use cases qui en ont besoin (SC-C-001, SC-C-011)
		c.Locals(LocalsIPHash, ipHash)
		return c.Next()
	}
}
