// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- options -----------------------------------------------------------------

func TestOptions(t *testing.T) {
	c := New("http://x", "k", "s",
		WithUserAgent("ua/1"),
		WithMaxRetries(2),
		WithRetryWait(time.Millisecond, time.Millisecond),
	)
	if c.userAgent != "ua/1" {
		t.Errorf("userAgent = %q", c.userAgent)
	}
	if c.httpClient.RetryMax != 2 {
		t.Errorf("RetryMax = %d", c.httpClient.RetryMax)
	}
}

// --- do() error branches -----------------------------------------------------

func TestDoMarshalError(t *testing.T) {
	c := New("http://x", "k", "s")
	// Channels cannot be JSON-encoded.
	err := c.do(context.Background(), http.MethodPost, "/x", make(chan int), nil, &requestOptions{})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestDoBadMethod(t *testing.T) {
	c := New("http://x", "k", "s")
	// A method containing a space is rejected by http.NewRequest.
	err := c.do(context.Background(), "BAD METHOD", "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("expected request-build error")
	}
}

func TestDoTransportError(t *testing.T) {
	// Point at a server that is immediately closed so the request fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	c := New(url, "k", "s", WithMaxRetries(0), WithRetryWait(time.Millisecond, time.Millisecond))
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
}

// errBody is a response body whose Read always fails.
type errBody struct{}

func (errBody) Read([]byte) (int, error) { return 0, errors.New("read boom") }
func (errBody) Close() error             { return nil }

// errTransport returns a response with a failing body.
type errTransport struct{}

func (errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       errBody{},
		Header:     make(http.Header),
	}, nil
}

func TestDoReadBodyError(t *testing.T) {
	c := New("http://x", "k", "s",
		WithHTTPClient(&http.Client{Transport: errTransport{}}),
		WithMaxRetries(0),
	)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("expected read-body error")
	}
}

func TestDoUnmarshalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := newTestClient(srv)
	var out struct{ Data string }
	err := c.do(context.Background(), http.MethodGet, "/x", nil, &out, nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- idempotencyKey ----------------------------------------------------------

func TestIdempotencyKeyMethod(t *testing.T) {
	c := New("http://x", "k", "s")
	if got := c.idempotencyKey(http.MethodGet, &requestOptions{generateIdempotencyKey: true}); got != "" {
		t.Errorf("GET should have no key, got %q", got)
	}
	if got := c.idempotencyKey(http.MethodPost, &requestOptions{idempotencyKey: "fixed"}); got != "fixed" {
		t.Errorf("explicit key = %q", got)
	}
	if got := c.idempotencyKey(http.MethodPost, &requestOptions{}); got != "" {
		t.Errorf("no key requested should be empty, got %q", got)
	}
	if got := c.idempotencyKey(http.MethodPatch, &requestOptions{generateIdempotencyKey: true}); got == "" {
		t.Error("expected a generated key")
	}
}

// --- retry / backoff ---------------------------------------------------------

func TestCheckRetryErrorBranch(t *testing.T) {
	retry, _ := checkRetry(context.Background(), nil, errors.New("dial fail"))
	if !retry {
		t.Error("expected retry on transport error")
	}
	// 501 is explicitly not retried.
	if retry, _ := checkRetry(context.Background(), &http.Response{StatusCode: http.StatusNotImplemented}, nil); retry {
		t.Error("did not expect retry on 501")
	}
}

func TestRateLimitBackoffClamp(t *testing.T) {
	minWait, maxWait := time.Second, 10*time.Second
	// Retry-After far exceeds max -> clamped to max.
	big := &http.Response{Header: http.Header{"Retry-After": []string{"3600"}}}
	if d := rateLimitBackoff(minWait, maxWait, 0, big); d != maxWait {
		t.Errorf("expected clamp to max, got %v", d)
	}
	// Retry-After below min -> clamped up to min.
	small := &http.Response{Header: http.Header{"Retry-After": []string{"0"}}}
	if d := rateLimitBackoff(minWait, maxWait, 0, small); d != minWait {
		t.Errorf("expected clamp to min, got %v", d)
	}
	// No headers -> falls back to default exponential backoff (>0).
	if d := rateLimitBackoff(minWait, maxWait, 1, &http.Response{Header: http.Header{}}); d <= 0 {
		t.Errorf("expected default backoff, got %v", d)
	}
	// Nil response -> default backoff.
	if d := rateLimitBackoff(minWait, maxWait, 1, nil); d <= 0 {
		t.Errorf("expected default backoff for nil resp, got %v", d)
	}
}

// --- pagination edge cases ---------------------------------------------------

func TestNextPageQueryEdges(t *testing.T) {
	if _, ok := nextPageQuery(nil); ok {
		t.Error("nil -> ok=false")
	}
	empty := ""
	if _, ok := nextPageQuery(&empty); ok {
		t.Error("empty -> ok=false")
	}
	bad := "://not a url"
	if _, ok := nextPageQuery(&bad); ok {
		t.Error("unparseable -> ok=false")
	}
	noq := "/rules"
	if _, ok := nextPageQuery(&noq); ok {
		t.Error("no query -> ok=false")
	}
}

// --- error returns on the resource methods -----------------------------------

func newErrServer(t *testing.T) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"boom"}`)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "k", "s", WithHTTPClient(srv.Client()), WithMaxRetries(0), WithRetryWait(time.Millisecond, time.Millisecond))
}

func TestResourceMethodErrorReturns(t *testing.T) {
	c := newErrServer(t)
	ctx := context.Background()
	if _, err := c.ListHosts(ctx, 1); err == nil {
		t.Error("ListHosts should error")
	}
	if _, err := c.GetHost(ctx, "x"); err == nil {
		t.Error("GetHost should error")
	}
	if _, err := c.GetHostByName(ctx, "x"); err == nil {
		t.Error("GetHostByName should error")
	}
	enabled := true
	if _, err := c.UpdateHost(ctx, "x", HostUpdate{ACMEEnabled: &enabled}); err == nil {
		t.Error("UpdateHost should error")
	}
	if _, err := c.ListRules(ctx, ListRulesOptions{Limit: 5}); err == nil {
		t.Error("ListRules should error")
	}
	if _, err := c.UpdateRule(ctx, "x", RuleAttributes{}); err == nil {
		t.Error("UpdateRule should error")
	}
}
