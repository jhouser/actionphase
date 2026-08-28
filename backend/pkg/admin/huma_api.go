package admin

// Huma (type-first) implementation of the admin API.
//
// This is the reference implementation for the migration: admin was converted
// first, its chi handlers deleted, and api_test.go repointed here with its
// assertions unchanged. See .claude/planning/huma-migration.md.
//
// The point of a type-first handler is that the OpenAPI spec is *derived* from
// the Go signature rather than written beside it. Path/query params, request
// bodies, status codes, and response schemas all come from the types below, so
// the two cannot drift: changing a field changes the spec on the next build.
//
// The manual chi.URLParam + strconv.ParseInt + renderError preamble the chi
// handlers needed is gone, absorbed by huma's binding.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"actionphase/pkg/core"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------- I/O types
//
// Each field's tag drives both binding and the generated schema:
//   path/query  -> parameter, with type and default published in the spec
//   doc         -> the parameter/field description shown in Swagger UI
//   required    -> validation, enforced before the handler runs

type userIDPathInput struct {
	ID int32 `path:"id" doc:"Target user ID"`
}

type banIDPathInput struct {
	ID int32 `path:"id" doc:"Ban record ID"`
}

type listAdminsOutput struct {
	Body []*core.User
}

type listUsersInput struct {
	Page   int    `query:"page" default:"1" minimum:"1" doc:"Page number"`
	Limit  int    `query:"limit" default:"25" minimum:"1" maximum:"100" doc:"Results per page"`
	Search string `query:"search" doc:"Filter by username or email"`
}

type listUsersOutput struct {
	Body userListResponse
}

type listUsersSimpleOutput struct {
	Body []*core.User
}

type listBannedUsersOutput struct {
	Body []*core.BannedUser
}

type sessionsOutput struct {
	Body []*core.SessionWithDetails
}

type createIPBanInput struct {
	Body struct {
		IPAddress    string     `json:"ip_address" required:"true" minLength:"1" doc:"IPv4 or IPv6 address to ban"`
		Reason       string     `json:"reason,omitempty" required:"false" doc:"Why the ban was issued"`
		ExpiresAt    *time.Time `json:"expires_at,omitempty" doc:"Ban expiry; omit for permanent"`
		BannedUserID *int32     `json:"banned_user_id,omitempty" doc:"User this ban originated from"`
	}
}

type ipBanOutput struct {
	Body *core.IPBan
}

type listIPBansOutput struct {
	Body []*core.IPBan
}

type createFingerprintBanInput struct {
	Body struct {
		// minLength rejects "" (required:"true" only rejects absence);
		// maxLength mirrors the chi handler's 512 check and keeps an
		// oversized value from reaching the varchar(255) column as a 500.
		Fingerprint  string `json:"fingerprint" required:"true" minLength:"1" maxLength:"512" doc:"Device fingerprint hash"`
		Reason       string `json:"reason,omitempty" required:"false" doc:"Why the ban was issued"`
		BannedUserID *int32 `json:"banned_user_id,omitempty" doc:"User this ban originated from"`
	}
}

type fingerprintBanOutput struct {
	Body *core.FingerprintBan
}

type listFingerprintBansOutput struct {
	Body []*core.FingerprintBan
}

type messageIDPathInput struct {
	MessageID int32 `path:"messageId" doc:"Message to delete"`
}

// ------------------------------------------------------------------ helpers

// adminIDFromContext mirrors getUserIDFromContext but reads from a plain
// context, since a type-first handler never sees *http.Request. core's helper
// is already context-based, so this is a thin wrapper for the error shape.
func adminIDFromContext(ctx context.Context) (int32, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		return 0, errors.New("authentication required")
	}
	return authUser.ID, nil
}

// ----------------------------------------------------------------- handlers

func (h *Handler) HumaListAdmins(ctx context.Context, _ *struct{}) (*listAdminsOutput, error) {
	admins, err := h.UserService.ListAdmins(ctx)
	if err != nil {
		h.App.Logger.Error("Failed to list admins", "error", err)
		return nil, huma.Error500InternalServerError("Failed to list admins")
	}
	h.App.Logger.Info("Listed admins", "count", len(admins))
	return &listAdminsOutput{Body: admins}, nil
}

