package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/vladrzvk/weepost/internal/domain"
	mediaUC "github.com/vladrzvk/weepost/internal/usecases/media"
)

type IUploadMediaUC interface {
	Execute(ctx context.Context, cmd mediaUC.UploadMediaCommand) domain.Result[mediaUC.UploadMediaResult]
}

type MediaHandler struct {
	uploadUC IUploadMediaUC
}

func NewMediaHandler(upload IUploadMediaUC) *MediaHandler {
	return &MediaHandler{uploadUC: upload}
}

// RegisterRoutes enregistre les routes media.
// NOTE : ?workspace_id=...&brand_id=... requis (brand-scoped, TX-08).
func (h *MediaHandler) RegisterRoutes(r fiber.Router, jwtMw fiber.Handler, permMw func(string) fiber.Handler) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/media?workspace_id=...&brand_id=...
	// Permission TX-08
	v1.Post("/media", jwtMw, permMw("media.upload"), h.Upload)
}

// Upload — POST /api/v1/media
// Upload multipart. FileHeader extrait par le UC via le context ou un champ dédié.
// T17 UploadMediaCommand contient le contenu du fichier ([]byte) et les métadonnées.
func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	var cmd mediaUC.UploadMediaCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}

	// Fichier multipart — extraire depuis FormFile si le UC accepte un FileHeader
	file, err := c.FormFile("file")
	if err != nil {
		return respondBadRequest(c, "Fichier 'file' manquant dans le formulaire multipart")
	}

	cmd.ActorID = getUserID(c)
	cmd.Filename = file.Filename
	cmd.SizeBytes = file.Size
	cmd.ContentType = file.Header.Get("Content-Type")
	// Lecture du contenu via io.Reader dans l'adapteur — T17 définit l'interface complète

	result := h.uploadUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}
