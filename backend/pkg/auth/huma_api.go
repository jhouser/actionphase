package auth

// Huma (type-first) implementation of the authentication API.
//
// Auth is the first package whose routes do not share one middleware stack.
// Under the single /api/v1/auth mount there are four chi groups:
//
//	public          logout, reset-password, validate-reset-token, verify-email,
//	                complete-email-change
//	rate limited    register, login, request-password-reset (+ resend-verification,
//	                which is also authenticated)
//	probe           me -- jwtauth.Verifier only, deliberately NOT Authenticator
//	protected       everything else (Verifier + Authenticator + session check +
//	                RequireAuthentication)
//
// Huma binds an API to a chi router, so each group gets its own huma.API and
// the middleware carries over byte-identically -- rate limiting in particular
// stays real chi middleware rather than being reimplemented per handler.
// generatedSpecFor merges the four documents under the one /auth prefix.
//
// Not converted: V1DiscordCallback. It answers a browser redirect (302 to the
// frontend) and writes plain-text errors via http.Error, so it has no JSON
// shape to document -- the same reasoning that leaves /ping on chi.
//
// See .claude/planning/huma-migration.md.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"actionphase/pkg/core"
	"actionphase/pkg/email"
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// ---------------------------------------------------------------------------
// Shared response types
// ---------------------------------------------------------------------------

// messageBody is the {"message": "..."} envelope most auth mutations return.
type messageBody struct {
	Message string `json:"message"`
}

type messageOutput struct {
	Body messageBody
}

// authUser mirrors core.User for auth responses, minus the password field.
//
// core.User carries `json:"password"` with no omitempty, so the chi handlers
// served the user's bcrypt hash to any authenticated caller of /auth/me. There
// is no legitimate client for it -- the frontend's User type even declares
// `password?: string` -- so the typed response omits it. Every other field is
// reproduced tag-for-tag; changing one would break the 236 frontend call sites.
type authUser struct {
	ID                   int        `json:"id"`
	Username             string     `json:"username"`
	Email                string     `json:"email"`
	EmailVerified        bool       `json:"email_verified"`
	Bio                  *string    `json:"bio,omitempty"`
	AvatarURL            *string    `json:"avatar_url,omitempty"`
	IsAdmin              bool       `json:"is_admin"`
	IsBanned             bool       `json:"is_banned"`
	BannedAt             *time.Time `json:"banned_at,omitempty"`
	BannedByUserID       *int32     `json:"banned_by_user_id,omitempty"`
	CreatedAt            *time.Time `json:"createdAt"`
	PendingApproval      bool       `json:"pending_approval"`
	PendingApprovalSince *time.Time `json:"pending_approval_since,omitempty"`
	DiscordUsername      *string    `json:"discord_username,omitempty"`

	// Token is capital-T because that is what the API has always sent, from an
	// untagged Go field. The frontend reads `data.Token || data.token`, so the
	// casing is load-bearing for the first half of that expression.
	Token string `json:"Token"`
}

// newAuthUser converts a core.User into the wire shape, dropping the password.
func newAuthUser(u *core.User, token string) *authUser {
	if u == nil {
		return nil
	}
	return &authUser{
		ID:                   u.ID,
		Username:             u.Username,
		Email:                u.Email,
		EmailVerified:        u.EmailVerified,
		Bio:                  u.Bio,
		AvatarURL:            u.AvatarURL,
		IsAdmin:              u.IsAdmin,
		IsBanned:             u.IsBanned,
		BannedAt:             u.BannedAt,
		BannedByUserID:       u.BannedByUserID,
		CreatedAt:            u.CreatedAt,
		PendingApproval:      u.PendingApproval,
		PendingApprovalSince: u.PendingApprovalSince,
		DiscordUsername:      u.DiscordUsername,
		Token:                token,
	}
}

// tokenOnlyBody is the login/refresh response: a bare token, no user object.
type tokenOnlyBody struct {
	Token string `json:"Token"`
}

type tokenOnlyOutput struct {
	Body tokenOnlyBody
}

// clientMeta pulls the request-derived session metadata. Falls back to empty
// strings when the request is unavailable, which only happens if an API was
// built without humaconfig.RequestMiddleware.
func clientMeta(ctx context.Context) (ip, userAgent string) {
	r, _, ok := humaconfig.RequestFrom(ctx)
	if !ok {
		return "", ""
	}
	return core.GetClientIP(r), r.UserAgent()
}

// ---------------------------------------------------------------------------
// Public: login, logout
// ---------------------------------------------------------------------------

// loginBody accepts either a username or an email in `username`, matching the
// chi handler: it prefers `email` when both are sent, and treats a `username`
// containing "@" as an email. Neither field is required on its own, because
// either one may carry the identifier -- the handler rejects the request when
// both are empty, exactly as before.
type loginBody struct {
	Username      string `json:"username,omitempty" required:"false" doc:"Username or email address"`
	Email         string `json:"email,omitempty" required:"false" doc:"Email address; takes precedence over username"`
	Password      string `json:"password" required:"false" doc:"Account password"`
	Fingerprint   string `json:"fingerprint,omitempty" required:"false" maxLength:"512" doc:"Device fingerprint, recorded on the session"`
	HCaptchaToken string `json:"hcaptcha_token,omitempty" required:"false"`
	HoneypotValue string `json:"honeypot_value,omitempty" required:"false"`
}

type loginInput struct {
	Body loginBody
}

