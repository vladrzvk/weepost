// internal/usecases/workspace/create_test.go
package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	wuc "github.com/vladrzvk/weepost/internal/usecases/workspace"
)

// ---- Mocks ----

type mockWorkspaceRepo struct {
	createFn    func(ctx context.Context, w *domain.Workspace) error
	getBySlugFn func(ctx context.Context, slug string) (*domain.Workspace, error)
	// Autres méthodes : implémentations vides pour satisfaire IWorkspaceRepo
	getByIDFn     func(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	updateFn      func(ctx context.Context, w *domain.Workspace) error
	softDeleteFn  func(ctx context.Context, id uuid.UUID) error
	listByUserIDFn func(ctx context.Context, userID uuid.UUID) ([]*domain.Workspace, error)
	getMemberFn   func(ctx context.Context, wID, uID uuid.UUID) (*domain.WorkspaceMember, error)
	addMemberFn   func(ctx context.Context, m *domain.WorkspaceMember) error
	updateMemberFn func(ctx context.Context, m *domain.WorkspaceMember) error
	removeMemberFn func(ctx context.Context, wID, mID uuid.UUID) error
	listMembersFn  func(ctx context.Context, wID uuid.UUID) ([]*domain.WorkspaceMember, error)
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, w *domain.Workspace) error {
	if m.createFn != nil { return m.createFn(ctx, w) }
	return nil
}
func (m *mockWorkspaceRepo) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	if m.getBySlugFn != nil { return m.getBySlugFn(ctx, slug) }
	return nil, wuc.ErrNotFound
}
func (m *mockWorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	if m.getByIDFn != nil { return m.getByIDFn(ctx, id) }
	return nil, wuc.ErrNotFound
}
func (m *mockWorkspaceRepo) Update(ctx context.Context, w *domain.Workspace) error {
	if m.updateFn != nil { return m.updateFn(ctx, w) }
	return nil
}
func (m *mockWorkspaceRepo) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockWorkspaceRepo) ListByUserID(ctx context.Context, uID uuid.UUID) ([]*domain.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) GetMember(ctx context.Context, wID, uID uuid.UUID) (*domain.WorkspaceMember, error) {
	if m.getMemberFn != nil { return m.getMemberFn(ctx, wID, uID) }
	return nil, wuc.ErrNotFound
}
func (m *mockWorkspaceRepo) AddMember(ctx context.Context, mem *domain.WorkspaceMember) error { return nil }
func (m *mockWorkspaceRepo) UpdateMember(ctx context.Context, mem *domain.WorkspaceMember) error { return nil }
func (m *mockWorkspaceRepo) RemoveMember(ctx context.Context, wID, mID uuid.UUID) error { return nil }
func (m *mockWorkspaceRepo) ListMembers(ctx context.Context, wID uuid.UUID) ([]*domain.WorkspaceMember, error) {
	if m.listMembersFn != nil { return m.listMembersFn(ctx, wID) }
	return nil, nil
}

type mockEventBus struct{}

func (m *mockEventBus) Publish(_ context.Context, _ ...domain.DomainEvent) error       { return nil }
func (m *mockEventBus) PublishSystem(_ context.Context, _ ...domain.DomainEvent) error { return nil }
func (m *mockEventBus) Subscribe(_ string, _ func(domain.DomainEvent)) error           { return nil }

// ---- Tests ----

func TestCreateWorkspace_Success(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Workspace, error) {
			return nil, wuc.ErrNotFound // slug libre
		},
		createFn: func(_ context.Context, _ *domain.Workspace) error { return nil },
	}

	uc := wuc.NewCreateWorkspaceUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.CreateWorkspaceCommand{
		OwnerID:      uuid.New(),
		Name:         "Acme Corp",
		Timezone:     "Europe/Paris",
		Language:     "fr",
		DateFormat:   "DD/MM/YYYY",
		TimeFormat:   "24h",
		WeekStartDay: 1,
	})

	if result.IsFail() {
		t.Fatalf("expected success, got error: %v", result.Err())
	}
	if result.Value().WorkspaceID == uuid.Nil {
		t.Error("WorkspaceID should not be nil")
	}
	if result.Value().Slug == "" {
		t.Error("Slug should be generated")
	}
}

func TestCreateWorkspace_SlugAlreadyExists(t *testing.T) {
	existingWS, _ := domain.NewWorkspace(uuid.New(), "Acme Corp", "Europe/Paris", "fr")
	repo := &mockWorkspaceRepo{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Workspace, error) {
			return existingWS, nil // slug occupé
		},
	}

	uc := wuc.NewCreateWorkspaceUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.CreateWorkspaceCommand{
		OwnerID:  uuid.New(),
		Name:     "Acme Corp",
		Timezone: "Europe/Paris",
		Language: "fr",
	})

	if result.IsOk() {
		t.Fatal("expected failure for duplicate slug")
	}
	if result.Err().Code != domain.ErrCodeSLUG_ALREADY_EXISTS {
		t.Errorf("expected SLUG_ALREADY_EXISTS, got %s", result.Err().Code)
	}
}

func TestCreateWorkspace_InvalidSettings(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getBySlugFn: func(_ context.Context, _ string) (*domain.Workspace, error) {
			return nil, wuc.ErrNotFound
		},
	}

	uc := wuc.NewCreateWorkspaceUseCase(repo, &mockEventBus{})
	result := uc.Execute(context.Background(), wuc.CreateWorkspaceCommand{
		OwnerID:  uuid.New(),
		Name:     "Valid Name",
		Timezone: "Invalid/Timezone", // invalide — isValidIANATimezone retourne false
		Language: "fr",
	})

	if result.IsOk() {
		t.Fatal("expected failure for invalid timezone")
	}
	if result.Err().Code != domain.ErrCodeINVALID_INPUT {
		t.Errorf("expected INVALID_INPUT, got %s", result.Err().Code)
	}
}

// Suppress unused import error
var _ = errors.New

