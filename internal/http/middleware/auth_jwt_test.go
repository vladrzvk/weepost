package middleware_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/http/middleware"
)

// mockJWTService retourne des claims valides si le token = "valid_token".
type mockJWTService struct {
	claims jwt.MapClaims
	err    error
}

func (m *mockJWTService) Parse(token string) (jwt.MapClaims, error) {
	return m.claims, m.err
}

// mockSessionRepo simule ISessionRepo.
type mockSessionRepo struct {
	session *domain.UserSession
	err     error
}

func (m *mockSessionRepo) Create(_ context.Context, _ *domain.UserSession) error { return nil }
func (m *mockSessionRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.UserSession, error) {
	return m.session, m.err
}
func (m *mockSessionRepo) Update(_ context.Context, _ *domain.UserSession) error        { return nil }
func (m *mockSessionRepo) RevokeAllByUserID(_ context.Context, _ uuid.UUID) error       { return nil }
func (m *mockSessionRepo) DeleteExpired(_ context.Context) error                        { return nil }
func (m *mockSessionRepo) GetByJTI(_ context.Context, _ string) (*domain.UserSession, error) {
	return nil, nil
}
func (m *mockSessionRepo) RevokeByJTI(_ context.Context, _ string) error { return nil }

// TestAuthJWT_PendingTwoFA_Rejected — SM-17 quality rule :
// session pending_2fa NE donne PAS accès aux ressources protégées.
func TestAuthJWT_PendingTwoFA_Rejected(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	jwtSvc := &mockJWTService{
		claims: jwt.MapClaims{
			"sub": userID.String(),
			"sid": sessionID.String(),
		},
	}
	session := &domain.UserSession{
		ID:     sessionID,
		UserID: userID,
		Status: domain.SessionStatusPending2FA, // 2FA pas encore validé
	}
	sessionRepo := &mockSessionRepo{session: session}

	app := fiber.New()
	app.Use(middleware.RequestIDMiddleware())
	app.Use(middleware.AuthJWTMiddleware(jwtSvc, sessionRepo))
	app.Get("/protected", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer any_token")
	resp, _ := app.Test(req)

	assert.Equal(t, 401, resp.StatusCode) // pending_2fa → 401
}

// TestAuthJWT_ActiveSession_Passes — session active → accès autorisé.
func TestAuthJWT_ActiveSession_Passes(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	jwtSvc := &mockJWTService{
		claims: jwt.MapClaims{
			"sub": userID.String(),
			"sid": sessionID.String(),
		},
	}
	session := &domain.UserSession{
		ID:     sessionID,
		UserID: userID,
		Status: domain.SessionStatusActive,
	}
	sessionRepo := &mockSessionRepo{session: session}

	app := fiber.New()
	app.Use(middleware.RequestIDMiddleware())
	app.Use(middleware.AuthJWTMiddleware(jwtSvc, sessionRepo))
	app.Get("/protected", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer any_token")
	resp, _ := app.Test(req)

	assert.Equal(t, 200, resp.StatusCode)
}

// TestRateLimit_Exceeded_Returns429 — dépasse maxReqs → 429.
func TestRateLimit_Exceeded_Returns429(t *testing.T) {
	limiter := middleware.NewInMemoryRateLimiter()
	app := fiber.New()
	app.Use(middleware.RequestIDMiddleware())

	// maxReqs=2, window=1min
	app.Post("/auth/login",
		middleware.RateLimitMiddleware(2, 60*1000000000, limiter), // 1min en nanoseconds
		func(c *fiber.Ctx) error { return c.SendStatus(200) },
	)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "1.2.3.4") // même IP
		resp, _ := app.Test(req)
		if i < 2 {
			assert.Equal(t, 200, resp.StatusCode, "requête %d doit passer", i+1)
		} else {
			assert.Equal(t, 429, resp.StatusCode, "3e requête doit être bloquée")
		}
	}
}
