// Copyright (c) Wesley Kirkland-Daily
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Rule is a redirect rule.
type Rule struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Attributes    RuleAttributes `json:"attributes"`
	Relationships map[string]any `json:"relationships,omitempty"`
}

// RuleAttributes holds the readable and writable attributes of a rule.
type RuleAttributes struct {
	ForwardParams bool     `json:"forward_params"`
	ForwardPath   bool     `json:"forward_path"`
	ResponseType  string   `json:"response_type"`
	SourceURLs    []string `json:"source_urls"`
	TargetURL     string   `json:"target_url"`
	Tags          []string `json:"tags,omitempty"`

	// Read-only, API-computed attributes. omitempty keeps them out of the
	// create/update request bodies, which reuse this struct for the payload.
	Name              string `json:"name,omitempty"`
	DNSStatus         string `json:"dns_status,omitempty"`
	CertificateStatus string `json:"certificate_status,omitempty"`

	// Analytics is only present when the request set include_analytics=true.
	Analytics *AnalyticsAttributes `json:"analytics,omitempty"`
}

// AnalyticsAttributes holds request-volume analytics for a rule over the
// requested date range. Only populated when a list/get call sets
// ListRulesOptions.IncludeAnalytics.
type AnalyticsAttributes struct {
	// AnalyticsStartDate and AnalyticsEndDate are the effective dates used,
	// which may differ from the requested range (the API clamps the start
	// date to your plan's earliest allowed date, and the end date to
	// yesterday).
	AnalyticsStartDate string `json:"analytics_start_date"`
	AnalyticsEndDate   string `json:"analytics_end_date"`
	RequestsProcessed  int64  `json:"requests_processed"`
}

// Response type values for a rule.
const (
	ResponseMovedPermanently = "moved_permanently"
	ResponseFound            = "found"
)

// ListRulesOptions are the filters accepted by GET /rules.
type ListRulesOptions struct {
	SourceQuery      string // sq
	TargetQuery      string // tq
	Tags             []string
	TagMatchStrategy string // "any" (default) or "all"
	Limit            int

	// IncludeAnalytics requests per-rule request-volume analytics
	// (include_analytics). AnalyticsStartDate/AnalyticsEndDate (YYYY-MM-DD)
	// are only sent when IncludeAnalytics is true; the API defaults them to
	// one month ago and yesterday respectively when omitted.
	IncludeAnalytics   bool
	AnalyticsStartDate string
	AnalyticsEndDate   string
}

func (o ListRulesOptions) query() url.Values {
	q := url.Values{}
	if o.SourceQuery != "" {
		q.Set("sq", o.SourceQuery)
	}
	if o.TargetQuery != "" {
		q.Set("tq", o.TargetQuery)
	}
	for _, t := range o.Tags {
		q.Add("tags[]", t)
	}
	if o.TagMatchStrategy != "" {
		q.Set("tag_match_strategy", o.TagMatchStrategy)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.IncludeAnalytics {
		q.Set("include_analytics", "true")
		if o.AnalyticsStartDate != "" {
			q.Set("analytics_start_date", o.AnalyticsStartDate)
		}
		if o.AnalyticsEndDate != "" {
			q.Set("analytics_end_date", o.AnalyticsEndDate)
		}
	}
	return q
}

// ListRules returns every matching rule, transparently following pagination.
func (c *Client) ListRules(ctx context.Context, opts ListRulesOptions) ([]Rule, error) {
	var all []Rule
	q := opts.query()
	for {
		var env listEnvelope[Rule]
		if err := c.do(ctx, http.MethodGet, "/rules", nil, &env, &requestOptions{query: q}); err != nil {
			return nil, err
		}
		all = append(all, env.Data...)
		next, ok := nextPageQuery(env.Links.Next)
		if !ok {
			break
		}
		q = next
	}
	return all, nil
}

// GetRule fetches a single rule by ID, or returns (nil, nil) if no rule with
// that ID exists. opts carries only the analytics fields (IncludeAnalytics,
// AnalyticsStartDate, AnalyticsEndDate); callers should leave the other
// ListRulesOptions fields zero-valued since the ID match happens client-side
// below, not via a server-side filter.
//
// This deliberately does not call GET /rules/{id}: on the live API that path
// shape is served by CloudFront, which returns a stale cached 404 for every
// rule ID (confirmed against real, existing rules), not just missing ones.
// Using that endpoint would make Read() think every rule had been deleted
// out-of-band and recreate it on every apply. Listing and filtering
// client-side avoids the broken route entirely.
func (c *Client) GetRule(ctx context.Context, id string, opts ListRulesOptions) (*Rule, error) {
	rules, err := c.ListRules(ctx, opts)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].ID == id {
			return &rules[i], nil
		}
	}
	return nil, nil
}

// CreateRule creates a new redirect rule.
func (c *Client) CreateRule(ctx context.Context, attrs RuleAttributes) (*Rule, error) {
	var env singleEnvelope[Rule]
	if err := c.do(ctx, http.MethodPost, "/rules", attrs, &env, &requestOptions{generateIdempotencyKey: true}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// UpdateRule updates an existing rule.
func (c *Client) UpdateRule(ctx context.Context, id string, attrs RuleAttributes) (*Rule, error) {
	var env singleEnvelope[Rule]
	if err := c.do(ctx, http.MethodPatch, "/rules/"+url.PathEscape(id), attrs, &env, &requestOptions{generateIdempotencyKey: true}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// DeleteRule removes a rule.
func (c *Client) DeleteRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/rules/"+url.PathEscape(id), nil, nil, nil)
}
