# Test: `user-deletion-garbage-collection`

Validates that User resource deletion properly triggers garbage collection of associated
PolicyBinding and UserPreference resources. Additionally, as of the current controller logic, the UserController now adds a `iam.miloapis.com/user-membership-cleanup` finalizer to every User. When the User is deleted, the controller explicitly finds and deletes all OrganizationMembership resources referencing that User before completing User deletion. This ensures all related OrganizationMemberships are removed alongside the User, instead of relying solely on Kubernetes garbage collection via owner references.

## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-user](#step-create-user) | 0 | 5 | 0 | 0 | 0 |
| 2 | [delete-user](#step-delete-user) | 0 | 5 | 0 | 0 | 0 |

### Step: `create-user`

Create a User resource and verify webhook creates associated resources

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |

### Step: `delete-user`

Delete the User resource and verify associated resources are garbage collected or explicitly deleted by the controller

- When the User resource is deleted, the UserController first removes all referenced OrganizationMembership resources via the `user-membership-cleanup` finalizer.
- PolicyBinding and UserPreference resources with ownerReferences pointing to the User are cleaned up by Kubernetes garbage collection as before.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |
| 2 | `error` | 0 | 0 | *No description* |
| 3 | `error` | 0 | 0 | *No description* |
| 4 | `error` | 0 | 0 | *No description* |
| 5 | `error` | 0 | 0 | *No description* |

---
