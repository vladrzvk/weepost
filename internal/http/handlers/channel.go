package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/domain"
	channelUC "github.com/vladrzvk/weepost/internal/usecases/channel"
)

type (
	IConnectChannelUC interface {
		Execute(ctx context.Context, cmd channelUC.ConnectChannelCommand) domain.Result[channelUC.ConnectChannelResult]
	}
	IDisconnectChannelUC interface {
		Execute(ctx context.Context, cmd channelUC.DisconnectChannelCommand) domain.Result[struct{}]
	}
)

type ChannelHandler struct {
	connectUC    IConnectChannelUC
	disconnectUC IDisconnectChannelUC
}

func NewChannelHandler(connect IConnectChannelUC, disconnect IDisconnectChannelUC) *ChannelHandler {
	return &ChannelHandler{connectUC: connect, disconnectUC: disconnect}
}

// RegisterRoutes enregistre les routes channel.
// NOTE brand-scoped : passer ?workspace_id={id} sur toutes ces routes.
func (h *ChannelHandler) RegisterRoutes(r fiber.Router, jwtMw fiber.Handler, permMw func(string) fiber.Handler) {
	v1 := r.Group("/api/v1")

	// POST /api/v1/brands/:brandID/channels?workspace_id=...
	// Permission BR-10
	v1.Post("/brands/:brandID/channels", jwtMw, permMw("channel.connect"), h.Connect)

	// DELETE /api/v1/channels/:channelID?workspace_id=...&brand_id=...
	// Permission BR-11
	v1.Delete("/channels/:channelID", jwtMw, permMw("channel.disconnect"), h.Disconnect)
}

// Connect — POST /api/v1/brands/:brandID/channels
// Connecte un réseau social via OAuth (T13). Vérifie quota MAX_CHANNELS_REACHED.
func (h *ChannelHandler) Connect(c *fiber.Ctx) error {
	var cmd channelUC.ConnectChannelCommand
	if err := c.BodyParser(&cmd); err != nil {
		return respondBadRequest(c, "Corps de requête invalide")
	}
	brandID, err := uuid.Parse(c.Params("brandID"))
	if err != nil {
		return respondBadRequest(c, "brandID invalide")
	}
	cmd.BrandID = brandID
	cmd.ActorID = getUserID(c)

	result := h.connectUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.Status(201).JSON(result.Value())
}

// Disconnect — DELETE /api/v1/channels/:channelID
// Déconnecte un channel (T13). SM-07 : connected → disconnected.
func (h *ChannelHandler) Disconnect(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("channelID"))
	if err != nil {
		return respondBadRequest(c, "channelID invalide")
	}
	cmd := channelUC.DisconnectChannelCommand{
		ChannelID: channelID,
		ActorID:   getUserID(c),
	}
	result := h.disconnectUC.Execute(c.UserContext(), cmd)
	if result.IsFail() {
		return respondErr(c, result.Err())
	}
	return c.SendStatus(204)
}
