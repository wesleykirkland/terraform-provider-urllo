// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetHostDecodesExtendedAttributes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hosts/h1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":{"id":"h1","type":"host","attributes":{
			"name":"www.example.com","dns_status":"active","certificate_status":"active",
			"acme_enabled":true,
			"match_options":{"case_insensitive":true,"slash_insensitive":false},
			"security":{"https_upgrade":true,"hsts_max_age":31536000},
			"not_found_action":{"response_code":302,"response_url":"https://x.com"},
			"required_dns_entries":{"recommended":{"type":"CNAME","values":["t.urllo.com"]}},
			"detected_dns_entries":[{"type":"A","values":["1.2.3.4"]}]}}}`)
	}))
	defer srv.Close()

	host, err := newTestClient(srv).GetHost(context.Background(), "h1")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	a := host.Attributes
	if a.Name != "www.example.com" || !a.ACMEEnabled {
		t.Fatalf("unexpected host attributes: %+v", a)
	}
	if a.MatchOptions == nil || !a.MatchOptions.CaseInsensitive {
		t.Fatalf("match_options not decoded: %+v", a.MatchOptions)
	}
	if a.Security == nil || a.Security.HSTSMaxAge == nil || *a.Security.HSTSMaxAge != 31536000 {
		t.Fatalf("security not decoded: %+v", a.Security)
	}
	if a.NotFoundAction == nil || a.NotFoundAction.ResponseCode == nil || *a.NotFoundAction.ResponseCode != 302 {
		t.Fatalf("not_found_action not decoded: %+v", a.NotFoundAction)
	}
	if a.RequiredDNSEntries == nil || a.RequiredDNSEntries.Recommended == nil {
		t.Fatalf("required_dns_entries not decoded: %+v", a.RequiredDNSEntries)
	}
	if len(a.DetectedDNSEntries) != 1 {
		t.Fatalf("detected_dns_entries not decoded: %+v", a.DetectedDNSEntries)
	}
}

func TestListHostsFollowsPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("starting_after") == "" {
			fmt.Fprint(w, `{"data":[{"id":"h1","type":"host","attributes":{"name":"a.com"}}],
				"meta":{"has_more":true},"links":{"next":"/v1/hosts?starting_after=h1"}}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"h2","type":"host","attributes":{"name":"b.com"}}],
			"links":{"next":null}}`)
	}))
	defer srv.Close()

	hosts, err := newTestClient(srv).ListHosts(context.Background(), 50)
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetHostByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hosts/h2" {
			// GetHostByName must re-fetch the matched host by ID: the list
			// endpoint omits required_dns_entries, which DNS validation needs.
			fmt.Fprint(w, `{"data":{"id":"h2","type":"host","attributes":{
				"name":"b.com","required_dns_entries":{"recommended":{"type":"CNAME","values":["t.urllo.com"]}}}}}`)
			return
		}
		fmt.Fprint(w, `{"data":[
			{"id":"h1","type":"host","attributes":{"name":"a.com"}},
			{"id":"h2","type":"host","attributes":{"name":"b.com"}}],"links":{"next":null}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	host, err := c.GetHostByName(context.Background(), "b.com")
	if err != nil {
		t.Fatalf("GetHostByName: %v", err)
	}
	if host == nil || host.ID != "h2" {
		t.Fatalf("expected h2, got %+v", host)
	}
	if host.Attributes.RequiredDNSEntries == nil {
		t.Fatalf("expected required_dns_entries to be populated from GetHost, got %+v", host.Attributes)
	}

	missing, err := c.GetHostByName(context.Background(), "nope.com")
	if err != nil {
		t.Fatalf("GetHostByName(missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing host, got %+v", missing)
	}
}

func TestUpdateHostSendsPayload(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Idempotency-Key") == "" {
			t.Errorf("expected Idempotency-Key on PATCH")
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		fmt.Fprint(w, `{"data":{"id":"h1","type":"host","attributes":{"name":"a.com","acme_enabled":true}}}`)
	}))
	defer srv.Close()

	enabled := true
	host, err := newTestClient(srv).UpdateHost(context.Background(), "h1", HostUpdate{
		ACMEEnabled:  &enabled,
		MatchOptions: &MatchOptions{CaseInsensitive: true},
	})
	if err != nil {
		t.Fatalf("UpdateHost: %v", err)
	}
	if !host.Attributes.ACMEEnabled {
		t.Fatalf("expected acme_enabled true, got %+v", host.Attributes)
	}
	if body["acme_enabled"] != true {
		t.Fatalf("acme_enabled not sent in body: %v", body)
	}
	if _, ok := body["match_options"]; !ok {
		t.Fatalf("match_options not sent in body: %v", body)
	}
}
