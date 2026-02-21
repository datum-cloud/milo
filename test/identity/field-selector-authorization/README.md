# Field Selector Authorization Tests

This test suite validates that field selector authorization works correctly for
UserIdentities and Sessions resources in the Identity API.

## Test Scenarios

### 1. Regular User - Self-Scoped Access (Default Behavior)
**Given:** A regular user without staff privileges
**When:** User lists useridentities without field selector
**Then:** User sees only their own identity provider links

**When:** User attempts to use field selector for another user
**Then:** Request is rejected with 403 Forbidden error

### 2. Staff User - Cross-User Access with Field Selector
**Given:** A user in the staff-users group
**When:** User lists useridentities with field selector for another user
**Then:** User successfully retrieves the target user's identity provider links

### 3. Field Selector Validation
**Given:** Any authenticated user
**When:** User provides invalid field selector (e.g., metadata.name)
**Then:** Request is rejected with appropriate error message

## Authorization Model

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Milo RBAC Check                                          │
│    - PolicyBinding grants access to useridentities resource │
│    - Required for both regular and staff users              │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Field Selector Passed to Backend                         │
│    - Milo passes field selector to auth-provider-zitadel    │
│    - No validation at Milo layer                            │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Backend Authorization (auth-provider-zitadel)            │
│    - If no field selector: use authenticated user's UID     │
│    - If field selector with different UID:                  │
│      → Check user groups (staff-users, fraud-manager)       │
│      → Allow if staff, deny if not                          │
└─────────────────────────────────────────────────────────────┘
```

## Required Setup

### PolicyBindings
```yaml
# Grant staff-users access to useridentities
apiVersion: iam.miloapis.com/v1alpha1
kind: PolicyBinding
metadata:
  name: staff-useridentities-viewer
  namespace: milo-system
spec:
  resourceSelector:
    resourceKind:
      apiGroup: identity.miloapis.com
      kind: UserIdentity
  roleRef:
    name: identity-user-session-viewer
    namespace: milo-system
  subjects:
  - kind: Group
    name: staff-users
    namespace: milo-system
    uid: <staff-users-group-uid>
```

### Zitadel Configuration
- Create group: `staff-users`
- Assign users to group via project roles
- Configure JWT claims to include groups

## Manual Testing

### Test 1: Regular User Cannot Use Field Selector
```bash
# As regular user
kubectl get useridentities --field-selector=status.userUID=<other-user-id>

# Expected: 403 Forbidden
# Error: "only staff users can query other users' identities"
```

### Test 2: Staff User Can Use Field Selector
```bash
# As staff user (member of staff-users group)
kubectl get useridentities --field-selector=status.userUID=<target-user-id>

# Expected: 200 OK
# Response: List of target user's identity provider links
```

### Test 3: Regular User Can See Own Data
```bash
# As regular user
kubectl get useridentities

# Expected: 200 OK
# Response: List of own identity provider links
```

## Security Considerations

1. **Defense in Depth**: Two layers of authorization (Milo RBAC + Backend groups)
2. **Audit Logging**: All requests logged with user context
3. **Principle of Least Privilege**: Regular users cannot access others' data
4. **Explicit Deny**: Field selector attempts by non-staff users are explicitly denied

## Future Enhancements

- [ ] Add automated E2E tests using Chainsaw
- [ ] Add rate limiting for staff user queries
- [ ] Add metrics for field selector usage
- [ ] Consider adding SubjectAccessReview checks in Milo layer
