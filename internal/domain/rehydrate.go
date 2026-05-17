package domain

import (
	"time"

	"github.com/google/uuid"
)

// WorkspaceLimits holds plan-based limits for a workspace.
type WorkspaceLimits struct {
	MaxMembers  int
	MaxBrands   int
	MaxChannels int
}

// BrandIdentity holds brand visual identity fields.
type BrandIdentity struct {
	LogoURL        *string
	PrimaryColor   *string
	SecondaryColor *string
}

// ToneOfVoice holds brand communication tone settings.
type ToneOfVoice struct {
	Formality     string
	HumorLevel    string
	EmojisAllowed bool
}

// RehydrateUser constructs a User from DB-scanned values, bypassing domain validation.
func RehydrateUser(
	id uuid.UUID,
	email, passwordHash string,
	status UserStatus,
	emailVerified bool,
	firstName, lastName string,
	twoFAEnabled bool,
	totpSecret *string,
	backupCodes []string,
	failedLoginAttempts int,
	lockedUntil *time.Time,
	locked bool,
	lastLoginAt *time.Time,
	lastLoginIP *string,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) *User {
	u := &User{}
	u.id = id
	u.email = email
	u.passwordHash = passwordHash
	u.status = status
	u.emailVerified = emailVerified
	u.firstName = firstName
	u.lastName = lastName
	u.twoFactorEnabled = twoFAEnabled
	u.totpSecret = totpSecret
	u.backupCodes = backupCodes
	u.failedLoginAttempts = failedLoginAttempts
	u.lockedUntil = lockedUntil
	u.lastLoginAt = lastLoginAt
	u.lastLoginIP = lastLoginIP
	u.createdAt = createdAt
	u.updatedAt = updatedAt
	u.deletedAt = deletedAt
	return u
}

// RehydrateWorkspace constructs a Workspace from DB-scanned values.
func RehydrateWorkspace(
	id uuid.UUID,
	slug, name string,
	ownerUserID uuid.UUID,
	status WorkspaceStatus,
	mode WorkspaceMode,
	planID string,
	settings WorkspaceSettings,
	limits WorkspaceLimits,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) *Workspace {
	w := &Workspace{}
	w.id = id
	w.slug = slug
	w.name = name
	w.ownerUserID = ownerUserID
	w.status = status
	w.mode = mode
	w.planID = planID
	w.settings = &settings
	w.limits = limits
	w.createdAt = createdAt
	w.updatedAt = updatedAt
	w.deletedAt = deletedAt
	return w
}

// RehydrateWorkspaceMember constructs a WorkspaceMember from DB-scanned values.
func RehydrateWorkspaceMember(
	id uuid.UUID,
	workspaceID, userID uuid.UUID,
	role MemberRole,
	status MemberStatus,
	invitedByMemberID *uuid.UUID,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) *WorkspaceMember {
	m := &WorkspaceMember{}
	m.ID = id
	m.WorkspaceID = workspaceID
	m.UserID = userID
	m.Role = role
	m.Status = status
	m.InvitedByMemberID = invitedByMemberID
	m.CreatedAt = createdAt
	m.UpdatedAt = updatedAt
	m.DeletedAt = deletedAt
	return m
}

// RehydrateBrand constructs a Brand from DB-scanned values.
func RehydrateBrand(
	id uuid.UUID,
	workspaceID uuid.UUID,
	name, slug string,
	status BrandStatus,
	industry *string,
	identity BrandIdentity,
	tone ToneOfVoice,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) *Brand {
	b := &Brand{}
	b.id = id
	b.workspaceID = workspaceID
	b.name = name
	b.slug = slug
	b.status = status
	b.industry = industry
	b.identity = identity
	b.tone = tone
	b.createdAt = createdAt
	b.updatedAt = updatedAt
	b.deletedAt = deletedAt
	return b
}

// RehydrateBrandAssignment constructs a BrandAssignment from DB-scanned values.
func RehydrateBrandAssignment(
	id uuid.UUID,
	brandID, memberID uuid.UUID,
	role BrandRole,
	assignedByMemberID *uuid.UUID,
	createdAt, updatedAt time.Time,
) *BrandAssignment {
	a := &BrandAssignment{}
	a.ID = id
	a.BrandID = brandID
	a.MemberID = memberID
	a.Role = role
	a.AssignedByMemberID = assignedByMemberID
	a.CreatedAt = createdAt
	a.UpdatedAt = updatedAt
	return a
}

// RehydrateChannel constructs a Channel from DB-scanned values.
func RehydrateChannel(
	id uuid.UUID,
	brandID, workspaceID uuid.UUID,
	channelType ChannelType,
	status ChannelStatus,
	platformAccountID string,   // maps to DB external_id (A071)
	platformAccountName string, // maps to DB display_name (A071)
	consecutiveFailures int,
	lastFailureAt *time.Time,
	tokenExpiresAt *time.Time,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) *Channel {
	c := &Channel{}
	c.id = id
	c.brandID = brandID
	c.workspaceID = workspaceID
	c.channelType = channelType
	c.status = status
	c.platformAccountID = platformAccountID
	c.platformAccountName = platformAccountName
	c.healthStatus = &ChannelHealth{
		ConsecutiveFailures: consecutiveFailures,
		TokenExpiresAt:      tokenExpiresAt,
		LastErrorAt:         lastFailureAt,
		IsHealthy:           consecutiveFailures == 0,
		TokenValid:          tokenExpiresAt == nil || time.Now().UTC().Before(*tokenExpiresAt),
		PermissionsValid:    true,
		RateLimitStatus:     "ok",
	}
	c.createdAt = createdAt
	c.updatedAt = updatedAt
	c.deletedAt = deletedAt
	return c
}

// RehydratePost constructs a Post from DB-scanned values.
func RehydratePost(
	id uuid.UUID,
	brandID, workspaceID uuid.UUID,
	createdByUserID uuid.UUID,
	title, mainCaption *string,
	status PostStatus,
	retryCount int,
	scheduledAt *time.Time,
	publishedAt *time.Time,
	publishType PublishType,
	rejectionReason *string,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
	version int,
) *Post {
	p := &Post{}
	p.id = id
	p.brandID = brandID
	p.workspaceID = workspaceID
	p.createdByUserID = createdByUserID
	p.title = title
	p.mainCaption = mainCaption
	p.status = status
	p.retryCount = retryCount
	p.version = version
	p.scheduledAt = scheduledAt
	p.publishedAt = publishedAt
	p.publishType = publishType
	p.rejectionReason = rejectionReason
	p.createdAt = createdAt
	p.updatedAt = updatedAt
	p.deletedAt = deletedAt
	return p
}
