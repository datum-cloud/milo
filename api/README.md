# Datum IAM API

## Open questions

## Standards

- Follow AIP rules as closely as feasible.
- Default to placing services in a `<serviceName>_service.proto` file.
- Default to placing resources in a `<serviceName>_resources.proto` file.
  - Break out into more fine grained files if size grows large.
- Use [Channel Based Versioning][channel-versioning] for proto packages, API
  endpoints, and generated code.
- Messages with a `spec` field are expected to either be marked as
  [Declarative-friendly interfaces][aip-128] or represent a template structure
  that will be used to create declarative friendly entities. For example, an
  `InstanceTemplate` message will be used to create one or more `Instance`
  entities which users can interact with directly.
- Etag values **should not** change upon an update to the status field of an
  entity.

[channel-versioning]: https://cloud.google.com/apis/design/versioning#channel-based_versioning

### Exceptions

#### AIP-128

[AIP-128][aip-128] states the following:

> Services responding to a GET request **must** return the resource's current state
> (not the intended state).

This requirement would result in an extremely unintuitive and difficult API
integration experience. While we could deal with the side effects of the
difficulty internally, it would place undue burden on end users who expect
certain behaviors from RESTful APIs (read-what-you-wrote).

As a result, we WILL NOT be adhering to this constraint.

## Running the linter

`task api:lint`

## Declarative Friendly Type Creation

A helper task has been created that can be used to output protobuf for a new
declarative friendly type. Two task variables must be provided:

- `S`: Singular type name in CamelCase format.
- `P`: Plural type name in CamelCase format.
- `SN`: Service name in domain format.

Usage:

```shell
task api:generate-type S=Example P=Examples SN=example.com
```

After placing the output in your desired file, consider the following:

- Ensure the appropriate `pattern` values exist, keeping in mind
  [AIP-123][aip-123].
- If necessary, adjust the service name in the `type` option value.

[aip-123]: https://google.aip.dev/123#annotating-resource-types
[aip-128]: https://google.aip.dev/128
