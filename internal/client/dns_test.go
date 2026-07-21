// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"errors"
	"testing"
)

// fakeResolver is an injectable DNSResolver for tests.
type fakeResolver struct {
	hosts  map[string][]string
	cnames map[string]string
	txts   map[string][]string
	err    error
}

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.hosts[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	return v, nil
}

func (f fakeResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.cnames[host], nil
}

func (f fakeResolver) LookupTXT(_ context.Context, host string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.txts[host], nil
}

func TestCheckDNS_ARecordMatch(t *testing.T) {
	r := fakeResolver{hosts: map[string][]string{"www.example.com": {"34.213.106.51"}}}
	required := &RequiredDNSValues{
		Recommended: &DNSValue{Type: "A", Values: []string{"34.213.106.51", "54.68.182.72"}},
	}
	matched, reasons, err := CheckDNS(context.Background(), r, "www.example.com", required)
	if err != nil {
		t.Fatalf("CheckDNS error: %v", err)
	}
	if !matched {
		t.Fatalf("expected match, got reasons: %v", reasons)
	}
}

func TestCheckDNS_ARecordMismatch(t *testing.T) {
	r := fakeResolver{hosts: map[string][]string{"www.example.com": {"1.2.3.4"}}}
	required := &RequiredDNSValues{
		Recommended: &DNSValue{Type: "A", Values: []string{"34.213.106.51"}},
	}
	matched, reasons, err := CheckDNS(context.Background(), r, "www.example.com", required)
	if err != nil {
		t.Fatalf("CheckDNS error: %v", err)
	}
	if matched || len(reasons) == 0 {
		t.Fatalf("expected mismatch with reasons, got matched=%v reasons=%v", matched, reasons)
	}
}

func TestCheckDNS_CNAMEMatchIgnoresTrailingDot(t *testing.T) {
	r := fakeResolver{cnames: map[string]string{"go.example.com": "target.urllo.com."}}
	required := &RequiredDNSValues{
		Recommended: &DNSValue{Type: "CNAME", Values: []string{"target.urllo.com"}},
	}
	matched, reasons, err := CheckDNS(context.Background(), r, "go.example.com", required)
	if err != nil {
		t.Fatalf("CheckDNS error: %v", err)
	}
	if !matched {
		t.Fatalf("expected CNAME match, got reasons: %v", reasons)
	}
}

func TestCheckDNS_VerificationTXT(t *testing.T) {
	r := fakeResolver{
		hosts: map[string][]string{"www.example.com": {"34.213.106.51"}},
		txts:  map[string][]string{"_er-challenge.www": {"wrong"}},
	}
	required := &RequiredDNSValues{
		Recommended:  &DNSValue{Type: "A", Values: []string{"34.213.106.51"}},
		Verification: &DNSVerification{Type: "TXT", Record: "_er-challenge.www", Value: "expected-code"},
	}
	matched, reasons, err := CheckDNS(context.Background(), r, "www.example.com", required)
	if err != nil {
		t.Fatalf("CheckDNS error: %v", err)
	}
	if matched {
		t.Fatalf("expected mismatch due to missing TXT value")
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason for TXT, got %v", reasons)
	}
}

func TestCheckDNS_NilRequiredIsSatisfied(t *testing.T) {
	matched, _, err := CheckDNS(context.Background(), fakeResolver{}, "x.com", nil)
	if err != nil || !matched {
		t.Fatalf("nil required should be satisfied; matched=%v err=%v", matched, err)
	}
}

func TestCheckDNS_NilResolverDefaultsToNet(t *testing.T) {
	// A nil resolver falls back to NetResolver; with nil required it returns
	// early without performing any lookup.
	matched, _, err := CheckDNS(context.Background(), nil, "x.com", nil)
	if err != nil || !matched {
		t.Fatalf("expected satisfied with nil resolver+required; matched=%v err=%v", matched, err)
	}
}

func TestCheckDNS_TXTLookupError(t *testing.T) {
	r := fakeResolver{err: errors.New("txt boom")}
	required := &RequiredDNSValues{
		Verification: &DNSVerification{Type: "TXT", Record: "_c.x", Value: "v"},
	}
	_, _, err := CheckDNS(context.Background(), r, "x.com", required)
	if err == nil {
		t.Fatal("expected TXT lookup error")
	}
}

func TestCheckDNS_CNAMELookupError(t *testing.T) {
	r := fakeResolver{err: errors.New("cname boom")}
	required := &RequiredDNSValues{Recommended: &DNSValue{Type: "CNAME", Values: []string{"t.com"}}}
	_, _, err := CheckDNS(context.Background(), r, "x.com", required)
	if err == nil {
		t.Fatal("expected CNAME lookup error")
	}
}

func TestCheckDNS_ARecordLookupError(t *testing.T) {
	r := fakeResolver{err: errors.New("host boom")}
	required := &RequiredDNSValues{Recommended: &DNSValue{Type: "A", Values: []string{"1.2.3.4"}}}
	_, _, err := CheckDNS(context.Background(), r, "x.com", required)
	if err == nil {
		t.Fatal("expected A lookup error")
	}
}

func TestCheckDNS_CNAMEMismatch(t *testing.T) {
	r := fakeResolver{cnames: map[string]string{"x.com": "other.com"}}
	required := &RequiredDNSValues{Recommended: &DNSValue{Type: "CNAME", Values: []string{"t.com"}}}
	matched, reasons, err := CheckDNS(context.Background(), r, "x.com", required)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if matched || len(reasons) == 0 {
		t.Fatalf("expected CNAME mismatch, got matched=%v reasons=%v", matched, reasons)
	}
}
