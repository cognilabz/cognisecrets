# E2E Test Concept

## 1. Purpose

This document defines the end-to-end test concept for CogniSecrets.

The E2E suite verifies that any CogniSecrets implementation conforms to the specification when running in a real Kubernetes cluster.

The E2E tests are intentionally implementation-independent. They interact only with:

- the Kubernetes API;
- installed CogniSecrets CRDs;
- a running CogniSecrets controller;
- ordinary Kubernetes resources such as Namespaces, Secrets, Events, and Status conditions.

An implementation that conforms to the CogniSecrets specifications MUST pass the E2E suite.

## 2. Test environment

The canonical E2E environment is a local `kind` cluster.

The E2E runner MUST be able to create and destroy a dedicated cluster for a test run.

Canonical cluster lifecycle:

```text
create kind cluster
install CogniSecrets CRDs
install CogniSecrets controller under test
run E2E test suite
collect diagnostics on failure
delete kind cluster
```

The test suite MUST NOT require access to cloud services or external secret backends.

## 3. Implementation independence

The E2E suite MUST NOT depend on:

- controller source code;
- internal packages;
- internal functions;
- controller logs for correctness;
- implementation-specific metrics;
- implementation-specific labels beyond those defined by the specification;
- implementation-specific deployment tooling.

The suite MAY use logs, metrics, and diagnostics only as failure artifacts.

The implementation under test is treated as a black box installed into the cluster.

Reference-implementation unit tests are separate from E2E conformance. They MAY import Go packages and assert internal helper behavior, but they MUST NOT be required for alternative implementations to claim E2E conformance.

## 4. Installation contract

The E2E runner needs a small installation contract so different implementations can be tested.

An implementation SHOULD provide one of:

- a Kubernetes manifest path;
- a Helm chart path;
- a Kustomize path;
- a container image plus standard deployment manifests.

The E2E runner MUST only assume that, after installation:

- the `SecretReference` CRD exists;
- the controller is running;
- the controller has the RBAC required by the specification;
- `SecretReference` resources can be created through the Kubernetes API.

## 5. Test isolation

Each test case SHOULD create unique namespaces.

Namespace names SHOULD include:

- a stable suite prefix;
- the test case name or id;
- a random suffix.

Example:

```text
cs-e2e-basic-copy-a1b2c
cs-e2e-source-store-a1b2c
```

Tests MUST NOT depend on execution order.

Tests MUST clean up their resources when possible.

The suite SHOULD support parallel execution, but tests that inspect global controller behavior MAY run serially.

## 6. Test resource model

Tests create only ordinary Kubernetes resources unless the specification requires otherwise.

Common resources:

- source namespaces;
- target namespaces;
- source Secrets;
- `SecretReference` resources;
- foreign target Secrets;
- managed target Secrets produced by the controller.

Tests MUST use real Kubernetes Secret objects and real base64-encoded Secret data as accepted by the Kubernetes API.

Tests MUST never print decoded secret values in logs, assertion messages, events, or failure summaries.

Tests SHOULD include arbitrary-byte Secret data to verify byte-for-byte copying without UTF-8 assumptions.

## 7. Assertion model

Each E2E test MUST assert externally observable behavior.

Required assertion types:

- target Secret existence or absence;
- target Secret `data`;
- target Secret `type`;
- target Secret owner references;
- target Secret managed-by label;
- preservation of unmanaged metadata;
- `SecretReference` `Ready` condition;
- `Ready` reason;
- absence of secret values in status messages;
- Kubernetes API rejection for invalid resources where applicable.

Tests SHOULD wait for eventual consistency using polling with timeouts.

Tests MUST NOT use fixed sleeps as the primary synchronization mechanism.

## 8. Waiting and timeouts

The suite SHOULD use bounded polling for reconciliation results.

Recommended defaults:

```text
poll interval: 250ms to 1s
timeout:       30s per reconciliation assertion
```

Longer timeouts MAY be used for controller startup or image pull.

Timeout failures SHOULD collect diagnostics before failing the test.

## 9. Diagnostics

On failure, the suite SHOULD collect:

- failing test name;
- relevant namespaces;
- `SecretReference` YAML;
- target Secret metadata and key names;
- source Secret metadata and key names;
- controller Pod status;
- controller Deployment status;
- relevant Events;
- controller logs when available.

Diagnostics MUST NOT include decoded secret values.

When Secret data must be referenced in diagnostics, tests SHOULD print only key names, byte lengths, or hashes.

## 10. Test categories

The E2E suite MUST cover every category from `08-conformance-test-specification.md`.

At minimum, E2E categories are:

- API validation;
- basic synchronization;
- authorization;
- fail-closed behavior;
- ownership;
- chain prevention;
- error reasons;
- lifecycle;
- watch-driven synchronization;
- multi-source conflicts;
- Kubernetes integration.

