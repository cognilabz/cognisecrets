# CogniSecrets Product Overview

## 1. Purpose

CogniSecrets extends Kubernetes with one narrowly defined capability:

> Compose and synchronize a Kubernetes `Secret` from one or more existing Kubernetes `Secret` objects across namespace boundaries under explicit authorization.

CogniSecrets does not introduce a new secret storage system. Source values remain stored in ordinary Kubernetes Secrets, and the generated target is also an ordinary Kubernetes Secret.

## 2. Problem statement

Applications often require values that already exist in several Kubernetes Secrets, potentially in other namespaces. Kubernetes does not provide a native mechanism to compose these values into a new Secret while enforcing explicit authorization by the source owner.

Without CogniSecrets, users typically duplicate Secrets manually, introduce application-specific synchronization logic, or deploy a substantially larger secret-management platform for a comparatively small requirement.

CogniSecrets solves only this composition and synchronization problem.

## 3. Product scope

CogniSecrets provides:

- Composition of one target Secret from one or more source Secrets
- Cross-namespace source references
- Explicit source-side namespace authorization
- Optional key selection
- Optional key renaming
- Automatic synchronization when a referenced source Secret changes
- Deterministic conflict detection
- A minimal status condition describing reconciliation success or failure

## 4. Non-goals

CogniSecrets is not:

- A secret store or vault
- An encryption or decryption system
- A key-management system
- A secret-generation system
- A secret-rotation system
- A versioning or revision system
- A GitOps engine
- A user interface
- A command-line secret editor
- A replacement for Sealed Secrets
- A replacement for External Secrets

CogniSecrets also does not support ConfigMaps or arbitrary Kubernetes resources.

## 5. Core resource model

CogniSecrets introduces one custom resource: `SecretReference`.

A `SecretReference`:

- Exists in the namespace where the target Secret is required
- Has the same name as the target Secret
- References one or more source Secrets by `name`, with optional `namespace`
- Optionally selects and renames keys
- Produces exactly one target Secret

Example:

```yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: application-credentials
  namespace: application
spec:
  type: Opaque
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
        - name: password
          target: DB_PASSWORD
    - namespace: shared
      name: messaging
```

This resource produces:

```text
Secret/application-credentials in namespace application
```

## 6. Authorization model

Authorization is declared on each source Secret using the annotation:

```yaml
metadata:
  annotations:
    cognisecrets.cognilabz.com/allowed-namespaces: "application,reporting"
```

Rules:

- The annotation is required for every source access.
- Authorization applies to the complete source Secret, not individual keys.
- The target namespace must be explicitly listed.
- The source namespace is not implicitly authorized.
- Wildcards are not supported.
- Invalid namespace entries are ignored for matching.
- Namespace names are compared exactly after parsing the comma-separated list.

Key selection is a composition feature, not a security boundary.

This authorization model assumes Kubernetes RBAC protects source Secret updates. Any actor that can update a source Secret annotation can grant access to that source Secret through CogniSecrets.

## 7. Synchronization behavior

The controller watches both `SecretReference` and `Secret` resources.

When a referenced source Secret changes, all dependent `SecretReference` resources are reconciled using a field index. Each reconciliation computes the complete desired target Secret from the current cluster state.

The controller is:

- Event-driven
- Stateless
- Idempotent
- Deterministic

It does not use incremental key updates or persistent internal state.

## 8. Failure philosophy

CogniSecrets follows a fail-closed model:

> If the desired target Secret cannot be computed correctly or is no longer authorized, CogniSecrets removes the managed target Secret rather than leaving stale or unauthorized data available.

Examples include:

- Missing source Secret
- Missing requested source key
- Revoked namespace authorization
- Duplicate target key
- Invalid target Secret rejected by the Kubernetes API

CogniSecrets never deletes or overwrites an existing target Secret that it does not own.

## 9. Ownership and managed fields

The generated target Secret is owned by its `SecretReference` through an owner reference and includes:

```yaml
app.kubernetes.io/managed-by: cognisecrets
```

CogniSecrets manages only:

- `data`
- `type`
- Its own owner reference
- Its own managed-by label

Unrelated labels, annotations, finalizers, and other owner references are preserved.

If an existing Secret with the target name is not owned by the corresponding `SecretReference`, reconciliation fails with `TargetAlreadyExists`. The Secret is not adopted, modified, or deleted.

## 10. Composition rules

- `spec.sources` is required and contains at least one source.
- Sources are always represented as a list.
- If a source `namespace` is omitted, it resolves to the `SecretReference` namespace.
- Source Secrets of all Kubernetes Secret types are accepted.
- If `keys` is omitted, all source keys are copied.
- If `keys` is present, only listed keys are copied.
- `keys[].name` identifies the source key.
- omitted `keys[].target` resolves to `name` during composition.
- A source key may be copied to multiple distinct target keys.
- Every resulting target key must be unique.
- Source ordering never defines overwrite precedence.
- Duplicate target keys are an error.
- A Secret generated by CogniSecrets cannot be used as a source. V1 detects generated sources by `SecretReference` controller owner reference, not by label.

## 11. Target Secret type

`spec.type` is optional and defaults to `Opaque`.

When provided, the value is passed to the generated Secret unchanged. CogniSecrets does not reproduce Kubernetes Secret-type validation. The Kubernetes API remains authoritative and may reject an invalid Secret.

Source Secret `type` does not affect composition. CogniSecrets copies only source `data` values, byte-for-byte, into target `data`.

## 12. Product principles

### 12.1 Minimal API

CogniSecrets exposes only concepts required for Secret composition. It avoids aliases, convenience shortcuts, and multiple representations of the same operation.

### 12.2 Kubernetes-native behavior

CogniSecrets reuses Kubernetes concepts such as namespaces, names, Secrets, owner references, labels, events, and status conditions instead of introducing equivalent abstractions.

### 12.3 Explicit behavior

Cross-namespace authorization is explicit. Source identification is explicit. Conflicting target keys are errors rather than implicit overwrites.

### 12.4 No stale secrets

Availability never takes precedence over correctness or authorization. If the desired state cannot be produced, the managed target Secret is removed.

### 12.5 Specification before implementation

The API, lifecycle, controller behavior, security semantics, error catalog, and conformance tests are defined before production implementation begins.

## 13. Success criteria

CogniSecrets is successful when it provides a small, predictable, and independently testable Kubernetes capability whose complete behavior can be understood without operating a broader secret-management platform.
