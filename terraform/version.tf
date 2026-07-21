terraform {
  required_providers {
    urllo = {
      source = "wesleykirkland/urllo"
      # Matches the version installed by ./build-local.sh into the local
      # filesystem mirror. Set to a released version when consuming the
      # published provider from the registry.
      version = "0.0.1"
    }
  }
}
