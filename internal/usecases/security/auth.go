package security

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

var validate = validator.New()

// ────────────────────────────────────────────────────────────────────────────
// Port interfaces locaux
// ────────────────────────────────────────────────────────────────────────────

// IJWTService — port vers l'adapteur JWT (infrastructure).
// Implémentation : internal/infrastructure/jwt/jwt_service.go
type IJWTService interface {
	Sign(claims jwt.MapClaims) (string, error)
	// Parse valide la signature et retourne les claims. Retourne erreur si expiré ou invalide.
	Parse(token string) (jwt.MapClaims, error)
}

// IPlanFeatureService — port vérification feature gate (Phase 6).
type IPlanFeatureService interface {
	// UserCanUse2FA retourne true si l'utilisateur est sur un workspace Starter+ (Phase 6 §3).
	UserCanUse2FA(ctx context.Context, userID uuid.UUID) (bool, error)
}

// SC-C-009 RecordFailedLoginCommand / RecordFailedLoginService → voir shared.go (même package).

// ════════════════════════════════════════════════════════════════════════════
// SC-C-001 — ValidateCredentialsUseCase
// Phase 4 §15 SC-C-001 — Source de vérité
// Correction : getters T4, ordre U-4 avant U-2 (quality rule)
// ════════════════════════════════════════════════════════════════════════════

type ValidateCredentialsCommand struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
	IPHash   string `json:"ip_hash"` // hash SHA-256 de l'IP — injecté par middleware
}

type ValidateCredentialsResult struct {
	UserID       uuid.UUID `json:"user_id"`
	Requires2FA  bool      `json:"requires_2fa"`
	SessionToken string    `json:"session_token"` // JWT final (sans 2FA) ou token temporaire (pending_2fa)
}

type ValidateCredentialsService struct {
	userRepo             domain.IUserRepo
	sessionRepo          domain.ISessionRepo
	recordFailedLoginSvc *RecordFailedLoginService
	generateJWTSvc       *GenerateJWTTokenService
	jwtSvc               IJWTService
	eventBus             domain.IEventBus
}

func NewValidateCredentialsService(
	userRepo domain.IUserRepo,
	sessionRepo domain.ISessionRepo,
	recordFailedLoginSvc *RecordFailedLoginService,
	generateJWTSvc *GenerateJWTTokenService,
	jwtSvc IJWTService,
	eventBus domain.IEventBus,
) *ValidateCredentialsService {
	return &ValidateCredentialsService{
		userRepo:             userRepo,
		sessionRepo:          sessionRepo,
		recordFailedLoginSvc: recordFailedLoginSvc,
		generateJWTSvc:       generateJWTSvc,
		jwtSvc:               jwtSvc,
		eventBus:             eventBus,
	}
}

