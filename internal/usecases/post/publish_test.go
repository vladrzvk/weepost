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

// ── Mocks SchedulePost ───────────────────────────────────────────────────────

type mockPostVariantRepo struct {
	createErr        error
	listByPostResult []*domain.PostVariant
	listByPostErr    error
	updateErr        error
}

func (m *mockPostVariantRepo) Create(ctx context.Context, v *domain.PostVariant) error {
	return m.createErr
}
func (m *mockPostVariantRepo) ListByPost(ctx context.Context, postID uuid.UUID) ([]*domain.PostVariant, error) {
	return m.listByPostResult, m.listByPostErr
}
func (m *mockPostVariantRepo) Update(ctx context.Context, v *domain.PostVariant) error {
	return m.updateErr
}
func (m *mockPostVariantRepo) DeleteByPost(ctx context.Context, postID uuid.UUID) error { return nil }

type mockChannelRepoPublish struct {
	getByIDResult *domain.Channel
	getByIDErr    error
	updateErr     error
}

func (m *mockChannelRepoPublish) Create(ctx context.Context, c *domain.Channel) error { return nil }
func (m *mockChannelRepoPublish) GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockChannelRepoPublish) Update(ctx context.Context, c *domain.Channel) error {
	return m.updateErr
}
func (m *mockChannelRepoPublish) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockChannelRepoPublish) ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.Channel, error) {
	return nil, nil
}
func (m *mockChannelRepoPublish) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Channel, error) {
	return nil, nil
}
func (m *mockChannelRepoPublish) GetByPlatformAccountID(ctx context.Context, id string) (*domain.Channel, error) {
	return nil, errors.New("not found")
}

type mockMediaAssetRepoPublish struct {
	listByPostResult []*domain.MediaAsset
	listByPostErr    error
}

func (m *mockMediaAssetRepoPublish) Create(ctx context.Context, a *domain.MediaAsset) error {
	return nil
}
func (m *mockMediaAssetRepoPublish) GetByID(ctx context.Context, id uuid.UUID) (*domain.MediaAsset, error) {
	return nil, nil
}
func (m *mockMediaAssetRepoPublish) Update(ctx context.Context, a *domain.MediaAsset) error {
	return nil
}
func (m *mockMediaAssetRepoPublish) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockMediaAssetRepoPublish) ListByBrand(ctx context.Context, brandID uuid.UUID) ([]*domain.MediaAsset, error) {
	return nil, nil
}
func (m *mockMediaAssetRepoPublish) ListByPost(ctx context.Context, postID uuid.UUID) ([]*domain.MediaAsset, error) {
	return m.listByPostResult, m.listByPostErr
}
func (m *mockMediaAssetRepoPublish) ListPendingScan(ctx context.Context) ([]*domain.MediaAsset, error) {
	return nil, nil
}

type mockSocialPublisher struct {
	platformPostID string
	platformURL    string
	err            error
}

func (m *mockSocialPublisher) Publish(
	ctx context.Context,
	ct domain.ChannelType,
	channelID uuid.UUID,
	variant *domain.PostVariant,
	mainCaption string,
) (string, string, error) {
	return m.platformPostID, m.platformURL, m.err
}

type mockEventBusPublish struct{}

func (m *mockEventBusPublish) Publish(ctx context.Context, events ...domain.DomainEvent) error {
	return nil
}

// ── Tests SchedulePostUseCase ────────────────────────────────────────────────

func validatedPost(brandID, authorID uuid.UUID) *domain.Post {
	p, _ := domain.NewPost(brandID, authorID, "Scheduled Post")
	_ = p.SubmitForValidation()
	_ = p.Validate(uuid.New())
	return p
}

func activeChannel(brandID uuid.UUID) *domain.Channel {
	c, _ := domain.NewConnectedChannel(brandID, uuid.New(), "facebook_page", "fb_123", "My Page")
	return c
}

func TestSchedulePost_Success(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	channelID := uuid.New()
	post := validatedPost(brandID, actorID)
	channel := activeChannel(brandID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	uc := ucPost.NewSchedulePostUseCase(
		&mockPostRepo{getByIDResult: post},
		&mockPostVariantRepo{},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockChannelRepoPublish{getByIDResult: channel},
		&mockMediaAssetRepoPublish{},
		&mockEventBusPublish{},
	)

	result := uc.Execute(context.Background(), ucPost.SchedulePostCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		ChannelIDs:  []uuid.UUID{channelID},
		ScheduledAt: time.Now().UTC().Add(10 * time.Minute),
	})

	if result.IsFail() {
		t.Fatalf("expected success, got: %v", result.Err())
	}
	if result.Value().Status != "scheduled" {
		t.Errorf("expected scheduled, got %s", result.Value().Status)
	}
}

