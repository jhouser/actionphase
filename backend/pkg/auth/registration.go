package auth

import (
	"actionphase/pkg/core"
)

func NewRegistrationResponse(user *core.User, token string) *Response {
	resp := &Response{User: user, Token: token}
	return resp
}
