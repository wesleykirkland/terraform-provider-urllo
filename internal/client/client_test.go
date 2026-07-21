// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testKey    = "test-key"
	testSecret = "test-secret"
)

// newTestClient builds a Client pointed at srv with fast retries.
func newTestClient(srv *httptest.Server) *Client {
	return New(srv.URL, testKey, testSecret,
		WithHTTPClient(srv.Client()),
		WithRetryWait(time.Millisecond, 5*time.Millisecond),
	)
}

func TestBasicAuthHeader(t *testing.T) {
	var gotUser, gotPass string
	var ok bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":"r1","type":"rule","attributes":{}}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if _, err := c.GetRule(context.Background(), "r1"); err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if !ok || gotUser != testKey || gotPass != testSecret {
		t.Fatalf("basic auth = (%q, %q, ok=%v); want (%q, %q, true)", gotUser, gotPass, ok, testKey, testSecret)
	}
}

func TestGetRuleDecodesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rules/abc" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":{"id":"abc","type":"rule","attributes":{
			"target_url":"dest.com","response_type":"found","forward_params":true,
			"source_urls":["a.com","b.com"],"tags":["t1"]}}}`)
	}))
	defer srv.Close()

	rule, err := newTestClient(srv).GetRule(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if rule.ID != "abc" || rule.Attributes.TargetURL != "dest.com" || rule.Attributes.ResponseType != ResponseFound {
		t.Fatalf("unexpected rule: %+v", rule)
	}
	if len(rule.Attributes.SourceURLs) != 2 || !rule.Attributes.ForwardParams {
		t.Fatalf("unexpected attributes: %+v", rule.Attributes)
	}
}

func TestListRulesFollowsPagination(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Query().Get("starting_after") == "" {
			// First page: signal there's more.
			fmt.Fprint(w, `{"data":[{"id":"r1","type":"rule","attributes":{}}],
				"meta":{"has_more":true},"links":{"next":"/v1/rules?starting_after=r1"}}`)
			return
		}
		if r.URL.Query().Get("starting_after") != "r1" {
			t.Errorf("expected cursor r1, got %q", r.URL.Query().Get("starting_after"))
		}
		fmt.Fprint(w, `{"data":[{"id":"r2","type":"rule","attributes":{}}],
			"meta":{"has_more":false},"links":{"next":null}}`)
	}))
	defer srv.Close()

	rules, err := newTestClient(srv).ListRules(context.Background(), ListRulesOptions{})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 2 || rules[0].ID != "r1" || rules[1].ID != "r2" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
	if calls != 2 {
		t.Fatalf("expected 2 API calls, got %d", calls)
	}
}

func TestListRulesSendsFilters(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		fmt.Fprint(w, `{"data":[],"meta":{"has_more":false},"links":{"next":null}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).ListRules(context.Background(), ListRulesOptions{
		SourceQuery: "src", TargetQuery: "dst", Tags: []string{"a", "b"}, TagMatchStrategy: "all",
	})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	for _, want := range []string{"sq=src", "tq=dst", "tags%5B%5D=a", "tags%5B%5D=b", "tag_match_strategy=all"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestUnauthorizedErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"unauthorized","message":"bad credentials"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetRule(context.Background(), "x")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "bad credentials" {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
}

func TestUnprocessableEntityErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"type":"invalid_request_error","message":"Invalid Request",
			"errors":[{"resource":"rule","param":"source_urls","code":"required","message":"need a url"}]}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreateRule(context.Background(), RuleAttributes{})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0].Param != "source_urls" {
		t.Fatalf("unexpected validation errors: %+v", apiErr.Errors)
	}
}

func TestNotFoundHelper(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"not found"}`)
	}))
	defer srv.Close()

	err := newTestClient(srv).DeleteRule(context.Background(), "gone")
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false for err %v", err)
	}
}

func TestIdempotencyKeyOnWrites(t *testing.T) {
	var postKey, patchKey, getKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postKey = r.Header.Get("Idempotency-Key")
		case http.MethodPatch:
			patchKey = r.Header.Get("Idempotency-Key")
		case http.MethodGet:
			getKey = r.Header.Get("Idempotency-Key")
		}
		fmt.Fprint(w, `{"data":{"id":"r1","type":"rule","attributes":{}}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	ctx := context.Background()
	if _, err := c.CreateRule(ctx, RuleAttributes{TargetURL: "d.com"}); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if _, err := c.UpdateRule(ctx, "r1", RuleAttributes{TargetURL: "d.com"}); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if _, err := c.GetRule(ctx, "r1"); err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if postKey == "" || patchKey == "" {
		t.Fatalf("expected idempotency keys on writes, got post=%q patch=%q", postKey, patchKey)
	}
	if postKey == patchKey {
		t.Fatalf("expected distinct idempotency keys per write")
	}
	if getKey != "" {
		t.Fatalf("did not expect idempotency key on GET, got %q", getKey)
	}
}

func TestRetriesOnRateLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"slow down"}`)
			return
		}
		fmt.Fprint(w, `{"data":{"id":"r1","type":"rule","attributes":{}}}`)
	}))
	defer srv.Close()

	rule, err := newTestClient(srv).GetRule(context.Background(), "r1")
	if err != nil {
		t.Fatalf("GetRule after retry: %v", err)
	}
	if rule.ID != "r1" {
		t.Fatalf("unexpected rule: %+v", rule)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (429 then 200), got %d", calls)
	}
}
