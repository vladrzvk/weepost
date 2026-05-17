package media_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	ucMedia "github.com/vladrzvk/weepost/internal/usecases/media"
)

type mockMediaAssetRepo struct {
	createErr     error
	getByIDResult *domain.MediaAsset
	getByIDErr    error
	updateErr     error
}

func (m *mockMediaAssetRepo) Create(ctx context.Context, a *domain.MediaAsset) error {
	return m.createErr
}
func (m *mockMediaAssetRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.MediaAsset, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockMediaAssetRepo) Update(ctx context.Context, a *domain.MediaAsset) error {
	return m.updateErr
}
func (m *mockMediaAssetRepo) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockMediaAssetRepo) ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.MediaAsset, error) {
	return nil, nil
}
func (m *mockMediaAssetRepo) ListByPost(ctx context.Context, postID uuid.UUID) ([]*domain.MediaAsset, error) {
	return nil, nil
}
func (m *mockMediaAssetRepo) ListPendingScan(ctx context.Context) ([]*domain.MediaAsset, error) {
	return nil, nil
}

type mockPostRepoMedia struct {
	getByIDResult *domain.Post
	getByIDErr    error
}

func (m *mockPostRepoMedia) Create(ctx context.Context, p *domain.Post) error { return nil }
func (m *mockPostRepoMedia) GetByID(ctx context.Context, id uuid.UUID) (*domain.Post, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockPostRepoMedia) Update(ctx context.Context, p *domain.Post) error  { return nil }
func (m *mockPostRepoMedia) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockPostRepoMedia) ListByBrand(ctx context.Context, brandID uuid.UUID, filter domain.PostFilter, page domain.PageRequest) ([]*domain.Post, domain.PageResult, error) {
	return nil, domain.PageResult{}, nil
}
func (m *mockPostRepoMedia) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, filter domain.PostFilter, page domain.PageRequest) ([]*domain.Post, domain.PageResult, error) {
	return nil, domain.PageResult{}, nil
}
func (m *mockPostRepoMedia) ListScheduledBefore(ctx context.Context, before interface{}) ([]*domain.Post, error) {
	return nil, nil
}
func (m *mockPostRepoMedia) ListFailed(ctx context.Context) ([]*domain.Post, error) { return nil, nil }

type mockBrandRepoMedia struct {
	getByIDResult       *domain.Brand
	getByIDErr          error
	getAssignmentResult *domain.BrandAssignment
	getAssignmentErr    error
}

func (m *mockBrandRepoMedia) Create(ctx context.Context, b *domain.Brand) error { return nil }
func (m *mockBrandRepoMedia) GetByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockBrandRepoMedia) Update(ctx context.Context, b *domain.Brand) error  { return nil }
func (m *mockBrandRepoMedia) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockBrandRepoMedia) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Brand, error) {
	return nil, nil
}
func (m *mockBrandRepoMedia) AddAssignment(ctx context.Context, a *domain.BrandAssignment) error {
	return nil
}
func (m *mockBrandRepoMedia) UpdateAssignment(ctx context.Context, a *domain.BrandAssignment) error {
	return nil
}
func (m *mockBrandRepoMedia) RemoveAssignment(ctx context.Context, brandID, memberID uuid.UUID) error {
	return nil
}
func (m *mockBrandRepoMedia) GetAssignment(ctx context.Context, brandID, memberID uuid.UUID) (*domain.BrandAssignment, error) {
	return m.getAssignmentResult, m.getAssignmentErr
}
func (m *mockBrandRepoMedia) ListAssignments(ctx context.Context, brandID uuid.UUID) ([]*domain.BrandAssignment, error) {
	return nil, nil
}

type mockWorkspaceRepoMedia struct {
	getMemberResult *domain.WorkspaceMember
	getMemberErr    error
}

func (m *mockWorkspaceRepoMedia) Create(ctx context.Context, w *domain.Workspace) error { return nil }
func (m *mockWorkspaceRepoMedia) GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepoMedia) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepoMedia) Update(ctx context.Context, w *domain.Workspace) error { return nil }
func (m *mockWorkspaceRepoMedia) SoftDelete(ctx context.Context, id uuid.UUID) error    { return nil }
func (m *mockWorkspaceRepoMedia) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepoMedia) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*domain.WorkspaceMember, error) {
	return m.getMemberResult, m.getMemberErr
}
func (m *mockWorkspaceRepoMedia) AddMember(ctx context.Context, mem *domain.WorkspaceMember) error {
	return nil
}
func (m *mockWorkspaceRepoMedia) UpdateMember(ctx context.Context, mem *domain.WorkspaceMember) error {
	return nil
}
func (m *mockWorkspaceRepoMedia) RemoveMember(ctx context.Context, workspaceID, memberID uuid.UUID) error {
	return nil
}
func (m *mockWorkspaceRepoMedia) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*domain.WorkspaceMember, error) {
	return nil, nil
}

