# OrganizationMembership Role Migration

Automatic migration for backfilling OrganizationMembership resources with roles from legacy PolicyBindings.

## How It Works

The migration controller automatically:
1. Finds OrganizationMemberships without roles
2. Discovers legacy PolicyBindings granting organization access
3. Updates memberships with role assignments
4. Cleans up legacy PolicyBindings after verification

## Deployment

The controller runs automatically when Milo is deployed:

```bash
task dev:deploy
```

## Monitoring

Watch migration progress:

```bash
kubectl logs -n milo-system -l app.kubernetes.io/name=milo-controller-manager -f | grep migration
```

## Verification

Check memberships have roles:

```bash
kubectl get organizationmemberships --all-namespaces -o json | \
  jq -r '.items[] | select(.spec.roles != null) | "\(.metadata.namespace)/\(.metadata.name): \(.spec.roles | length) roles"'
```

Verify managed PolicyBindings created:

```bash
kubectl get policybindings --all-namespaces -l resourcemanager.miloapis.com/managed-by=organization-membership-controller
```

## Safety

- Non-destructive: Legacy bindings remain until managed replacements exist
- Idempotent: Safe to run multiple times
- Automatic: No manual intervention needed
- Observable: Detailed logging throughout

## Controller Details

- File: [internal/controllers/migration/migration_controller.go](../../internal/controllers/migration/migration_controller.go)
- Tests: [migration_controller_test.go](../../internal/controllers/migration/migration_controller_test.go)
- E2E Test: [test/migration/organization-membership-role-backfill](../../test/migration/organization-membership-role-backfill/)