func (h *Handler) HumaGrantAdmin(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	requesterID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	if err := h.UserService.SetAdminStatus(ctx, in.ID, true, requesterID); err != nil {
		h.App.Logger.Error("Failed to grant admin status",
			"error", err, "target_user_id", in.ID, "requester_id", requesterID)
		return nil, huma.Error500InternalServerError("Failed to grant admin status")
	}
	h.App.Logger.Info("Granted admin status", "user_id", in.ID, "granted_by", requesterID)
	return nil, nil
}

func (h *Handler) HumaRevokeAdmin(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	requesterID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	if err := h.UserService.SetAdminStatus(ctx, in.ID, false, requesterID); err != nil {
		h.App.Logger.Error("Failed to revoke admin status",
			"error", err, "target_user_id", in.ID, "requester_id", requesterID)
		return nil, huma.Error500InternalServerError("Failed to revoke admin status")
	}
	h.App.Logger.Info("Revoked admin status", "user_id", in.ID, "revoked_by", requesterID)
	return nil, nil
}

func (h *Handler) HumaBanUser(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	adminID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	if err := h.UserService.BanUser(ctx, in.ID, adminID); err != nil {
		h.App.Logger.Error("Failed to ban user", "error", err, "target_user_id", in.ID, "admin_id", adminID)
		return nil, huma.Error500InternalServerError("Failed to ban user")
	}
	// Session invalidation is best-effort, matching the Chi handler.
	if err := h.SessionService.InvalidateAllUserSessions(ctx, in.ID); err != nil {
		h.App.Logger.Error("Failed to invalidate sessions for banned user", "error", err, "user_id", in.ID)
	}
	h.App.Logger.Info("Banned user", "user_id", in.ID, "banned_by", adminID)
	return nil, nil
}

func (h *Handler) HumaUnbanUser(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	adminID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	if err := h.UserService.UnbanUser(ctx, in.ID); err != nil {
		h.App.Logger.Error("Failed to unban user", "error", err, "target_user_id", in.ID, "admin_id", adminID)
		return nil, huma.Error500InternalServerError("Failed to unban user")
	}
	h.App.Logger.Info("Unbanned user", "user_id", in.ID, "unbanned_by", adminID)
	return nil, nil
}

func (h *Handler) HumaListBannedUsers(ctx context.Context, _ *struct{}) (*listBannedUsersOutput, error) {
	users, err := h.UserService.ListBannedUsers(ctx)
	if err != nil {
		h.App.Logger.Error("Failed to list banned users", "error", err)
		return nil, huma.Error500InternalServerError("Failed to list banned users")
	}
	return &listBannedUsersOutput{Body: users}, nil
}

func (h *Handler) HumaListUsers(ctx context.Context, in *listUsersInput) (*listUsersOutput, error) {
	users, total, err := h.UserService.ListAllUsersAdmin(ctx, in.Page, in.Limit, in.Search)
	if err != nil {
		h.App.Logger.Error("Failed to list users", "error", err)
		return nil, huma.Error500InternalServerError("Failed to list users")
	}
	return &listUsersOutput{Body: userListResponse{
		Users: users, Total: total, Page: in.Page, PageSize: in.Limit,
	}}, nil
}

func (h *Handler) HumaListPendingUsers(ctx context.Context, _ *struct{}) (*listUsersSimpleOutput, error) {
	users, err := h.UserService.ListPendingApprovalUsers(ctx)
	if err != nil {
		h.App.Logger.Error("Failed to list pending users", "error", err)
		return nil, huma.Error500InternalServerError("Failed to list pending users")
	}
	return &listUsersSimpleOutput{Body: users}, nil
}

func (h *Handler) HumaApproveUser(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	user, err := h.UserService.GetUserByID(int(in.ID))
	if err != nil {
		return nil, huma.Error404NotFound("user not found")
	}
	if !user.PendingApproval {
		return nil, huma.Error400BadRequest("user is not pending approval")
	}
	if err := h.UserService.ApproveUser(ctx, in.ID); err != nil {
		h.App.Logger.Error("Failed to approve user", "error", err, "target_user_id", in.ID)
		return nil, huma.Error500InternalServerError("Failed to approve user")
	}
	h.App.Logger.Info("User approved", "user_id", in.ID)
	return nil, nil
}

func (h *Handler) HumaRejectUser(ctx context.Context, in *userIDPathInput) (*struct{}, error) {
	user, err := h.UserService.GetUserByID(int(in.ID))
	if err != nil {
		return nil, huma.Error404NotFound("user not found")
	}
	if !user.PendingApproval {
		return nil, huma.Error400BadRequest("user is not pending approval")
	}
	if err := h.UserService.RejectUser(ctx, in.ID); err != nil {
		h.App.Logger.Error("Failed to reject user", "error", err, "target_user_id", in.ID)
		return nil, huma.Error500InternalServerError("Failed to reject user")
	}
	h.App.Logger.Info("Pending user rejected and deleted", "user_id", in.ID)
	return nil, nil
}

