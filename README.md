# Nomad Autoscaler Holodeck

You control a false reality to play with core
[nomad-autoscaler](https://github.com/hashicorp/nomad-autoscaler)
functionality.

## Demo

Build the docker container:

```
make docker
```

Run Nomad:

```
make nomad
```

In another shell, setup Nomad and run the job:

```
make job
```

Set env vars to use CLI / web UI

```
eval $(make env)
nomad ui -authenticate
```