func (s *ValidateCredentialsService) Execute(
	ctx context.Context,
	cmd ValidateCredentialsCommand,
) domain.Result[ValidateCredentialsResult] {

	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[ValidateCredentialsResult](
			domain.NewDomainError(domain.ErrCodeVALIDATION_FAILED, err.Error(),
				nil, domain.SeverityLOW, false))
	}

	// 1. FindByEmail — anti-énumération : même message d'erreur si absent (quality rule)
	user, err := s.userRepo.GetByEmail(ctx, cmd.Email)
	if err != nil {
		return domain.Fail[ValidateCredentialsResult](
			domain.NewDomainError(domain.ErrCodeINVALID_CREDENTIALS,
				"Email ou mot de passe incorrect",
				nil, domain.SeverityMEDIUM, false))
	}

	// 2. U-4 : email vérifié — AVANT U-2 (quality rule : U-4 précède U-2)
	if err := user.RequireEmailVerified(); err != nil {
		return domain.Fail[ValidateCredentialsResult](err.(*domain.DomainError))
	}

	// 3. U-2 : compte verrouillé (T4 : IsLocked() vérifie status=locked ET TTL)
	if user.IsLocked() {
		return domain.Fail[ValidateCredentialsResult](
			domain.NewDomainError(domain.ErrCodeACCOUNT_LOCKED,
				"Compte temporairement verrouillé — réessayez dans 30 minutes",
				map[string]interface{}{
					"user_id":      user.ID().String(),
					"locked_until": user.LockedUntil(),
				},
				domain.SeverityHIGH, true))
	}

	// 4. Vérification mot de passe — U-3 (délégué au domaine, compatible DA-2 Argon2id)
	ok, err := user.VerifyPassword(cmd.Password)
	if err != nil {
		return domain.Fail[ValidateCredentialsResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL, "password verification failed",
				nil, domain.SeverityHIGH, true))
	}
	if !ok {
		// SC-C-009 délégué : incrémente compteur, lock si seuil 5 atteint
		_ = s.recordFailedLoginSvc.Execute(ctx, RecordFailedLoginCommand{
			UserID: user.ID(),
			IPHash: cmd.IPHash,
		})
		// Anti-énumération : même message que email inconnu
		return domain.Fail[ValidateCredentialsResult](
			domain.NewDomainError(domain.ErrCodeINVALID_CREDENTIALS,
				"Email ou mot de passe incorrect",
				nil, domain.SeverityMEDIUM, false))
	}

	// Réinitialiser le compteur d'échecs après succès
	user.RecordSuccessfulLogin(cmd.IPHash)
	_ = s.userRepo.Update(ctx, user)

	// 5. 2FA activée → session temporaire pending_2fa (SM-17 : pending_2fa → active via SC-C-006)
	if user.TwoFactorEnabled() {
		tempToken, err := s.generateTempSession(ctx, user.ID())
		if err != nil {
			return domain.Fail[ValidateCredentialsResult](
				domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
					"Erreur lors de la création de la session temporaire 2FA",
					nil, domain.SeverityCRITICAL, true))
		}
		return domain.Ok(ValidateCredentialsResult{
			UserID:       user.ID(),
			Requires2FA:  true,
			SessionToken: tempToken,
		})
	}

	// 6. Pas de 2FA → SC-C-002 : générer JWT final (SM-17 : → active)
	tokenResult := s.generateJWTSvc.Execute(ctx, GenerateJWTTokenCommand{UserID: user.ID()})
	if tokenResult.IsFail() {
		return domain.Fail[ValidateCredentialsResult](tokenResult.Err())
	}

	return domain.Ok(ValidateCredentialsResult{
		UserID:       user.ID(),
		Requires2FA:  false,
		SessionToken: tokenResult.Value().Value,
	})
}

// generateTempSession crée une session pending_2fa (TTL=5min) et retourne un JWT signé.
// ANOMALIE A045 : ISessionRepo T5 n'a pas FindByTempToken.
// Workaround : le token JWT encode le sessionID (sid claim). SC-C-006 extrait sid et utilise GetByID.
func (s *ValidateCredentialsService) generateTempSession(
	ctx context.Context,
	userID uuid.UUID,
) (string, error) {
	expiresAt := time.Now().UTC().Add(5 * time.Minute) // TTL 5min — Phase 4 §SC-C-006
	session, err := domain.NewUserSession(userID, domain.SessionStatusPending2FA, expiresAt)
	if err != nil {
		return "", err
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"sid":  session.ID.String(),
		"type": "2fa_pending",
		"exp":  expiresAt.Unix(),
		"iat":  time.Now().UTC().Unix(),
	}
	return s.jwtSvc.Sign(claims)
}

// ════════════════════════════════════════════════════════════════════════════
// SC-C-002 — GenerateJWTTokenUseCase
// Phase 4 §15 SC-C-002 — Source de vérité
// TTL session : 24h ; SM-17 : → active
// ════════════════════════════════════════════════════════════════════════════

type GenerateJWTTokenCommand struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

type GenerateJWTTokenResult struct {
	Value     string    `json:"value"`      // JWT signé
	ExpiresAt time.Time `json:"expires_at"`
	SessionID uuid.UUID `json:"session_id"`
}

type GenerateJWTTokenService struct {
	sessionRepo domain.ISessionRepo
	jwtSvc      IJWTService
}

func NewGenerateJWTTokenService(
	sessionRepo domain.ISessionRepo,
	jwtSvc IJWTService,
) *GenerateJWTTokenService {
	return &GenerateJWTTokenService{
		sessionRepo: sessionRepo,
		jwtSvc:      jwtSvc,
	}
}

