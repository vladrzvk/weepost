// internal/usecases/workspace/update_test.go
package workspace_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	wuc "github.com/vladrzvk/weepost/internal/usecases/workspace"
)

func buildActiveWorkspace(ownerID uuid.UUID) *domain.Workspace {
	ws, _ := domain.NewWorkspace(ownerID, "Test Workspace", "Europe/Paris", "fr")
	return ws
}

func buildOwnerMember(ownerID, workspaceID uuid.UUID) *domain.WorkspaceMember {
	return &domain.WorkspaceMember{
		UserID:      ownerID,
		WorkspaceID: workspaceID,
		Role:        domain.MemberRoleOwner,
		Status:      domain.MemberStatusActive,
	}
}

func buildViewerMember(viewerID, workspaceID uuid.UUID) *domain.WorkspaceMember {
	return &domain.WorkspaceMember{
		UserID:      viewerID,
		WorkspaceID: workspaceID,
		Role:        domain.MemberRoleViewer,
		Status:      domain.MemberStatusActive,
	}
}

func TestUpdateWorkspace_Success(t *testing.T) {
	ownerID := uuid.New()
	workspaceID := uuid.New()
	ws := buildActiveWorkspace(ownerID)

	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return ws, nil
		},
		getMemberFn: func(_ context.Context, _, _ uuid.UUID) (*domain.WorkspaceMember, error) {
			return buildOwnerMember(ownerID, workspaceID), nil
		},
	}

	newName := "New Name"
	uc := wuc.NewUpdateWorkspaceUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.UpdateWorkspaceCommand{
		WorkspaceID: workspaceID,
		ActorID:     ownerID,
		Name:        &newName,
	})

	if result.IsFail() {
		t.Fatalf("expected success, got: %v", result.Err())
	}
}

func TestUpdateWorkspace_Unauthorized(t *testing.T) {
	ownerID := uuid.New()
	viewerID := uuid.New()
	workspaceID := uuid.New()
	ws := buildActiveWorkspace(ownerID)

	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return ws, nil
		},
		getMemberFn: func(_ context.Context, _, _ uuid.UUID) (*domain.WorkspaceMember, error) {
			return buildViewerMember(viewerID, workspaceID), nil
		},
	}

	newName := "Unauthorized Update"
	uc := wuc.NewUpdateWorkspaceUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.UpdateWorkspaceCommand{
		WorkspaceID: workspaceID,
		ActorID:     viewerID,
		Name:        &newName,
	})

	if result.IsOk() {
		t.Fatal("expected failure for Viewer actor")
	}
	if result.Err().Code != domain.ErrCodeINSUFFICIENT_PERMISSIONS {
		t.Errorf("expected INSUFFICIENT_PERMISSIONS, got %s", result.Err().Code)
	}
}

func TestUpdateWorkspaceSettings_InvalidTimezone(t *testing.T) {
	ownerID := uuid.New()
	workspaceID := uuid.New()
	ws := buildActiveWorkspace(ownerID)

	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
			return ws, nil
		},
		getMemberFn: func(_ context.Context, _, _ uuid.UUID) (*domain.WorkspaceMember, error) {
			return buildOwnerMember(ownerID, workspaceID), nil
		},
	}

	invalidTZ := "Invalid/Timezone"
	uc := wuc.NewUpdateWorkspaceSettingsUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.UpdateWorkspaceSettingsCommand{
		WorkspaceID: workspaceID,
		ActorID:     ownerID,
		Timezone:    &invalidTZ,
	})

	if result.IsOk() {
		t.Fatal("expected failure for invalid timezone")
	}
	if result.Err().Code != domain.ErrCodeINVALID_INPUT {
		t.Errorf("expected INVALID_INPUT, got %s", result.Err().Code)
	}
}