// HumaLogin authenticates a user and issues a JWT, setting it as the jwt cookie.
func (h *Handler) HumaLogin(ctx context.Context, in *loginInput) (*tokenOnlyOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_login")()

	req, w, haveReq := humaconfig.RequestFrom(ctx)

	// IP ban is checked before any user lookup, so a banned network cannot
	// probe which accounts exist.
	if haveReq {
		clientIP := core.GetClientIP(req)
		if banned, _ := h.IPBanService.IsIPBanned(ctx, clientIP); banned {
			h.App.ObsLogger.Warn(ctx, "Blocked request from banned IP", "ip", clientIP)
			return nil, huma.Error403Forbidden("Access from this location is not allowed.")
		}
	}

	if in.Body.Fingerprint != "" {
		if banned, _ := h.FingerprintBanService.IsFingerprintBanned(ctx, in.Body.Fingerprint); banned {
			h.App.ObsLogger.Warn(ctx, "Blocked request from banned device fingerprint")
			return nil, huma.Error403Forbidden("Access from this device is not allowed.")
		}
	}

	usernameOrEmail := in.Body.Username
	if in.Body.Email != "" {
		usernameOrEmail = in.Body.Email
	}
	if usernameOrEmail == "" {
		h.App.ObsLogger.Warn(ctx, "Login attempt with no username or email provided")
		return nil, huma.Error401Unauthorized("Invalid username or password")
	}

	var user *core.User
	var err error
	if strings.Contains(usernameOrEmail, "@") {
		user, err = h.UserService.UserByEmail(usernameOrEmail)
	} else {
		user, err = h.UserService.UserByUsername(usernameOrEmail)
	}
	if err != nil {
		h.App.ObsLogger.Info(ctx, "Login attempt for non-existent user",
			"username", in.Body.Username, "email", in.Body.Email)
		return nil, huma.Error401Unauthorized("Invalid username or password")
	}

	if user.IsBanned {
		h.App.ObsLogger.Warn(ctx, "Login attempt by banned user",
			"username", user.Username, "user_id", user.ID, "banned_at", user.BannedAt)
		return nil, huma.Error403Forbidden("Your account has been banned. Please contact support.")
	}

	if user.PendingApproval {
		h.App.ObsLogger.Info(ctx, "Login attempt by pending-approval user",
			"username", user.Username, "user_id", user.ID)
		return nil, huma.Error403Forbidden("Your account is pending admin approval.")
	}

	// A wrong password is a 400 here, not a 401: that is what the chi handler
	// sent (core.ErrInvalidRequest) and what the tests assert.
	if !user.CheckPasswordHash(in.Body.Password) {
		h.App.ObsLogger.Warn(ctx, "Login failed: invalid password", "username", user.Username)
		return nil, huma.Error400BadRequest("invalid username or password")
	}

	h.App.ObsLogger.Info(ctx, "User logged in successfully", "username", user.Username, "user_id", user.ID)

	ip, ua := clientMeta(ctx)
	jwtHandler := JWTHandler{App: h.App}
	token, err := jwtHandler.CreateToken(user, SessionMetadata{
		IPAddress:   ip,
		UserAgent:   ua,
		Fingerprint: fingerprintPtr(in.Body.Fingerprint),
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create JWT token", "error", err, "user_id", user.ID)
		return nil, huma.Error500InternalServerError("Failed to create JWT token")
	}

	if haveReq {
		SetJWTCookie(w, token)
	}
	return &tokenOnlyOutput{Body: tokenOnlyBody{Token: token}}, nil
}

// logoutOutput has no body: the chi handler wrote WriteHeader(200) and nothing
// else. The hand-written spec claimed a {"message": ...} body and a 401; both
// were fiction -- the route is public and has never returned either.
//
// Status is set explicitly because huma infers 204 for a bodyless output, and
// this endpoint has always answered 200.
type logoutOutput struct {
	Status int
}

// HumaLogout clears the jwt cookie.
func (h *Handler) HumaLogout(ctx context.Context, _ *struct{}) (*logoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_logout")()

	if _, w, ok := humaconfig.RequestFrom(ctx); ok {
		ClearJWTCookie(w)
	}
	h.App.ObsLogger.Info(ctx, "User logged out successfully")
	return &logoutOutput{Status: http.StatusOK}, nil
}

// ---------------------------------------------------------------------------
// Probe: /me
// ---------------------------------------------------------------------------

// meOutput carries whichever of the two shapes /me answers with.
//
// This endpoint never returns 401 -- that is its entire purpose. It is mounted
// with jwtauth.Verifier but deliberately WITHOUT Authenticator, so the frontend
// can poll auth state without provoking console errors on a logged-out visitor.
// Authenticated requests get a bare user object; unauthenticated ones get
// {"user": null}. The two shapes are unioned in a raw body because the response
// really is one or the other, never a merge.
type meOutput struct {
	Body any
}

// nullUserBody is the logged-out answer: 200 with {"user": null}.
type nullUserBody struct {
	User *authUser `json:"user"`
}

// HumaMe returns the current user, or {"user": null} when not authenticated.
func (h *Handler) HumaMe(ctx context.Context, _ *struct{}) (*meOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_current_user")()

	loggedOut := &meOutput{Body: nullUserBody{User: nil}}

	token, _, err := jwtauth.FromContext(ctx)
	if err != nil || token == nil {
		return loggedOut, nil
	}

	// A valid JWT whose session row is gone (ban, forced logout, revoked
	// session) must read as logged out, or the frontend shows the user as
	// signed in against a session the server has already destroyed. Only
	// login-issued tokens carry session_id; tokens without it skip the check.
	if sessionIDVal, ok := token.Get("session_id"); ok {
		var sessionID int32
		switch v := sessionIDVal.(type) {
		case float64:
			sessionID = int32(v)
		case int32:
			sessionID = v
		case int64:
			sessionID = int32(v)
		}
		if sessionID > 0 {
			session, err := h.SessionService.GetSessionByID(ctx, sessionID)
			if err != nil || session == nil {
				return loggedOut, nil
			}
		}
	}

	userIDStr, ok := token.Get("sub")
	if !ok {
		return loggedOut, nil
	}

	var uid int
	if _, err := fmt.Sscanf(userIDStr.(string), "%d", &uid); err != nil || uid == 0 {
		return loggedOut, nil
	}

	user, err := h.UserService.GetUserByID(uid)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to find user", "error", err, "user_id", uid)
		return loggedOut, nil
	}

	h.App.ObsLogger.Info(ctx, "Current user retrieved", "user_id", uid)

	// Token is empty here, as it always has been on this endpoint.
	return &meOutput{Body: newAuthUser(user, "")}, nil
}

// ---------------------------------------------------------------------------
// Rate limited: register, request-password-reset
// ---------------------------------------------------------------------------

type registerBody struct {
	Username      string `json:"username" required:"true" doc:"Desired username"`
	Email         string `json:"email" required:"true" doc:"Email address; a verification mail is sent here"`
	Password      string `json:"password" required:"true" doc:"Account password"`
	Bio           string `json:"bio,omitempty" required:"false"`
	Fingerprint   string `json:"fingerprint,omitempty" required:"false" maxLength:"512" doc:"Device fingerprint, recorded on the session"`
	HCaptchaToken string `json:"hcaptcha_token,omitempty" required:"false"`
	HoneypotValue string `json:"honeypot_value,omitempty" required:"false" doc:"Must stay empty; a value marks the caller as a bot"`
}

