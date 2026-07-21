// Copyright IBM Corp. 2021, 2025
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

// GetRule fetches a single rule by ID.
func (c *Client) GetRule(ctx context.Context, id string) (*Rule, error) {
	var env singleEnvelope[Rule]
	if err := c.do(ctx, http.MethodGet, "/rules/"+url.PathEscape(id), nil, &env, nil); err != nil {
		return nil, err
	}
	return &env.Data, nil
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
