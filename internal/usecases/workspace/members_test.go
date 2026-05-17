package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/usecases/workspace"
)

// ── mocks ────────────────────────────────────────────────

type mockWorkspaceRepo struct {
	ws           *domain.Workspace
	member       *domain.WorkspaceMember
	getMemberErr error
	addMemberErr error
}

func (m *mockWorkspaceRepo) GetByID(_ context.Context, _ string) (*domain.Workspace, error) {
	if m.ws == nil {
		return nil, domain.ErrNotFound
	}
	return m.ws, nil
}
func (m *mockWorkspaceRepo) GetMember(_ context.Context, _, _ string) (*domain.WorkspaceMember, error) {
	if m.getMemberErr != nil {
		return nil, m.getMemberErr
	}
	if m.member == nil {
		return nil, domain.ErrNotFound
	}
	return m.member, nil
}
func (m *mockWorkspaceRepo) AddMember(_ context.Context, _ *domain.WorkspaceMember) error {
	return m.addMemberErr
}
func (m *mockWorkspaceRepo) ListMembers(_ context.Context, _ string) ([]*domain.WorkspaceMember, error) {
	return nil, nil
}

type mockInvitationRepo struct {
	byToken     *domain.WorkspaceInvitation
	invitations []*domain.WorkspaceInvitation
	updateErr   error
}

func (m *mockInvitationRepo) Create(_ context.Context, _ *domain.WorkspaceInvitation) error { return nil }
func (m *mockInvitationRepo) GetByToken(_ context.Context, _ string) (*domain.WorkspaceInvitation, error) {
	if m.byToken == nil {
		return nil, domain.ErrNotFound
	}
	return m.byToken, nil
}
func (m *mockInvitationRepo) Update(_ context.Context, inv *domain.WorkspaceInvitation) error {
	return m.updateErr
}
func (m *mockInvitationRepo) ListByWorkspace(_ context.Context, _ string) ([]*domain.WorkspaceInvitation, error) {
	return m.invitations, nil
}

type mockUserRepo struct {
	user         *domain.User
	byEmail      *domain.User
	getByIDErr   error
	getByEmailErr error
}

func (m *mockUserRepo) GetByID(_ context.Context, _ string) (*domain.User, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.user == nil {
		return nil, domain.ErrNotFound
	}
	return m.user, nil
}
func (m *mockUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	if m.getByEmailErr != nil {
		return nil, m.getByEmailErr
	}
	return m.byEmail, nil
}

type mockEventBus struct{}

func (m *mockEventBus) Publish(_ context.Context, _ ...domain.DomainEvent) error { return nil }

type mockPlanChecker struct{ err error }

func (m *mockPlanChecker) CheckMemberLimit(_ context.Context, _ uuid.UUID) error { return m.err }

// ── InviteMember tests ────────────────────────────────────

func TestInviteMemberUseCase(t *testing.T) {
	baseRepo := func(actorRole domain.MemberRole) *mockWorkspaceRepo {
		return &mockWorkspaceRepo{
			ws: &domain.Workspace{ID: "ws-001", Status: domain.WorkspaceStatusActive},
			member: &domain.WorkspaceMember{
				ID:     "mem-001",
				UserID: "user-001",
				Role:   actorRole,
				Status: domain.MemberStatusActive,
			},
		}
	}

	t.Run("success — owner invites editor", func(t *testing.T) {
		uc := workspace.NewInviteMemberUseCase(
			baseRepo(domain.MemberRoleOwner),
			&mockInvitationRepo{},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "owner@example.com", EmailVerified: true}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.InviteMemberCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Email: "new@example.com", Role: "editor",
		})
		assert.True(t, result.IsOk())
		assert.Equal(t, "editor", result.Value().Role)
		assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), result.Value().ExpiresAt, time.Minute)
	})

	t.Run("error — manager cannot invite admin (CANNOT_INVITE_HIGHER_ROLE)", func(t *testing.T) {
		uc := workspace.NewInviteMemberUseCase(
			baseRepo(domain.MemberRoleManager),
			&mockInvitationRepo{},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "mgr@example.com", EmailVerified: true}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.InviteMemberCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Email: "admin@example.com", Role: "admin",
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeCANNOT_INVITE_HIGHER_ROLE, result.Err().Code)
	})

	t.Run("error — invitation already pending for email", func(t *testing.T) {
		invRepo := &mockInvitationRepo{
			invitations: []*domain.WorkspaceInvitation{
				{Email: "target@example.com", Status: domain.InvitationStatusPending, ExpiresAt: time.Now().Add(time.Hour)},
			},
		}
		uc := workspace.NewInviteMemberUseCase(
			baseRepo(domain.MemberRoleAdmin),
			invRepo,
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "admin@example.com", EmailVerified: true}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.InviteMemberCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Email: "target@example.com", Role: "viewer",
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeINVITATION_ALREADY_PENDING, result.Err().Code)
	})

	t.Run("error — member role cannot invite (FORBIDDEN)", func(t *testing.T) {
		uc := workspace.NewInviteMemberUseCase(
			baseRepo(domain.MemberRoleMember),
			&mockInvitationRepo{},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "m@example.com", EmailVerified: true}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.InviteMemberCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Email: "x@example.com", Role: "editor",
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeFORBIDDEN, result.Err().Code)
	})
}