type registerInput struct {
	Body registerBody
}

// registerOutput answers 201 with the created user plus a token, or 202 with an
// error-shaped body when registration approval is enabled. Status is dynamic
// for that reason.
type registerOutput struct {
	Status int
	Body   any
}

// pendingApprovalBody reproduces the ErrResponse shape the chi handler sent on
// the 202 path -- it rendered core.ErrResponse even though nothing failed.
type pendingApprovalBody struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// HumaRegister creates an account, sends a verification email, and returns a token.
func (h *Handler) HumaRegister(ctx context.Context, in *registerInput) (*registerOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_register")()

	user := &core.User{
		Username: in.Body.Username,
		Email:    in.Body.Email,
		Password: in.Body.Password,
	}
	if in.Body.Bio != "" {
		bio := in.Body.Bio
		user.Bio = &bio
	}

	if err := user.Validate(); err != nil {
		h.App.ObsLogger.Warn(ctx, "Invalid v1 register request", "error", err)
		return nil, huma.Error400BadRequest(err.Error())
	}

	if err := validateUsername(in.Body.Username); err != nil {
		h.App.ObsLogger.Warn(ctx, "Invalid v1 register request", "error", err)
		return nil, huma.Error400BadRequest(err.Error())
	}

	ipAddress, userAgent := clientMeta(ctx)

	if req, _, ok := humaconfig.RequestFrom(ctx); ok {
		clientIP := core.GetClientIP(req)
		if banned, _ := h.IPBanService.IsIPBanned(ctx, clientIP); banned {
			h.App.ObsLogger.Warn(ctx, "Blocked request from banned IP", "ip", clientIP)
			return nil, huma.Error403Forbidden("Access from this location is not allowed.")
		}
	}
	if in.Body.Fingerprint != "" {
		if banned, _ := h.FingerprintBanService.IsFingerprintBanned(ctx, in.Body.Fingerprint); banned {
			h.App.ObsLogger.Warn(ctx, "Blocked request from banned device fingerprint")
			return nil, huma.Error403Forbidden("Access from this device is not allowed.")
		}
	}

	botService := NewBotPreventionService(h.App.Pool)
	checkRequest := &RegistrationCheckRequest{
		Email:         in.Body.Email,
		Username:      in.Body.Username,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		HCaptchaToken: in.Body.HCaptchaToken,
		HoneypotValue: in.Body.HoneypotValue,
	}

	result, err := botService.CheckRegistrationAttempt(ctx, checkRequest)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Bot prevention check failed", "error", err, "email", in.Body.Email)
		return nil, huma.Error500InternalServerError("Bot prevention check failed")
	}

	if !result.Allowed {
		h.App.ObsLogger.Warn(ctx, "Registration blocked by bot prevention",
			"reason", result.BlockedReason, "email", in.Body.Email, "ip", ipAddress)

		var errorMsg string
		switch result.BlockedReason {
		case "honeypot":
			errorMsg = "Invalid registration attempt detected"
		case "captcha_failed":
			errorMsg = "CAPTCHA verification failed. Please try again."
		case "rate_limit_ip":
			errorMsg = "Too many registration attempts from this IP address. Please try again later."
		case "rate_limit_email":
			errorMsg = "Too many registration attempts for this email. Please try again later."
		case "disposable_email":
			errorMsg = "Disposable email addresses are not allowed. Please use a permanent email address."
		default:
			errorMsg = "Registration not allowed at this time"
		}
		return nil, huma.Error400BadRequest(errorMsg)
	}

	h.App.ObsLogger.Info(ctx, "Creating user", "username", in.Body.Username)
	returnUser, err := h.UserService.CreateUser(user)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Invalid v1 register request", "error", err)
		return nil, huma.Error400BadRequest(err.Error())
	}

	if h.App.Config.App.RequireRegistrationApproval {
		if err := h.UserService.SetPendingApproval(ctx, int32(returnUser.ID)); err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to set pending approval", "error", err, "user_id", returnUser.ID)
		} else {
			h.App.ObsLogger.Info(ctx, "New account pending admin approval",
				"user_id", returnUser.ID, "username", returnUser.Username)
		}
		return &registerOutput{
			Status: http.StatusAccepted,
			Body: pendingApprovalBody{
				Status: "Pending Approval",
				Error:  "Your account has been created and is pending admin approval.",
			},
		}, nil
	}

	if err := botService.LogSuccessfulRegistration(ctx, checkRequest); err != nil {
		// A logging failure must not fail the registration.
		h.App.ObsLogger.Warn(ctx, "Failed to log successful registration", "error", err, "username", returnUser.Username)
	}

	h.App.ObsLogger.Info(ctx, "Creating token for new user", "username", returnUser.Username)
	jwtHandler := JWTHandler{App: h.App}
	token, err := jwtHandler.CreateToken(returnUser, SessionMetadata{
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Fingerprint: fingerprintPtr(in.Body.Fingerprint),
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to v1 register", "error", err)
		return nil, huma.Error500InternalServerError("Failed to create token")
	}

	if emailService, err := email.NewEmailServiceFromEnv(); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create email service", "error", err)
	} else {
		accountService := &AccountService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
		_ = accountService.SendVerificationEmail(ctx, &SendVerificationEmailRequest{
			UserID: returnUser.ID,
			Email:  returnUser.Email,
		})
	}

	return &registerOutput{Status: http.StatusCreated, Body: newAuthUser(returnUser, token)}, nil
}

// ---------------------------------------------------------------------------
// Protected: refresh, preferences, user search
// ---------------------------------------------------------------------------

