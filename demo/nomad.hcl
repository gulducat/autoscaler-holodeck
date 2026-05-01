# data_dir provided as -data-dir flag in Makefile

# tasks use workload identity to hit nomad API
acl {
  enabled    = true
  token_ttl  = "30s"
  policy_ttl = "60s"
  role_ttl   = "60s"
}

server {
  enabled = true

  bootstrap_expect = 1
}

client {
  enabled = true
}

plugin "docker" {
  config {
    allow_privileged = true
    gc {
      image = false
    }
  }
}
