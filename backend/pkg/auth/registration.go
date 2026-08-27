package auth

import (
	"actionphase/pkg/core"
	"actionphase/pkg/email"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
)

func (h *Handler) V1Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_register")()

	data := &Request{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid v1 register request", "error", err)
		return
	}

	if err := data.User.Validate(); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid v1 register request", "error", err)
		return
	}

	// Validate username format
	if err := validateUsername(data.User.Username); err != nil {
		if pwdErr, ok := err.(*PasswordValidationError); ok {
			render.Render(w, r, &core.ErrResponse{
				Err:            err,
				HTTPStatusCode: http.StatusBadRequest,
				StatusText:     "Validation Error",
				ErrorText:      pwdErr.Error(),
			})
			return
		}
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid v1 register request", "error", err)
		return
	}

	// Extract IP address and user agent
	ipAddress := core.GetClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Check IP and device fingerprint bans before any expensive checks
	if h.ipBanCheck(w, r) {
		return
	}
	if h.fingerprintBanCheck(w, r, data.Fingerprint) {
		return
	}

	// Perform bot prevention checks
	botService := NewBotPreventionService(h.App.Pool)
	checkRequest := &RegistrationCheckRequest{
		Email:         data.User.Email,
		Username:      data.User.Username,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		HCaptchaToken: data.HCaptchaToken,
		HoneypotValue: data.HoneypotValue,
	}

	result, err := botService.CheckRegistrationAttempt(ctx, checkRequest)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Bot prevention check failed", "error", err, "email", data.User.Email)
		return
	}

	if !result.Allowed {
		h.App.ObsLogger.Warn(ctx, "Registration blocked by bot prevention",
			"reason", result.BlockedReason,
			"email", data.User.Email,
			"ip", ipAddress)

		// Return appropriate error message based on block reason
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

		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("%s", errorMsg)), "Invalid v1 register request")
		return
	}

	h.App.ObsLogger.Info(ctx, "Creating user", "username", data.User.Username)
	returnUser, err := h.UserService.CreateUser(data.User)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid v1 register request", "error", err)
		return
	}

	// If registration approval mode is enabled, place user in pending state
	if h.App.Config.App.RequireRegistrationApproval {
		if err := h.UserService.SetPendingApproval(ctx, int32(returnUser.ID)); err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to set pending approval", "error", err, "user_id", returnUser.ID)
		} else {
			h.App.ObsLogger.Info(ctx, "New account pending admin approval", "user_id", returnUser.ID, "username", returnUser.Username)
		}
		render.Status(r, http.StatusAccepted)
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusAccepted,
			StatusText:     "Pending Approval",
			ErrorText:      "Your account has been created and is pending admin approval.",
		})
		return
	}

	// Log successful registration
	if err := botService.LogSuccessfulRegistration(ctx, checkRequest); err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to log successful registration", "error", err, "username", returnUser.Username)
		// Don't fail the registration if logging fails
	}

	h.App.ObsLogger.Info(ctx, "Creating token for new user", "username", returnUser.Username)
	jwtHandler := JWTHandler{App: h.App}
	token, err := jwtHandler.CreateToken(returnUser, SessionMetadata{
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Fingerprint: fingerprintPtr(data.Fingerprint),
	})
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to v1 register", "error", err)
		return
	}

	// Send verification email
	// Create account service
	emailService, err := email.NewEmailServiceFromEnv()
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create email service", "error", err)
	} else {
		accountService := &AccountService{
			DB:           h.App.Pool,
			EmailService: emailService,
			Logger:       h.App.ObsLogger,
		}
		_ = accountService.SendVerificationEmail(ctx, &SendVerificationEmailRequest{
			UserID: returnUser.ID,
			Email:  returnUser.Email,
		})
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, NewRegistrationResponse(returnUser, token))
}

func NewRegistrationResponse(user *core.User, token string) *Response {
	resp := &Response{User: user, Token: token}
	return resp
}
