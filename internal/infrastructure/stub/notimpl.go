package stub

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	brandUC   "github.com/vladrzvk/weepost/internal/usecases/brand"
	channelUC "github.com/vladrzvk/weepost/internal/usecases/channel"
	postUC    "github.com/vladrzvk/weepost/internal/usecases/post"
	secUC     "github.com/vladrzvk/weepost/internal/usecases/security"
	wsUC      "github.com/vladrzvk/weepost/internal/usecases/workspace"
)

func notImplErr() *domain.DomainError {
	return domain.NewDomainError(domain.ErrCodeNOT_IMPLEMENTED, "not implemented", nil, domain.SeverityLOW, false)
}

// ── Plan checkers (no-op — toutes les limites acceptées en local) ──────────

// NoOpWorkspacePlanChecker satisfait workspace.PlanLimitsChecker.
type NoOpWorkspacePlanChecker struct{}

func (NoOpWorkspacePlanChecker) CheckMemberLimit(_ context.Context, _ uuid.UUID) error { return nil }

// NoOpBrandPlanChecker satisfait brand.PlanLimitsChecker.
type NoOpBrandPlanChecker struct{}

func (NoOpBrandPlanChecker) CheckMemberLimit(_ context.Context, _ uuid.UUID) error { return nil }
func (NoOpBrandPlanChecker) CheckBrandLimit(_ context.Context, _ uuid.UUID) error  { return nil }

// NoOpChannelPlanChecker satisfait channel.IPlanLimitsCheckerChannel.
type NoOpChannelPlanChecker struct{}

func (NoOpChannelPlanChecker) CheckChannelLimit(_ context.Context, _, _ uuid.UUID) error { return nil }

// ── Channel credential repo (no-op) ──────────────────────────────────────

// NoOpChannelCredentialRepo satisfait channel.IChannelCredentialRepo.
type NoOpChannelCredentialRepo struct{}

func (NoOpChannelCredentialRepo) Create(_ context.Context, _ uuid.UUID, _, _ string, _ *time.Time) error {
	return nil
}
func (NoOpChannelCredentialRepo) Update(_ context.Context, _ uuid.UUID, _, _ string, _ *time.Time) error {
	return nil
}
func (NoOpChannelCredentialRepo) DeleteByChannelID(_ context.Context, _ uuid.UUID) error { return nil }

// ── Plan feature service (no-op) ─────────────────────────────────────────

// NoOpPlanFeatureService satisfait security.IPlanFeatureService.
type NoOpPlanFeatureService struct{}

func (NoOpPlanFeatureService) UserCanUse2FA(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

// ── Security stubs ────────────────────────────────────────────────────────

type Enable2FAStub struct{}

func (Enable2FAStub) Execute(_ context.Context, _ secUC.Enable2FACommand) domain.Result[secUC.Enable2FAResult] {
	return domain.Fail[secUC.Enable2FAResult](notImplErr())
}

type Disable2FAStub struct{}

func (Disable2FAStub) Execute(_ context.Context, _ secUC.Disable2FACommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type SendPasswordResetStub struct{}

func (SendPasswordResetStub) Execute(_ context.Context, _ secUC.SendPasswordResetCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type ValidatePasswordResetStub struct{}

func (ValidatePasswordResetStub) Execute(_ context.Context, _ secUC.ValidatePasswordResetCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type UnlockUserStub struct{}

func (UnlockUserStub) Execute(_ context.Context, _ secUC.UnlockUserAccountCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type RotateKeysStub struct{}

func (RotateKeysStub) Execute(_ context.Context, _ secUC.RotateEncryptionKeysCommand) domain.Result[secUC.RotateEncryptionKeysResult] {
	return domain.Fail[secUC.RotateEncryptionKeysResult](notImplErr())
}

// ── Channel stubs ─────────────────────────────────────────────────────────

type DisconnectChannelStub struct{}

func (DisconnectChannelStub) Execute(_ context.Context, _ channelUC.DisconnectChannelCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

// ── Post stubs ────────────────────────────────────────────────────────────

type CreatePostStub struct{}

func (CreatePostStub) Execute(_ context.Context, _ postUC.CreatePostCommand) domain.Result[postUC.CreatePostResult] {
	return domain.Fail[postUC.CreatePostResult](notImplErr())
}

type SubmitPostStub struct{}

func (SubmitPostStub) Execute(_ context.Context, _ postUC.SubmitPostCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type ValidatePostStub struct{}

func (ValidatePostStub) Execute(_ context.Context, _ postUC.ValidatePostCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type RejectPostStub struct{}

func (RejectPostStub) Execute(_ context.Context, _ postUC.RejectPostCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type SchedulePostStub struct{}

func (SchedulePostStub) Execute(_ context.Context, _ postUC.SchedulePostCommand) domain.Result[postUC.SchedulePostResult] {
	return domain.Fail[postUC.SchedulePostResult](notImplErr())
}

type PublishPostStub struct{}

func (PublishPostStub) Execute(_ context.Context, _ postUC.PublishPostCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

type CancelScheduleStub struct{}

func (CancelScheduleStub) Execute(_ context.Context, _ postUC.CancelScheduleCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

// ── Workspace stubs ───────────────────────────────────────────────────────

type AcceptInvitationStub struct{}

func (AcceptInvitationStub) Execute(_ context.Context, _ wsUC.AcceptInvitationCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}

// ── Brand stubs ───────────────────────────────────────────────────────────

type RevokeBrandAccessStub struct{}

func (RevokeBrandAccessStub) Execute(_ context.Context, _ brandUC.RevokeBrandAccessCommand) domain.Result[struct{}] {
	return domain.Fail[struct{}](notImplErr())
}
