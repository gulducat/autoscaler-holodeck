variable "bin_dir" {
  type        = string
  description = "Absolute path to the directory containing compiled binaries (e.g. /home/user/code/gulducat/autoscaler-holodeck/bin)"
} 

//sample_urls format is '<metric_type>:<url>:<metric_type>:<url>'."
//possible metric types include 'holodeck_apm', 'nomad', and 'prometheus' though the last is not implemented"
variable "sample_urls" {
  type = string
  description = "optional colon deleniated string used to load any available sample metrics"
  }

job "holodeck" {
  type = "service"

  group "holodeck-observer" {
    network {
      port "holodeck" { static = 9091 }
      port "observer"  { static = 9090 }
    }

    task "observer" {
      driver = "raw_exec"

      # observer starts first so holodeck can reach it on startup
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      config {
        command = "${var.bin_dir}/observer"
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
      driver = "raw_exec"

      config {
        command = "${var.bin_dir}/holodeck"
      }

      env {
        SAMPLE_METRICS = var.sample_urls
        HOLODECK_ADDR = ":${NOMAD_PORT_holodeck}"
        OBSERVER_ADDR = "http://localhost:${NOMAD_PORT_observer}"
        NOMAD_ADDR    = "${NOMAD_UNIX_ADDR}"
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
  }
}
