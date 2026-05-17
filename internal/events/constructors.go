package events

import (
	"time"

	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
)

// newEvent — internal factory.
// aggregateID must not be uuid.Nil — each event must identify its aggregate.
// workspaceID == uuid.Nil means system/cross-tenant event (use bus.PublishSystem).
// newEvent — parameter order: aggregateID first, workspaceID second.
// For Workspace events: aggregateID == workspaceID (workspace IS its own tenant boundary).
// For other aggregates: aggregateID = the aggregate's ID, workspaceID = the owning workspace.
// workspaceID == uuid.Nil → system/cross-tenant event → caller must use PublishSystem().
func newEvent(eventType string, aggregateID, workspaceID uuid.UUID, boundedCtx string, payload map[string]interface{}) domain.DomainEvent {
	if aggregateID == uuid.Nil {
		panic("events: aggregateID cannot be uuid.Nil — every event must identify its aggregate (SEC-03)")
	}
	return domain.DomainEvent{
		ID:          uuid.NewString(),
		EventType:   eventType,
		WorkspaceID: workspaceID,
		AggregateID: aggregateID,
		BoundedCtx:  boundedCtx,
		OccurredAt:  time.Now().UTC(),
		Version:     1, // V0: per-event-type versioning deferred
		Payload:     payload,
	}
}

// ============================================================
// BC: Workspace  (Phase 3 §BC01)
// WorkspaceID == aggregateID for workspace events (workspace-scoped → Publish())
// ============================================================

func NewWorkspaceCreatedEvent(workspaceID uuid.UUID, workspaceName, workspaceSlug, ownerUserID, planID string) domain.DomainEvent {
	return newEvent("workspace.created", workspaceID, workspaceID, "workspace", map[string]interface{}{
		"workspace_name": workspaceName,
		"workspace_slug": workspaceSlug,
		"owner_user_id":  ownerUserID,
		"plan_id":        planID,
	})
}

func NewWorkspaceUpdatedEvent(workspaceID uuid.UUID, changedFields map[string]interface{}) domain.DomainEvent {
	return newEvent("workspace.updated", workspaceID, workspaceID, "workspace", map[string]interface{}{
		"changed_fields": changedFields,
	})
}

func NewWorkspaceDeletedEvent(workspaceID uuid.UUID, deletedByUserID string, deletedAt time.Time) domain.DomainEvent {
	return newEvent("workspace.deleted", workspaceID, workspaceID, "workspace", map[string]interface{}{
		"deleted_by_user_id": deletedByUserID,
		"deleted_at":         deletedAt.UTC(),
	})
}

func NewWorkspaceMemberInvitedEvent(workspaceID uuid.UUID, email, role, invitedByUserID string, expiresAt time.Time) domain.DomainEvent {
	return newEvent("workspace.member.invited", workspaceID, workspaceID, "workspace", map[string]interface{}{
		"email":              email,
		"role":               role,
		"invited_by_user_id": invitedByUserID,
		"expires_at":         expiresAt.UTC(),
	})
}

func NewWorkspaceMemberAddedEvent(workspaceID, memberID, userID uuid.UUID, role string, joinedAt time.Time) domain.DomainEvent {
	return newEvent("workspace.member.added", workspaceID, workspaceID, "workspace", map[string]interface{}{
		"member_id": memberID.String(),
		"user_id":   userID.String(),
		"role":      role,
		"joined_at": joinedAt.UTC(),
	})
}

// ============================================================
// BC: User  (Phase 3 §BC02)
// User events are cross-tenant — WorkspaceID = uuid.Nil → PublishSystem()
// ============================================================

func NewUserRegisteredEvent(userID uuid.UUID, email, fullName string, createdAt time.Time) domain.DomainEvent {
	return newEvent("user.registered", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":    userID.String(),
		"email":      email,
		"full_name":  fullName,
		"created_at": createdAt.UTC(),
	})
}

func NewUserEmailVerifiedEvent(userID uuid.UUID, verifiedAt time.Time) domain.DomainEvent {
	return newEvent("user.email.verified", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":     userID.String(),
		"verified_at": verifiedAt.UTC(),
	})
}

// NewUserAccountLockedEvent — U-2: seuil 5 échecs, lock 30 min
func NewUserAccountLockedEvent(userID uuid.UUID, failedAttempts int, lockedUntil time.Time, lastFailedIPHash string) domain.DomainEvent {
	return newEvent("user.account.locked", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":             userID.String(),
		"locked_until":        lockedUntil.UTC(),
		"failed_attempts":     failedAttempts,
		"last_failed_ip_hash": lastFailedIPHash,
	})
}

