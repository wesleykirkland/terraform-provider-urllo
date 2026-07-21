// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	urlloprovider "github.com/wesleykirkland/terraform-provider-urllo/internal/provider"
)

var (
	// version is set by the goreleaser configuration to appropriate values for
	// the compiled binary. It is "dev" when built and run locally.
	version string = "dev"

	// serve is the provider server entrypoint, overridable in tests.
	serve = providerserver.Serve
)

func main() {
	mainWithExit(log.Fatal)
}

// mainWithExit runs the provider and reports a fatal error via onError, which is
// injected so the error path is testable without terminating the process.
func mainWithExit(onError func(...any)) {
	if err := run(context.Background(), version, os.Args[1:]); err != nil {
		onError(err.Error())
	}
}

// run parses args and serves the provider. It returns any parse or serve error.
func run(ctx context.Context, version string, args []string) error {
	fs := flag.NewFlagSet("terraform-provider-urllo", flag.ContinueOnError)
	debug := fs.Bool("debug", false, "set to true to run the provider with support for debuggers like delve")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/wesleykirkland/urllo",
		Debug:   *debug,
	}

	return serve(ctx, urlloprovider.New(version), opts)
}