func (s *GenerateJWTTokenService) Execute(
	ctx context.Context,
	cmd GenerateJWTTokenCommand,
) domain.Result[GenerateJWTTokenResult] {

	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[GenerateJWTTokenResult](
			domain.NewDomainError(domain.ErrCodeVALIDATION_FAILED, err.Error(),
				nil, domain.SeverityLOW, false))
	}

	expiresAt := time.Now().UTC().Add(24 * time.Hour) // TTL 24h — Phase 4 §SC-C-002

	// SM-17 : nouvelle session status=active
	session, err := domain.NewUserSession(cmd.UserID, domain.SessionStatusActive, expiresAt)
	if err != nil {
		return domain.Fail[GenerateJWTTokenResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur création session", nil, domain.SeverityHIGH, true))
	}

	claims := jwt.MapClaims{
		"sub": cmd.UserID.String(),
		"sid": session.ID.String(),
		"exp": expiresAt.Unix(),
		"iat": time.Now().UTC().Unix(),
	}
	token, err := s.jwtSvc.Sign(claims)
	if err != nil {
		return domain.Fail[GenerateJWTTokenResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur lors de la génération du token JWT",
				nil, domain.SeverityCRITICAL, true))
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return domain.Fail[GenerateJWTTokenResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur de persistance session", nil, domain.SeverityHIGH, true))
	}

	return domain.Ok(GenerateJWTTokenResult{
		Value:     token,
		ExpiresAt: expiresAt,
		SessionID: session.ID,
	})
}

// ════════════════════════════════════════════════════════════════════════════
// SC-C-006 — Validate2FACodeUseCase
// Phase 4 §15 SC-C-006 — Source de vérité
// Feature gate : Starter+ (Phase 6 §3)
// SM-17 : pending_2fa → active (via SC-C-002 après validation TOTP)
// ════════════════════════════════════════════════════════════════════════════

type Validate2FACodeCommand struct {
	SessionToken string `json:"session_token" validate:"required"`
	TOTPCode     string `json:"totp_code"     validate:"required,len=6"`
}

type Validate2FAResult struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Validate2FAService struct {
	userRepo       domain.IUserRepo
	sessionRepo    domain.ISessionRepo
	generateJWTSvc *GenerateJWTTokenService
	jwtSvc         IJWTService
	planFeatureSvc IPlanFeatureService
}

func NewValidate2FAService(
	userRepo domain.IUserRepo,
	sessionRepo domain.ISessionRepo,
	generateJWTSvc *GenerateJWTTokenService,
	jwtSvc IJWTService,
	planFeatureSvc IPlanFeatureService,
) *Validate2FAService {
	return &Validate2FAService{
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
		generateJWTSvc: generateJWTSvc,
		jwtSvc:         jwtSvc,
		planFeatureSvc: planFeatureSvc,
	}
}

