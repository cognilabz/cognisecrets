# API Specification

## 1. Purpose

This document defines the normative API specification for CogniSecrets.

CogniSecrets exposes one custom resource:

```text
SecretReference
```

The `SecretReference` resource describes one target Kubernetes `Secret` that is composed from one or more existing source Kubernetes `Secret` objects.

## 2. API group and version

Initial API group:

```text
cognilabz.com
```

Initial version:

```text
v1
```

The first implementation MUST use:

```yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
```

The `v1` version is the release API version for CogniSecrets.

Releases MUST preserve compatible `v1` behavior unless release notes explicitly document an incompatible change and migration path.

## 3. Resource identity

A `SecretReference` is namespaced.

The generated target Secret identity is derived from the `SecretReference` identity:

```text
target namespace = SecretReference metadata.namespace
target name      = SecretReference metadata.name
```

The API MUST NOT support a separate target namespace or target name in V1.

## 4. Resource schema

Canonical shape:

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
status:
  conditions:
    - type: Ready
      status: "True"
      reason: Synced
      message: Target Secret is synchronized
      observedGeneration: 1
```

## 5. Spec fields

### 5.1 `spec.type`

Type: `string`

Required: no

API default: `Opaque`

`spec.type` is copied to the generated Secret `type` field. The CRD MUST default an omitted `spec.type` to `Opaque`.

CogniSecrets MUST NOT reproduce Kubernetes validation for built-in Secret types. The Kubernetes API server remains authoritative. If the generated Secret is rejected, reconciliation fails with `TargetRejected`.

### 5.2 `spec.sources`

Type: list of `SecretSource`

Required: yes

Minimum items: `1`

`spec.sources` defines the source Secrets used to compose the target Secret.

The list form is mandatory even when exactly one source is used. The API MUST NOT support shorthand forms such as `source`, `namespace/name`, or string references.

### 5.3 `spec.sources[].namespace`

Type: `string`

Required: no

The namespace of the source Secret.

If omitted, the controller resolves the source namespace to the `SecretReference` namespace.

The value MUST be a valid Kubernetes namespace name.

### 5.4 `spec.sources[].name`

Type: `string`

Required: yes

The name of the source Secret.

The value MUST be a valid Kubernetes Secret name.

### 5.5 `spec.sources[].keys`

Type: list of `SecretKeyMapping`

Required: no

Minimum items when present: `1`

If omitted, all keys from the source Secret are copied.

If present, only the listed keys are copied.

### 5.6 `spec.sources[].keys[].name`

Type: `string`

Required: yes

The key name in the source Secret `data` map.

### 5.7 `spec.sources[].keys[].target`

Type: `string`

Required: no

Controller resolution: value of `name` when omitted

The key name in the generated target Secret.

A source key MAY be copied to multiple distinct target keys. Two mappings MUST NOT produce the same target key.

The CRD SHOULD NOT attempt to default `target` from `name`. The controller MUST resolve an omitted `target` as though it were equal to `name` during composition.

## 6. Status fields

`SecretReference` exposes a small status model based on Kubernetes conditions.

### 6.1 `status.conditions`

Type: list of conditions

CogniSecrets owns only this condition type:

```text
Ready
```

The controller MUST preserve condition types written by other actors when updating the `Ready` condition, but it MUST NOT depend on them.

### 6.2 Ready condition

Successful reconciliation:

```yaml
type: Ready
status: "True"
reason: Synced
message: Target Secret is synchronized
```

Failed reconciliation:

```yaml
type: Ready
status: "False"
reason: <precise reason>
message: <human-readable explanation>
```

`observedGeneration` MUST be set to the `metadata.generation` value that was reconciled, including failed reconciliations and operational write failures.

## 7. Source authorization annotation

Source authorization is declared on the source Secret:

```yaml
metadata:
  annotations:
    cognisecrets.cognilabz.com/allowed-namespaces: "application,reporting"
```

Rules:

- The annotation is required.
- Values are comma-separated namespace names.
- Whitespace around entries is ignored.
- Empty entries are ignored.
- Duplicate entries are ignored.
- Invalid namespace names are ignored for matching.
- Namespace comparison is exact after parsing.
- `*` is not supported in V1.
- The source namespace is not implicitly authorized.

Authorization through this annotation assumes cluster RBAC protects source Secret updates. Any actor that can update a source Secret annotation can grant access to that Secret through CogniSecrets.

If the target namespace is not listed, reconciliation fails with `AccessDenied`.

## 8. Validation split

The CRD SHOULD validate:

- required fields;
- object types;
- minimum list sizes;
- Kubernetes name formats where practical.

The CRD MUST reject unknown fields under `spec`.

The controller MUST validate:

- source Secret existence;
- source authorization;
- requested key existence;
- duplicate target keys;
- managed target ownership;
- prohibition against using CogniSecrets-managed Secrets as sources;
- generated Secret acceptance by the Kubernetes API.

## 9. Examples

### 9.1 Copy all keys from one source

```yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: database
  namespace: application
spec:
  sources:
    - namespace: shared
      name: database
```

### 9.2 Select and rename keys

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
          target: DB_USERNAME
        - name: password
          target: DB_PASSWORD
```

### 9.3 Compose multiple sources

```yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: application-config
  namespace: application
spec:
  sources:
    - namespace: shared
      name: database
    - namespace: shared
      name: messaging
```

## 10. Explicitly unsupported in V1

The V1 API MUST NOT include:

- `SecretGrant`;
- target namespace override;
- target name override;
- wildcard authorization;
- deny rules;
- key-level authorization;
- optional stale-secret retention;
- template rendering;
- generated values;
- ConfigMap support;
- references to SealedSecret resources;
- references to ExternalSecret resources;
- shorthand source syntax.
