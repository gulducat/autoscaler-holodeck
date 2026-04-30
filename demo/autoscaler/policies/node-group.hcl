# Sample policy exercising the full holodeck-apm → nodesim-target loop.
# Adjust node_group, query, target, min/max, and cooldown to taste.
#
# Prerequisites:
#   - nodesim running with a group named "my-group" (POST /v1/groups)
#   - holodeck running with a metric named "cpu_utilization" authored

scaling "my-group" {
  enabled = true
  min     = 0
  max     = 5

  policy {
    cooldown            = "30s"
    evaluation_interval = "10s"

    check "cpu_utilization" {
      source = "holodeck-apm"
      query  = "cpu_utilization"

      strategy "target-value" {
        target = "0.7"
      }
    }

    target "nodesim-target" {
      node_group = "my-group"
    }
  }
}
