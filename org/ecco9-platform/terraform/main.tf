# ecco9 cognitive platform cluster.
# Provisions the Kubernetes cluster and node pools for the distributed
# Deep Tree Echo deployment.

terraform {
  required_version = ">= 1.6"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

variable "project_id" { type = string }
variable "region" {
  type    = string
  default = "us-central1"
}

resource "google_container_cluster" "ecco9" {
  name     = "ecco9-cognitive"
  location = var.region

  remove_default_node_pool = true
  initial_node_count       = 1

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }
}

# General cognitive services.
resource "google_container_node_pool" "cognitive" {
  name       = "cognitive-pool"
  cluster    = google_container_cluster.ecco9.name
  location   = var.region

  autoscaling {
    min_node_count = 2
    max_node_count = 10
  }

  node_config {
    machine_type = "n2-standard-4"
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    labels = {
      "ecco9.io/tier" = "cognitive"
    }
  }
}

# GPU pool for model runners, co-located with the reservoir for
# minimal inference pipeline latency.
resource "google_container_node_pool" "gpu" {
  name     = "gpu-runner-pool"
  cluster  = google_container_cluster.ecco9.name
  location = var.region

  autoscaling {
    min_node_count = 0
    max_node_count = 8
  }

  node_config {
    machine_type = "g2-standard-8"
    guest_accelerator {
      type  = "nvidia-l4"
      count = 1
    }
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    labels = {
      "ecco9.io/tier" = "inference"
    }
    taint {
      key    = "ecco9.io/gpu"
      value  = "true"
      effect = "NO_SCHEDULE"
    }
  }
}

output "cluster_endpoint" {
  value = google_container_cluster.ecco9.endpoint
}
