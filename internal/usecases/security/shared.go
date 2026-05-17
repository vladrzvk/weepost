package security

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/events"
)

// ════════════════════════════════════════════════════════════════════════════
// SC-C-009 — RecordFailedLoginService
// Utilisé par SC-C-001 (ValidateCredentials) et SC-C-004 (Enable2FA).
// Seuil U-2 User = 5 ≠ seuil C-4 Channel = 3 (quality rule — ne pas confondre).
// SC-C-007 (Lock 30min) est inliné ici pour éviter dépendance circulaire.
// ════════════════════════════════════════════════════════════════════════════

type RecordFailedLoginCommand struct {
	UserID uuid.UUID `validate:"required"`
	IPHash string
}

type RecordFailedLoginService struct {
	userRepo    domain.IUserRepo
	sessionRepo domain.ISessionRepo
	eventBus    domain.IEventBus
}

func NewRecordFailedLoginService(
	userRepo domain.IUserRepo,
	sessionRepo domain.ISessionRepo,
	eventBus domain.IEventBus,
) *RecordFailedLoginService {
	return &RecordFailedLoginService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		eventBus:    eventBus,
	}
}

func (s *RecordFailedLoginService) Execute(
	ctx context.Context,
	cmd RecordFailedLoginCommand,
) domain.Result[struct{}] {

	user, err := s.userRepo.GetByID(ctx, cmd.UserID)
	if err != nil {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Utilisateur introuvable lors de l'enregistrement d'échec",
				nil, domain.SeverityHIGH, true))
	}

	// U-2 : RecordFailedLogin retourne true si seuil 5 atteint (T4 canonique)
	thresholdReached := user.RecordFailedLogin()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return domain.Fail[struct{}](
			domain.NewDomainError(domain.ErrCodeINTERNAL_SERVER_ERROR,
				"Erreur de persistance", nil, domain.SeverityHIGH, true))
	}

	if thresholdReached {
		// SC-C-007 inline : Lock 30min (U-2 durée fixe non paramétrable — Phase 4 §SC-C-007)
		if lockErr := user.Lock(30 * time.Minute); lockErr == nil {
			if err := s.userRepo.Update(ctx, user); err == nil {
				// Révoquer toutes les sessions actives (cascade SM-17)
				_ = s.sessionRepo.RevokeAllByUserID(ctx, user.ID())

				// SEC-01 : sécurité cross-tenant → PublishSystem
				evt := events.NewUserAccountLockedEvent(user.ID(), user.FailedLoginAttempts(), *user.LockedUntil(), cmd.IPHash)
				_ = s.eventBus.PublishSystem(ctx, evt)
			}
		}
	}

	return domain.Ok(struct{}{})
}
