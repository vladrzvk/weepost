package stub

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTService gère génération + parsing de tokens JWT.
// Implémente middleware.IJWTService (Parse) et fournit Generate* pour les use cases.
type JWTService struct {
	accessSecret  []byte
	refreshSecret []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTService(accessSecret, refreshSecret string, accessExpiry, refreshExpiry time.Duration) *JWTService {
	return &JWTService{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (s *JWTService) GenerateAccessToken(userID, sessionID uuid.UUID) (string, error) {
	claims := jwtlib.MapClaims{
		"sub": userID.String(),
		"sid": sessionID.String(),
		"exp": time.Now().Add(s.accessExpiry).Unix(),
		"iat": time.Now().Unix(),
	}
	t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return t.SignedString(s.accessSecret)
}

func (s *JWTService) GenerateRefreshToken(sessionID uuid.UUID) (string, error) {
	claims := jwtlib.MapClaims{
		"sid": sessionID.String(),
		"exp": time.Now().Add(s.refreshExpiry).Unix(),
		"iat": time.Now().Unix(),
	}
	t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return t.SignedString(s.refreshSecret)
}

// Sign implémente security.IJWTService.
func (s *JWTService) Sign(claims jwtlib.MapClaims) (string, error) {
	t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return t.SignedString(s.accessSecret)
}

// Parse implémente middleware.IJWTService.
func (s *JWTService) Parse(token string) (jwtlib.MapClaims, error) {
	t, err := jwtlib.Parse(token, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.accessSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(jwtlib.MapClaims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