// HumaRefresh issues a fresh token and session for the authenticated user.
func (h *Handler) HumaRefresh(ctx context.Context, _ *struct{}) (*tokenOnlyOutput, error) {
	token, claims, _ := jwtauth.FromContext(ctx)
	if token == nil || jwt.Validate(token) != nil {
		h.App.ObsLogger.Warn(ctx, "Unauthorized")
		return nil, huma.Error401Unauthorized("Invalid token")
	}

	sub, ok := claims["sub"]
	if !ok {
		h.App.Logger.Warn("Refresh token: sub (user_id) not found in token")
		return nil, huma.Error401Unauthorized("invalid token")
	}

	userID, err := strconv.Atoi(sub.(string))
	if err != nil {
		h.App.Logger.Error("Refresh token: invalid user_id in token", "error", err, "sub", sub)
		return nil, huma.Error401Unauthorized("invalid token")
	}

	user, err := h.UserService.GetUserByID(userID)
	if err != nil {
		h.App.Logger.Error("Error getting user", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to v1 refresh")
	}

	h.App.Logger.Info("Creating token for user", "user_id", user.ID, "username", user.Username)
	ip, ua := clientMeta(ctx)
	jwtHandler := JWTHandler{App: h.App}
	tokenString, err := jwtHandler.CreateToken(user, SessionMetadata{IPAddress: ip, UserAgent: ua})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to v1 refresh", "error", err)
		return nil, huma.Error500InternalServerError("Failed to v1 refresh")
	}

	if _, w, ok := humaconfig.RequestFrom(ctx); ok {
		SetJWTCookie(w, tokenString)
	}
	return &tokenOnlyOutput{Body: tokenOnlyBody{Token: tokenString}}, nil
}

// preferencesPayload mirrors core.PreferencesData but marks every field
// optional.
//
// Huma makes a plain (non-pointer) field required, and PreferencesData's three
// string fields carry no tags -- so reusing it directly rejected the partial
// object the chi handler accepted, e.g. {"preferences":{"theme":"dark"}} became
// "expected required property comment_read_mode to be present". Preferences
// updates are a full replace of whatever the caller supplies, and the frontend
// does send all three, but a partial body must keep working (gotcha 9).
type preferencesPayload struct {
	Theme                string          `json:"theme,omitempty" required:"false"`
	CommentReadMode      string          `json:"comment_read_mode,omitempty" required:"false"`
	FontSize             string          `json:"font_size,omitempty" required:"false"`
	DiscordNotifications map[string]bool `json:"discord_notifications,omitempty" required:"false"`
}

func (p *preferencesPayload) toCore() core.PreferencesData {
	return core.PreferencesData{
		Theme:                p.Theme,
		CommentReadMode:      p.CommentReadMode,
		FontSize:             p.FontSize,
		DiscordNotifications: p.DiscordNotifications,
	}
}

// preferencesBody is the response envelope, which returns the stored values.
type preferencesBody struct {
	Preferences *core.PreferencesData `json:"preferences"`
}

// updatePreferencesBody is the request envelope, which accepts a partial object.
type updatePreferencesBody struct {
	Preferences *preferencesPayload `json:"preferences"`
}

type preferencesOutput struct {
	Body preferencesBody
}

// currentUserID resolves the authenticated user from the JWT, matching the chi
// handlers: token -> sub claim -> database lookup, each step a 401 on failure.
func (h *Handler) currentUserID(ctx context.Context) (*core.User, error) {
	token, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get token from context", "error", err)
		return nil, huma.Error401Unauthorized("invalid token")
	}
	userIDStr, ok := token.Get("sub")
	if !ok {
		h.App.ObsLogger.Warn(ctx, "User ID not found in token")
		return nil, huma.Error401Unauthorized("user id not found in token")
	}
	var userID int
	fmt.Sscanf(userIDStr.(string), "%d", &userID)

	user, err := h.UserService.GetUserByID(userID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to find user", "error", err, "user_id", userID)
		return nil, huma.Error401Unauthorized("user not found")
	}
	return user, nil
}

// HumaGetPreferences returns the authenticated user's preferences.
func (h *Handler) HumaGetPreferences(ctx context.Context, _ *struct{}) (*preferencesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_preferences")()

	user, err := h.currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	prefs, err := h.UserPreferencesService.GetUserPreferences(ctx, int32(user.ID))
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get user preferences", "error", err, "user_id", user.ID)
		return nil, huma.Error500InternalServerError("Failed to get user preferences")
	}

	h.App.ObsLogger.Info(ctx, "User preferences retrieved", "user_id", user.ID)
	return &preferencesOutput{Body: preferencesBody{Preferences: prefs}}, nil
}

type updatePreferencesInput struct {
	Body updatePreferencesBody
}

// HumaUpdatePreferences replaces the authenticated user's preferences.
func (h *Handler) HumaUpdatePreferences(ctx context.Context, in *updatePreferencesInput) (*preferencesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_user_preferences")()

	user, err := h.currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	// The chi handler's Bind rejected a missing preferences object as a 400.
	if in.Body.Preferences == nil {
		h.App.ObsLogger.Warn(ctx, "Invalid request body", "user_id", user.ID)
		return nil, huma.Error400BadRequest("missing required preferences field")
	}

	prefs, err := h.UserPreferencesService.UpdateUserPreferences(ctx, int32(user.ID), in.Body.Preferences.toCore())
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update user preferences", "error", err, "user_id", user.ID)
		return nil, huma.Error500InternalServerError("Failed to update user preferences")
	}

	h.App.ObsLogger.Info(ctx, "User preferences updated", "user_id", user.ID, "theme", prefs.Theme)
	return &preferencesOutput{Body: preferencesBody{Preferences: prefs}}, nil
}

type searchUsersInput struct {
	Query string `query:"q" required:"true" minLength:"1" doc:"Username substring to search for"`
}

type searchUsersOutput struct {
	Body SearchUsersResponse
}

