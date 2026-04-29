# Nomad Autoscaler Holodeck

A false reality to exercise core [nomad-autoscaler](https://github.com/hashicorp/nomad-autoscaler)
functionality.

## Local Testing

Run all unit tests across every module:

```sh
make test
```

Launch the Observer UI with mock fixture data for visual inspection in a browser:

```sh
make visual
```

The visual test starts a local HTTP server, opens the page automatically, and exits.

