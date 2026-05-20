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

// headerInjector is an http.RoundTripper that injects a fixed set of HTTP
// headers on every outgoing request. It wraps a base RoundTripper (typically
// http.DefaultTransport or another injector). When layered with bearerInjector,
// place headerInjector as the base so that bearerInjector's Authorization header
// takes precedence over any Authorization key in headers.
type headerInjector struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h headerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(h.headers) > 0 {
		req = req.Clone(req.Context())
		for k, v := range h.headers {
			req.Header.Set(k, v)
		}
	}
	return h.base.RoundTrip(req)
}