// HumaSearchUsers finds users by username substring.
func (h *Handler) HumaSearchUsers(ctx context.Context, in *searchUsersInput) (*searchUsersOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_search_users")()

	users, err := h.UserService.SearchUsers(ctx, in.Query)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to search users", "error", err, "query", in.Query)
		return nil, huma.Error500InternalServerError("Failed to search users")
	}

	h.App.ObsLogger.Info(ctx, "User search completed", "query", in.Query, "result_count", len(users))

	results := make([]UserSearchResult, 0, len(users))
	for _, user := range users {
		results = append(results, UserSearchResult{
			ID:        user.ID,
			Username:  user.Username,
			CreatedAt: user.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return &searchUsersOutput{Body: SearchUsersResponse{Users: results}}, nil
}

// ---------------------------------------------------------------------------
// Password: change, request reset, reset, validate token
// ---------------------------------------------------------------------------

// asHumaError maps a service error to the response the chi handlers produced:
// a PasswordValidationError became a 400 carrying its message, anything else
// the given 500 text.
func asHumaError(err error, serverMsg string) error {
	if pwdErr, ok := err.(*PasswordValidationError); ok {
		return huma.Error400BadRequest(pwdErr.Error())
	}
	return huma.Error500InternalServerError(serverMsg)
}

type changePasswordInput struct {
	Body ChangePasswordRequest
}

// HumaChangePassword changes the authenticated user's password.
func (h *Handler) HumaChangePassword(ctx context.Context, in *changePasswordInput) (*messageOutput, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		// Password change still works without mail; only the notice is lost.
		h.App.Logger.Error("Failed to create email service", "error", err)
		emailService = nil
	}

	passwordService := &PasswordService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
	if err := passwordService.ChangePassword(ctx, userID, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.Logger.Error("Failed to change password", "error", err, "user_id", userID)
		}
		return nil, asHumaError(err, "Failed to v1 change password")
	}

	h.App.Logger.Info("Password changed successfully", "user_id", userID)
	return &messageOutput{Body: messageBody{Message: "Password changed successfully"}}, nil
}

type requestPasswordResetInput struct {
	Body RequestPasswordResetRequest
}

// HumaRequestPasswordReset emails a reset link.
//
// It always answers 200 with the same message, whether or not the address
// exists, so the endpoint cannot be used to enumerate accounts.
func (h *Handler) HumaRequestPasswordReset(ctx context.Context, in *requestPasswordResetInput) (*messageOutput, error) {
	const sameAnswer = "If an account exists with this email, a password reset link will be sent"

	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.Logger.Error("Failed to create email service", "error", err)
		return &messageOutput{Body: messageBody{Message: sameAnswer}}, nil
	}

	passwordService := &PasswordService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
	if err := passwordService.RequestPasswordReset(ctx, &in.Body); err != nil {
		if pwdErr, ok := err.(*PasswordValidationError); ok {
			return nil, huma.Error400BadRequest(pwdErr.Error())
		}
		// Internal failures are logged but not revealed, for the same reason.
		h.App.Logger.Error("Failed to request password reset", "error", err)
	}

	h.App.Logger.Info("Password reset requested", "email", in.Body.Email)
	return &messageOutput{Body: messageBody{Message: sameAnswer}}, nil
}

type resetPasswordInput struct {
	Body ResetPasswordRequest
}

// HumaResetPassword completes a password reset using an emailed token.
func (h *Handler) HumaResetPassword(ctx context.Context, in *resetPasswordInput) (*messageOutput, error) {
	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.Logger.Error("Failed to create email service", "error", err)
		emailService = nil
	}

	passwordService := &PasswordService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
	if err := passwordService.ResetPassword(ctx, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.Logger.Error("Failed to reset password", "error", err)
		}
		return nil, asHumaError(err, "Failed to v1 reset password")
	}

	h.App.Logger.Info("Password reset successfully")
	return &messageOutput{Body: messageBody{Message: "Password reset successfully"}}, nil
}

type validateResetTokenInput struct {
	Token string `query:"token" required:"true" minLength:"1" doc:"Reset token from the emailed link"`
}

type validateResetTokenOutput struct {
	Body struct {
		Valid bool `json:"valid"`
	}
}

// HumaValidateResetToken reports whether a reset token is still usable, without
// consuming it, so the reset page can fail fast on an expired link.
func (h *Handler) HumaValidateResetToken(ctx context.Context, in *validateResetTokenInput) (*validateResetTokenOutput, error) {
	rows, err := h.App.Pool.Query(ctx,
		"SELECT id, user_id, expires_at, used_at FROM password_reset_tokens WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()",
		in.Token)
	if err != nil || !rows.Next() {
		if rows != nil {
			rows.Close()
		}
		return nil, huma.Error400BadRequest("This password reset link is invalid or has expired")
	}
	rows.Close()

	out := &validateResetTokenOutput{}
	out.Body.Valid = true
	return out, nil
}

// ---------------------------------------------------------------------------
// Account: email verification, username/email change, deletion
// ---------------------------------------------------------------------------

// accountService builds the service, tolerating a missing mail transport the
// same way the chi handlers did: nil EmailService rather than a failed request.
func (h *Handler) accountService(ctx context.Context) *AccountService {
	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create email service", "error", err)
		emailService = nil
	}
	return &AccountService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
}

type verifyEmailInput struct {
	Body VerifyEmailRequest
}

// HumaVerifyEmail confirms an email address using an emailed token.
func (h *Handler) HumaVerifyEmail(ctx context.Context, in *verifyEmailInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_verify_email")()

	if err := h.accountService(ctx).VerifyEmail(ctx, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.ObsLogger.Error(ctx, "Failed to verify email", "error", err)
		}
		return nil, asHumaError(err, "Failed to verify email")
	}

	h.App.ObsLogger.Info(ctx, "Email verified successfully")
	return &messageOutput{Body: messageBody{Message: "Email verified successfully"}}, nil
}

// HumaResendVerificationEmail re-sends the verification mail. Rate limited.
func (h *Handler) HumaResendVerificationEmail(ctx context.Context, _ *struct{}) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_resend_verification_email")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	// Unlike the other account operations this one requires a working mail
	// transport -- there is nothing to do without it.
	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create email service", "error", err)
		return nil, huma.Error500InternalServerError("Failed to create email service")
	}

	accountService := &AccountService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
	if err := accountService.ResendVerificationEmail(ctx, userID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to resend verification email", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to resend verification email")
	}

	h.App.ObsLogger.Info(ctx, "Verification email resent", "user_id", userID)
	return &messageOutput{Body: messageBody{Message: "Verification email sent"}}, nil
}

type changeUsernameInput struct {
	Body ChangeUsernameRequest
}

// HumaChangeUsername renames the authenticated user's account.
func (h *Handler) HumaChangeUsername(ctx context.Context, in *changeUsernameInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_change_username")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	accountService := &AccountService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	if err := accountService.ChangeUsername(ctx, userID, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.ObsLogger.Error(ctx, "Failed to change username", "error", err, "user_id", userID)
		}
		return nil, asHumaError(err, "Failed to change username")
	}

	h.App.ObsLogger.Info(ctx, "Username changed successfully", "user_id", userID, "new_username", in.Body.NewUsername)
	return &messageOutput{Body: messageBody{Message: "Username changed successfully"}}, nil
}

