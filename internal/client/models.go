// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

// Envelope wrappers ----------------------------------------------------------

// singleEnvelope wraps a single resource response: {"data": {...}}.
type singleEnvelope[T any] struct {
	Data T `json:"data"`
}

// listEnvelope wraps a list response: {"data": [...], "meta": {...}, "links": {...}}.
type listEnvelope[T any] struct {
	Data  []T            `json:"data"`
	Meta  meta           `json:"meta"`
	Links paginationLink `json:"links"`
}

type meta struct {
	HasMore bool `json:"has_more"`
}

type paginationLink struct {
	Next *string `json:"next"`
	Prev *string `json:"prev"`
}

// ResourceLinks holds the self link for a resource.
type ResourceLinks struct {
	Self string `json:"self,omitempty"`
}

// Shared attribute objects ---------------------------------------------------

// MatchOptions controls how source URLs are matched.
type MatchOptions struct {
	CaseInsensitive  bool `json:"case_insensitive"`
	SlashInsensitive bool `json:"slash_insensitive"`
}

// NotFoundAction configures behaviour when no matching redirect is found.
type NotFoundAction struct {
	ForwardParams        bool    `json:"forward_params"`
	ForwardPath          bool    `json:"forward_path"`
	Custom404BodyPresent bool    `json:"custom_404_body_present,omitempty"`
	ResponseCode         *int    `json:"response_code,omitempty"`
	ResponseURL          *string `json:"response_url,omitempty"`
}

// Security holds HTTPS/HSTS settings for a host.
type Security struct {
	HTTPSUpgrade            bool `json:"https_upgrade"`
	PreventForeignEmbedding bool `json:"prevent_foreign_embedding"`
	HSTSIncludeSubDomains   bool `json:"hsts_include_sub_domains"`
	HSTSMaxAge              *int `json:"hsts_max_age,omitempty"`
	HSTSPreload             bool `json:"hsts_preload"`
}

// DNSValue describes an A/CNAME DNS record and its expected values.
type DNSValue struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
}

// DNSVerification holds the record required to verify domain ownership.
type DNSVerification struct {
	Type   string `json:"type"`
	Record string `json:"record"`
	Value  string `json:"value"`
}

// RequiredDNSValues describes the DNS records that must be configured for a host.
type RequiredDNSValues struct {
	Recommended  *DNSValue        `json:"recommended,omitempty"`
	Alternatives []DNSValue       `json:"alternatives,omitempty"`
	Verification *DNSVerification `json:"verification,omitempty"`
}
