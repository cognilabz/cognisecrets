# E2E Conformance Coverage

This checklist maps `docs/08-conformance-test-specification.md` to the current black-box E2E suite.

Legend:

- `[x]` covered by `test/e2e/run.sh`
- `[u]` covered by reference implementation unit tests only
- `[ ]` not covered yet

## API Validation

- `[ ]` reject `SecretReference` without `spec.sources`
- `[ ]` reject empty `spec.sources`
- `[ ]` reject source without `namespace`
- `[ ]` reject source without `name`
- `[ ]` reject empty `keys` list when present
- `[ ]` reject key mapping without `name`
- `[x]` default omitted `spec.type` to `Opaque` at the API level
- `[x]` resolve omitted `keys[].target` as `keys[].name` during controller composition
- `[ ]` reject unknown fields under `spec`

## Basic Synchronization

- `[x]` copy all keys from one authorized source
- `[x]` copy selected keys from one authorized source
- `[x]` rename selected keys
- `[x]` copy one source key to multiple distinct target keys
- `[x]` compose keys from multiple authorized sources
- `[x]` update target Secret when source data changes
- `[u]` copy source `data` values byte-for-byte without UTF-8 assumptions
- `[x]` ignore source `type` during composition
- `[x]` avoid updating target Secret when managed fields are unchanged
- `[x]` avoid updating status when the `Ready` condition is unchanged, including `observedGeneration`

## Authorization

- `[x]` deny when annotation is missing
- `[x]` deny when annotation is empty
- `[x]` allow when target namespace is listed
- `[x]` deny when target namespace is not listed
- `[x]` deny same-namespace access without explicit listing
- `[x]` trim whitespace around annotation entries
- `[x]` ignore empty annotation entries
- `[x]` ignore duplicate annotation entries
- `[x]` ignore invalid namespace names for authorization matching
- `[x]` reject or deny wildcard authorization in V1

## Fail Closed

- `[x]` delete managed target when source Secret is deleted
- `[x]` delete managed target when authorization is revoked
- `[x]` delete managed target when requested key is removed
- `[x]` delete managed target when duplicate target key appears
- `[x]` recreate target when failure is fixed
- `[x]` never delete a foreign target Secret during failure

## Ownership

- `[x]` create target Secret with controller owner reference
- `[x]` create target Secret with managed-by label
- `[x]` update owned target Secret
- `[x]` preserve unrelated labels
- `[x]` preserve annotations
- `[x]` preserve finalizers
- `[x]` preserve unrelated owner references
- `[x]` fail with `TargetAlreadyExists` when target exists without matching owner reference
- `[x]` fail with `TargetAlreadyExists` when target has managed-by label but wrong owner
- `[x]` do not adopt foreign target Secret

## Chain Prevention

- `[x]` reject source Secret with a controller owner reference pointing to a `SecretReference`
- `[x]` delete managed target when a source becomes CogniSecrets-managed
- `[x]` report `ManagedSourceRejected`

## Error Reasons

- `[x]` `SourceNotFound`
- `[x]` `AccessDenied`
- `[x]` `SourceKeyNotFound`
- `[x]` `DuplicateTargetKey`
- `[x]` `TargetAlreadyExists`
- `[x]` `ManagedSourceRejected`
- `[x]` `TargetRejected` where feasible
- `[u]` `WriteFailed` when fail-closed deletion fails
- `[x]` `Synced` after recovery

Every E2E scenario that asserts status messages must also ensure secret values are not present in status messages.

## Lifecycle

- `[x]` create `SecretReference` before source Secret exists
- `[x]` create source Secret after failed reference
- `[x]` update `SecretReference` source list
- `[x]` update `SecretReference` key mappings
- `[x]` update `SecretReference` type
- `[x]` delete `SecretReference` and verify target cleanup through owner reference or controller action
- `[x]` restart controller and verify reconciliation from Kubernetes state

## Watch Behavior

- `[x]` source Secret data update enqueues dependent `SecretReference`
- `[x]` source Secret annotation update enqueues dependent `SecretReference`
- `[x]` unrelated Secret update does not require dependent target changes
- `[x]` multiple references to one source are reconciled after source update

## Multi-Source Conflicts

- `[x]` duplicate target key from two sources fails
- `[x]` duplicate target key within one source mapping fails
- `[x]` source order does not create overwrite precedence
- `[x]` conflict recovery creates correct target after target names become unique

## Kubernetes Integration

- `[x]` generated target Secret is accepted for `Opaque`
- `[x]` Kubernetes API rejection is surfaced as `TargetRejected` where reproducible
- `[x]` resource version does not change after no-op reconcile
- `[x]` target Secret remains valid after unrelated metadata mutation

## Remaining Known Gaps

- `WriteFailed` is operational-failure behavior and remains unit-test-only.
- Arbitrary-byte data copying is unit-tested; E2E currently uses printable literal values.
