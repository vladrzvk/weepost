package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/vladrzvk/weepost/internal/application"
	"github.com/vladrzvk/weepost/internal/domain"
)

// PermissionMiddleware vérifie qu'un utilisateur authentifié possède la permission donnée.
//
// permission : string canonique Phase 6 (ex. "workspace.update_settings", "brand.update").
//
// Extraction des IDs :
//   - workspaceID : c.Params("workspaceID") → fallback c.Query("workspace_id")
//   - brandID     : c.Params("brandID")     → fallback c.Query("brand_id")
//
// NOTE : les routes brand-scoped (/brands/:brandID) doivent passer ?workspace_id=... en query
// car le workspaceID n'est pas dans le chemin. Limitation V0 — voir Anomalie A067/A068.
//
// BillingWarning : si subscription.status = past_due → header X-Billing-Warning: subscription_past_due.
func PermissionMiddleware(permission string, checker *application.PermissionChecker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals(LocalsUserID).(uuid.UUID)
		if !ok {
			return respondError(c, 401, "UNAUTHENTICATED", "Utilisateur non authentifié")
		}

		// Résolution workspaceID — route param en priorité, query param en fallback
		wsIDStr := c.Params("workspaceID")
		if wsIDStr == "" {
			wsIDStr = c.Query("workspace_id")
		}
		workspaceID, _ := uuid.Parse(wsIDStr)

		// Résolution brandID
		brandIDStr := c.Params("brandID")
		if brandIDStr == "" {
			brandIDStr = c.Query("brand_id")
		}
		brandID, _ := uuid.Parse(brandIDStr)

		resource, resErr := buildResource(permission, workspaceID, brandID)
		if resErr != nil {
			return c.Status(domain.GetHTTPStatus(resErr.Code)).JSON(ErrorResponse{
				Code:    string(resErr.Code),
				Message: resErr.Message,
				TraceID: getTraceID(c),
			})
		}

		result := checker.CheckPermission(c.UserContext(), userID, domain.Action(permission), resource)
		if result.IsFail() {
			domErr := result.Err()
			return c.Status(domain.GetHTTPStatus(domErr.Code)).JSON(ErrorResponse{
				Code:    string(domErr.Code),
				Message: domErr.Message,
				Details: domErr.Details,
				TraceID: getTraceID(c),
			})
		}

		// Phase 6 §0.2 — subscription past_due : accès maintenu, warning client
		if result.Value().BillingWarning {
			c.Set("X-Billing-Warning", "subscription_past_due")
		}

		return c.Next()
	}
}

// workspaceScopedActions — Phase 6 §2.1 + brand.create (WS-15)
// Ces actions utilisent ResourceTypeWorkspace ; toutes les autres → ResourceTypeBrand.
var workspaceScopedActions = map[string]bool{
	"workspace.read":                  true,
	"workspace.update_settings":       true,
	"workspace.delete":                true,
	"workspace.billing.read":          true,
	"workspace.billing.manage":        true,
	"workspace.gdpr_manage":           true,
	"member.invite":                   true,
	"member.cancel_invitation":        true,
	"member.resend_invitation":        true,
	"member.remove":                   true,
	"member.update_role":              true,
	"member.suspend":                  true,
	"member.reactivate":               true,
	"member.view_list":                true,
	"ownership.transfer":              true,
	"brand.create":                    true,
	"audit.read":                      true,
	"security.manage":                 true,
	"notification_preferences.manage": true,
}

func buildResource(permission string, workspaceID, brandID uuid.UUID) (domain.Resource, *domain.DomainError) {
	if workspaceID == uuid.Nil {
		return domain.Resource{}, domain.NewDomainError(
			domain.ErrCodeSECURITY_VIOLATION,
			"workspaceID invalide — uuid.Nil non autorisé",
			nil,
			domain.SeverityHIGH,
			false,
		)
	}
	if workspaceScopedActions[permission] {
		return domain.Resource{
			Type:        domain.ResourceTypeWorkspace,
			WorkspaceID: workspaceID,
			ResourceID:  workspaceID,
		}, nil
	}
	return domain.Resource{
		Type:        domain.ResourceTypeBrand,
		WorkspaceID: workspaceID,
		BrandID:     brandID,
		ResourceID:  brandID,
	}, nil
}
