# Release

This document defines the minimum gate for CogniSecrets releases.

`v0.1.0` is the first non-alpha release state for CogniSecrets.

## 1. Versioning

Initial release tag:

```text
v0.1.0
```

The release image is:

```text
ghcr.io/cognilabz/cognisecrets:v0.1.0
```

## 2. API Stability

Before a release:

- the served API version MUST be `cognilabz.com/v1`;
- every user-visible API or behavior change since the previous release MUST be documented;
- incompatible changes MUST include migration guidance;
- the compatibility policy for the released API version MUST be documented.

The `v1` API, status reasons, lifecycle behavior, and security behavior are the release contract for `v0.1.0`.

## 3. Release Gate

Run the full local gate from a clean worktree:

```sh
make release-gate
```

This expands to:

```sh
make verify-generate
make test
make verify-render
make build
make e2e
git diff --check
```

The E2E suite creates and destroys a fresh `kind` cluster.

## 4. Operational Gate

At minimum, the release candidate MUST have:

- documented operational guidance for installation, rollout verification, logs, events, status conditions, and recovery from failed reconciliation;
- documented RBAC review covering why each granted verb and resource is required;
- leader election enabled in the default deployment;
- health and readiness probes enabled in the default deployment;
- conformance tests running in CI;
- restart behavior covered by E2E tests;
- watch-driven synchronization covered by E2E tests;
- known failure modes mapped to clear status reasons or operational diagnostics;
- a current list of known limitations in the release notes.

Operational guidance and RBAC review live in `docs/12-operations.md`.

The broader hardening phase in `docs/09-roadmap.md` continues after the initial release.

## 5. Scale And Retry Gate

Before the first release, maintainers SHOULD run a larger local or staging test that verifies:

- many namespaces can each reconcile authorized `SecretReference` resources;
- one source Secret update reconciles multiple dependent references;
- no-op reconciles avoid unnecessary target Secret writes;
- transient Kubernetes API conflicts or retries do not leave stale managed target Secrets.

If this gate is not fully automated, the exact command, cluster shape, result, date, and known gaps MUST be recorded in release notes.

## 6. Publication

Generate the versioned manifest:

```sh
make release-manifest VERSION=<tag>
```

Publish the image and manifest through the tag-driven GitHub Actions workflow.

After publication, smoke test the published image in a fresh test cluster:

```sh
kubectl apply -f dist/cognisecrets-<tag>.yaml
kubectl -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager
```

Apply the README example and confirm the `SecretReference` reports `Ready=True` with reason `Synced`.

## 7. Release Exit Criteria

CogniSecrets has reached release status only when:

- the selected release tag has been published;
- the published image and manifest have passed the smoke test;
- the release notes document API stability, compatibility, migrations, known limitations, and any uncovered MUST statements;
- the README status describes the latest release as release software;
- the roadmap marks the release phase as complete.
