// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorError(t *testing.T) {
	e := &APIError{
		StatusCode: 422,
		Type:       "invalid_request_error",
		Message:    "Invalid Request",
		Errors: []ValidationError{
			{Resource: "rule", Param: "source_urls", Code: "required", Message: "need a url"},
		},
	}
	got := e.Error()
	for _, want := range []string{"status 422", "invalid_request_error", "Invalid Request", "source_urls", "need a url"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q; missing %q", got, want)
		}
	}
}

func TestDecodeAPIErrorFallbacks(t *testing.T) {
	// Non-JSON body: message falls back to the raw text.
	e := decodeAPIError(500, []byte("boom"))
	if e.StatusCode != 500 || e.Message != "boom" {
		t.Fatalf("unexpected: %+v", e)
	}
	// Empty body: message falls back to the status text.
	e = decodeAPIError(http.StatusServiceUnavailable, nil)
	if e.Message != http.StatusText(http.StatusServiceUnavailable) {
		t.Fatalf("expected status text, got %q", e.Message)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(&APIError{StatusCode: http.StatusNotFound}) {
		t.Error("expected true for 404 APIError")
	}
	if IsNotFound(&APIError{StatusCode: http.StatusInternalServerError}) {
		t.Error("expected false for 500 APIError")
	}
	if IsNotFound(context.Canceled) {
		t.Error("expected false for non-APIError")
	}
}

func TestWaitFromHeaders(t *testing.T) {
	// Retry-After in seconds.
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"2"}}}
	if d, ok := waitFromHeaders(resp); !ok || d != 2*time.Second {
		t.Fatalf("Retry-After: got %v ok=%v", d, ok)
	}
	// x-ratelimit-reset as a future epoch second.
	future := time.Now().Add(3 * time.Second).Unix()
	resp = &http.Response{Header: http.Header{"X-Ratelimit-Reset": []string{strconv.FormatInt(future, 10)}}}
	if d, ok := waitFromHeaders(resp); !ok || d <= 0 {
		t.Fatalf("x-ratelimit-reset: got %v ok=%v", d, ok)
	}
	// No relevant headers.
	if _, ok := waitFromHeaders(&http.Response{Header: http.Header{}}); ok {
		t.Fatal("expected ok=false with no headers")
	}
}

func TestCheckRetry(t *testing.T) {
	ctx := context.Background()
	if retry, _ := checkRetry(ctx, &http.Response{StatusCode: http.StatusTooManyRequests}, nil); !retry {
		t.Error("expected retry on 429")
	}
	if retry, _ := checkRetry(ctx, &http.Response{StatusCode: http.StatusServiceUnavailable}, nil); !retry {
		t.Error("expected retry on 503")
	}
	if retry, _ := checkRetry(ctx, &http.Response{StatusCode: 200}, nil); retry {
		t.Error("did not expect retry on 200")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if retry, err := checkRetry(cancelled, &http.Response{StatusCode: http.StatusTooManyRequests}, nil); retry || err == nil {
		t.Error("expected no retry and an error on cancelled context")
	}
}

func TestNextPageQuery(t *testing.T) {
	if _, ok := nextPageQuery(nil); ok {
		t.Error("nil should yield ok=false")
	}
	empty := ""
	if _, ok := nextPageQuery(&empty); ok {
		t.Error("empty should yield ok=false")
	}
	link := "/v1/rules?starting_after=abc"
	q, ok := nextPageQuery(&link)
	if !ok || q.Get("starting_after") != "abc" {
		t.Fatalf("expected cursor abc, got %v ok=%v", q, ok)
	}
}

func TestDefaultBaseURLWhenEmpty(t *testing.T) {
	c := New("", "k", "s")
	if c.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %q; want %q", c.baseURL, DefaultBaseURL)
	}
}