type requestEmailChangeInput struct {
	Body ChangeEmailRequest
}

// HumaRequestEmailChange starts an email change by mailing the new address.
func (h *Handler) HumaRequestEmailChange(ctx context.Context, in *requestEmailChangeInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_request_email_change")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create email service", "error", err)
		return nil, huma.Error500InternalServerError("Failed to create email service")
	}

	accountService := &AccountService{DB: h.App.Pool, EmailService: emailService, Logger: h.App.ObsLogger}
	if err := accountService.RequestEmailChange(ctx, userID, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.ObsLogger.Error(ctx, "Failed to request email change", "error", err, "user_id", userID)
		}
		return nil, asHumaError(err, "Failed to request email change")
	}

	h.App.ObsLogger.Info(ctx, "Email change requested", "user_id", userID, "new_email", in.Body.NewEmail)
	return &messageOutput{Body: messageBody{Message: "Verification email sent to new address"}}, nil
}

type completeEmailChangeInput struct {
	Body VerifyEmailRequest
}

// HumaCompleteEmailChange finishes an email change using the emailed token.
func (h *Handler) HumaCompleteEmailChange(ctx context.Context, in *completeEmailChangeInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_complete_email_change")()

	if err := h.accountService(ctx).CompleteEmailChange(ctx, &in.Body); err != nil {
		if _, ok := err.(*PasswordValidationError); !ok {
			h.App.ObsLogger.Error(ctx, "Failed to complete email change", "error", err)
		}
		return nil, asHumaError(err, "Failed to complete email change")
	}

	h.App.ObsLogger.Info(ctx, "Email change completed successfully")
	return &messageOutput{Body: messageBody{Message: "Email changed successfully"}}, nil
}

// HumaDeleteAccount soft deletes the authenticated user's account.
func (h *Handler) HumaDeleteAccount(ctx context.Context, _ *struct{}) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_account")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	accountService := &AccountService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	if err := accountService.SoftDeleteAccount(ctx, userID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete account", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to delete account")
	}

	h.App.ObsLogger.Info(ctx, "Account deleted successfully", "user_id", userID)
	return &messageOutput{Body: messageBody{
		Message: "Account deleted successfully. You have 30 days to restore your account.",
	}}, nil
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

type listSessionsOutput struct {
	Body SessionsListResponse
}

// HumaListSessions lists the authenticated user's active sessions.
func (h *Handler) HumaListSessions(ctx context.Context, _ *struct{}) (*listSessionsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_sessions")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}

	// The current session is identified by matching the bearer token, so the
	// UI can mark "this device" and refuse to revoke it.
	var currentTokenString string
	if req, _, ok := humaconfig.RequestFrom(ctx); ok {
		currentTokenString = jwtauth.TokenFromHeader(req)
	}

	sessions, err := h.SessionService.GetUserSessions(ctx, authUser.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get user sessions", "error", err, "user_id", authUser.ID)
		return nil, huma.Error500InternalServerError("Failed to get user sessions")
	}

	sessionResponses := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		sessionResponses = append(sessionResponses, SessionResponse{
			ID:        session.ID,
			CreatedAt: "", // the sessions table has no created_at column
			Expires:   session.Expires.Time.Format("2006-01-02T15:04:05Z07:00"),
			IsCurrent: session.Data == currentTokenString,
		})
	}
	return &listSessionsOutput{Body: SessionsListResponse{Sessions: sessionResponses}}, nil
}

type revokeSessionInput struct {
	SessionID int32 `path:"sessionID" doc:"Session to revoke"`
}

// HumaRevokeSession revokes one of the authenticated user's sessions.
func (h *Handler) HumaRevokeSession(ctx context.Context, in *revokeSessionInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_revoke_session")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}

	sessions, err := h.SessionService.GetUserSessions(ctx, authUser.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get user sessions", "error", err, "user_id", authUser.ID)
		return nil, huma.Error500InternalServerError("Failed to get user sessions")
	}

	// Ownership is verified by listing the caller's own sessions, so a session
	// id alone never grants the ability to revoke someone else's.
	found := false
	for _, session := range sessions {
		if session.ID == in.SessionID {
			found = true
			break
		}
	}
	if !found {
		h.App.ObsLogger.Warn(ctx, "V1 revoke session not found")
		return nil, huma.Error404NotFound("session not found or does not belong to user")
	}

	if err := h.SessionService.DeleteSession(ctx, in.SessionID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete session", "error", err, "session_id", in.SessionID)
		return nil, huma.Error500InternalServerError("Failed to delete session")
	}

	h.App.ObsLogger.Info(ctx, "Session revoked successfully", "user_id", authUser.ID, "session_id", in.SessionID)
	return &messageOutput{Body: messageBody{Message: "Session revoked successfully"}}, nil
}

// HumaRevokeAllSessions revokes every session except the caller's current one.
func (h *Handler) HumaRevokeAllSessions(ctx context.Context, _ *struct{}) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_revoke_all_sessions")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := int(authUser.ID)

	token, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get token from context", "error", err)
		return nil, huma.Error401Unauthorized("invalid token")
	}

	sessionIDVal, ok := token.Get("session_id")
	if !ok {
		h.App.ObsLogger.Warn(ctx, "session_id not found in token")
		return nil, huma.Error401Unauthorized("session_id not found in token")
	}

	// The claim arrives as float64 through JSON, but tolerate the integer
	// forms too rather than panicking on a type assertion.
	var currentSessionID int32
	switch v := sessionIDVal.(type) {
	case float64:
		currentSessionID = int32(v)
	case int32:
		currentSessionID = v
	case int64:
		currentSessionID = int32(v)
	default:
		h.App.ObsLogger.Warn(ctx, "session_id claim has unexpected type")
		return nil, huma.Error401Unauthorized("session_id not found in token")
	}

	accountService := &AccountService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	if err := accountService.RevokeAllSessions(ctx, userID, currentSessionID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to revoke all sessions", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to revoke all sessions")
	}

	h.App.ObsLogger.Info(ctx, "All sessions revoked except current", "user_id", userID)
	return &messageOutput{Body: messageBody{Message: "All other sessions revoked successfully"}}, nil
}

