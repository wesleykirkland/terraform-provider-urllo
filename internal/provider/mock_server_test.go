// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// mockUrllo is an in-memory implementation of the Urllo API used by mock-backed
// acceptance tests. It lets the full provider CRUD path run through the real
// Terraform plugin protocol without a live account or credentials.
type mockUrllo struct {
	mu    sync.Mutex
	rules map[string]*client.Rule
	hosts map[string]*client.Host
	seq   int

	// failStatus, when non-zero, makes every request return this status (until
	// cleared). failOnce fails only the next request; failWriteOnce fails only
	// the next write (POST/PATCH/DELETE). The one-shot variants let a test inject
	// a single failure without breaking teardown. All are used to exercise
	// client-error branches in the provider.
	failStatus    int
	failOnce      int
	failWriteOnce int
}

// newMockUrlloServer returns a running server seeded with one host so the host
// resource (which adopts an existing host) has something to manage.
func newMockUrlloServer(t *testing.T) *httptest.Server {
	srv, _ := newMockUrlloServerWithControl(t)
	return srv
}

// newMockUrlloServerWithControl also returns the backing store so a test can
// mutate it (e.g. inject failures or delete records out of band).
func newMockUrlloServerWithControl(t *testing.T) (*httptest.Server, *mockUrllo) {
	t.Helper()
	m := &mockUrllo{
		rules: map[string]*client.Rule{},
		hosts: map[string]*client.Host{},
	}
	m.hosts["host-1"] = &client.Host{
		ID:   "host-1",
		Type: "host",
		Attributes: client.HostAttributes{
			Name:              mockHostName,
			DNSStatus:         "active",
			CertificateStatus: "active",
		},
	}
	// A host whose required DNS will never be satisfied by a local lookup of the
	// reserved .invalid TLD, used to exercise the DNS validation failure path.
	m.hosts["host-2"] = &client.Host{
		ID:   "host-2",
		Type: "host",
		Attributes: client.HostAttributes{
			Name:              mockDNSFailHost,
			DNSStatus:         "requires_verification",
			CertificateStatus: "pending",
			RequiredDNSEntries: &client.RequiredDNSValues{
				Recommended: &client.DNSValue{Type: "A", Values: []string{"203.0.113.10"}},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(m.route))
	t.Cleanup(srv.Close)
	return srv, m
}

func (m *mockUrllo) setFail(status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failStatus = status
}

func (m *mockUrllo) setFailOnce(status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failOnce = status
}

// setFailWriteOnce makes the next write (POST/PATCH/DELETE) fail with status.
func (m *mockUrllo) setFailWriteOnce(status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failWriteOnce = status
}

func (m *mockUrllo) deleteRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
}

const (
	mockHostName    = "mock.example.com"
	mockDNSFailHost = "dnsfail.invalid"
)

func (m *mockUrllo) route(w http.ResponseWriter, r *http.Request) {
	// All requests must be authenticated.
	if _, _, ok := r.BasicAuth(); !ok {
		writeErr(w, http.StatusUnauthorized, "missing credentials")
		return
	}

	write := r.Method != http.MethodGet
	m.mu.Lock()
	status := 0
	switch {
	case m.failStatus != 0:
		status = m.failStatus
	case m.failOnce != 0:
		status, m.failOnce = m.failOnce, 0
	case write && m.failWriteOnce != 0:
		status, m.failWriteOnce = m.failWriteOnce, 0
	}
	m.mu.Unlock()
	if status != 0 {
		writeErr(w, status, "injected failure")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case len(parts) == 1 && parts[0] == "rules":
		m.rulesCollection(w, r)
	case len(parts) == 2 && parts[0] == "rules":
		m.ruleItem(w, r, parts[1])
	case len(parts) == 1 && parts[0] == "hosts":
		m.hostsCollection(w, r)
	case len(parts) == 2 && parts[0] == "hosts":
		m.hostItem(w, r, parts[1])
	default:
		writeErr(w, http.StatusNotFound, "unknown path")
	}
}

func (m *mockUrllo) rulesCollection(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		var attrs client.RuleAttributes
		if err := json.NewDecoder(r.Body).Decode(&attrs); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "bad body")
			return
		}
		m.seq++
		id := fmt.Sprintf("rule-%d", m.seq)
		attrs = normalizeRuleAttributes(attrs)
		rule := &client.Rule{ID: id, Type: "rule", Attributes: attrs}
		m.rules[id] = rule
		writeData(w, http.StatusCreated, rule)
	case http.MethodGet:
		sq := r.URL.Query().Get("sq")
		var out []client.Rule
		for _, rule := range m.rules {
			if sq != "" && !containsAny(rule.Attributes.SourceURLs, sq) {
				continue
			}
			out = append(out, *rule)
		}
		writeList(w, out)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (m *mockUrllo) ruleItem(w http.ResponseWriter, r *http.Request, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		writeErr(w, http.StatusNotFound, "rule not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeData(w, http.StatusOK, rule)
	case http.MethodPatch:
		var attrs client.RuleAttributes
		if err := json.NewDecoder(r.Body).Decode(&attrs); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "bad body")
			return
		}
		rule.Attributes = normalizeRuleAttributes(attrs)
		writeData(w, http.StatusOK, rule)
	case http.MethodDelete:
		delete(m.rules, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (m *mockUrllo) hostsCollection(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var out []client.Host
	for _, h := range m.hosts {
		out = append(out, *h)
	}
	writeList(w, out)
}

func (m *mockUrllo) hostItem(w http.ResponseWriter, r *http.Request, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	host, ok := m.hosts[id]
	if !ok {
		writeErr(w, http.StatusNotFound, "host not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeData(w, http.StatusOK, host)
	case http.MethodPatch:
		var upd client.HostUpdate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "bad body")
			return
		}
		if upd.ACMEEnabled != nil {
			host.Attributes.ACMEEnabled = *upd.ACMEEnabled
		}
		if upd.MatchOptions != nil {
			host.Attributes.MatchOptions = upd.MatchOptions
		}
		if upd.NotFoundAction != nil {
			host.Attributes.NotFoundAction = upd.NotFoundAction
		}
		if upd.Security != nil {
			host.Attributes.Security = upd.Security
		}
		writeData(w, http.StatusOK, host)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- response helpers --------------------------------------------------------

func writeData(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
}

func writeList(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  v,
		"meta":  map[string]any{"has_more": false},
		"links": map[string]any{"next": nil},
	})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "message": msg})
}

func containsAny(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.Contains(h, needle) {
			return true
		}
	}
	return false
}

// normalizeRuleAttributes mimics the real Urllo API's server-side URL
// normalization: target_url gets a trailing slash when it has no path, and
// source_urls get an "https://" scheme prefix when bare hostnames are sent.
// This reproduces the "Provider produced inconsistent result after apply" bug
// reported against a live account, where the API's echoed values differed from
// what was configured.
func normalizeRuleAttributes(attrs client.RuleAttributes) client.RuleAttributes {
	attrs.TargetURL = normalizeURL(attrs.TargetURL)
	normalized := make([]string, len(attrs.SourceURLs))
	for i, s := range attrs.SourceURLs {
		normalized[i] = normalizeURL(s)
	}
	attrs.SourceURLs = normalized
	return attrs
}

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "" {
		u.Scheme = "https"
		// url.Parse treats a bare "host.com" as a relative path, not a host.
		if u.Host == "" {
			u.Host = u.Path
			u.Path = ""
		}
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
