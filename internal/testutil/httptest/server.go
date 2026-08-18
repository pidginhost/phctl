// Package httptest provides an in-memory substitute for httptest.Server.
package httptest

import (
	"fmt"
	"net/http"
	stdhttptest "net/http/httptest"
	"sync"
	"sync/atomic"
)

var nextServerID atomic.Uint64

// Server dispatches HTTP requests directly to a handler without binding a
// local socket. It exposes the subset of net/http/httptest.Server used by the
// repository's tests.
type Server struct {
	URL               string
	handlerTransport  *handlerRoundTripper
	routingTransport  *routingRoundTripper
	previousTransport http.RoundTripper
	previousDefault   http.RoundTripper
	closeOnce         sync.Once
}

// NewServer returns an in-memory server and temporarily routes matching
// requests made through http.DefaultClient to its handler.
func NewServer(handler http.Handler) *Server {
	host := fmt.Sprintf("phctl-test-%d.invalid", nextServerID.Add(1))
	handlerTransport := &handlerRoundTripper{handler: handler}
	previousTransport := http.DefaultClient.Transport
	previousDefault := http.DefaultTransport
	fallback := previousTransport
	if fallback == nil {
		fallback = previousDefault
	}
	routingTransport := &routingRoundTripper{
		host:     host,
		handler:  handlerTransport,
		fallback: fallback,
	}
	server := &Server{
		URL:               "http://" + host,
		handlerTransport:  handlerTransport,
		routingTransport:  routingTransport,
		previousTransport: previousTransport,
		previousDefault:   previousDefault,
	}
	http.DefaultClient.Transport = routingTransport
	http.DefaultTransport = routingTransport
	return server
}

// Client returns a client that sends every request to the server's handler.
func (s *Server) Client() *http.Client {
	return &http.Client{Transport: s.handlerTransport}
}

// Close restores the default client transport that NewServer replaced.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		if http.DefaultClient.Transport == s.routingTransport {
			http.DefaultClient.Transport = s.previousTransport
		}
		if http.DefaultTransport == s.routingTransport {
			http.DefaultTransport = s.previousDefault
		}
	})
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (rt *handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	default:
	}
	if req.Body == nil {
		req.Body = http.NoBody
	}
	recorder := stdhttptest.NewRecorder()
	rt.handler.ServeHTTP(recorder, req)
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	default:
	}
	resp := recorder.Result()
	resp.Request = req
	return resp, nil
}

type routingRoundTripper struct {
	host     string
	handler  http.RoundTripper
	fallback http.RoundTripper
}

func (rt *routingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == rt.host {
		return rt.handler.RoundTrip(req)
	}
	if rt.fallback != nil {
		return rt.fallback.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}
