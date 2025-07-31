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

## Starting the API Server with Authentication Webhook

To start the API server locally with authentication enabled, follow these steps:

1. **Ensure a kind cluster is running**  
   The `start-apiserver-with-authentication-webhook` task will automatically create a local kind cluster named `kind-etcd` if it does not already exist. Be sure that the webhook http server is running. This is an external service.

2. **Start the API server with authentication webhook**  
   Run the following command:
   ```shell
   task start-apiserver-with-authentication-webhook
   ```

3. **Verify authentication**  
   To check if authentication is working, use the following `curl` command (replace `{token}` with a valid token):
   ```shell
   curl -k -H "Authorization: Bearer {token}" https://127.0.0.1:6443/healthz
   ```
   If authentication is configured correctly and the token is valid, you should receive a `200 OK` response.
