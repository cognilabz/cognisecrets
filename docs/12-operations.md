# Operations

This document describes the operational checks and diagnostics expected for beta CogniSecrets releases.

## 1. Install Verification

Install the rendered manifest:

```sh
make render IMG=ghcr.io/cognilabz/cognisecrets:<tag> | kubectl apply -f -
```

Verify rollout:

```sh
kubectl -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager
```

Verify probes:

```sh
kubectl -n cognisecrets-system get pods -l app.kubernetes.io/name=cognisecrets
```

The default deployment enables leader election, liveness probes, and readiness probes.

## 2. Status Diagnostics

Inspect `SecretReference` status first:

```sh
kubectl -n <namespace> get secretreference <name> -o yaml
```

The controller owns the `Ready` condition.

Successful reconciliation reports:

```text
type=Ready status=True reason=Synced
```

Failures report:

```text
type=Ready status=False reason=<error reason>
```

Status messages MUST NOT contain secret values.

## 3. Events And Logs

Inspect events in the target namespace:

```sh
kubectl -n <namespace> get events --sort-by=.lastTimestamp
```

Inspect controller logs:

```sh
kubectl -n cognisecrets-system logs deployment/cognisecrets-controller-manager
```

Events are best-effort diagnostics. `SecretReference` status is the authoritative user-facing state.

## 4. Recovery

CogniSecrets is stateless. Recovery should normally require correcting Kubernetes state rather than repairing controller-local data.

Common recovery paths:

- `AccessDenied`: add the target namespace to the source Secret authorization annotation;
- `SourceNotFound`: create or restore the referenced source Secret;
- `SourceKeyNotFound`: restore the requested source key or update the key mapping;
- `DuplicateTargetKey`: change mappings so each target key is produced once;
- `TargetAlreadyExists`: delete, rename, or stop using the foreign target Secret;
- `ManagedSourceRejected`: use an original source Secret instead of a CogniSecrets-managed target;
- `TargetRejected`: fix the requested target Secret type or data shape.

After the underlying state is fixed, the controller reconciles the `SecretReference` again and recreates the managed target Secret when allowed.

## 5. RBAC Review

The default controller runs with one cluster-scoped role because it watches `SecretReference` resources and source `Secret` objects across namespaces.

Granted permissions:

- `secretreferences get,list,watch`: observe desired state;
- `secretreferences/status get,patch,update`: publish the `Ready` condition;
- `secrets get,list,watch`: read source Secrets and watch source changes;
- `secrets create,patch,update,delete`: create, update, and fail-closed delete managed target Secrets;
- `events create,patch`: publish best-effort diagnostics;
- `leases create,get,list,watch,patch,update,delete`: coordinate leader election.

The managed-by label is not used as a security proof. The controller treats a target Secret as managed only when it has a controller owner reference pointing to the current `SecretReference` UID.

Cluster operators MUST protect write access to source Secrets and to the authorization annotation.

## 6. Beta Operational Gate

Before a beta release, maintainers MUST verify:

- the default deployment rolls out successfully;
- liveness and readiness probes are present;
- leader election is enabled in the rendered manifest;
- status reasons map to the documented error catalog;
- RBAC grants are still consistent with this document;
- known release limitations are documented.
