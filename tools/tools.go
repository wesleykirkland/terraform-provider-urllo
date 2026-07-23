// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build generate

package tools

// Blank-imported purely so `go mod tidy` tracks these CLI tools and go.sum
// pins their versions; they're not used as libraries here. tfplugindocs and
// copywrite are invoked via `go run` in the go:generate directives below;
// gocover-cobertura is built from this module and run in CI's coverage step
// (see .github/workflows/test.yml).
import (
	_ "github.com/boumenot/gocover-cobertura"
	_ "github.com/hashicorp/copywrite"
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// Generate copyright headers
//go:generate go run github.com/hashicorp/copywrite headers -d .. --config ../.copywrite.hcl

// Format Terraform code for use in documentation.
// If you do not have Terraform installed, you can remove the formatting command, but it is suggested
// to ensure the documentation is formatted properly.
//go:generate terraform fmt -recursive ../examples/

// Generate documentation.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name urllo
