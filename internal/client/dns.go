// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// DNSResolver is the subset of net.Resolver used for local DNS validation. It is
// an interface so tests can inject a fake resolver.
type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupTXT(ctx context.Context, host string) ([]string, error)
}

// NetResolver is the default DNSResolver backed by the system resolver.
var NetResolver DNSResolver = net.DefaultResolver

// CheckDNS resolves hostname using the local resolver and reports whether its
// live records satisfy required. It returns matched=true when the host is
// correctly configured, otherwise a slice of human-readable reasons describing
// what is missing. A non-nil error indicates the lookup itself failed
// (e.g. NXDOMAIN), which callers typically treat as "not ready yet".
func CheckDNS(ctx context.Context, r DNSResolver, hostname string, required *RequiredDNSValues) (bool, []string, error) {
	if r == nil {
		r = NetResolver
	}
	if required == nil {
		// Nothing to validate against; treat as satisfied.
		return true, nil, nil
	}

	var reasons []string

	// Ownership verification via a TXT challenge record, when required.
	if v := required.Verification; v != nil && strings.EqualFold(v.Type, "TXT") && v.Record != "" {
		txts, err := r.LookupTXT(ctx, v.Record)
		if err != nil {
			return false, nil, fmt.Errorf("looking up TXT %s: %w", v.Record, err)
		}
		if !containsFold(txts, v.Value) {
			reasons = append(reasons, fmt.Sprintf("TXT record %s must contain %q", v.Record, v.Value))
		}
	}

	// Recommended A/CNAME record.
	if rec := required.Recommended; rec != nil {
		ok, reason, err := checkRecord(ctx, r, hostname, *rec)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			reasons = append(reasons, reason)
		}
	}

	return len(reasons) == 0, reasons, nil
}

// checkRecord validates a single recommended DNS record against the live values
// for hostname.
func checkRecord(ctx context.Context, r DNSResolver, hostname string, rec DNSValue) (bool, string, error) {
	switch strings.ToUpper(rec.Type) {
	case "CNAME":
		cname, err := r.LookupCNAME(ctx, hostname)
		if err != nil {
			return false, "", fmt.Errorf("looking up CNAME %s: %w", hostname, err)
		}
		if containsFold(rec.Values, cname) {
			return true, "", nil
		}
		return false, fmt.Sprintf("CNAME for %s is %q; expected one of %v", hostname, canonical(cname), rec.Values), nil
	default: // "A" (or unspecified) — validate resolved addresses.
		addrs, err := r.LookupHost(ctx, hostname)
		if err != nil {
			return false, "", fmt.Errorf("looking up %s: %w", hostname, err)
		}
		if intersects(addrs, rec.Values) {
			return true, "", nil
		}
		return false, fmt.Sprintf("%s resolves to %v; expected it to include one of %v", hostname, addrs, rec.Values), nil
	}
}

// canonical trims a trailing dot from a DNS name.
func canonical(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(name), ".")
}

// containsFold reports whether want is present in got, comparing case- and
// trailing-dot-insensitively.
func containsFold(got []string, want string) bool {
	w := canonical(want)
	for _, g := range got {
		if strings.EqualFold(canonical(g), w) {
			return true
		}
	}
	return false
}

// intersects reports whether any expected value appears in got.
func intersects(got, expected []string) bool {
	for _, e := range expected {
		if containsFold(got, e) {
			return true
		}
	}
	return false
}