// ---------------------------------------------------------------------------
// Discord linking
// ---------------------------------------------------------------------------

type discordConnectOutput struct {
	Body DiscordConnectResponse
}

// humaUserIDFromJWT is the context-based twin of getUserIDFromJWT, which needs
// an *http.Request the huma handlers do not have.
func humaUserIDFromJWT(ctx context.Context) (int32, error) {
	token, _, err := jwtauth.FromContext(ctx)
	if err != nil || token == nil {
		return 0, fmt.Errorf("missing or invalid token")
	}
	userIDStr, ok := token.Get("sub")
	if !ok {
		return 0, fmt.Errorf("sub claim missing")
	}
	var uid int
	if _, err := fmt.Sscanf(userIDStr.(string), "%d", &uid); err != nil || uid == 0 {
		return 0, fmt.Errorf("invalid sub claim")
	}
	return int32(uid), nil
}

// HumaDiscordConnect returns the Discord OAuth2 authorization URL.
func (h *Handler) HumaDiscordConnect(ctx context.Context, _ *struct{}) (*discordConnectOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_discord_connect")()

	userID, err := humaUserIDFromJWT(ctx)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Unauthorized")
		return nil, huma.Error401Unauthorized("invalid token")
	}

	params := url.Values{}
	params.Set("client_id", h.App.Config.Discord.OAuthClientID)
	params.Set("redirect_uri", h.App.Config.Discord.OAuthRedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "identify")
	params.Set("state", h.buildDiscordState(userID))

	h.App.ObsLogger.Info(ctx, "Discord connect URL generated", "user_id", userID)
	return &discordConnectOutput{Body: DiscordConnectResponse{
		URL: "https://discord.com/api/oauth2/authorize?" + params.Encode(),
	}}, nil
}

type discordStatusOutput struct {
	Body DiscordStatusResponse
}

// HumaDiscordStatus reports whether the caller has a linked Discord account.
func (h *Handler) HumaDiscordStatus(ctx context.Context, _ *struct{}) (*discordStatusOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_discord_status")()

	userID, err := humaUserIDFromJWT(ctx)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Unauthorized")
		return nil, huma.Error401Unauthorized("invalid token")
	}

	acct, err := h.DiscordService.GetDiscordAccount(ctx, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get Discord account", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to get Discord account")
	}

	resp := DiscordStatusResponse{Linked: false}
	if acct != nil {
		resp.Linked = true
		resp.DiscordUsername = &acct.DiscordUsername
	}
	return &discordStatusOutput{Body: resp}, nil
}

// HumaDiscordDisconnect unlinks the caller's Discord account.
func (h *Handler) HumaDiscordDisconnect(ctx context.Context, _ *struct{}) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_discord_disconnect")()

	userID, err := humaUserIDFromJWT(ctx)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Unauthorized")
		return nil, huma.Error401Unauthorized("invalid token")
	}

	if err := h.DiscordService.DeleteDiscordAccount(ctx, userID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete Discord account", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to delete Discord account")
	}

	h.App.ObsLogger.Info(ctx, "Discord account unlinked", "user_id", userID)
	return &messageOutput{Body: messageBody{Message: "Discord account disconnected"}}, nil
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
//
// Four functions, one per middleware group, all mounted at /api/v1/auth. The
// split exists because huma binds an API to a chi router: putting a rate
// limited and an unlimited route on the same API would apply the limiter to
// both, or neither. Paths are relative to the /auth mount (gotcha 4).

const tagAuth = "Authentication"

// noAuth documents an endpoint as requiring no credentials. Without it huma
// inherits the document-level security scheme and the spec claims a bearer
// token is needed to log in.
var noAuth = []map[string][]string{}

var bearerAuth = []map[string][]string{{"BearerAuth": {}}}

// RegisterHumaAuthPublic registers the unauthenticated, unlimited routes.
func RegisterHumaAuthPublic(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/logout",
		Summary:     "Log out",
		Description: "Clears the jwt cookie. Always succeeds with an empty body, " +
			"including for a caller who was never logged in.",
		Tags:     []string{tagAuth},
		Security: noAuth,
	}, h.HumaLogout)

	huma.Register(api, huma.Operation{
		OperationID: "resetPassword",
		Method:      http.MethodPost,
		Path:        "/reset-password",
		Summary:     "Reset a password using an emailed token",
		Tags:        []string{tagAuth},
		Security:    noAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Token invalid or expired, or the passwords do not match"},
		},
	}, h.HumaResetPassword)

	huma.Register(api, huma.Operation{
		OperationID: "validateResetToken",
		Method:      http.MethodGet,
		Path:        "/validate-reset-token",
		Summary:     "Check a password reset token without consuming it",
		Description: "Lets the reset page reject an expired link before the user " +
			"types a new password.",
		Tags:     []string{tagAuth},
		Security: noAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Token invalid or expired"},
		},
	}, h.HumaValidateResetToken)

	huma.Register(api, huma.Operation{
		OperationID: "verifyEmail",
		Method:      http.MethodPost,
		Path:        "/verify-email",
		Summary:     "Verify an email address using an emailed token",
		Tags:        []string{tagAuth},
		Security:    noAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Token invalid or expired"},
		},
	}, h.HumaVerifyEmail)

	huma.Register(api, huma.Operation{
		OperationID: "completeEmailChange",
		Method:      http.MethodPost,
		Path:        "/complete-email-change",
		Summary:     "Complete an email change using an emailed token",
		Tags:        []string{tagAuth},
		Security:    noAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Token invalid or expired"},
		},
	}, h.HumaCompleteEmailChange)
}

