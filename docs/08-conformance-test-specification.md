# Conformance Test Specification

## 1. Purpose

This document defines the conformance test categories required for CogniSecrets compatibility.

The conformance suite is the executable form of the API, controller, lifecycle, security, and error specifications.

The E2E execution model for this suite is defined separately in `10-e2e-test-concept.md`.

## 2. Test style

Each conformance case SHOULD be written in this form:

```text
Given <cluster state>
When <operation or reconcile trigger>
Then <target Secret and SecretReference status expectations>
```

Tests MUST assert both target Secret behavior and `Ready` condition behavior.

Tests MUST NOT assert implementation-private details.

## 3. API validation tests

Required cases:

- reject `SecretReference` without `spec.sources`;
- reject empty `spec.sources`;
- reject source without `namespace`;
- reject source without `name`;
- reject empty `keys` list when present;
- reject key mapping without `name`;
- default omitted `spec.type` to `Opaque`;
- default omitted `keys[].target` to `keys[].name`;
- reject unknown fields when CRD pruning or validation supports it.

## 4. Basic synchronization tests

Required cases:

- copy all keys from one authorized source;
- copy selected keys from one authorized source;
- rename selected keys;
- copy one source key to multiple distinct target keys;
- compose keys from multiple authorized sources;
- update target Secret when source data changes;
- avoid updating target Secret when managed fields are unchanged.

## 5. Authorization tests

Required cases:

- deny when annotation is missing;
- deny when annotation is empty;
- allow when target namespace is listed;
- deny when target namespace is not listed;
- deny same-namespace access without explicit listing;
- trim whitespace around annotation entries;
- ignore empty annotation entries;
- ignore duplicate annotation entries;
- reject or deny wildcard authorization in V1.

## 6. Fail-closed tests

Required cases:

- delete managed target when source Secret is deleted;
- delete managed target when authorization is revoked;
- delete managed target when requested key is removed;
- delete managed target when duplicate target key appears;
- recreate target when failure is fixed;
- never delete a foreign target Secret during failure.

## 7. Ownership tests

Required cases:

- create target Secret with controller owner reference;
- create target Secret with managed-by label;
- update owned target Secret;
- preserve unrelated labels;
- preserve annotations;
- preserve finalizers;
- preserve unrelated owner references;
- fail with `TargetAlreadyExists` when target exists without matching owner reference;
- fail with `TargetAlreadyExists` when target has managed-by label but wrong owner;
- do not adopt foreign target Secret.

## 8. Chain prevention tests

Required cases:

- reject source Secret with `app.kubernetes.io/managed-by: cognisecrets`;
- delete managed target when a source becomes CogniSecrets-managed;
- report `ManagedSourceRejected`.

## 9. Error reason tests

Required cases:

- `SourceNotFound`;
- `AccessDenied`;
- `SourceKeyNotFound`;
- `DuplicateTargetKey`;
- `TargetAlreadyExists`;
- `ManagedSourceRejected`;
- `TargetRejected` where feasible;
- `Synced` after recovery.

Each test MUST assert that secret values are not present in status messages or events.

## 10. Lifecycle tests

Required cases:

- create `SecretReference` before source Secret exists;
- create source Secret after failed reference;
- update `SecretReference` source list;
- update `SecretReference` key mappings;
- update `SecretReference` type;
- delete `SecretReference` and verify target cleanup through owner reference or controller action;
- restart controller and verify reconciliation from Kubernetes state.

## 11. Watch behavior tests

Required cases:

- source Secret data update enqueues dependent `SecretReference`;
- source Secret annotation update enqueues dependent `SecretReference`;
- unrelated Secret update does not require dependent target changes;
- multiple references to one source are reconciled after source update.

Conformance MUST test behavior, not a specific indexing implementation.

## 12. Multi-source conflict tests

Required cases:

- duplicate target key from two sources fails;
- duplicate target key within one source mapping fails;
- source order does not create overwrite precedence;
- conflict recovery creates correct target after target names become unique.

## 13. Kubernetes integration tests

Required cases:

- generated target Secret is accepted for `Opaque`;
- Kubernetes API rejection is surfaced as `TargetRejected` where reproducible;
- resource version does not change after no-op reconcile;
- target Secret remains valid after unrelated metadata mutation.

## 14. Minimum release gate

Before a production release, every normative MUST in the documentation SHOULD be covered by at least one conformance test.

Known uncovered MUST statements MUST be documented in release notes.

The implementation-independent E2E suite MUST pass against a fresh local `kind` cluster before a release candidate is considered conformant.
