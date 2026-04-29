# ACL policy for holodeck and observer workload identity tokens.
# Grants the read access needed to watch allocations, nodes, and the event stream.

namespace "default" {
  capabilities = ["list-jobs", "read-job"]
}

node {
  policy = "read"
}