func (s *Validate2FAService) Execute(
	ctx context.Context,
	cmd Validate2FACodeCommand,
) domain.Result[Validate2FAResult] {

	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeVALIDATION_FAILED, err.Error(),
				nil, domain.SeverityLOW, false))
	}

	// 1. Parser le token temporaire JWT pour extraire le sessionID (ANOMALIE A045 workaround)
	claims, err := s.jwtSvc.Parse(cmd.SessionToken)
	if err != nil {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Session temporaire invalide ou expirée",
				nil, domain.SeverityMEDIUM, false))
	}
	tokenType, _ := claims["type"].(string)
	if tokenType != "2fa_pending" {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Token de type incorrect pour la validation 2FA",
				nil, domain.SeverityMEDIUM, false))
	}
	sidStr, _ := claims["sid"].(string)
	sessionID, parseErr := uuid.Parse(sidStr)
	if parseErr != nil {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Session temporaire invalide",
				nil, domain.SeverityMEDIUM, false))
	}

	// 2. Charger la session et vérifier SM-17 : status = pending_2fa
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil || session.Status != domain.SessionStatusPending2FA {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Session temporaire invalide ou expirée",
				nil, domain.SeverityMEDIUM, false))
	}

	// 3. Vérifier le TTL de la session (5min — défense en profondeur post-JWT-exp)
	if time.Now().UTC().After(session.ExpiresAt) {
		session.Status = domain.SessionStatusRevoked
		session.RevokedReason = "2fa_timeout"
		_ = s.sessionRepo.Update(ctx, session)
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Session temporaire expirée — veuillez vous reconnecter",
				nil, domain.SeverityMEDIUM, false))
	}

	// 4. Charger l'utilisateur
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Utilisateur introuvable", nil, domain.SeverityHIGH, true))
	}

	// 5. Feature gate Starter+ (Phase 6 §3 — 2FA requiert plan Starter minimum)
	// Note : user.TwoFactorEnabled() = true implique déjà Starter+ lors de l'activation (SC-C-004).
	// Check explicite pour défense en profondeur (plan potentiellement rétrogradé depuis).
	canUse2FA, err := s.planFeatureSvc.UserCanUse2FA(ctx, user.ID())
	if err != nil || !canUse2FA {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINSUFFICIENT_ROLE,
				"La double authentification requiert le plan Starter ou supérieur",
				nil, domain.SeverityMEDIUM, false))
	}

	// 6. Valider le code TOTP (pquerna/otp/totp)
	// T4 canonique : user.TOTPSecret() retourne la valeur décryptée (déchiffrement au niveau repo)
	// ANOMALIE A044 : Phase 4 appelait cryptoService.Decrypt(user.TwoFASecretEnc) → incorrect post-T4
	totpSecret := ""
	if s := user.TOTPSecret(); s != nil {
		totpSecret = *s
	}
	if !totp.Validate(cmd.TOTPCode, totpSecret) {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINVALID_2FA_CODE,
				"Code TOTP invalide",
				nil, domain.SeverityMEDIUM, false))
	}

	// 7. Invalider la session temporaire (SM-17 : pending_2fa → revoked)
	session.Status = domain.SessionStatusRevoked
	session.RevokedReason = "2fa_validated"
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return domain.Fail[Validate2FAResult](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur révocation session temporaire", nil, domain.SeverityHIGH, true))
	}

	// 8. SC-C-002 : générer JWT final (SM-17 : → active)
	tokenResult := s.generateJWTSvc.Execute(ctx, GenerateJWTTokenCommand{UserID: user.ID()})
	if tokenResult.IsFail() {
		return domain.Fail[Validate2FAResult](tokenResult.Err())
	}

	return domain.Ok(Validate2FAResult{
		AccessToken: tokenResult.Value().Value,
		ExpiresAt:   tokenResult.Value().ExpiresAt,
	})
}

// ════════════════════════════════════════════════════════════════════════════
// SC-C-003 — RevokeJWTTokenUseCase
// Phase 4 §15 SC-C-003 — Source de vérité
// SM-17 : active|pending_2fa → revoked
// ════════════════════════════════════════════════════════════════════════════

type RevokeJWTTokenCommand struct {
	SessionID uuid.UUID `json:"session_id" validate:"required"`
	UserID    uuid.UUID `json:"user_id"    validate:"required"`
	Reason    string    `json:"reason"     validate:"required,oneof=logout suspended password_changed admin_revoke"`
}

type RevokeJWTTokenService struct {
	sessionRepo domain.ISessionRepo
}

func NewRevokeJWTTokenService(sessionRepo domain.ISessionRepo) *RevokeJWTTokenService {
	return &RevokeJWTTokenService{sessionRepo: sessionRepo}
}

func (s *RevokeJWTTokenService) Execute(
	ctx context.Context,
	cmd RevokeJWTTokenCommand,
) domain.Result[struct{}] {

	if err := validate.Struct(cmd); err != nil {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodeVALIDATION_FAILED, err.Error(),
				nil, domain.SeverityLOW, false))
	}

	session, err := s.sessionRepo.GetByID(ctx, cmd.SessionID)
	if err != nil {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodeINVALID_TOKEN,
				"Session introuvable",
				map[string]interface{}{"session_id": cmd.SessionID},
				domain.SeverityLOW, false))
	}

	// Vérifier que la session appartient à l'utilisateur (prévention d'accès croisé)
	if session.UserID != cmd.UserID {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodePERMISSION_DENIED,
				"Cette session n'appartient pas à cet utilisateur",
				nil, domain.SeverityHIGH, false))
	}

	// SM-17 : CanTransitionTo vérifie les transitions autorisées
	// active → revoked ✓ ; pending_2fa → revoked ✓ ; expired/revoked → revoked ✗
	if err := session.CanTransitionTo(domain.SessionStatusRevoked); err != nil {
		return domain.Fail[struct{}](err.(*domain.DomainError))
	}

	session.Status = domain.SessionStatusRevoked
	session.RevokedReason = cmd.Reason

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur de persistance", nil, domain.SeverityHIGH, true))
	}

	return domain.Ok(struct{}{})
}

// suppress unused import
var _ = events.NewUserAccountLockedEvent
