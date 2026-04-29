package holodeck

import hclog "github.com/hashicorp/go-hclog"

// noopLogger returns a logger that discards all output, for use in tests.
func noopLogger() hclog.Logger { return hclog.NewNullLogger() }
