# Controller Specification

## 1. Purpose

This document defines the normative controller behavior for CogniSecrets.

The controller reconciles `SecretReference` resources into ordinary Kubernetes `Secret` objects.

## 2. Controller scope

The controller watches:

- `SecretReference` resources;
- Kubernetes `Secret` resources.

The controller creates, updates, and deletes only target Secrets owned by a `SecretReference`.

The controller MUST NOT modify source Secrets.

## 3. Reconcile model

Every reconcile MUST be a full recomputation from current cluster state.

The controller MUST follow this logical sequence:

1. Read the `SecretReference`.
2. If the `SecretReference` no longer exists, rely on Kubernetes garbage collection for owned target Secrets.
3. Read the current target Secret.
4. If the target Secret exists but is not owned by the current `SecretReference`, fail with `TargetAlreadyExists`.
5. Read every referenced source Secret.
6. Validate that every source Secret authorizes the target namespace.
7. Validate that no source Secret is managed by CogniSecrets.
8. Resolve key mappings into a complete target `data` map.
9. Detect duplicate target keys.
10. Build the desired target Secret.
11. Create, update, delete, or leave unchanged according to ownership and desired state.
12. Update the `Ready` condition.

The controller MUST NOT depend on prior reconciliation state for correctness.

## 4. Watches and indexing

The controller MUST watch `SecretReference` changes directly.

The controller SHOULD maintain an index from source Secret identity to referencing `SecretReference` resources:

```text
<source namespace>/<source name> -> []SecretReference
```

When a source Secret changes, the controller SHOULD enqueue only references that mention that source.

The controller MUST also handle missed watch events by being correct on the next reconcile.

## 5. Desired target Secret

The target Secret MUST have:

```text
metadata.name      = SecretReference metadata.name
metadata.namespace = SecretReference metadata.namespace
type               = SecretReference spec.type or Opaque
data               = composed data map
```

CogniSecrets reads source Secret values from `data` and writes target Secret values to `data`. It MUST NOT assign semantics to `stringData`.

Secret data values MUST be copied byte-for-byte as stored by the Kubernetes API. The controller MUST NOT decode, normalize, transcode, trim, or assume UTF-8 content.

The target Secret MUST include:

```yaml
metadata:
  labels:
    app.kubernetes.io/managed-by: cognisecrets
  ownerReferences:
    - apiVersion: cognilabz.com/v1alpha1
      kind: SecretReference
      name: <SecretReference name>
      uid: <SecretReference uid>
      controller: true
```

`blockOwnerDeletion` is optional in V1 and MUST NOT be required for ownership detection.

## 6. Ownership check

Before updating or deleting an existing target Secret, the controller MUST verify that the Secret has a controller owner reference pointing to the current `SecretReference` UID.

The label `app.kubernetes.io/managed-by: cognisecrets` is not sufficient proof of ownership.

If the target Secret exists but is not owned by the current `SecretReference`, reconciliation MUST fail with `TargetAlreadyExists`.

The controller MUST NOT adopt the existing Secret.

## 7. Managed fields

CogniSecrets manages only:

- `data`;
- `type`;
- its own controller owner reference;
- `app.kubernetes.io/managed-by: cognisecrets`.

The controller MUST preserve:

- unrelated labels;
- annotations;
- finalizers;
- unrelated owner references;
- Kubernetes-managed metadata.

When comparing current and desired state, the controller MUST compare only managed fields.

If the target Secret and status condition already match the desired state, including `observedGeneration`, the controller SHOULD avoid writing either resource.

## 8. Write behavior

The controller MUST create the target Secret when:

- desired state is valid;
- authorization succeeds;
- no target Secret exists.

The controller MUST update the target Secret when:

- desired state is valid;
- authorization succeeds;
- the target Secret is owned by the current `SecretReference`;
- at least one managed field differs.

The controller MUST avoid writes when managed fields are already equal.

## 9. Fail-closed deletion

If desired state cannot be computed safely, the controller MUST delete the managed target Secret when it exists and is owned by the current `SecretReference`.

Examples:

- missing source Secret;
- access denied;
- missing source key;
- duplicate target key;
- source Secret is managed by CogniSecrets;
- Kubernetes API rejects the generated target Secret.

The controller MUST NOT delete a foreign target Secret.

If fail-closed deletion is required but the delete operation fails, reconciliation MUST report `WriteFailed` rather than the original semantic reason because stale managed data still exists.

## 10. Status behavior

The controller MUST update the `Ready` condition after reconciliation.

Successful synchronization:

```text
Ready=True
Reason=Synced
```

Failure:

```text
Ready=False
Reason=<error reason>
```

The condition message SHOULD explain the failing source, key, target key, or target Secret where applicable.

## 11. Events

The controller SHOULD emit Kubernetes events for significant transitions:

- target Secret created;
- target Secret updated;
- managed target Secret deleted because reconciliation failed;
- reconciliation failure.

Events are diagnostic only. Correctness MUST NOT depend on event delivery.

## 12. Concurrency and idempotence

Reconciliation MUST be safe under repeated, concurrent, and delayed execution.

The controller SHOULD use Kubernetes optimistic concurrency as provided by the client library.

If an update conflict occurs, the controller SHOULD retry by requeueing.

## 13. RBAC requirements

The controller requires permissions to:

- get, list, and watch `SecretReference` resources;
- update `SecretReference` status;
- get, list, and watch Kubernetes Secrets;
- create, update, patch, and delete target Secrets;
- create Kubernetes events.

The controller requires broad Secret read/list/watch permissions for the namespaces it manages. In the default V1 deployment, this is cluster-wide because source Secrets may live in any namespace and revocation must be observed promptly.

The controller MUST be implemented so that it never needs to read resources outside these requirements.

## 14. Explicitly forbidden behavior

The controller MUST NOT:

- decrypt `SealedSecret` resources;
- read External Secrets provider backends;
- mutate source Secrets;
- adopt existing target Secrets;
- overwrite foreign target Secrets;
- retain stale managed target Secrets after authorization or source failure;
- use source ordering as overwrite precedence;
- use CogniSecrets-managed Secrets as sources.
