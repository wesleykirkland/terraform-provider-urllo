// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

// Package client is a small typed HTTP client for the Urllo redirection API
// (https://api.urllo.com/v1). It handles HTTP Basic authentication, retry with
// backoff that honours the API rate limits, idempotency keys on writes, cursor
// pagination, and typed error decoding.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/go-uuid"
)

// DefaultBaseURL is the production Urllo API base URL. The published OpenAPI
// document lists an easyredir.com server because Urllo is a whitelabel of that
// service, but the live API is served from api.urllo.com.
const DefaultBaseURL = "https://api.urllo.com/v1"

// defaultMaxRetries is how many times a retryable request is retried before
// giving up. Rate-limited (429) and 5xx responses are retried.
const defaultMaxRetries = 4

// Client talks to the Urllo API.
type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	userAgent  string
	httpClient *retryablehttp.Client
}

// Option customises a Client.
type Option func(*Client)

// WithHTTPClient overrides the underlying *http.Client (used by tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient.HTTPClient = hc
	}
}

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithMaxRetries overrides the number of retries for rate-limited/5xx responses.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.httpClient.RetryMax = n }
}

// WithRetryWait overrides the minimum and maximum backoff between retries.
// Primarily useful for tests that need fast retries.
func WithRetryWait(minWait, maxWait time.Duration) Option {
	return func(c *Client) {
		c.httpClient.RetryWaitMin = minWait
		c.httpClient.RetryWaitMax = maxWait
	}
}

// New builds a Client. baseURL may be empty to use DefaultBaseURL. apiKey and
// apiSecret are the HTTP Basic credentials (key = username, secret = password).
func New(baseURL, apiKey, apiSecret string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = defaultMaxRetries
	rc.Logger = nil // Terraform provides its own logging; keep the client quiet.
	rc.CheckRetry = checkRetry
	rc.Backoff = rateLimitBackoff

	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		userAgent:  "terraform-provider-urllo",
		httpClient: rc,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError is a structured error returned by the Urllo API for a non-2xx
// response. It decodes both the unauthorized and unprocessable-entity shapes.
type APIError struct {
	StatusCode int
	Type       string            `json:"type"`
	Message    string            `json:"message"`
	Errors     []ValidationError `json:"errors"`
}

// ValidationError describes a single field-level validation failure.
type ValidationError struct {
	Resource string `json:"resource"`
	Param    string `json:"param"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "urllo api error (status %d)", e.StatusCode)
	if e.Type != "" {
		fmt.Fprintf(&b, " [%s]", e.Type)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	for _, ve := range e.Errors {
		fmt.Fprintf(&b, "\n  - %s.%s (%s): %s", ve.Resource, ve.Param, ve.Code, ve.Message)
	}
	return b.String()
}

// IsNotFound reports whether err is an APIError with a 404 status.
func IsNotFound(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.StatusCode == http.StatusNotFound
}

// requestOptions configure a single request.
type requestOptions struct {
	// idempotencyKey, when set, is sent as the Idempotency-Key header. Write
	// methods populate it automatically when left empty.
	idempotencyKey string
	// generateIdempotencyKey requests an auto-generated key for writes.
	generateIdempotencyKey bool
	query                  url.Values
}

// do performs a request against path (relative to the base URL), optionally
// encoding body as JSON and decoding the response into out. A nil out discards
// the response body.
func (c *Client) do(ctx context.Context, method, path string, body, out any, ro *requestOptions) error {
	if ro == nil {
		ro = &requestOptions{}
	}

	u := c.baseURL + path
	if len(ro.query) > 0 {
		u += "?" + ro.query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	req.SetBasicAuth(c.apiKey, c.apiSecret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Idempotency keys make retried writes safe. The API accepts them on
	// POST/PUT/PATCH only.
	if key := c.idempotencyKey(method, ro); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// idempotencyKey returns the Idempotency-Key header value to use, generating one
// for writes when requested and none was supplied.
func (c *Client) idempotencyKey(method string, ro *requestOptions) string {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return ""
	}
	if ro.idempotencyKey != "" {
		return ro.idempotencyKey
	}
	if ro.generateIdempotencyKey {
		if key, err := uuid.GenerateUUID(); err == nil {
			return key
		}
	}
	return ""
}

func decodeAPIError(status int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: status}
	if len(body) > 0 {
		// Best-effort decode; keep the status even if the body is not JSON.
		_ = json.Unmarshal(body, apiErr)
		if apiErr.Message == "" {
			apiErr.Message = strings.TrimSpace(string(body))
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(status)
	}
	return apiErr
}

// checkRetry retries on connection errors, 429, and 5xx responses. It respects
// context cancellation.
func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}
	if resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented {
		return true, nil
	}
	return false, nil
}

// rateLimitBackoff honours Retry-After and the x-ratelimit-reset header when the
// server tells us how long to wait, otherwise falls back to exponential backoff.
func rateLimitBackoff(minWait, maxWait time.Duration, attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if wait, ok := waitFromHeaders(resp); ok {
			if wait < minWait {
				wait = minWait
			}
			if wait > maxWait {
				wait = maxWait
			}
			return wait
		}
	}
	return retryablehttp.DefaultBackoff(minWait, maxWait, attempt, resp)
}

// waitFromHeaders extracts a wait duration from Retry-After (delta seconds) or
// x-ratelimit-reset (an epoch-second timestamp).
func waitFromHeaders(resp *http.Response) (time.Duration, bool) {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second, true
		}
	}
	if reset := resp.Header.Get("x-ratelimit-reset"); reset != "" {
		if epoch, err := strconv.ParseInt(strings.TrimSpace(reset), 10, 64); err == nil {
			if d := time.Until(time.Unix(epoch, 0)); d > 0 {
				return d, true
			}
		}
	}
	return 0, false
}
