package post_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	ucPost "github.com/vladrzvk/weepost/internal/usecases/post"
)

func scheduledPost(brandID, authorID uuid.UUID) *domain.Post {
	p, _ := domain.NewPost(brandID, authorID, "Scheduled Post")
	_ = p.SubmitForValidation()
	_ = p.Validate(uuid.New())
	_ = p.Schedule(time.Now().UTC().Add(30 * time.Minute))
	return p
}

func buildCancelUC(
	postRepo *mockPostRepo,
	brandRepo *mockBrandRepoPost,
	workspaceRepo *mockWorkspaceRepoPost,
) *ucPost.CancelScheduledPostUseCase {
	return ucPost.NewCancelScheduledPostUseCase(
		postRepo,
		brandRepo,
		workspaceRepo,
		&mockEventBusPost{},
	)
}

// ── Tests CancelScheduledPostUseCase ────────────────────────────────────────

func TestCancelScheduled_AuthorCanCancel(t *testing.T) {
	authorID := uuid.New()
	brandID := uuid.New()
	post := scheduledPost(brandID, authorID)

	uc := buildCancelUC(
		&mockPostRepo{getByIDResult: post},
		&mockBrandRepoPost{}, // pas consulté (auteur court-circuite)
		&mockWorkspaceRepoPost{},
	)

	result := uc.Execute(context.Background(), ucPost.CancelScheduledPostCommand{
		PostID:  post.ID(),
		ActorID: authorID, // même acteur que l'auteur
	})

	if result.IsFail() {
		t.Fatalf("auteur doit pouvoir annuler sa propre planification, got: %v", result.Err())
	}
	if result.Value().Status != "draft" {
		t.Errorf("expected draft (scheduled→draft), got %s", result.Value().Status)
	}
}

func TestCancelScheduled_ManagerCanCancel_OtherAuthor(t *testing.T) {
	authorID := uuid.New()
	managerID := uuid.New()
	brandID := uuid.New()
	workspaceID := uuid.New()
	post := scheduledPost(brandID, authorID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Brand")

	uc := buildCancelUC(
		&mockPostRepo{getByIDResult: post},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleManager},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{
				ID:   managerID,
				Role: domain.MemberRoleManager,
			},
		},
	)

	result := uc.Execute(context.Background(), ucPost.CancelScheduledPostCommand{
		PostID:  post.ID(),
		ActorID: managerID, // Manager annule le post d'un Editor
	})

	if result.IsFail() {
		t.Fatalf("Manager doit pouvoir annuler le post d'un autre membre, got: %v", result.Err())
	}
}

func TestCancelScheduled_EditorDenied_OtherAuthor(t *testing.T) {
	authorID := uuid.New()
	editorID := uuid.New()
	brandID := uuid.New()
	workspaceID := uuid.New()
	post := scheduledPost(brandID, authorID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Brand")

	uc := buildCancelUC(
		&mockPostRepo{getByIDResult: post},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{
				ID:   editorID,
				Role: domain.MemberRoleEditor,
			},
		},
	)

	// Editor ne peut pas annuler le post d'un autre member
	result := uc.Execute(context.Background(), ucPost.CancelScheduledPostCommand{
		PostID:  post.ID(),
		ActorID: editorID,
	})

	if result.IsOk() {
		t.Fatal("expected PERMISSION_DENIED pour Editor annulant post d'un autre membre")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodePERMISSION_DENIED {
		t.Errorf("expected PERMISSION_DENIED, got %s", domErr.Code)
	}
}

func TestCancelScheduled_NotScheduled_Fails(t *testing.T) {
	authorID := uuid.New()
	brandID := uuid.New()
	// Post validé, pas encore scheduled
	validatedPost := validatedPost(brandID, authorID)

	uc := buildCancelUC(
		&mockPostRepo{getByIDResult: validatedPost},
		&mockBrandRepoPost{},
		&mockWorkspaceRepoPost{},
	)

	result := uc.Execute(context.Background(), ucPost.CancelScheduledPostCommand{
		PostID:  validatedPost.ID(),
		ActorID: authorID,
	})

	if result.IsOk() {
		t.Fatal("expected INVALID_STATUS_TRANSITION (validated n'est pas scheduled)")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeINVALID_STATUS_TRANSITION {
		t.Errorf("expected INVALID_STATUS_TRANSITION, got %s", domErr.Code)
	}
}
