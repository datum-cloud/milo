# Test: `user-deletion-garbage-collection`

Validates that User resource deletion properly triggers garbage collection of associated
PolicyBinding and UserPreference resources. This test ensures the webhook creates resources
with correct owner references and the controller adds them post-creation, allowing
Kubernetes garbage collector to clean them up when the User is deleted.


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
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |

### Step: `delete-user`

Delete the User resource and verify associated resources are garbage collected

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |
| 2 | `error` | 0 | 0 | *No description* |
| 3 | `error` | 0 | 0 | *No description* |
| 4 | `error` | 0 | 0 | *No description* |
| 5 | `error` | 0 | 0 | *No description* |

---

