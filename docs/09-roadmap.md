# Roadmap

## 1. Purpose

This document defines the intended development path for CogniSecrets.

The roadmap is subordinate to the product overview, design principles, and normative specifications.

## 2. Phase 0: Specification

Status: complete

Goals:

- define product scope;
- define design principles;
- define `SecretReference` API;
- define controller behavior;
- define security model;
- define lifecycle behavior;
- define error catalog;
- define conformance test requirements;
- define implementation-independent E2E test concept.

Exit criteria:

- documentation files `01` through `10` exist;
- major V1 semantics are consistent across documents;
- open questions are captured explicitly.

## 3. Phase 1: API scaffolding

Status: complete

Goals:

- create Kubernetes API types for `SecretReference`;
- generate CRD manifests;
- define status condition helpers;
- add schema validation;
- add sample manifests;
- use Go, controller-runtime, and Kubebuilder-style markers for the reference implementation.

Expected directories:

```text
api/
config/
```

Exit criteria:

- CRD can be installed in a test cluster;
- invalid resources are rejected by schema where appropriate;
- valid examples are accepted.

## 4. Phase 2: Controller implementation

Status: complete

Goals:

- implement reconcile loop;
- implement source Secret indexing;
- implement authorization parsing;
- implement composition;
- implement ownership checks;
- implement fail-closed deletion;
- implement status updates and events;
- add unit tests for package-level controller helpers.

Expected directories:

```text
controllers/
internal/
```

Exit criteria:

- basic synchronization works in envtest or a real test cluster;
- all defined error reasons are reachable;
- no-op reconciliation avoids unnecessary writes.

## 5. Phase 3: Conformance suite

Status: complete

Goals:

- implement conformance tests from `08-conformance-test-specification.md`;
- implement E2E execution model from `10-e2e-test-concept.md`;
- add reference-implementation unit tests that may import Go packages;
- run tests in CI;
- document unsupported or deferred cases.

Expected directories:

```text
test/
```

Exit criteria:

- every V1 MUST has direct or indirect test coverage;
- E2E tests pass against a fresh local `kind` cluster;
- controller restart and watch-driven synchronization are tested;
- security-sensitive behavior is covered.

Notes:

- `WriteFailed` is covered by reference-implementation unit tests only because it represents operational write or delete failures that are not portable to reproduce in a black-box conformance suite.

## 6. Phase 4: Beta release

Status: in progress

Goals:

- publish initial container image;
- publish CRD manifests;
- publish installation instructions;
- document known limitations;
- tag first beta release.

Release procedure:

- `docs/11-beta-release.md`

Exit criteria:

- users can install CogniSecrets into a test or early production evaluation cluster;
- examples work end to end;
- README clearly states beta status.

## 7. Phase 5: Hardening

Status: in progress

Goals:

- improve observability;
- review RBAC minimization;
- run larger-scale synchronization tests;
- test controller restarts and leader election;
- verify behavior under API conflicts and retries.

Exit criteria:

- documented operational guidance exists;
- known failure modes have clear diagnostics;
- conformance suite is stable in CI.

## 8. V1 excluded features

The following features are intentionally excluded from V1:

- `SecretGrant`;
- wildcard authorization;
- target name override;
- target namespace override;
- key-level authorization;
- deny rules;
- templating;
- secret generation;
- secret rotation;
- encryption or decryption;
- ConfigMap support;
- direct SealedSecret references;
- direct ExternalSecret references;
- UI or CLI secret editing.

## 9. Possible future features

Future features may be considered only if they preserve the design principles.

Candidates:

- optional `SecretGrant` resource for environments where source Secret annotations are impractical;
- additional status details for observability;
- metrics for reconciliation results;
- richer examples for Helm, Sealed Secrets, and External Secrets workflows;
- stricter CRD validation as Kubernetes CEL support permits.

These are not commitments.

## 10. Decision rule

Features that weaken explicit authorization, fail-closed behavior, deterministic composition, or narrow field ownership should be rejected even if they are convenient.
