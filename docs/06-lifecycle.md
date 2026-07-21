# Lifecycle Specification

## 1. Purpose

This document defines the lifecycle behavior of `SecretReference` resources and generated target Secrets.

## 2. Create

When a `SecretReference` is created, the controller MUST reconcile it.

If all sources exist, authorization succeeds, and composition is valid, the controller MUST create the target Secret.

If reconciliation fails, the controller MUST set `Ready=False` and MUST NOT create a target Secret.

If a foreign target Secret already exists, reconciliation MUST fail with `TargetAlreadyExists`.

## 3. Update

When a `SecretReference` spec changes, the controller MUST recompute the full desired target Secret.

Changes that may affect the target include:

- source list changes;
- source namespace or name changes;
- key selection changes;
- key target name changes;
- target Secret type changes.

If the new desired state is valid, the managed target Secret MUST be updated only when managed fields differ.

If the only safe way to apply the new desired state is replacement, such as changing Kubernetes Secret `type`, the controller MUST delete the owned target Secret and recreate it with the desired managed fields after deletion completes. The controller MUST NOT remove user finalizers to force replacement.

If the new desired state is invalid, the managed target Secret MUST be deleted.

If Kubernetes rejects the new desired target Secret, any existing managed target Secret MUST be deleted.

## 4. Delete

When a `SecretReference` is deleted, the owned target Secret SHOULD be removed by Kubernetes garbage collection through owner references.

The controller MAY also delete the managed target Secret during final reconciliation if it can prove ownership.

CogniSecrets MUST NOT use finalizers in V1 unless implementation constraints make them strictly necessary.

## 5. Source Secret creation

If reconciliation fails because a source Secret is missing, the controller MUST set `Ready=False` with `SourceNotFound` and delete the managed target Secret if present.

When the missing source Secret is later created, the controller MUST reconcile dependent `SecretReference` resources.

If all requirements are then satisfied, the target Secret MUST be created or updated.

## 6. Source Secret update

When a referenced source Secret changes, dependent `SecretReference` resources MUST be reconciled.

Relevant changes include:

- data changes;
- annotation changes;
- owner reference changes that affect CogniSecrets-managed detection.

CogniSecrets copies only source `data`, not source metadata or source `type`. Values are copied byte-for-byte as stored by the Kubernetes API.

## 7. Source Secret deletion

When a referenced source Secret is deleted, dependent `SecretReference` resources MUST be reconciled.

The managed target Secret MUST be deleted.

The `Ready` condition MUST become:

```text
Ready=False
Reason=SourceNotFound
```

## 8. Authorization grant

When a source Secret annotation is updated to include the target namespace, dependent `SecretReference` resources MUST be reconciled.

If all other requirements are satisfied, the target Secret MUST be created or updated.

## 9. Authorization revoke

When a source Secret annotation no longer includes the target namespace, dependent `SecretReference` resources MUST be reconciled.

The managed target Secret MUST be deleted.

The `Ready` condition MUST become:

```text
Ready=False
Reason=AccessDenied
```

## 10. Missing key

If a requested key is absent from a source Secret, reconciliation MUST fail with `SourceKeyNotFound`.

The managed target Secret MUST be deleted.

If the key later appears, reconciliation MUST recreate or update the target Secret.

## 11. Duplicate target key

If two selected source keys produce the same target key, reconciliation MUST fail with `DuplicateTargetKey`.

The managed target Secret MUST be deleted.

Source ordering MUST NOT resolve the conflict.

## 12. Target Secret external mutation

If a managed target Secret is externally modified, the next reconciliation MUST restore managed fields to the desired state.

CogniSecrets MUST preserve unmanaged fields.

If the owner reference is removed or changed so that CogniSecrets can no longer prove ownership, reconciliation MUST fail with `TargetAlreadyExists` and MUST NOT modify or delete the target Secret.

## 13. Namespace deletion

When a namespace containing `SecretReference` resources is deleted, Kubernetes garbage collection handles namespaced resources.

CogniSecrets does not define additional namespace-deletion behavior in V1.

## 14. Controller restart

After controller restart, reconciliation MUST remain correct using only Kubernetes API state.

No persisted controller-local state is required for correctness.