func NewUserAccountUnlockedEvent(userID uuid.UUID, unlockedAt time.Time, unlockMethod string) domain.DomainEvent {
	return newEvent("user.account.unlocked", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":       userID.String(),
		"unlocked_at":   unlockedAt.UTC(),
		"unlock_method": unlockMethod,
	})
}

func NewUserPasswordChangedEvent(userID uuid.UUID, changedFromIPHash string, changedAt time.Time) domain.DomainEvent {
	return newEvent("user.password.changed", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":              userID.String(),
		"changed_at":           changedAt.UTC(),
		"changed_from_ip_hash": changedFromIPHash,
	})
}

func NewUser2FAEnabledEvent(userID uuid.UUID, enabledAt time.Time) domain.DomainEvent {
	return newEvent("user.2fa.enabled", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":    userID.String(),
		"enabled_at": enabledAt.UTC(),
	})
}

func NewUser2FADisabledEvent(userID uuid.UUID, disabledAt time.Time) domain.DomainEvent {
	return newEvent("user.2fa.disabled", userID, uuid.Nil, "user", map[string]interface{}{
		"user_id":     userID.String(),
		"disabled_at": disabledAt.UTC(),
	})
}

// ============================================================
// BC: Brand  (Phase 3 §BC03)
// Brand events are workspace-scoped → Publish()
// ============================================================

func NewBrandCreatedEvent(brandID, workspaceID uuid.UUID, name, slug, createdByUserID string) domain.DomainEvent {
	return newEvent("brand.created", brandID, workspaceID, "brand", map[string]interface{}{
		"brand_id":           brandID.String(),
		"workspace_id":       workspaceID.String(),
		"name":               name,
		"slug":               slug,
		"created_by_user_id": createdByUserID,
	})
}

func NewBrandUpdatedEvent(brandID, workspaceID uuid.UUID, changedFields map[string]interface{}) domain.DomainEvent {
	return newEvent("brand.updated", brandID, workspaceID, "brand", map[string]interface{}{
		"brand_id":       brandID.String(),
		"changed_fields": changedFields,
	})
}

func NewBrandMemberAssignedEvent(assignmentID, brandID, workspaceID uuid.UUID, memberID, role, createdByUserID string) domain.DomainEvent {
	return newEvent("brand.member.assigned", brandID, workspaceID, "brand", map[string]interface{}{
		"assignment_id":      assignmentID.String(),
		"brand_id":           brandID.String(),
		"member_id":          memberID,
		"role":               role,
		"created_by_user_id": createdByUserID,
	})
}

// ============================================================
// BC: Channel  (Phase 3 §BC04)
// Channel events are workspace-scoped → Publish()
// ============================================================

func NewChannelConnectedEvent(channelID, workspaceID, brandID uuid.UUID, channelType, platformAccountID string, connectedAt time.Time) domain.DomainEvent {
	return newEvent("channel.connected", channelID, workspaceID, "channel", map[string]interface{}{
		"channel_id":          channelID.String(),
		"brand_id":            brandID.String(),
		"channel_type":        channelType,
		"platform_account_id": platformAccountID,
		"connected_at":        connectedAt.UTC(),
	})
}

func NewChannelDisconnectedEvent(channelID, workspaceID, brandID uuid.UUID, disconnectedByUserID, reason string) domain.DomainEvent {
	return newEvent("channel.disconnected", channelID, workspaceID, "channel", map[string]interface{}{
		"channel_id":              channelID.String(),
		"brand_id":                brandID.String(),
		"disconnected_by_user_id": disconnectedByUserID,
		"reason":                  reason,
	})
}

// ============================================================
// BC: Content Publishing  (Phase 3 §BC05)
// Post/media events are workspace-scoped → Publish()
// ============================================================

func NewPostCreatedEvent(postID, workspaceID, brandID uuid.UUID, title, createdByUserID string) domain.DomainEvent {
	return newEvent("post.created", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":            postID.String(),
		"brand_id":           brandID.String(),
		"title":              title,
		"created_by_user_id": createdByUserID,
	})
}

// NewPostStatusChangedEvent — fired on every SM-01 transition
func NewPostStatusChangedEvent(postID, workspaceID, brandID uuid.UUID, oldStatus, newStatus string) domain.DomainEvent {
	return newEvent("post.status.changed", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":    postID.String(),
		"brand_id":   brandID.String(),
		"old_status": oldStatus,
		"new_status": newStatus,
	})
}

