package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.opentelemetry.io/otel"
)

// roundTripperFunc lets us stub transports inline.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// dummyResp returns a minimal OK response.
func dummyResp() *http.Response {
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	return rec.Result()
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u
}

func TestPathPrefixRT_RewriteAndNoRequestURI(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane"
	calls := 0

	rt := &pathPrefixRT{
		prefix: prefix,
		rt: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			// Ensure RequestURI is empty for client requests
			if r.RequestURI != "" {
				return nil, errors.New("RequestURI should be empty on client requests")
			}
			// Expect the rewritten path and preserved query
			wantPath := prefix + "/apis/dns.networking.miloapis.com/v1alpha1/namespaces/default/dnszones/testctl"
			if r.URL.Path != wantPath {
				return nil, errors.New("unexpected path: " + r.URL.Path)
			}
			if r.URL.RawQuery != "watch=1" {
				return nil, errors.New("query not preserved: " + r.URL.RawQuery)
			}
			return dummyResp(), nil
		}),
	}

	// Original request (unprefixed path)
	orig := &http.Request{
		Method: "PATCH",
		URL:    mustURL(t, "https://milo-apiserver.svc/apis/dns.networking.miloapis.com/v1alpha1/namespaces/default/dnszones/testctl?watch=1"),
		Header: make(http.Header),
	}

	// No recording parent span; should not panic.
	ctx := context.Background()
	// (optional) add a no-op tracer provider explicitly
	_ = otel.GetTracerProvider()

	orig = orig.WithContext(ctx)
	resp, err := rt.RoundTrip(orig)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	defer resp.Body.Close()

	if calls != 1 {
		t.Fatalf("transport not called exactly once, got %d", calls)
	}
	// Original request should be unchanged (clone correctness)
	if orig.URL.Path != "/apis/dns.networking.miloapis.com/v1alpha1/namespaces/default/dnszones/testctl" {
		t.Fatalf("original request mutated: %s", orig.URL.Path)
	}
}

func TestPathPrefixRT_AlreadyPrefixed_NoRewrite(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane"
	calls := 0

	rt := &pathPrefixRT{
		prefix: prefix,
		rt: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			// Should pass through unchanged
			want := prefix + "/apis/dns.networking.miloapis.com/v1alpha1/namespaces/ns1/dnszones/x"
			if r.URL.Path != want {
				return nil, errors.New("path unexpectedly changed: " + r.URL.Path)
			}
			if r.RequestURI != "" {
				return nil, errors.New("RequestURI should be empty on client requests")
			}
			return dummyResp(), nil
		}),
	}

	req := &http.Request{
		Method: "GET",
		URL:    mustURL(t, "https://milo-apiserver.svc"+prefix+"/apis/dns.networking.miloapis.com/v1alpha1/namespaces/ns1/dnszones/x"),
		Header: make(http.Header),
	}
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("transport not called exactly once, got %d", calls)
	}
}

func TestPathPrefixRT_DefensiveNilTransport(t *testing.T) {
	rt := &pathPrefixRT{prefix: "/x", rt: nil}
	req := &http.Request{
		Method: "GET",
		URL:    mustURL(t, "https://example/x"),
	}
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatalf("expected error for nil transport, got nil")
	}
}

func TestPathPrefixRT_DefensiveNilURL(t *testing.T) {
	rt := &pathPrefixRT{
		prefix: "/x",
		rt:     roundTripperFunc(func(r *http.Request) (*http.Response, error) { return dummyResp(), nil }),
	}
	// Manually craft a request with nil URL (http.NewRequest won't allow this)
	req := &http.Request{Method: "GET"}
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatalf("expected error for nil URL, got nil")
	}
}

func TestPathPrefixRT_NoPanicWithoutRecordingParentSpan(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane"
	rt := &pathPrefixRT{
		prefix: prefix,
		rt:     roundTripperFunc(func(r *http.Request) (*http.Response, error) { return dummyResp(), nil }),
	}

	// Context without any active/recording span
	ctx := context.Background()
	req := &http.Request{
		Method: "GET",
		URL:    mustURL(t, "https://example/apis/foo"),
	}
	req = req.WithContext(ctx)

	// Should not panic; just work.
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}
func TestPathPrefixRT_PreservesMultipleQueryParamsAndEncoding(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane"

	rt := &pathPrefixRT{
		prefix: prefix,
		rt: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			// Clients must leave RequestURI empty
			if r.RequestURI != "" {
				return nil, errors.New("RequestURI should be empty on client requests")
			}

			// Query string preserved exactly (order + encoding)
			if got, want := r.URL.RawQuery, "watch=1&label=env%2Fprod&x=1&x=2"; got != want {
				return nil, fmt.Errorf("query mismatch: got %q want %q", got, want)
			}

			// Encoded path on the wire (what the client will send)
			wantEscaped := prefix + "/apis/dns.networking.miloapis.com/v1alpha1/namespaces/ns-a/dnszones/name%2Fwith%2Fslash"
			if got := r.URL.EscapedPath(); got != wantEscaped {
				return nil, fmt.Errorf("escaped path mismatch: got %q want %q", got, wantEscaped)
			}

			// Decoded Path in memory (Go always keeps this unescaped)
			wantDecoded := prefix + "/apis/dns.networking.miloapis.com/v1alpha1/namespaces/ns-a/dnszones/name/with/slash"
			if r.URL.Path != wantDecoded {
				return nil, fmt.Errorf("decoded path mismatch: got %q want %q", r.URL.Path, wantDecoded)
			}

			return dummyResp(), nil
		}),
	}

	// Encoded segment and multiple query params (including repeated keys)
	u := mustURL(t, "https://milo-apiserver.svc/apis/dns.networking.miloapis.com/v1alpha1/namespaces/ns-a/dnszones/name%2Fwith%2Fslash?watch=1&label=env%2Fprod&x=1&x=2")
	req := &http.Request{Method: "GET", URL: u}

	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestPathPrefixRT_PrefixWithTrailingSlash_IdempotentJoin(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane/" // trailing slash
	rt := &pathPrefixRT{
		prefix: prefix,
		rt: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			want := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane/apis/foo/v1/namespaces/n1/things/t1"
			if r.URL.Path != want {
				return nil, fmt.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.RequestURI != "" {
				return nil, errors.New("RequestURI should be empty")
			}
			return dummyResp(), nil
		}),
	}
	req := &http.Request{
		Method: "GET",
		URL:    mustURL(t, "https://example/apis/foo/v1/namespaces/n1/things/t1"),
	}
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
}

func TestPathPrefixRT_CloneDoesNotMutateOriginal_HostPreserved(t *testing.T) {
	prefix := "/apis/resourcemanager.miloapis.com/v1alpha1/projects/p-123/control-plane"
	rt := &pathPrefixRT{
		prefix: prefix,
		rt:     roundTripperFunc(func(r *http.Request) (*http.Response, error) { return dummyResp(), nil }),
	}

	orig := &http.Request{
		Method: "GET",
		URL:    mustURL(t, "https://orig-host.example/apis/a/b"),
		Header: make(http.Header),
		Host:   "orig-host.example",
	}
	// RoundTrip shouldn’t mutate `orig`
	if _, err := rt.RoundTrip(orig); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if orig.URL.Host != "orig-host.example" || orig.Host != "orig-host.example" {
		t.Fatalf("original request Host mutated: URL.Host=%q Host=%q", orig.URL.Host, orig.Host)
	}
	if orig.URL.Path != "/apis/a/b" {
		t.Fatalf("original path mutated: %s", orig.URL.Path)
	}
}
