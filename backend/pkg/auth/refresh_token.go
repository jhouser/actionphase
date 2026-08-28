package auth

func NewRefreshResponse(token string) *Response {
	resp := &Response{Token: token}
	return resp
}