// NewPostScheduledEvent — P-2: scheduledAt must be >now+5min (enforced in domain)
func NewPostScheduledEvent(postID, workspaceID, brandID uuid.UUID, scheduledAt time.Time, scheduledByUserID string, channelIDs []string) domain.DomainEvent {
	return newEvent("post.scheduled", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":               postID.String(),
		"brand_id":              brandID.String(),
		"scheduled_at":          scheduledAt.UTC(),
		"scheduled_by_user_id":  scheduledByUserID,
		"channel_ids":           channelIDs,
	})
}

func NewPostPublishedEvent(postID, workspaceID, brandID uuid.UUID, publishedAt time.Time, channelsPublished []string) domain.DomainEvent {
	return newEvent("post.published", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":            postID.String(),
		"brand_id":           brandID.String(),
		"published_at":       publishedAt.UTC(),
		"channels_published": channelsPublished,
	})
}

// NewPostFailedEvent — D-05: retryCount tracked; CanRetry() returns true if <3
func NewPostFailedEvent(postID, workspaceID, brandID uuid.UUID, errorCode string, failedAt time.Time, retryCount int) domain.DomainEvent {
	return newEvent("post.failed", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":     postID.String(),
		"brand_id":    brandID.String(),
		"failed_at":   failedAt.UTC(),
		"error_code":  errorCode,
		"retry_count": retryCount,
	})
}

func NewPostDeletedEvent(postID, workspaceID, brandID uuid.UUID, deletedByUserID string, wasScheduled bool) domain.DomainEvent {
	return newEvent("post.deleted", postID, workspaceID, "content_publishing", map[string]interface{}{
		"post_id":            postID.String(),
		"brand_id":           brandID.String(),
		"deleted_by_user_id": deletedByUserID,
		"was_scheduled":      wasScheduled,
	})
}

// NewMediaUploadedEvent — A016: initial status is pending_scan (not processing)
func NewMediaUploadedEvent(mediaID, workspaceID, brandID uuid.UUID, uploadedByUserID, mimeType string, fileSizeBytes int64) domain.DomainEvent {
	return newEvent("media.uploaded", mediaID, workspaceID, "content_publishing", map[string]interface{}{
		"media_id":             mediaID.String(),
		"brand_id":             brandID.String(),
		"uploaded_by_user_id":  uploadedByUserID,
		"mime_type":            mimeType,
		"file_size_bytes":      fileSizeBytes,
	})
}

func NewMediaQuarantinedEvent(mediaID, workspaceID, brandID uuid.UUID, antivirusEngine, threatName string) domain.DomainEvent {
	return newEvent("media.quarantined", mediaID, workspaceID, "content_publishing", map[string]interface{}{
		"media_id":         mediaID.String(),
		"brand_id":         brandID.String(),
		"antivirus_engine": antivirusEngine,
		"threat_name":      threatName,
	})
}

// ============================================================
// BC: Collaboration  (Phase 3 §BC06)
// Approval events are workspace-scoped → Publish()
// ============================================================

func NewApprovalRequestedEvent(approvalRequestID, workspaceID, postID, brandID uuid.UUID, requestedByUserID string) domain.DomainEvent {
	return newEvent("approval.requested", approvalRequestID, workspaceID, "collaboration", map[string]interface{}{
		"approval_request_id":  approvalRequestID.String(),
		"post_id":              postID.String(),
		"brand_id":             brandID.String(),
		"requested_by_user_id": requestedByUserID,
	})
}

func NewApprovalGrantedEvent(approvalRequestID, workspaceID, postID, brandID uuid.UUID, approvedByUserID string) domain.DomainEvent {
	return newEvent("approval.granted", approvalRequestID, workspaceID, "collaboration", map[string]interface{}{
		"approval_request_id": approvalRequestID.String(),
		"post_id":             postID.String(),
		"brand_id":            brandID.String(),
		"approved_by_user_id": approvedByUserID,
	})
}

func NewApprovalRejectedEvent(approvalRequestID, workspaceID, postID, brandID uuid.UUID, rejectedByUserID, reason string) domain.DomainEvent {
	return newEvent("approval.rejected", approvalRequestID, workspaceID, "collaboration", map[string]interface{}{
		"approval_request_id": approvalRequestID.String(),
		"post_id":             postID.String(),
		"brand_id":            brandID.String(),
		"rejected_by_user_id": rejectedByUserID,
		"reason":              reason,
	})
}

