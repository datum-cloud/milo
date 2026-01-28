# Test: `user-contact-sync`

End-to-end tests for the UserContactController.

This test verifies the following scenarios:
1. Creating a new User creates a corresponding Contact with SubjectRef
2. Creating a User with an existing newsletter Contact (same email) links them together
3. When a User changes email, their linked Contact is updated
4. When a User changes email to match an unlinked Contact, the unlinked Contact is deleted
5. When a User is deleted, the Contact's SubjectRef is cleaned up


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-new-user-creates-contact](#step-create-new-user-creates-contact) | 0 | 4 | 0 | 0 | 0 |
| 2 | [setup-newsletter-contact](#step-setup-newsletter-contact) | 0 | 2 | 0 | 0 | 0 |
| 3 | [create-user-links-existing-contact](#step-create-user-links-existing-contact) | 0 | 4 | 0 | 0 | 0 |
| 4 | [update-user-email-syncs-contact](#step-update-user-email-syncs-contact) | 0 | 3 | 0 | 0 | 0 |
| 5 | [setup-unlinked-contact-for-collision](#step-setup-unlinked-contact-for-collision) | 0 | 2 | 0 | 0 | 0 |
| 6 | [update-user-email-deletes-unlinked-contact](#step-update-user-email-deletes-unlinked-contact) | 0 | 3 | 0 | 0 | 0 |
| 7 | [delete-user-cleans-contact-subjectref](#step-delete-user-cleans-contact-subjectref) | 0 | 3 | 0 | 0 | 0 |
| 8 | [cleanup](#step-cleanup) | 0 | 2 | 0 | 0 | 0 |

### Step: `create-new-user-creates-contact`

Create a new User and verify a corresponding Contact is created

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `assert` | 0 | 0 | *No description* |

### Step: `setup-newsletter-contact`

Create a newsletter Contact without a SubjectRef

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `create-user-links-existing-contact`

Create a User with the same email as the newsletter Contact

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `assert` | 0 | 0 | *No description* |

### Step: `update-user-email-syncs-contact`

Update User email and verify Contact is synced

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

### Step: `setup-unlinked-contact-for-collision`

Create an unlinked Contact that will collide with user email change

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `update-user-email-deletes-unlinked-contact`

Update User email to match unlinked Contact and verify it is deleted

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `error` | 0 | 0 | *No description* |

### Step: `delete-user-cleans-contact-subjectref`

Delete User and verify Contact's SubjectRef is removed

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |
| 2 | `error` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `cleanup`

Clean up all test resources

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |
| 2 | `script` | 0 | 0 | *No description* |

---

