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

## Test Infrastructure Cluster

The recommended approach for local development is using the test infrastructure cluster that includes production-like components (Flux, cert-manager, Kyverno) along with the Milo controller manager.

### Quick Start

For first-time setup, use the complete setup command:

```shell
task test-infra-setup-complete
```

This single command will:
1. Create a kind cluster with test infrastructure (Flux, cert-manager, Kyverno)
2. Generate all CRDs and manifests
3. Build the Milo container image
4. Load the image into the kind cluster
5. Deploy the controller manager and webhooks
6. Verify the deployment is healthy

### Development Workflow

After the initial setup, use these commands for daily development:

#### Code Changes
When you make changes to Go code:

```shell
task test-infra-redeploy
```

This will:
- Rebuild the container image with your latest changes
- Load the updated image into the kind cluster
- Restart the controller manager deployment
- Wait for the deployment to be ready

#### Configuration Changes
When you modify Kubernetes manifests or Kustomize overlays:

```shell
kubectl apply -k config/dev/
```

#### View Logs
To see controller manager logs in real-time:

```shell
task test-infra-logs
```

Press `Ctrl+C` to stop following the logs.

#### Check Status
Verify the deployment status:

```shell
kubectl get pods -n milo-system
kubectl get nodes
kubectl config current-context  # Should show: kind-test-infra
```

### Manual Step-by-Step Setup

For learning or debugging purposes, you can run the setup steps individually:

```shell
# 1. Create test infrastructure cluster
task test-infra-cluster

# 2. Deploy Milo components
task deploy-to-test-infra
```

### Individual Commands

The following commands are available for specific tasks:

| Command | Purpose |
|---------|---------|
| `task build-milo-image` | Build the Milo container image |
| `task load-image-to-kind` | Load image into kind cluster |
| `task deploy-to-test-infra` | Deploy controller with CRDs and webhooks |
| `task test-infra-redeploy` | Quick rebuild and redeploy |
| `task test-infra-logs` | Follow controller manager logs |
| `task test-infra-setup-complete` | Full environment setup |

### Testing Your Changes

After deployment, test that everything works:

1. **Check cluster health:**
   ```shell
   kubectl get pods -A
   ```

2. **Verify webhooks are working:**
   ```shell
   # This should work without errors
   kubectl apply -f config/samples/resourcemanager/v1alpha1/organization.yaml
   ```

3. **Check controller logs:**
   ```shell
   task test-infra-logs
   ```

### Configuration

The test infrastructure uses these configurable variables (in `Taskfile.yaml`):

- `MILO_IMAGE_NAME`: Container image name (default: `ghcr.io/datum-cloud/milo`)
- `MILO_IMAGE_TAG`: Container image tag (default: `dev`)
- `TEST_INFRA_CLUSTER_NAME`: Kind cluster name (default: `test-infra`)

### Cleanup

When you're done developing, clean up the test cluster:

```shell
kind delete cluster --name test-infra
```

## Legacy Local Kind Cluster

> **NOTE:** The following approach is legacy. Use the test infrastructure cluster above instead.

Run the following task to run to create a local kind cluster
and run the controller manager in the core control plane. This is an
alternative approach to local development than using
[devcontainer](https://containers.dev).

```shell
task kind-setup-and-run
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
