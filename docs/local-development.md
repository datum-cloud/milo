# Local Development

This document provides an overview on how to develop the IAM system locally.

## Dev Environment

This repo uses a [devcontainer environment][devcontainer] for creating a local
environment. This is designed to work with [Visual Studio Code][vscode] out of
the box, but may support other IDEs in the future. For more information, review
the [Developing Inside a Container documentation][vscode-devcontainer].

[devcontainer]: https://containers.dev
[vscode]: https://code.visualstudio.com/
[vscode-devcontainer]:
    https://code.visualstudio.com/docs/devcontainers/containers

This environment includes a lot of tooling to help with local development:

- Golang
- Docker
- Act (GitHub Actions Testing)
- Buf CLI (Protobuf publishing)
- Kubectl / Helm / Kind / Kustomize
- Task
- Protobuf
- OpenFGA CLI

Run the following command to see all available tasks.

```shell
task --list-all
```

## Starting the API Server

A docker compose testing environment is available to do end-to-end testing of
the API service locally. The task command `task apiserver:serve` is available to
quickly start the environment and run the API server.

```shell
task apiserver:serve
```

Additional arguments can be passed to the API server after specifying the `--`
parameter.

```shell
$ task apiserver:serve -- --help
```
