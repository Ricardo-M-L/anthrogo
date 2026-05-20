package mcp

import "net/http"

// bearerInjector is an http.RoundTripper that injects an Authorization: Bearer
// header on every outgoing request. It wraps a base RoundTripper (typically
// http.DefaultTransport).
type bearerInjector struct {
	base  http.RoundTripper
	token string
}

func (b bearerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	if b.token != "" {
		// Clone the request to avoid mutating the caller's copy.
		r2 := req.Clone(req.Context())
		r2.Header.Set("Authorization", "Bearer "+b.token)
		return b.base.RoundTrip(r2)
	}
	return b.base.RoundTrip(req)
}
