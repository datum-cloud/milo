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

## Manual Setup

The below sections describe the commands to run to create a local kind cluster
and run the controller manager in the project control plane. This is an
alternative approach to local development than using
[devcontainer](https://containers.dev).


Create a local kind cluster named `milo-local`, which will be used to run the
controller manager.

```shell
kind create cluster --name milo-local
```

### Validate Current Context

The current context should be `kind-milo-local`.

```shell
kubectl config current-context
```

### Generate Base CRDs

This generates code including deepcopy, objects, CRDs, and potentially protobuf marshallers.

```shell
task generate
```

### Setup Overlay CRDs

This applies the core-control-plane and infra-control-plane overlay CRDs to the cluster.

```shell
kubectl apply -k config/crd/overlays/core-control-plane/
kubectl apply -k config/crd/overlays/infra-control-plane/
```

### Generate the Development Certificates

```shell
task generate-dev-certs
```

### Build the Project

This builds the project and places the binary in the `bin/milo` directory.

```shell
go build -o bin/milo ./cmd/milo
```

### Run the Controller Manager

This runs the controller manager in the project control plane. Ensure that the
`--cert-dir` is set to the directory where the certificates are stored, which
should be the `certs` directory in the root of the repo. This also skips
authentication, which is useful for local development. `$HOME/.kube/config`
should contain the local kind cluster config and have it set as the current
context.

```shell
./bin/milo controller-manager \
  --leader-elect=false \
  --authentication-skip-lookup \
  --secure-port=43271 \
  --cert-dir=./certs \
  --tls-cert-file=./certs/tls.crt \
  --tls-private-key-file=./certs/tls.key \
  --control-plane-scope=project \
  --kubeconfig=$HOME/.kube/config \
  --infra-cluster-kubeconfig=$HOME/.kube/config \
  --v=4
```

Then, test the server is running locally on the port specified in the
`--secure-port` flag above.

```shell
curl -k https://localhost:43271/healthz
```

Successful output:

```shell
ok
```

## Starting the API Server

> **NOTE:** This is not currently used for local development as the below task does not
> exist.

<!-- A docker compose testing environment is available to do end-to-end testing of
the API service locally. The task command `task apiserver:serve` is available to
quickly start the environment and run the API server.

```shell
task apiserver:serve
```

Additional arguments can be passed to the API server after specifying the `--`
parameter.

```shell
$ task apiserver:serve -- --help
``` -->

## Getting an Access Token for the API Server

To obtain an access token for use in Postman, follow these steps:

1. [Create a Zitadel Web
   App](https://github.com/datum-cloud/auth-playground/blob/main/providers/zitadel/README.md#-setting-up-zitadel-for-auth-playground-with-gogle-identity-provider)
   and set the Callback URL to `https://oauth.pstmn.io/v1/callback`

2. Get the `Access Token` from Postman
   1. Go to `Authorization` tab
   2. Configure the following settings:
      - Header Prefix: `Bearer`
      - Grant type: `Authorization Code (With PKCE)
      - Enable `Authorize using browser`
      - Auth URL: `http://localhost:8082/oauth/v2/authorize`
      - Access Token URL: `http://localhost:8082/oauth/v2/token`
      -  Client ID: `<The Zitadel App Id>`
      -  Scope: `email` or the neccesary scope
      -  Client Authentication: `Send as Basic Auth header`
   3. Click on `Get New Access Token`

## Accessing the Zitadel Web Interface

The Zitadel web interface is accessible at `http://localhost:8082`.

The default credentials for the preconfigured user are as follows:

- **Login Name**: `datum-admin@datum.localhost`
- **Password**: `Password1!`

## Accessing Mailhog - Test SMTP Server

Mailhog is a lightweight test SMTP server designed for local development. It captures all outgoing emails, providing a convenient way to test email-related functionality without relying on external email services.

### Web Interface

- **URL**: `http://localhost:8025`
  Access the Mailhog web interface to view and manage emails sent during development.

### API Access

- **Inbox Data**: `http://localhost:8025/api/v2/messages`
- 
  Retrieve all email data programmatically via the Mailhog API.

- **API Documentation**: [Mailhog API v2 Documentation](https://github.com/mailhog/MailHog/tree/master/docs/APIv2)

### Integration with Zitadel

The Zitadel service is preconfigured to route all outgoing emails, such as:

- One-Time Passwords (OTP)
- Two-Factor Authentication (2FA) messages
- Verification emails

All emails, regardless of the recipient address, are captured and available for testing within Mailhog.

This setup ensures a seamless and secure way to test email functionality during local development.

