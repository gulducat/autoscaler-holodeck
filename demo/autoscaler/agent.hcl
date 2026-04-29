log_level = "debug"

# Looks for binaries named after the driver in this directory.
# Relative to repo root (where make is run from).
plugin_dir = "./bin/plugins"

nomad {
  address = "http://127.0.0.1:4646"
}

policy {
  dir = "./demo/autoscaler/policies"
}

apm "holodeck-apm" {
  driver = "holodeck-apm"
  config = {
    holodeck_address = "http://localhost:9091"
    observer_address = "http://localhost:9090"
  }
}

target "nodesim-target" {
  driver = "nodesim-target"
  config = {
    nodesim_address  = "http://localhost:4649"
    observer_address = "http://localhost:9090"
  }
}
