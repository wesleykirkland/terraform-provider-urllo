terraform {
  required_providers {
    urllo = {
      source = "wesleykirkland/urllo"
    }
  }
}

# Credentials can also be supplied via the URLLO_API_KEY and URLLO_API_SECRET
# environment variables, and the endpoint via URLLO_ENDPOINT.
provider "urllo" {
  api_key    = var.urllo_api_key
  api_secret = var.urllo_api_secret
}
