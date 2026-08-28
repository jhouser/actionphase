package auth

import (
	"actionphase/pkg/core"
	"net/http"
)

// ipBanCheck returns true and writes a 403 response if the client IP is banned.
func (h *Handler) ipBanCheck(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	clientIP := core.GetClientIP(r)
	banned, _ := h.IPBanService.IsIPBanned(ctx, clientIP)
	if banned {
		h.renderError(ctx, w, r, core.ErrForbidden("Access from this location is not allowed."), "Blocked request from banned IP", "ip", clientIP)
		return true
	}
	return false
}

// fingerprintBanCheck returns true and writes a 403 response if the fingerprint is banned.
func (h *Handler) fingerprintBanCheck(w http.ResponseWriter, r *http.Request, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	ctx := r.Context()
	banned, _ := h.FingerprintBanService.IsFingerprintBanned(ctx, fingerprint)
	if banned {
		h.renderError(ctx, w, r, core.ErrForbidden("Access from this device is not allowed."), "Blocked request from banned device fingerprint")
		return true
	}
	return false
}

func fingerprintPtr(fp string) *string {
	if fp == "" {
		return nil
	}
	return &fp
}

type LoginError struct {
	Message string `json:"message"`
}

func (e LoginError) Error() string {
	return e.Message
}

func NewLoginResponse(token string) *Response {
	resp := &Response{Token: token}
	return resp
}
