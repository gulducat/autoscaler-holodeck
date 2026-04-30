variable "bin_dir" {
  type        = string
  description = "Absolute path to the directory containing compiled binaries (e.g. /home/user/code/gulducat/autoscaler-holodeck/bin)"
}

//sample_urls format is '<metric_type>:<url>:<metric_type>:<url>'."
//possible metric types include 'holodeck_apm', 'nomad', and 'prometheus' though the last is not implemented"
variable "sample_urls" {
  type        = string
  description = "optional colon deleniated string used to load any available sample metrics"
  default     = ""
}

job "holodeck" {
  type = "service"

  group "holodeck-observer" {
    network {
      port "holodeck" { static = 9091 }
      port "observer" { static = 9090 }
      port "nodesim" { static = 4649 }
    }

    task "nodesim" {
      # autoscaler target plugin talks to nodesim
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
      driver = "docker"
      config {
        image = "holodeck:local"
        args  = ["nodesim"]
        ports = ["nodesim"]
      }
      env {
        NODESIM_GROUPS_ADDR = ":${NOMAD_PORT_nodesim}"
      }
      service {
        provider = "nomad"
        name     = "nodesim"
        port     = "nodesim"
      }
    }

    task "observer" {
      # observer starts first so other stuff can reach it on startup
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      driver = "docker"
      config {
        image = "holodeck:local"
        args  = ["observer"]
        ports = ["observer"]
      }

      env {
        OBSERVER_ADDR = ":${NOMAD_PORT_observer}"
        NOMAD_ADDR    = "${NOMAD_UNIX_ADDR}"
      }

      identity {
        env = true
      }

      service {
        provider = "nomad"
        name     = "observer"
        port     = "observer"

        check {
          type     = "http"
          path     = "/v1/health"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }

    task "holodeck" {
      driver = "docker"
      config {
        image = "holodeck:local"
        args  = ["holodeck"]
        ports = ["holodeck"]
      }

      env {
        SAMPLE_METRICS = var.sample_urls
        HOLODECK_ADDR  = ":${NOMAD_PORT_holodeck}"
        OBSERVER_ADDR  = "http://${NOMAD_ADDR_observer}"
        NOMAD_ADDR     = "${NOMAD_UNIX_ADDR}"
      }

      identity {
        env = true
      }

      service {
        provider = "nomad"
        name     = "holodeck"
        port     = "holodeck"

        check {
          type     = "http"
          path     = "/v1/health"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }

    task "autoscaler" {
      driver = "docker"
      config {
        image = "holodeck:local"
        args  = ["autoscaler"]
      }
      identity {
        env = true
      }
      env {
        HOLODECK_ADDR = "http://${NOMAD_ADDR_holodeck}"
        OBSERVER_ADDR = "http://${NOMAD_ADDR_observer}"
        #NOMAD_ADDR    = "${NOMAD_UNIX_ADDR}"
        NOMAD_ADDR              = "http://192.168.10.11:4646"
        AUTOSCALER_AGENT_CONFIG = "${NOMAD_TASK_DIR}/agent.hcl"
        NODESIM_GROUPS_ADDR     = "${NODESIM_ADDR_nodesim}:${NOMAD_PORT_nodesim}"
      }
      template {
        destination = "${NOMAD_TASK_DIR}/agent.hcl"
        data        = <<-EOF
          log_level = "debug"
          plugin_dir = "/app/bin/plugins"
          policy {
            dir = "/app/demo/autoscaler/policies"
          }
          apm "holodeck-apm" {
            driver = "holodeck-apm"
          }
          target "nodesim-target" {
            driver = "nodesim-target"
          }
        EOF
      }
    }
  }
}
