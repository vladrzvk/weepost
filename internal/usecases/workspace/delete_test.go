// internal/usecases/workspace/delete_test.go
package workspace_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	wuc "github.com/vladrzvk/weepost/internal/usecases/workspace"
)

type mockBrandRepo struct {
	listByWorkspaceFn func(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Brand, error)
	updateFn          func(ctx context.Context, b *domain.Brand) error
}

func (m *mockBrandRepo) Create(_ context.Context, _ *domain.Brand) error  { return nil }
func (m *mockBrandRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Brand, error) { return nil, nil }
func (m *mockBrandRepo) Update(ctx context.Context, b *domain.Brand) error {
	if m.updateFn != nil { return m.updateFn(ctx, b) }
	return nil
}
func (m *mockBrandRepo) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockBrandRepo) ListByWorkspace(ctx context.Context, wID uuid.UUID) ([]*domain.Brand, error) {
	if m.listByWorkspaceFn != nil { return m.listByWorkspaceFn(ctx, wID) }
	return nil, nil
}
func (m *mockBrandRepo) AddAssignment(_ context.Context, _ *domain.BrandAssignment) error    { return nil }
func (m *mockBrandRepo) UpdateAssignment(_ context.Context, _ *domain.BrandAssignment) error { return nil }
func (m *mockBrandRepo) RemoveAssignment(_ context.Context, _, _ uuid.UUID) error            { return nil }
func (m *mockBrandRepo) GetAssignment(_ context.Context, _, _ uuid.UUID) (*domain.BrandAssignment, error) { return nil, nil }
func (m *mockBrandRepo) ListAssignments(_ context.Context, _ uuid.UUID) ([]*domain.BrandAssignment, error) { return nil, nil }

type mockSessionRepo struct{}

func (m *mockSessionRepo) Create(_ context.Context, _ *domain.UserSession) error                    { return nil }
func (m *mockSessionRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.UserSession, error)     { return nil, nil }
func (m *mockSessionRepo) Update(_ context.Context, _ *domain.UserSession) error                   { return nil }
func (m *mockSessionRepo) RevokeAllByUserID(_ context.Context, _ uuid.UUID) error                  { return nil }
func (m *mockSessionRepo) DeleteExpired(_ context.Context) error                                    { return nil }

func TestDeleteWorkspace_Success(t *testing.T) {
	ownerID := uuid.New()
	workspaceID := uuid.New()
	ws := buildActiveWorkspace(ownerID)

	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getMemberFn: func(_ context.Context, _, _ uuid.UUID) (*domain.WorkspaceMember, error) {
			return buildOwnerMember(ownerID, workspaceID), nil
		},
		listMembersFn: func(_ context.Context, _ uuid.UUID) ([]*domain.WorkspaceMember, error) {
			return []*domain.WorkspaceMember{buildOwnerMember(ownerID, workspaceID)}, nil
		},
	}

	uc := wuc.NewDeleteWorkspaceUseCase(repo, &mockBrandRepo{}, &mockSessionRepo{}, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.DeleteWorkspaceCommand{
		WorkspaceID: workspaceID,
		ActorID:     ownerID,
		Confirm:     true,
	})

	if result.IsFail() {
		t.Fatalf("expected success, got: %v", result.Err())
	}
}

func TestDeleteWorkspace_NotOwner(t *testing.T) {
	ownerID := uuid.New()
	adminID := uuid.New()
	workspaceID := uuid.New()
	ws := buildActiveWorkspace(ownerID)

	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) { return ws, nil },
		getMemberFn: func(_ context.Context, _, _ uuid.UUID) (*domain.WorkspaceMember, error) {
			return &domain.WorkspaceMember{
				UserID:      adminID,
				WorkspaceID: workspaceID,
				Role:        domain.MemberRoleAdmin, // Admin — insuffisant
				Status:      domain.MemberStatusActive,
			}, nil
		},
	}

	uc := wuc.NewDeleteWorkspaceUseCase(repo, &mockBrandRepo{}, &mockSessionRepo{}, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.DeleteWorkspaceCommand{
		WorkspaceID: workspaceID,
		ActorID:     adminID,
		Confirm:     true,
	})

	if result.IsOk() {
		t.Fatal("expected failure for Admin actor")
	}
	if result.Err().Code != domain.ErrCodeNOT_WORKSPACE_OWNER {
		t.Errorf("expected NOT_WORKSPACE_OWNER, got %s", result.Err().Code)
	}
}

func TestDeleteWorkspace_NoConfirmation(t *testing.T) {
	ownerID := uuid.New()
	workspaceID := uuid.New()

	uc := wuc.NewDeleteWorkspaceUseCase(&mockWorkspaceRepo{}, &mockBrandRepo{}, &mockSessionRepo{}, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.DeleteWorkspaceCommand{
		WorkspaceID: workspaceID,
		ActorID:     ownerID,
		Confirm:     false, // non confirmé
	})

	if result.IsOk() {
		t.Fatal("expected failure without confirmation")
	}
	if result.Err().Code != domain.ErrCodeINVALID_INPUT {
		t.Errorf("expected INVALID_INPUT, got %s", result.Err().Code)
	}
}

