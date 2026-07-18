# Error Catalog

## 1. Purpose

This document defines standard `Ready=False` reasons for `SecretReference` reconciliation failures.

Each failed reconciliation MUST report exactly one primary reason.

## 2. Condition format

Failure condition:

```yaml
type: Ready
status: "False"
reason: <Reason>
message: <Human-readable explanation>
observedGeneration: <metadata.generation>
```

Success condition:

```yaml
type: Ready
status: "True"
reason: Synced
message: Target Secret is synchronized
observedGeneration: <metadata.generation>
```

## 3. Reasons

### 3.1 `Synced`

Status: `True`

The target Secret exists and its managed fields match desired state.

### 3.2 `SourceNotFound`

Status: `False`

A referenced source Secret does not exist.

Required behavior:

- delete managed target Secret if present;
- do not create target Secret;
- include source namespace and name in the message.

### 3.3 `AccessDenied`

Status: `False`

A source Secret does not authorize the target namespace.

Required behavior:

- delete managed target Secret if present;
- do not create target Secret;
- include source namespace, source name, and target namespace in the message.

### 3.4 `SourceKeyNotFound`

Status: `False`

A requested source key is missing.

Required behavior:

- delete managed target Secret if present;
- do not create target Secret;
- include source namespace, source name, and key name in the message.

### 3.5 `DuplicateTargetKey`

Status: `False`

Two or more selected values map to the same target key.

Required behavior:

- delete managed target Secret if present;
- do not create target Secret;
- include duplicated target key in the message.

### 3.6 `TargetAlreadyExists`

Status: `False`

A target Secret with the desired name exists but is not owned by the current `SecretReference`.

Required behavior:

- do not adopt the target Secret;
- do not modify the target Secret;
- do not delete the target Secret;
- include target namespace and name in the message.

### 3.7 `ManagedSourceRejected`

Status: `False`

A referenced source Secret is managed by CogniSecrets.

Required behavior:

- delete managed target Secret if present;
- do not create target Secret;
- include source namespace and name in the message.

### 3.8 `TargetRejected`

Status: `False`

The Kubernetes API rejected the generated target Secret.

Required behavior:

- delete managed target Secret if present when ownership can be proven;
- report the Kubernetes rejection reason in the message where safe and useful.

### 3.9 `WriteFailed`

Status: `False`

The controller failed to create, update, delete, or patch a resource for an operational reason not covered by a more specific reason.

Required behavior:

- preserve correctness;
- retry according to controller-runtime behavior;
- use a more specific reason whenever possible.

If a semantic failure requires deleting a managed target Secret but deletion fails, the controller MUST report `WriteFailed` because stale managed data still exists.

## 4. Reason precedence

When multiple failures are present, the controller SHOULD report the first failure encountered in deterministic source order using this precedence:

1. `TargetAlreadyExists`
2. `SourceNotFound`
3. `ManagedSourceRejected`
4. `AccessDenied`
5. `SourceKeyNotFound`
6. `DuplicateTargetKey`
7. `TargetRejected`
8. `WriteFailed`

`TargetAlreadyExists` is evaluated first because foreign target ownership prevents safe mutation or deletion regardless of source state.

For source-related failures with the same precedence, the controller MUST report the first failure in `spec.sources` order.

## 5. Message requirements

Messages SHOULD be concise and operational.

Messages SHOULD identify the relevant object and field.

Messages MUST NOT include secret values.

## 6. Events and logs

Events and logs MAY use the same reason names.

Events and logs MUST NOT include secret values.