type mockEventBusMedia struct{}

func (m *mockEventBusMedia) Publish(ctx context.Context, event domain.DomainEvent) error { return nil }

// ── Tests UploadMediaAssetUseCase ────────────────────────────────────────────

func TestUploadMedia_Success_PendingScan(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	post, _ := domain.NewPost(brandID, actorID, "Post with media")
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	uc := ucMedia.NewUploadMediaAssetUseCase(
		&mockMediaAssetRepo{},
		&mockPostRepoMedia{getByIDResult: post},
		&mockBrandRepoMedia{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoMedia{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockEventBusMedia{},
	)

	result := uc.Execute(context.Background(), ucMedia.UploadMediaAssetCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024 * 1024,
		StorageKey:  "workspace/brand/photo-123.jpg",
	})

	if result.IsFail() {
		t.Fatalf("expected success, got: %v", result.Err())
	}
	// M-1 : A016 — status initial doit être pending_scan
	if result.Value().Status != "pending_scan" {
		t.Errorf("expected pending_scan (A016/M-1), got %s", result.Value().Status)
	}
}

func TestUploadMedia_InvalidMIME(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	post, _ := domain.NewPost(brandID, actorID, "Post")
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Brand")

	uc := ucMedia.NewUploadMediaAssetUseCase(
		&mockMediaAssetRepo{},
		&mockPostRepoMedia{getByIDResult: post},
		&mockBrandRepoMedia{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoMedia{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockEventBusMedia{},
	)

	// PDF non supporté — T4 NewMediaAsset valide le MIME
	result := uc.Execute(context.Background(), ucMedia.UploadMediaAssetCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		Filename:    "doc.pdf",
		ContentType: "application/pdf",
		SizeBytes:   512 * 1024,
		StorageKey:  "workspace/brand/doc.pdf",
	})

	if result.IsOk() {
		t.Fatal("expected INVALID_MEDIA_TYPE pour PDF")
	}
}

// ── Tests QuarantineMediaUseCase ─────────────────────────────────────────────

func TestQuarantine_Success(t *testing.T) {
	brandID := uuid.New()
	asset, _ := domain.NewMediaAsset(
		uuid.New(), uuid.New(),
		"photo.jpg", "photo.jpg",
		"bucket/photo.jpg", "weepost-media",
		"image/jpeg", 1024,
	)
	asset.BrandID = &brandID

	uc := ucMedia.NewQuarantineMediaUseCase(
		&mockMediaAssetRepo{getByIDResult: asset},
		&mockEventBusMedia{},
	)

	result := uc.Execute(context.Background(), ucMedia.QuarantineMediaCommand{
		MediaAssetID:    asset.ID,
		Reason:          "Virus détecté : Win.Malware.Eicar",
		AntivirusEngine: "ClamAV 1.0",
		ThreatName:      "Win.Malware.Eicar",
	})

	if result.IsFail() {
		t.Fatalf("expected success, got: %v", result.Err())
	}
	if result.Value().Status != "quarantined" {
		t.Errorf("expected quarantined, got %s", result.Value().Status)
	}
}

func TestQuarantine_Idempotent_AlreadyQuarantined(t *testing.T) {
	asset, _ := domain.NewMediaAsset(
		uuid.New(), uuid.New(),
		"photo.jpg", "photo.jpg",
		"bucket/photo.jpg", "weepost-media",
		"image/jpeg", 1024,
	)
	asset.Quarantine("ClamAV") // déjà quarantiné

	uc := ucMedia.NewQuarantineMediaUseCase(
		&mockMediaAssetRepo{getByIDResult: asset},
		&mockEventBusMedia{},
	)

	result := uc.Execute(context.Background(), ucMedia.QuarantineMediaCommand{
		MediaAssetID:    asset.ID,
		Reason:          "Double scan",
		AntivirusEngine: "ClamAV 1.0",
		ThreatName:      "Win.Malware.Test",
	})

	if result.IsFail() {
		t.Fatalf("expected idempotent success, got: %v", result.Err())
	}
	if result.Value().Status != "quarantined" {
		t.Errorf("expected quarantined, got %s", result.Value().Status)
	}
}

func TestQuarantine_ReasonEmpty_Fails(t *testing.T) {
	uc := ucMedia.NewQuarantineMediaUseCase(
		&mockMediaAssetRepo{},
		&mockEventBusMedia{},
	)

	result := uc.Execute(context.Background(), ucMedia.QuarantineMediaCommand{
		MediaAssetID:    uuid.New(),
		Reason:          "", // vide → validate:"required"
		AntivirusEngine: "ClamAV",
		ThreatName:      "Eicar",
	})

	if result.IsOk() {
		t.Fatal("expected VALIDATION_FAILED pour reason vide")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeVALIDATION_FAILED {
		t.Errorf("expected VALIDATION_FAILED, got %s", domErr.Code)
	}
}