func (h *Handler) HumaGetUserSessions(ctx context.Context, in *userIDPathInput) (*sessionsOutput, error) {
	sessions, err := h.SessionService.GetUserSessionsWithDetails(ctx, in.ID)
	if err != nil {
		h.App.Logger.Error("Failed to get user sessions", "error", err, "user_id", in.ID)
		return nil, huma.Error500InternalServerError("Failed to get user sessions")
	}
	return &sessionsOutput{Body: sessions}, nil
}

func (h *Handler) HumaListIPBans(ctx context.Context, _ *struct{}) (*listIPBansOutput, error) {
	bans, err := h.IPBanService.ListIPBans(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to list i p bans")
	}
	return &listIPBansOutput{Body: bans}, nil
}

func (h *Handler) HumaCreateIPBan(ctx context.Context, in *createIPBanInput) (*ipBanOutput, error) {
	// `required:"true"` already rejected an empty string upstream; this is the
	// semantic check the tag cannot express.
	if net.ParseIP(strings.TrimSpace(in.Body.IPAddress)) == nil {
		return nil, huma.Error400BadRequest("ip_address is not a valid IPv4 or IPv6 address")
	}

	adminID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}

	ban, err := h.IPBanService.CreateIPBan(ctx, in.Body.IPAddress, in.Body.Reason, adminID, in.Body.ExpiresAt, in.Body.BannedUserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, huma.Error400BadRequest("this IP address is already banned")
		}
		return nil, huma.Error500InternalServerError("Failed to create i p ban")
	}

	if err := h.SessionService.InvalidateSessionsByIP(ctx, in.Body.IPAddress); err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to invalidate sessions for banned IP",
			"ip_address", in.Body.IPAddress, "error", err)
	}
	return &ipBanOutput{Body: ban}, nil
}

func (h *Handler) HumaDeleteIPBan(ctx context.Context, in *banIDPathInput) (*struct{}, error) {
	if err := h.IPBanService.DeleteIPBan(ctx, in.ID); err != nil {
		return nil, huma.Error500InternalServerError("Failed to delete i p ban")
	}
	return nil, nil
}

func (h *Handler) HumaListFingerprintBans(ctx context.Context, _ *struct{}) (*listFingerprintBansOutput, error) {
	bans, err := h.FingerprintBanService.ListFingerprintBans(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to list fingerprint bans")
	}
	return &listFingerprintBansOutput{Body: bans}, nil
}

func (h *Handler) HumaCreateFingerprintBan(ctx context.Context, in *createFingerprintBanInput) (*fingerprintBanOutput, error) {
	adminID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	ban, err := h.FingerprintBanService.CreateFingerprintBan(ctx, in.Body.Fingerprint, in.Body.Reason, adminID, in.Body.BannedUserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, huma.Error400BadRequest("this fingerprint is already banned")
		}
		return nil, huma.Error500InternalServerError("Failed to create fingerprint ban")
	}
	return &fingerprintBanOutput{Body: ban}, nil
}

func (h *Handler) HumaDeleteFingerprintBan(ctx context.Context, in *banIDPathInput) (*struct{}, error) {
	if err := h.FingerprintBanService.DeleteFingerprintBan(ctx, in.ID); err != nil {
		return nil, huma.Error500InternalServerError("Failed to delete fingerprint ban")
	}
	return nil, nil
}

func (h *Handler) HumaDeleteMessage(ctx context.Context, in *messageIDPathInput) (*struct{}, error) {
	adminID, err := adminIDFromContext(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid token")
	}
	canDelete, err := h.MessageService.CanUserDeleteComment(ctx, in.MessageID, adminID, true)
	if err != nil {
		h.App.Logger.Error("Failed to check delete permission",
			"error", err, "message_id", in.MessageID, "admin_id", adminID)
		return nil, huma.Error500InternalServerError("Failed to delete message")
	}
	if !canDelete {
		h.App.Logger.Warn("Admin attempted to delete already-deleted message",
			"message_id", in.MessageID, "admin_id", adminID)
		return nil, huma.Error403Forbidden("Message is already deleted")
	}
	if err := h.MessageService.DeleteComment(ctx, in.MessageID, adminID); err != nil {
		h.App.Logger.Error("Failed to delete message",
			"error", err, "message_id", in.MessageID, "admin_id", adminID)
		return nil, huma.Error500InternalServerError("Failed to delete message")
	}
	return nil, nil
}

