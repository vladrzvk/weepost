package brand_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/vladrzvk/weepost/internal/domain"
	"github.com/vladrzvk/weepost/internal/usecases/brand"
)

// ── mocks ────────────────────────────────────────────────

type mockWorkspaceRepo struct {
	ws           *domain.Workspace
	member       *domain.WorkspaceMember
	getMemberErr error
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
func (m *mockWorkspaceRepo) AddMember(_ context.Context, _ *domain.WorkspaceMember) error { return nil }
func (m *mockWorkspaceRepo) ListMembers(_ context.Context, _ string) ([]*domain.WorkspaceMember, error) {
	return nil, nil
}

type mockBrandRepo struct {
	brand         *domain.Brand
	brands        []*domain.Brand
	assignment    *domain.BrandMemberAssignment
	getByIDErr    error
	createErr     error
	updateErr     error
	getAssignErr  error
}

func (m *mockBrandRepo) GetByID(_ context.Context, _ string) (*domain.Brand, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.brand == nil {
		return nil, domain.ErrNotFound
	}
	return m.brand, nil
}
func (m *mockBrandRepo) Create(_ context.Context, _ *domain.Brand) error       { return m.createErr }
func (m *mockBrandRepo) Update(_ context.Context, _ *domain.Brand) error       { return m.updateErr }
func (m *mockBrandRepo) ListByWorkspace(_ context.Context, _ string) ([]*domain.Brand, error) {
	return m.brands, nil
}
func (m *mockBrandRepo) AddAssignment(_ context.Context, _ *domain.BrandMemberAssignment) error {
	return nil
}
func (m *mockBrandRepo) RemoveAssignment(_ context.Context, _, _ string) error { return nil }
func (m *mockBrandRepo) GetAssignment(_ context.Context, _, _ string) (*domain.BrandMemberAssignment, error) {
	if m.getAssignErr != nil {
		return nil, m.getAssignErr
	}
	return m.assignment, nil
}
func (m *mockBrandRepo) ListAssignments(_ context.Context, _ string) ([]*domain.BrandMemberAssignment, error) {
	return nil, nil
}

type mockUserRepo struct{ user *domain.User }

func (m *mockUserRepo) GetByID(_ context.Context, _ string) (*domain.User, error) {
	if m.user == nil {
		return nil, domain.ErrNotFound
	}
	return m.user, nil
}
func (m *mockUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

type mockEventBus struct{}

func (m *mockEventBus) Publish(_ context.Context, _ ...domain.DomainEvent) error { return nil }

type mockPlanChecker struct{ brandErr error }

func (m *mockPlanChecker) CheckMemberLimit(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockPlanChecker) CheckBrandLimit(_ context.Context, _ uuid.UUID) error  { return m.brandErr }

// ── CreateBrand tests ─────────────────────────────────────

func TestCreateBrandUseCase(t *testing.T) {
	ownerMember := &domain.WorkspaceMember{ID: "mem-001", UserID: "user-001", Role: domain.MemberRoleOwner, Status: domain.MemberStatusActive}
	verifiedUser := &domain.User{ID: "user-001", Email: "owner@example.com", EmailVerified: true}

	t.Run("success — owner creates brand", func(t *testing.T) {
		uc := brand.NewCreateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: ownerMember},
			&mockBrandRepo{},
			&mockUserRepo{user: verifiedUser},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), brand.CreateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Name: "My Brand",
		})
		assert.True(t, result.IsOk())
		assert.Equal(t, "My Brand", result.Value().Name)
		assert.Equal(t, "my-brand", result.Value().Slug)
	})

	t.Run("error — slug already exists (BRAND_SLUG_ALREADY_EXISTS)", func(t *testing.T) {
		existing := &domain.Brand{ID: "brand-other", Slug: "my-brand", WorkspaceID: "ws-001", Status: domain.BrandStatusActive}
		uc := brand.NewCreateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: ownerMember},
			&mockBrandRepo{brands: []*domain.Brand{existing}},
			&mockUserRepo{user: verifiedUser},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), brand.CreateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Name: "My Brand",
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeBRAND_SLUG_ALREADY_EXISTS, result.Err().Code)
	})

	t.Run("error — member role cannot create brand (FORBIDDEN)", func(t *testing.T) {
		memberRole := &domain.WorkspaceMember{ID: "mem-002", UserID: "user-001", Role: domain.MemberRoleMember, Status: domain.MemberStatusActive}
		uc := brand.NewCreateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: memberRole},
			&mockBrandRepo{},
			&mockUserRepo{user: verifiedUser},
			&mockEventBus{},
			&mockPlanChecker{},
		)
		result := uc.Execute(context.Background(), brand.CreateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), Name: "New Brand",
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeFORBIDDEN, result.Err().Code)
	})
}

// ── UpdateBrand tests ─────────────────────────────────────

func ptr(s string) *string { return &s }

func TestUpdateBrandUseCase(t *testing.T) {
	activeBrand := &domain.Brand{
		ID:          "brand-001",
		WorkspaceID: "ws-001",
		Name:        "Old Name",
		Slug:        "old-name",
		Status:      domain.BrandStatusActive,
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	ownerMember := &domain.WorkspaceMember{ID: "mem-001", UserID: "user-001", Role: domain.MemberRoleOwner}

	t.Run("success — owner renames brand", func(t *testing.T) {
		uc := brand.NewUpdateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: ownerMember},
			&mockBrandRepo{brand: activeBrand},
			&mockEventBus{},
		)
		result := uc.Execute(context.Background(), brand.UpdateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), BrandID: uuid.MustParse("00000000-0000-0000-0002-000000000001"), Name: ptr("New Name"),
		})
		assert.True(t, result.IsOk())
		assert.Equal(t, "new-name", result.Value().Slug)
	})

	t.Run("success — owner archives brand (SM-06 active→archived)", func(t *testing.T) {
		uc := brand.NewUpdateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: ownerMember},
			&mockBrandRepo{brand: activeBrand},
			&mockEventBus{},
		)
		result := uc.Execute(context.Background(), brand.UpdateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), BrandID: uuid.MustParse("00000000-0000-0000-0002-000000000001"), Status: ptr("archived"),
		})
		assert.True(t, result.IsOk())
		assert.Equal(t, "archived", result.Value().Status)
	})

	t.Run("error — admin without brand assignment (FORBIDDEN)", func(t *testing.T) {
		adminMember := &domain.WorkspaceMember{ID: "mem-002", UserID: "user-002", Role: domain.MemberRoleAdmin}
		uc := brand.NewUpdateBrandUseCase(
			&mockWorkspaceRepo{ws: &domain.Workspace{ID: "ws-001"}, member: adminMember},
			&mockBrandRepo{brand: activeBrand, getAssignErr: domain.ErrNotFound},
			&mockEventBus{},
		)
		result := uc.Execute(context.Background(), brand.UpdateBrandCommand{
			ActorID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WorkspaceID: uuid.MustParse("00000000-0000-0000-0001-000000000001"), BrandID: uuid.MustParse("00000000-0000-0000-0002-000000000001"), Name: ptr("New"),
		})
		assert.False(t, result.IsOk())
		assert.Equal(t, domain.ErrCodeFORBIDDEN, result.Err().Code)
	})
}