func TestSchedulePost_A004_DraftFails(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	// Post en draft (pas validated)
	draftPost, _ := domain.NewPost(brandID, actorID, "Draft Post")
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	uc := ucPost.NewSchedulePostUseCase(
		&mockPostRepo{getByIDResult: draftPost},
		&mockPostVariantRepo{},
		&mockBrandRepoPost{getByIDResult: brand},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleOwner},
		},
		&mockChannelRepoPublish{},
		&mockMediaAssetRepoPublish{},
		&mockEventBusPublish{},
	)

	result := uc.Execute(context.Background(), ucPost.SchedulePostCommand{
		PostID:      draftPost.ID(),
		ActorID:     actorID,
		ChannelIDs:  []uuid.UUID{uuid.New()},
		ScheduledAt: time.Now().UTC().Add(10 * time.Minute),
	})

	if result.IsOk() {
		t.Fatal("expected INVALID_STATUS_TRANSITION (A004 — draft ne peut pas être planifié)")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeINVALID_STATUS_TRANSITION {
		t.Errorf("expected INVALID_STATUS_TRANSITION, got %s", domErr.Code)
	}
}

func TestSchedulePost_X7_QuarantinedMediaBlocks(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	post := validatedPost(brandID, actorID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	// Média en quarantaine
	quarantinedAsset := &domain.MediaAsset{
		ID:     uuid.New(),
		Status: domain.MediaAssetStatusQuarantined,
	}

	uc := ucPost.NewSchedulePostUseCase(
		&mockPostRepo{getByIDResult: post},
		&mockPostVariantRepo{},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockChannelRepoPublish{},
		&mockMediaAssetRepoPublish{listByPostResult: []*domain.MediaAsset{quarantinedAsset}},
		&mockEventBusPublish{},
	)

	result := uc.Execute(context.Background(), ucPost.SchedulePostCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		ChannelIDs:  []uuid.UUID{uuid.New()},
		ScheduledAt: time.Now().UTC().Add(10 * time.Minute),
	})

	if result.IsOk() {
		t.Fatal("expected VIRUS_DETECTED — média quarantined bloque planification (X-7)")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeVIRUS_DETECTED {
		t.Errorf("expected VIRUS_DETECTED, got %s", domErr.Code)
	}
}

func TestSchedulePost_P2_TooSoon(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	post := validatedPost(brandID, actorID)
	channel := activeChannel(brandID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	uc := ucPost.NewSchedulePostUseCase(
		&mockPostRepo{getByIDResult: post},
		&mockPostVariantRepo{},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockChannelRepoPublish{getByIDResult: channel},
		&mockMediaAssetRepoPublish{},
		&mockEventBusPublish{},
	)

	// ScheduledAt dans 2min — P-2 requiert >5min
	result := uc.Execute(context.Background(), ucPost.SchedulePostCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		ChannelIDs:  []uuid.UUID{uuid.New()},
		ScheduledAt: time.Now().UTC().Add(2 * time.Minute),
	})

	if result.IsOk() {
		t.Fatal("expected SCHEDULE_DATE_IN_PAST — P-2 : scheduledAt < now+5min")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeSCHEDULE_DATE_IN_PAST {
		t.Errorf("expected SCHEDULE_DATE_IN_PAST, got %s", domErr.Code)
	}
}

func TestSchedulePost_RevokedChannelBlocked(t *testing.T) {
	brandID := uuid.New()
	actorID := uuid.New()
	workspaceID := uuid.New()
	post := validatedPost(brandID, actorID)
	brand, _ := domain.NewBrand(workspaceID, uuid.New(), "Test Brand")

	// Channel révoqué — C-1b
	revokedChannel, _ := domain.NewConnectedChannel(brandID, workspaceID, "facebook_page", "fb_123", "Revoked Page")
	revokedChannel.SetStatus(domain.ChannelStatusRevoked)

	uc := ucPost.NewSchedulePostUseCase(
		&mockPostRepo{getByIDResult: post},
		&mockPostVariantRepo{},
		&mockBrandRepoPost{
			getByIDResult:       brand,
			getAssignmentResult: &domain.BrandAssignment{Role: domain.BrandRoleEditor},
		},
		&mockWorkspaceRepoPost{
			getMemberResult: &domain.WorkspaceMember{ID: actorID, Role: domain.MemberRoleEditor},
		},
		&mockChannelRepoPublish{getByIDResult: revokedChannel},
		&mockMediaAssetRepoPublish{},
		&mockEventBusPublish{},
	)

	result := uc.Execute(context.Background(), ucPost.SchedulePostCommand{
		PostID:      post.ID(),
		ActorID:     actorID,
		ChannelIDs:  []uuid.UUID{uuid.New()},
		ScheduledAt: time.Now().UTC().Add(10 * time.Minute),
	})

	if result.IsOk() {
		t.Fatal("expected CHANNEL_REVOKED — C-1b")
	}
	var domErr *domain.DomainError
	if errors.As(result.Err(), &domErr) && domErr.Code != domain.ErrCodeCHANNEL_REVOKED {
		t.Errorf("expected CHANNEL_REVOKED, got %s", domErr.Code)
	}
}
