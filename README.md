# CogniSecrets

> Compose Kubernetes Secrets from existing Secrets across namespaces with explicit authorization.

CogniSecrets is a minimal Kubernetes controller that composes and synchronizes one target `Secret` from one or more existing source `Secret` objects.

It adds exactly one capability to Kubernetes: explicit, authorized Secret composition across namespace boundaries.

## Status

CogniSecrets is beta software.

The Go reference implementation includes the `SecretReference` API, generated CRD, controller manager, RBAC, install manifests, samples, unit tests, and a local kind E2E conformance suite.

## Core principles

- Minimal API and controller
- Kubernetes-native behavior
- Explicit cross-namespace authorization
- Stateless, event-driven reconciliation
- Fail closed: no stale or unauthorized target Secrets
- One `SecretReference` produces exactly one target `Secret`

## Non-goals

CogniSecrets is not a vault, secret store, encryption solution, rotation system, GitOps engine, or replacement for Sealed Secrets or External Secrets.

## Documentation

1. [Product overview](docs/01-product-overview.md)
2. [Design principles](docs/02-design-principles.md)
3. [API specification](docs/03-api-specification.md)
4. [Controller specification](docs/04-controller-specification.md)
5. [Security model](docs/05-security-model.md)
6. [Lifecycle specification](docs/06-lifecycle.md)
7. [Error catalog](docs/07-error-catalog.md)
8. [Conformance test specification](docs/08-conformance-test-specification.md)
9. [Roadmap](docs/09-roadmap.md)
10. [E2E test concept](docs/10-e2e-test-concept.md)
11. [Beta release](docs/11-beta-release.md)
12. [Operations](docs/12-operations.md)
13. [v0.2.0-beta.1 release notes](docs/release-notes/v0.2.0-beta.1.md)

## Install

Render and apply the default manifests:

```sh
make render | kubectl apply -f -
kubectl -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager
```

The default deployment uses `ghcr.io/cognilabz/cognisecrets:latest`. To render manifests for a specific image tag:

```sh
make render IMG=ghcr.io/cognilabz/cognisecrets:v0.2.0-beta.1 | kubectl apply -f -
```

For local kind testing, build and load a local image before applying manifests:

```sh
make docker-build IMG=ghcr.io/cognilabz/cognisecrets:dev
kind load docker-image ghcr.io/cognilabz/cognisecrets:dev
make render IMG=ghcr.io/cognilabz/cognisecrets:dev | kubectl apply -f -
```

## Example

Create source and target namespaces:

```sh
kubectl create namespace shared
kubectl create namespace application
```

Create an authorized source Secret:

```sh
kubectl -n shared create secret generic database \
  --from-literal=username=app \
  --from-literal=password=s3cr3t

kubectl -n shared annotate secret database \
  cognisecrets.cognilabz.com/allowed-namespaces=application
```

Apply a `SecretReference`:

```sh
kubectl apply -f config/samples/cognilabz_v1beta1_secretreference_renamed_keys.yaml
```

Inspect the generated target Secret and status:

```sh
kubectl -n application get secretreference application-credentials
kubectl -n application get secret application-credentials -o yaml
```

## Known Limitations

- Beta releases are intended for test and early production evaluation clusters.
- `WriteFailed` conformance is unit-tested only because it represents operational Kubernetes write or delete failures that are not portable to force in black-box E2E tests.

## Development

```sh
make tools
make generate manifests
make test
```

Run the local kind E2E smoke suite:

```sh
make e2e
```

Run the full beta local gate:

```sh
make beta-gate
```

Render install manifests:

```sh
make render
```

Build the controller image:

```sh
make docker-build
```

Generate a versioned beta install manifest:

```sh
make release-manifest VERSION=v0.2.0-beta.1
```

## License

CogniSecrets is licensed under the Apache License 2.0.