// ============================================================
// BC: Security  (Phase 3 §BC14)
// ============================================================
// ANOMALIE A017: Phase 3 §BC14 Security section was not in read window (lines 1–1100).
// Event types and payloads sourced from Phase 8 mission template spec.
// Verify event_type strings against Phase 3 §BC14 when available.

// NewSuspiciousActivityDetectedEvent — cross-tenant security event → WorkspaceID=uuid.Nil → PublishSystem()
func NewSuspiciousActivityDetectedEvent(userID uuid.UUID, activityType, ipHash string, detectedAt time.Time, riskLevel string) domain.DomainEvent {
	return newEvent("security.suspicious_activity.detected", userID, uuid.Nil, "security", map[string]interface{}{
		"user_id":       userID.String(),
		"activity_type": activityType,
		"ip_hash":       ipHash,
		"detected_at":   detectedAt.UTC(),
		"risk_level":    riskLevel,
	})
}

// NewEncryptionKeysRotatedEvent — Phase 9 §A9-8, D-KR invariant → cross-tenant → PublishSystem()
func NewEncryptionKeysRotatedEvent(rotationID uuid.UUID, initiatedBy string, channelsTotal, channelsRotated, channelsFailed int, completedAt time.Time) domain.DomainEvent {
	return newEvent("security.encryption_keys.rotated", rotationID, uuid.Nil, "security", map[string]interface{}{
		"rotation_id":      rotationID.String(),
		"initiated_by":     initiatedBy,
		"channels_total":   channelsTotal,
		"channels_rotated": channelsRotated,
		"channels_failed":  channelsFailed,
		"completed_at":     completedAt.UTC(),
	})
}

// NewChannelTokenAccessedEvent — Phase 2 P4c §UC-S-03 (audit OAuth token access)
// workspace-scoped → WorkspaceID != uuid.Nil → Publish()
func NewChannelTokenAccessedEvent(channelID, workspaceID uuid.UUID, accessedBy, ipHash string, accessedAt time.Time) domain.DomainEvent {
	return newEvent("security.channel_token.accessed", channelID, workspaceID, "security", map[string]interface{}{
		"channel_id":   channelID.String(),
		"workspace_id": workspaceID.String(),
		"accessed_by":  accessedBy,
		"ip_hash":      ipHash,
		"accessed_at":  accessedAt.UTC(),
	})
}

// ============================================================
// Constructeurs additionnels — Correctif P14 (AUD2-035)
// Addendum partie-1e
// ============================================================

// NewBrandMemberRevokedEvent crée un événement de révocation d'accès d'un membre à une brand.
// Émis par : T12 RevokeBrandAccessUseCase
// Agrégat : Brand (brandID) — workspace-scoped → Publish()
// Phase 3 §14 — BrandMemberRevoked
func NewBrandMemberRevokedEvent(
	brandID     uuid.UUID,
	workspaceID uuid.UUID,
	memberID    uuid.UUID,
	revokedBy   uuid.UUID,
	reason      string,
	revokedAt   time.Time,
) domain.DomainEvent {
	return newEvent(
		"brand.member.revoked",
		brandID,      // aggregateID — SEC-03 : panic si uuid.Nil
		workspaceID,
		"brand",
		map[string]interface{}{
			"member_id":  memberID.String(),
			"revoked_by": revokedBy.String(),
			"reason":     reason,
			"revoked_at": revokedAt.Format(time.RFC3339),
		},
	)
}

// NewApprovalRequestCancelledEvent crée un événement d'annulation
// d'une demande d'approbation.
// Émis par : T19 CancelApprovalRequestService
// Agrégat : ApprovalRequest (approvalRequestID) — workspace-scoped → Publish()
// Phase 3 §14 — ApprovalRequestCancelled
// ANOMALIE P15 : EventType canonique Phase 3 §14 = "approval_request.cancelled".
// Le constructeur local T19 utilisait "collaboration.approval.cancelled" — corrigé ici.
func NewApprovalRequestCancelledEvent(
	approvalRequestID uuid.UUID,
	workspaceID       uuid.UUID,
	postID            uuid.UUID,
	cancelledBy       uuid.UUID,
	reason            string,
	cancelledAt       time.Time,
) domain.DomainEvent {
	return newEvent(
		"approval_request.cancelled",
		approvalRequestID, // aggregateID — SEC-03 : panic si uuid.Nil
		workspaceID,
		"collaboration",
		map[string]interface{}{
			"post_id":      postID.String(),
			"cancelled_by": cancelledBy.String(),
			"reason":       reason,
			"cancelled_at": cancelledAt.Format(time.RFC3339),
		},
	)
}
