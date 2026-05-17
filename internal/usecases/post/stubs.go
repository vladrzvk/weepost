package post

import "github.com/google/uuid"

// Stubs/aliases pour compatibilité handlers HTTP — implémentations complètes en Phase D.

// CreatePostCommand — stub Phase D.
type CreatePostCommand struct {
	BrandID  uuid.UUID `json:"brand_id"  validate:"required"`
	AuthorID uuid.UUID `json:"author_id"`
	Title    string    `json:"title,omitempty"`
	Body     string    `json:"body,omitempty"`
}

// CreatePostResult — stub Phase D.
type CreatePostResult struct {
	PostID    string `json:"post_id"`
	BrandID   string `json:"brand_id"`
	Status    string `json:"status"`
}

// SubmitPostCommand — alias vers RequestApprovalCommand (PostID + ActorID).
type SubmitPostCommand = RequestApprovalCommand

// ValidatePostCommand — stub Phase D (approve interne; champs alignés sur le handler).
type ValidatePostCommand struct {
	PostID  uuid.UUID `json:"post_id"  validate:"required"`
	ActorID uuid.UUID `json:"actor_id" validate:"required"`
}

// CancelScheduleCommand — alias vers CancelScheduledPostCommand.
type CancelScheduleCommand = CancelScheduledPostCommand
