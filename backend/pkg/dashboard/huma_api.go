package dashboard

// Huma (type-first) implementation of the dashboard API.
//
// The response schema is derived from *core.DashboardData, so the documented
// shape cannot drift from what the handler actually returns. See
// .claude/planning/huma-migration.md.

import (
	"context"

	"actionphase/pkg/core"

	"github.com/danielgtaylor/huma/v2"
)

type getDashboardOutput struct {
	Body *core.DashboardData
}

// HumaGetUserDashboard returns aggregated dashboard data for the authenticated
// user.
func (h *Handler) HumaGetUserDashboard(ctx context.Context, _ *struct{}) (*getDashboardOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_dashboard")()

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to authenticate user from JWT")
		return nil, huma.Error401Unauthorized("no valid token found")
	}
	h.App.ObsLogger.Info(ctx, "Authenticated user for dashboard retrieval", "user_id", userID)

	dashboard, err := h.DashboardService.GetUserDashboard(ctx, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get dashboard data", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to get dashboard data")
	}

	h.App.ObsLogger.Info(ctx, "Dashboard data retrieved successfully",
		"user_id", userID,
		"has_games", dashboard.HasGames,
		"game_count", len(dashboard.PlayerGames)+len(dashboard.GMGames)+len(dashboard.MixedRoleGames))

	return &getDashboardOutput{Body: dashboard}, nil
}

// RegisterHumaDashboard registers the dashboard operations on api.
//
// Paths are relative to the package's chi mount point (/api/v1/dashboard).
func RegisterHumaDashboard(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "getUserDashboard",
		Method:      "GET",
		Path:        "/",
		Summary:     "Get the authenticated user's dashboard",
		Description: "Aggregated dashboard for the authenticated user, combining " +
			"several queries into one response.\n\nReturns the user's games split " +
			"by role (player, GM, audience, mixed), recent messages, upcoming " +
			"deadlines, and unread notification counts broken down by type.",
		Tags:     []string{"Dashboard"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaGetUserDashboard)
}
