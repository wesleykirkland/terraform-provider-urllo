// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Host is a source host as returned by the API. Extended attributes are only
// populated by GetHost/UpdateHost; ListHosts returns the basic subset.
type Host struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes HostAttributes `json:"attributes"`
	Links      ResourceLinks  `json:"links"`
}

// HostAttributes holds all readable host attributes.
type HostAttributes struct {
	Name               string             `json:"name"`
	DNSStatus          string             `json:"dns_status"`
	CertificateStatus  string             `json:"certificate_status"`
	DNSTestedAt        *string            `json:"dns_tested_at,omitempty"`
	ACMEEnabled        bool               `json:"acme_enabled"`
	MatchOptions       *MatchOptions      `json:"match_options,omitempty"`
	NotFoundAction     *NotFoundAction    `json:"not_found_action,omitempty"`
	Security           *Security          `json:"security,omitempty"`
	RequiredDNSEntries *RequiredDNSValues `json:"required_dns_entries,omitempty"`
	DetectedDNSEntries []DNSValue         `json:"detected_dns_entries,omitempty"`
}

// HostUpdate is the set of writable host attributes for PATCH /hosts/{id}. The
// custom 404 body is set via NotFoundAction.Custom404Body: the API's
// patchHost request schema nests it under not_found_action, not at the top
// level (confirmed against the live API and its OpenAPI spec — a top-level
// custom_404_body is silently dropped).
type HostUpdate struct {
	ACMEEnabled    *bool           `json:"acme_enabled,omitempty"`
	MatchOptions   *MatchOptions   `json:"match_options,omitempty"`
	NotFoundAction *NotFoundAction `json:"not_found_action,omitempty"`
	Security       *Security       `json:"security,omitempty"`
}

// ListHosts returns every host, transparently following pagination. limit sets
// the per-page size (0 uses the API default).
func (c *Client) ListHosts(ctx context.Context, limit int) ([]Host, error) {
	var all []Host
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	for {
		var env listEnvelope[Host]
		if err := c.do(ctx, http.MethodGet, "/hosts", nil, &env, &requestOptions{query: q}); err != nil {
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

// GetHost fetches a single host with its extended attributes.
func (c *Client) GetHost(ctx context.Context, id string) (*Host, error) {
	var env singleEnvelope[Host]
	if err := c.do(ctx, http.MethodGet, "/hosts/"+url.PathEscape(id), nil, &env, nil); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// GetHostByName returns the host whose name matches name, or nil if none
// match. ListHosts only returns the basic attribute subset (no
// RequiredDNSEntries, needed for DNS validation), so once a name match is
// found this re-fetches it via GetHost to get the extended attributes.
func (c *Client) GetHostByName(ctx context.Context, name string) (*Host, error) {
	hosts, err := c.ListHosts(ctx, 100)
	if err != nil {
		return nil, err
	}
	for i := range hosts {
		if hosts[i].Attributes.Name == name {
			return c.GetHost(ctx, hosts[i].ID)
		}
	}
	return nil, nil
}

// UpdateHost applies writable settings to an existing host.
func (c *Client) UpdateHost(ctx context.Context, id string, upd HostUpdate) (*Host, error) {
	var env singleEnvelope[Host]
	if err := c.do(ctx, http.MethodPatch, "/hosts/"+url.PathEscape(id), upd, &env, &requestOptions{generateIdempotencyKey: true}); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
