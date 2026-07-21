// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// withServe swaps the serve entrypoint for the duration of a test.
func withServe(t *testing.T, fn func(context.Context, func() provider.Provider, providerserver.ServeOpts) error) {
	t.Helper()
	orig := serve
	serve = fn
	t.Cleanup(func() { serve = orig })
}

func TestRunServesProvider(t *testing.T) {
	var gotAddr string
	withServe(t, func(_ context.Context, factory func() provider.Provider, opts providerserver.ServeOpts) error {
		gotAddr = opts.Address
		if factory() == nil {
			t.Error("provider factory returned nil")
		}
		return nil
	})
	if err := run(context.Background(), "test", []string{"-debug"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if gotAddr != "registry.terraform.io/wesleykirkland/urllo" {
		t.Errorf("address = %q", gotAddr)
	}

	// An unknown flag surfaces as a parse error.
	if err := run(context.Background(), "test", []string{"-nope"}); err == nil {
		t.Error("expected parse error for unknown flag")
	}
}

// cleanArgs replaces os.Args with just the program name so mainWithExit's flag
// parsing does not choke on the test binary's own flags.
func cleanArgs(t *testing.T) {
	t.Helper()
	orig := os.Args
	os.Args = []string{"terraform-provider-urllo"}
	t.Cleanup(func() { os.Args = orig })
}

func TestMainWithExitSuccessAndError(t *testing.T) {
	cleanArgs(t)
	// Success path: serve returns nil, onError must not be called.
	withServe(t, func(context.Context, func() provider.Provider, providerserver.ServeOpts) error {
		return nil
	})
	called := false
	mainWithExit(func(...any) { called = true })
	if called {
		t.Error("onError should not be called on success")
	}

	// Error path: serve returns an error, onError must be called.
	withServe(t, func(context.Context, func() provider.Provider, providerserver.ServeOpts) error {
		return errors.New("serve failed")
	})
	called = false
	mainWithExit(func(...any) { called = true })
	if !called {
		t.Error("onError should be called on serve error")
	}
}

func TestMainDelegates(t *testing.T) {
	cleanArgs(t)
	// main() delegates to mainWithExit; with serve stubbed to succeed it is a
	// no-op that must not exit the process.
	withServe(t, func(context.Context, func() provider.Provider, providerserver.ServeOpts) error {
		return nil
	})
	main()
}