// ── AcceptInvitation tests ────────────────────────────────

func TestAcceptInvitationUseCase(t *testing.T) {
	validToken := strings.Repeat("a", 128)

	pendingInv := func(email string, expiresAt time.Time) *domain.WorkspaceInvitation {
		return &domain.WorkspaceInvitation{
			ID:          "inv-001",
			WorkspaceID: "ws-001",
			Email:       email,
			Role:        domain.MemberRoleMember,
			Token:       validToken,
			Status:      domain.InvitationStatusPending,
			ExpiresAt:   expiresAt,
		}
	}

	t.Run("success — user accepts valid invitation", func(t *testing.T) {
		repo := &mockWorkspaceRepo{
			ws:           &domain.Workspace{ID: "ws-001"},
			getMemberErr: domain.ErrNotFound,
		}
		uc := workspace.NewAcceptInvitationUseCase(
			repo,
			&mockInvitationRepo{byToken: pendingInv("user@example.com", time.Now().Add(24*time.Hour))},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "user@example.com"}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.AcceptInvitationCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Token: validToken,
		})
		assert.True(t, result.IsOk())
		assert.Equal(t, "ws-001", result.Value().WorkspaceID)
		assert.Equal(t, "editor", result.Value().Role)
	})

	t.Run("error — invitation expired", func(t *testing.T) {
		uc := workspace.NewAcceptInvitationUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}},
			&mockInvitationRepo{byToken: pendingInv("user@example.com", time.Now().Add(-time.Hour))},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "user@example.com"}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.AcceptInvitationCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Token: validToken,
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeINVITATION_EXPIRED, result.Err().Code)
	})

	t.Run("error — email mismatch (INVITATION_EMAIL_MISMATCH)", func(t *testing.T) {
		uc := workspace.NewAcceptInvitationUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, getMemberErr: domain.ErrNotFound},
			&mockInvitationRepo{byToken: pendingInv("other@example.com", time.Now().Add(time.Hour))},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "user@example.com"}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.AcceptInvitationCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Token: validToken,
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeINVITATION_EMAIL_MISMATCH, result.Err().Code)
	})

	t.Run("error — already a member", func(t *testing.T) {
		repo := &mockWorkspaceRepo{
			ws:     &domain.Workspace{ID: "ws-001"},
			member: &domain.WorkspaceMember{UserID: "user-001", Status: domain.MemberStatusActive},
		}
		uc := workspace.NewAcceptInvitationUseCase(
			repo,
			&mockInvitationRepo{byToken: pendingInv("user@example.com", time.Now().Add(time.Hour))},
			&mockUserRepo{user: &domain.User{ID: "user-001", Email: "user@example.com"}},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), workspace.AcceptInvitationCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Token: validToken,
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeALREADY_MEMBER, result.Err().Code)
	})
}

// suppress unused import
var _ = errors.New