// RegisterHumaAuthRateLimited registers the public routes that carry strict
// rate limiting: account creation, login, and password reset requests.
func RegisterHumaAuthRateLimited(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Method:        http.MethodPost,
		Path:          "/register",
		Summary:       "Create an account",
		DefaultStatus: http.StatusCreated,
		Description: "Creates an account and returns a token. Responds 202 with a " +
			"pending-approval notice instead when the instance requires admin " +
			"approval of new accounts. Rate limited.",
		Tags:     []string{tagAuth},
		Security: noAuth,
		Responses: map[string]*huma.Response{
			"202": {Description: "Account created and awaiting admin approval"},
			"400": {Description: "Validation failed, username taken, or blocked by bot prevention"},
			"403": {Description: "IP address or device fingerprint is banned"},
			"429": {Description: "Too many attempts"},
		},
	}, h.HumaRegister)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Log in",
		Description: "Authenticates with a username or email plus password, sets the " +
			"jwt cookie, and returns the token. Rate limited.",
		Tags:     []string{tagAuth},
		Security: noAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Incorrect password"},
			"401": {Description: "No such account, or no identifier supplied"},
			"403": {Description: "Account banned, pending approval, or a banned IP/device"},
			"429": {Description: "Too many attempts"},
		},
	}, h.HumaLogin)

	huma.Register(api, huma.Operation{
		OperationID: "requestPasswordReset",
		Method:      http.MethodPost,
		Path:        "/request-password-reset",
		Summary:     "Request a password reset email",
		Description: "Always answers 200 with the same message whether or not the " +
			"address exists, so the endpoint cannot be used to enumerate accounts. " +
			"Rate limited.",
		Tags:     []string{tagAuth},
		Security: noAuth,
		Responses: map[string]*huma.Response{
			"429": {Description: "Too many attempts"},
		},
	}, h.HumaRequestPasswordReset)
}

// RegisterHumaAuthProbe registers /me.
//
// Its router has jwtauth.Verifier but NOT Authenticator, which is what lets the
// endpoint answer 200 for an anonymous caller instead of 401.
func RegisterHumaAuthProbe(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "getCurrentUser",
		Method:      http.MethodGet,
		Path:        "/me",
		Summary:     "Get the current user, or null when not signed in",
		Description: "Auth-state probe. Never returns 401: an absent, malformed or " +
			"expired token yields 200 with {\"user\": null}, so the frontend can " +
			"poll it without provoking console errors. A token whose session has " +
			"been revoked also reads as signed out.",
		Tags:     []string{tagAuth},
		Security: noAuth,
	}, h.HumaMe)
}

// RegisterHumaAuthProtected registers the routes behind full authentication.
func RegisterHumaAuthProtected(api huma.API, h *Handler) {
	unauthorized := map[string]*huma.Response{"401": {Description: "Not authenticated"}}

	huma.Register(api, huma.Operation{
		OperationID: "refreshToken",
		Method:      http.MethodGet,
		Path:        "/refresh",
		Summary:     "Exchange a valid token for a fresh one",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaRefresh)

	huma.Register(api, huma.Operation{
		OperationID: "getUserPreferences",
		Method:      http.MethodGet,
		Path:        "/preferences",
		Summary:     "Get the current user's preferences",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaGetPreferences)

	huma.Register(api, huma.Operation{
		OperationID: "updateUserPreferences",
		Method:      http.MethodPut,
		Path:        "/preferences",
		Summary:     "Update the current user's preferences",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing preferences object"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaUpdatePreferences)

	huma.Register(api, huma.Operation{
		OperationID: "searchUsers",
		Method:      http.MethodGet,
		Path:        "/users/search",
		Summary:     "Search users by username",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing or empty q parameter"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaSearchUsers)

	huma.Register(api, huma.Operation{
		OperationID: "getDiscordConnectURL",
		Method:      http.MethodGet,
		Path:        "/discord/connect",
		Summary:     "Get the Discord OAuth2 authorization URL",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaDiscordConnect)

	huma.Register(api, huma.Operation{
		OperationID: "getDiscordStatus",
		Method:      http.MethodGet,
		Path:        "/discord/status",
		Summary:     "Check whether a Discord account is linked",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaDiscordStatus)

	huma.Register(api, huma.Operation{
		OperationID: "disconnectDiscord",
		Method:      http.MethodDelete,
		Path:        "/discord/disconnect",
		Summary:     "Unlink the Discord account",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaDiscordDisconnect)

	huma.Register(api, huma.Operation{
		OperationID: "changePassword",
		Method:      http.MethodPost,
		Path:        "/change-password",
		Summary:     "Change the current user's password",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Current password wrong, or the new password is invalid"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaChangePassword)

	huma.Register(api, huma.Operation{
		OperationID: "changeUsername",
		Method:      http.MethodPost,
		Path:        "/change-username",
		Summary:     "Change the current user's username",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Password wrong, or the username is invalid or taken"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaChangeUsername)

	huma.Register(api, huma.Operation{
		OperationID: "requestEmailChange",
		Method:      http.MethodPost,
		Path:        "/request-email-change",
		Summary:     "Request a change of email address",
		Description: "Sends a confirmation link to the new address; the change only " +
			"takes effect once that link is followed.",
		Tags:     []string{tagAuth},
		Security: bearerAuth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Password wrong, or the address is invalid or taken"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaRequestEmailChange)

	huma.Register(api, huma.Operation{
		OperationID: "deleteAccount",
		Method:      http.MethodDelete,
		Path:        "/account",
		Summary:     "Soft delete the current user's account",
		Description: "The account is recoverable for 30 days.",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaDeleteAccount)

	huma.Register(api, huma.Operation{
		OperationID: "listSessions",
		Method:      http.MethodGet,
		Path:        "/sessions",
		Summary:     "List the current user's active sessions",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaListSessions)

	huma.Register(api, huma.Operation{
		OperationID: "revokeSession",
		Method:      http.MethodDelete,
		Path:        "/sessions/{sessionID}",
		Summary:     "Revoke one session",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "No such session, or it belongs to another user"},
		},
	}, h.HumaRevokeSession)

	huma.Register(api, huma.Operation{
		OperationID: "revokeAllSessions",
		Method:      http.MethodPost,
		Path:        "/revoke-all-sessions",
		Summary:     "Revoke every session except the current one",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses:   unauthorized,
	}, h.HumaRevokeAllSessions)
}

// RegisterHumaAuthRateLimitedProtected registers resend-verification, the one
// route that is both authenticated and rate limited.
func RegisterHumaAuthRateLimitedProtected(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "resendVerificationEmail",
		Method:      http.MethodPost,
		Path:        "/resend-verification",
		Summary:     "Resend the email verification message",
		Tags:        []string{tagAuth},
		Security:    bearerAuth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"429": {Description: "Too many attempts"},
		},
	}, h.HumaResendVerificationEmail)
}
