// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

func TestHostnameOf(t *testing.T) {
	cases := map[string]string{
		"example.com":                 "example.com",
		"example.com/path":            "example.com",
		"https://example.com/a/b?q=1": "example.com",
		"http://sub.example.com":      "sub.example.com",
		"  spaced.com/x  ":            "spaced.com",
		"":                            "",
	}
	for in, want := range cases {
		if got := hostnameOf(in); got != want {
			t.Errorf("hostnameOf(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestDistinctHostnames(t *testing.T) {
	got := distinctHostnames([]string{"b.com/x", "a.com", "b.com", "https://a.com/y"})
	want := []string{"a.com", "b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("distinctHostnames = %v; want %v", got, want)
	}
}

// fakeResolver satisfies client.DNSResolver for validation tests.
type fakeResolver struct {
	hosts map[string][]string
	err   error
}

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hosts[host], nil
}
func (f fakeResolver) LookupCNAME(_ context.Context, host string) (string, error) { return "", f.err }
func (f fakeResolver) LookupTXT(_ context.Context, host string) ([]string, error) { return nil, f.err }

func TestWaitForDNS_SuccessAndTimeout(t *testing.T) {
	// Shorten the poll interval for the timeout path.
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	defer func() { dnsPollInterval = orig }()

	required := &client.RequiredDNSValues{
		Recommended: &client.DNSValue{Type: "A", Values: []string{"34.213.106.51"}},
	}

	// Matching resolver -> immediate success (nil reasons).
	r := &RuleResource{resolver: fakeResolver{hosts: map[string][]string{"ok.com": {"34.213.106.51"}}}}
	if reasons := r.waitForDNS(context.Background(), "ok.com", required, time.Now().Add(time.Second)); reasons != nil {
		t.Fatalf("expected success, got reasons %v", reasons)
	}

	// Non-matching resolver -> timeout returns reasons.
	r = &RuleResource{resolver: fakeResolver{hosts: map[string][]string{"bad.com": {"9.9.9.9"}}}}
	if r.waitForDNS(context.Background(), "bad.com", required, time.Now().Add(5*time.Millisecond)) == nil {
		t.Fatalf("expected timeout reasons, got nil")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Errorf("firstNonEmpty = %q; want third", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty of empties = %q; want empty", got)
	}
}
