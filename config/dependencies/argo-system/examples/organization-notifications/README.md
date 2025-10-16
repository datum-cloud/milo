# Organization Slack Notifications

Sends Slack notifications when Organizations are created in Milo using Argo
Events.

## Components

1. **EventSource** ([eventsource.yaml](eventsource.yaml)): Watches Milo
   Organizations via kubeconfig
2. **Sensor** ([sensor.yaml](sensor.yaml)): Sends Slack notifications when
   Organizations are created
3. **Kubeconfig Secret**
   ([eventsource-milo-kubeconfig.yaml](eventsource-milo-kubeconfig.yaml)):
   Credentials for EventSource pods to access Milo API

## How It Works

EventSource pods (created by Argo Events controller) watch Milo's Organizations
using a mounted kubeconfig. When a new Organization is created, an event flows
through the EventBus to the Sensor, which formats and sends a Slack Block Kit
message.

## Setup

### 1. Configure Slack Webhook

Edit the Slack webhook URL directly in [sensor.yaml](sensor.yaml):

```yaml
http:
  url: "https://hooks.slack.com/services/YOUR/ACTUAL/WEBHOOK"
```

Create your webhook URL at [Slack App Directory](https://api.slack.com/apps) →
Enable "Incoming Webhooks" → Add to workspace.

### 2. Deploy

```bash
task test-infra:kubectl -- apply -k config/dependencies/argo-system/examples/organization-notifications/
```

### 3. Verify

```bash
task test-infra:kubectl -- get eventsources,sensors -n milo-system
task test-infra:kubectl -- logs -n milo-system -l eventsource-name=milo-organization-events
```

## Testing

```bash
task kubectl -- apply -f - <<EOF
apiVersion: resourcemanager.miloapis.com/v1alpha1
kind: Organization
metadata:
  name: test-notification
spec:
  type: Standard
  displayName: "Test Organization"
EOF
```

You should receive a Slack notification with the organization details formatted
using [Block Kit](https://api.slack.com/block-kit).

## Customization

### Filter by Labels

Add a filter to the EventSource to only notify for specific organizations:

```yaml
spec:
  resource:
    milo-organizations:
      filter:
        labels:
          - key: environment
            operation: "=="
            value: production
```

### Modify Message

Edit the `dataTemplate` in [sensor.yaml](sensor.yaml) to customize the Slack
Block Kit message format and fields.

## Troubleshooting

```bash
# Check EventSource logs
task test-infra:kubectl -- logs -n milo-system -l eventsource-name=milo-organization-events

# Check Sensor logs
task test-infra:kubectl -- logs -n milo-system -l sensor-name=milo-organization-slack-notifier

# Verify EventBus
task test-infra:kubectl -- get eventbus -n milo-system
```

## Related Documentation

- [Argo Events Documentation](https://argoproj.github.io/argo-events/)
- [Resource
  EventSource](https://argoproj.github.io/argo-events/eventsources/resource/)
- [Slack Block Kit](https://api.slack.com/block-kit)
