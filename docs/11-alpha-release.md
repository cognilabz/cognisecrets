# Alpha Release

This document defines the local release procedure for the first CogniSecrets alpha.

## 1. Version

Initial alpha tag:

```text
v0.1.0-alpha.1
```

Initial image:

```text
ghcr.io/cognilabz/cognisecrets:v0.1.0-alpha.1
```

## 2. Release Gate

Run the full local gate from a clean worktree:

```sh
make verify-generate test
make render >/tmp/cognisecrets-install.yaml
make e2e
git diff --check
```

The E2E suite creates and destroys a fresh `kind` cluster.

## 3. Generate Install Manifest

Generate the versioned install manifest:

```sh
make release-manifest VERSION=v0.1.0-alpha.1
```

The generated file is:

```text
dist/cognisecrets-v0.1.0-alpha.1.yaml
```

## 4. Publish

The primary publication path is the tag-driven GitHub Actions release workflow.

After the release gate passes:

```sh
git tag v0.1.0-alpha.1
git push origin v0.1.0-alpha.1
```

The workflow publishes:

```text
ghcr.io/cognilabz/cognisecrets:v0.1.0-alpha.1
```

It also uploads the versioned install manifest as a workflow artifact.

## 5. Manual Image Publication

Build the versioned image:

```sh
make docker-build IMG=ghcr.io/cognilabz/cognisecrets:v0.1.0-alpha.1
```

Push the versioned image:

```sh
make docker-push IMG=ghcr.io/cognilabz/cognisecrets:v0.1.0-alpha.1
```

## 6. Smoke Test Published Image

Install the generated manifest into a test cluster:

```sh
kubectl apply -f dist/cognisecrets-v0.1.0-alpha.1.yaml
kubectl -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager
```

Apply the README example and confirm the `SecretReference` reports `Ready=True` with reason `Synced`.

## 7. Known Limitation

`WriteFailed` conformance is covered by reference-implementation unit tests only because it represents operational Kubernetes write or delete failures that are not portable to force in a black-box E2E suite.
