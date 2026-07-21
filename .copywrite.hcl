# NOTE: This file is for HashiCorp specific licensing automation and can be deleted after creating a new repo with this template.
schema_version = 1

project {
  license        = "MPL-2.0"
  copyright_year = 2026

  header_ignore = [
    # internal catalog metadata (prose)
    "META.d/**/*.yaml",

    # examples used within documentation (prose)
    "examples/**",

    # runnable local-build example (HCL/shell, not part of the licensed provider)
    "terraform/**",

    # GitHub issue template configuration
    ".github/ISSUE_TEMPLATE/*.yml",

    # golangci-lint tooling configuration
    ".golangci.yml",

    # GoReleaser tooling configuration
    ".goreleaser.yml",
  ]
}
