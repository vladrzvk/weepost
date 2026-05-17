package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	postUC "github.com/vladrzvk/weepost/internal/usecases/post"
)

type (
	ICreatePostUC interface {
		Execute(ctx context.Context, cmd postUC.CreatePostCommand) domain.Result[postUC.CreatePostResult]
	}
	ISubmitPostUC interface {
		Execute(ctx context.Context, cmd postUC.SubmitPostCommand) domain.Result[struct{}]
	}
	IValidatePostUC interface {
		Execute(ctx context.Context, cmd postUC.ValidatePostCommand) domain.Result[struct{}]
	}
	IRejectPostUC interface {
		Execute(ctx context.Context, cmd postUC.RejectPostCommand) domain.Result[struct{}]
	}
	ISchedulePostUC interface {
		Execute(ctx context.Context, cmd postUC.SchedulePostCommand) domain.Result[postUC.SchedulePostResult]
	}
	IPublishPostUC interface {
		Execute(ctx context.Context, cmd postUC.PublishPostCommand) domain.Result[struct{}]
	}
	ICancelScheduleUC interface {
		Execute(ctx context.Context, cmd postUC.CancelScheduleCommand) domain.Result[struct{}]
	}
)

type PostHandler struct {
	createUC         ICreatePostUC
	submitUC         ISubmitPostUC
	validateUC       IValidatePostUC
	rejectUC         IRejectPostUC
	scheduleUC       ISchedulePostUC
	publishUC        IPublishPostUC
	cancelScheduleUC ICancelScheduleUC
}

func NewPostHandler(
	create ICreatePostUC,
	submit ISubmitPostUC,
	validate IValidatePostUC,
	reject IRejectPostUC,
	schedule ISchedulePostUC,
	publish IPublishPostUC,
	cancelSchedule ICancelScheduleUC,
) *PostHandler {
	return &PostHandler{
		createUC:         create,
		submitUC:         submit,
		validateUC:       validate,
		rejectUC:         reject,
		scheduleUC:       schedule,
		publishUC:        publish,
		cancelScheduleUC: cancelSchedule,
	}
}

// RegisterRoutes enregistre les routes post.
// NOTE permission strings corrigés — A063/A064/A065/A066.
func (h *PostHandler) RegisterRoutes(r fiber.Router, jwtMw fiber.Handler, permMw func(string) fiber.Handler) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/brands/:brandID/posts?workspace_id=...
	// Permission CH-02 (channel-scoped — vérification granulaire dans l'UC)
	v1.Post("/brands/:brandID/posts", jwtMw, permMw("post.create"), h.Create)

	// POST /api/v1/posts/:postID/submit?workspace_id=...&brand_id=...
	// Permission TX-03 — A063 : mission disait "post.submit"
	v1.Post("/posts/:postID/submit", jwtMw, permMw("post.request_approval"), h.Submit)

	// POST /api/v1/posts/:postID/validate?workspace_id=...&brand_id=...
	// Permission TX-04 — A064 : mission disait "post.validate"
	v1.Post("/posts/:postID/validate", jwtMw, permMw("post.approve_internal"), h.Validate)

	// POST /api/v1/posts/:postID/reject?workspace_id=...&brand_id=...
	// Permission TX-05 — A065 : mission disait "post.reject"
	v1.Post("/posts/:postID/reject", jwtMw, permMw("post.reject_internal"), h.Reject)

	// POST /api/v1/posts/:postID/schedule?workspace_id=...&brand_id=...
	// Permission CH-04
	v1.Post("/posts/:postID/schedule", jwtMw, permMw("post.schedule"), h.Schedule)

	// POST /api/v1/posts/:postID/publish?workspace_id=...&brand_id=...
	// Permission CH-05 (publish + schedule dans channel_permissions)
	v1.Post("/posts/:postID/publish", jwtMw, permMw("post.publish"), h.Publish)

	// DELETE /api/v1/posts/:postID/schedule?workspace_id=...&brand_id=...
	// Permission TX-02 — A066 : mission disait "post.cancel" → "post.delete" → corrigé "post.cancel_schedule"
	v1.Delete("/posts/:postID/schedule", jwtMw, permMw("post.cancel_schedule"), h.CancelSchedule)
}

// Create — POST /api/v1/brands/:brandID/posts
func (h *PostHandler) Create(c *fiber.Ctx) error {
	var cmd postUC.CreatePostCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	brandID, err := uuid.Parse(c.Params("brandID"))
	if err != nil {
		return respondBadRequest(c, "brandID invalide")
	}
	cmd.BrandID = brandID
	cmd.AuthorID = getUserID(c)

	result := h.createUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

// Submit — POST /api/v1/posts/:postID/submit
// Soumet un post pour validation interne (request_approval TX-03).
func (h *PostHandler) Submit(c *fiber.Ctx) error {
	var cmd postUC.SubmitPostCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd.PostID = postID
	cmd.ActorID = getUserID(c)

	result := h.submitUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "submitted"})
}

// Validate — POST /api/v1/posts/:postID/validate
// Approuve un post (validation interne, TX-04 — approve_internal).
func (h *PostHandler) Validate(c *fiber.Ctx) error {
	var cmd postUC.ValidatePostCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd.PostID = postID
	cmd.ActorID = getUserID(c)

	result := h.validateUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "approved"})
}

// Reject — POST /api/v1/posts/:postID/reject
// Rejette un post (validation interne, TX-05 — reject_internal).
func (h *PostHandler) Reject(c *fiber.Ctx) error {
	var cmd postUC.RejectPostCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd.PostID = postID
	cmd.ActorID = getUserID(c)

	result := h.rejectUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "rejected"})
}

// Schedule — POST /api/v1/posts/:postID/schedule
// Programme une publication (T16, CH-04).
func (h *PostHandler) Schedule(c *fiber.Ctx) error {
	var cmd postUC.SchedulePostCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd.PostID = postID
	cmd.ActorID = getUserID(c)

	result := h.scheduleUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(result.Value())
}

// Publish — POST /api/v1/posts/:postID/publish
// Publication immédiate (T16, CH-05 — publish+schedule requis).
func (h *PostHandler) Publish(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd := postUC.PublishPostCommand{
		PostID:  postID,
		ActorID: getUserID(c),
	}
	result := h.publishUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "queued"})
}

// CancelSchedule — DELETE /api/v1/posts/:postID/schedule
// Annule la programmation d'un post (T18, TX-02 post.cancel_schedule).
func (h *PostHandler) CancelSchedule(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("postID"))
	if err != nil {
		return respondBadRequest(c, "postID invalide")
	}
	cmd := postUC.CancelScheduleCommand{
		PostID:  postID,
		ActorID: getUserID(c),
	}
	result := h.cancelScheduleUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(200).JSON(fiber.Map{"status": "unscheduled"})
}