## 11. E2E scenarios

### 11.1 Basic all-key copy

Given an authorized source Secret with multiple keys.

When a `SecretReference` points to the source without `keys`.

Then the target Secret exists with all source keys, `Ready=True`, and `Reason=Synced`.

### 11.2 Selected key copy

Given an authorized source Secret with multiple keys.

When a `SecretReference` lists one key.

Then the target Secret contains only that key.

### 11.3 Key rename

Given an authorized source Secret with key `password`.

When the mapping targets `DB_PASSWORD`.

Then the target Secret contains `DB_PASSWORD` and does not contain `password` unless separately mapped.

### 11.4 Multi-source composition

Given two authorized source Secrets.

When a `SecretReference` references both.

Then the target Secret contains the union of selected keys.

### 11.5 Source data update

Given a synchronized target Secret.

When source Secret data changes.

Then the target Secret is updated without changing unmanaged metadata.

### 11.6 Authorization revoke

Given a synchronized target Secret.

When the source authorization annotation no longer includes the target namespace.

Then the managed target Secret is deleted and `Ready=False` with `Reason=AccessDenied`.

### 11.7 Missing source recovery

Given a `SecretReference` that points to a missing source Secret.

When the source Secret is later created with valid authorization.

Then the target Secret is created and `Ready=True`.

### 11.8 Missing key recovery

Given a synchronized target Secret.

When a requested source key is removed.

Then the target Secret is deleted and `Ready=False` with `Reason=SourceKeyNotFound`.

When the key is restored.

Then the target Secret is recreated.

### 11.9 Duplicate target key

Given two selected source values that map to the same target key.

When reconciliation runs.

Then the target Secret is absent and `Ready=False` with `Reason=DuplicateTargetKey`.

### 11.10 Foreign target Secret conflict

Given a target Secret with the same name as the `SecretReference` but no matching owner reference.

When reconciliation runs.

Then CogniSecrets does not modify or delete the foreign Secret and reports `TargetAlreadyExists`.

### 11.11 Managed target external mutation

Given a synchronized managed target Secret.

When managed fields are externally changed.

Then the next reconciliation restores the specified `data`, `type`, owner reference, and managed-by label.

### 11.12 Unmanaged metadata preservation

Given a synchronized managed target Secret with unrelated labels, annotations, finalizers, and owner references.

When reconciliation runs after a source change.

Then unrelated metadata remains present.

### 11.13 Chain prevention

Given a source Secret with a controller owner reference pointing to a `SecretReference`.

When a `SecretReference` uses it as a source.

Then the target Secret is absent and `Ready=False` with `Reason=ManagedSourceRejected`.

### 11.14 Same-namespace explicit authorization

Given a source Secret in the same namespace as the `SecretReference`.

When the authorization annotation does not list that namespace.

Then access is denied.

When the annotation explicitly lists that namespace.

Then synchronization succeeds.

### 11.15 No-op reconciliation

Given a synchronized target Secret.

When reconciliation is triggered without changing desired managed fields.

Then the target Secret resource version SHOULD remain unchanged.

## 12. Negative API scenarios

The E2E suite SHOULD apply invalid manifests through the Kubernetes API and assert rejection for structural validation cases.

Required examples:

- missing `spec.sources`;
- empty `spec.sources`;
- source missing `namespace`;
- source missing `name`;
- key mapping missing `name`.

If a validation cannot be expressed in the CRD and is intentionally controller-side, the E2E suite MUST assert the corresponding `Ready=False` condition instead.

## 13. Release gate

The E2E suite is a release gate.

A release candidate MUST NOT be considered conformant unless:

- the E2E suite passes against a fresh `kind` cluster;
- no test depends on implementation internals;
- every externally observable normative MUST in the specification is covered by E2E conformance;
- internal reference implementation behavior is covered by unit tests where direct package-level testing is useful;
- uncovered normative behavior is explicitly documented.

## 14. Suggested repository layout

The implementation phase SHOULD add:

```text
test/
  unit/
  e2e/
    README.md
    suite/
    manifests/
    scripts/
```

Suggested responsibilities:

- `test/unit/`: reference-implementation tests that may import Go packages;
- `test/e2e/README.md`: how to run the suite;
- `test/e2e/suite/`: test code;
- `test/e2e/manifests/`: reusable Kubernetes fixtures;
- `test/e2e/scripts/`: kind cluster setup and teardown.

The reference implementation is expected to use Go, controller-runtime, and Kubebuilder-style CRD generation.

The exact E2E test language is not specified. Go, shell plus `kubectl`, or another runner may be used as long as the suite remains black-box and implementation-independent.

## 15. Future compatibility

When new features are added, their externally observable behavior MUST be added to the E2E suite before the feature is considered complete.

The E2E suite should remain usable by alternative implementations that claim CogniSecrets compatibility.