// -------------------------------------------------------------- registration
//
// This block replaces the r.Get/r.Put/... lines in root.go. Everything the
// spec needs — summary, tags, status code, auth requirement — is declared
// here or inferred from the handler's types.

// RegisterHumaAdmin wires the admin operations onto a huma API.
func RegisterHumaAdmin(api huma.API, h *Handler) {
	op := func(id, method, path, summary string, status int) huma.Operation {
		return huma.Operation{
			OperationID:   id,
			Method:        method,
			Path:          path,
			Summary:       summary,
			Tags:          []string{"Admin"},
			DefaultStatus: status,
			Security:      []map[string][]string{{"BearerAuth": {}}},
			Responses: map[string]*huma.Response{
				"401": {Description: "Not authenticated"},
				"403": {Description: "Not an admin"},
			},
		}
	}

	huma.Register(api, op("adminListAdmins", http.MethodGet, "/admins",
		"List all admin users", http.StatusOK), h.HumaListAdmins)

	huma.Register(api, op("adminGrantAdmin", http.MethodPut, "/users/{id}/admin",
		"Grant admin privileges", http.StatusNoContent), h.HumaGrantAdmin)
	huma.Register(api, op("adminRevokeAdmin", http.MethodDelete, "/users/{id}/admin",
		"Revoke admin privileges", http.StatusNoContent), h.HumaRevokeAdmin)

	huma.Register(api, op("adminBanUser", http.MethodPost, "/users/{id}/ban",
		"Ban a user and invalidate their sessions", http.StatusNoContent), h.HumaBanUser)
	huma.Register(api, op("adminUnbanUser", http.MethodDelete, "/users/{id}/ban",
		"Lift a user ban", http.StatusNoContent), h.HumaUnbanUser)
	huma.Register(api, op("adminListBannedUsers", http.MethodGet, "/users/banned",
		"List banned users", http.StatusOK), h.HumaListBannedUsers)

	huma.Register(api, op("adminListUsers", http.MethodGet, "/users",
		"List all users (paginated, searchable)", http.StatusOK), h.HumaListUsers)
	huma.Register(api, op("adminListPendingUsers", http.MethodGet, "/users/pending",
		"List accounts awaiting approval", http.StatusOK), h.HumaListPendingUsers)
	huma.Register(api, op("adminApproveUser", http.MethodPost, "/users/{id}/approve",
		"Approve a pending account", http.StatusNoContent), h.HumaApproveUser)
	huma.Register(api, op("adminRejectUser", http.MethodPost, "/users/{id}/reject",
		"Reject a pending account", http.StatusNoContent), h.HumaRejectUser)
	huma.Register(api, op("adminGetUserSessions", http.MethodGet, "/users/{id}/sessions",
		"List a user's active sessions", http.StatusOK), h.HumaGetUserSessions)

	huma.Register(api, op("adminListIPBans", http.MethodGet, "/ip-bans",
		"List IP bans", http.StatusOK), h.HumaListIPBans)
	huma.Register(api, op("adminCreateIPBan", http.MethodPost, "/ip-bans",
		"Ban an IP address", http.StatusCreated), h.HumaCreateIPBan)
	huma.Register(api, op("adminDeleteIPBan", http.MethodDelete, "/ip-bans/{id}",
		"Remove an IP ban", http.StatusNoContent), h.HumaDeleteIPBan)

	huma.Register(api, op("adminListFingerprintBans", http.MethodGet, "/fingerprint-bans",
		"List device fingerprint bans", http.StatusOK), h.HumaListFingerprintBans)
	huma.Register(api, op("adminCreateFingerprintBan", http.MethodPost, "/fingerprint-bans",
		"Ban a device fingerprint", http.StatusCreated), h.HumaCreateFingerprintBan)
	huma.Register(api, op("adminDeleteFingerprintBan", http.MethodDelete, "/fingerprint-bans/{id}",
		"Remove a fingerprint ban", http.StatusNoContent), h.HumaDeleteFingerprintBan)

	huma.Register(api, op("adminDeleteMessage", http.MethodDelete, "/messages/{messageId}",
		"Delete a message (moderation)", http.StatusNoContent), h.HumaDeleteMessage)
}
