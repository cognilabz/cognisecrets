# CogniSecrets

> Compose Kubernetes Secrets from existing Secrets across namespaces with explicit authorization.

CogniSecrets is a minimal Kubernetes controller that composes and synchronizes one target `Secret` from one or more existing source `Secret` objects.

It adds exactly one capability to Kubernetes: explicit, authorized Secret composition across namespace boundaries.

## Best Practice (GitOps)

For GitOps workflows, keep raw Kubernetes Secret manifests local only. Do not commit plaintext Secret data.

One common pattern is:

1. Create a local `secrets.yaml` file containing source Secrets.
2. Seal it with `kubeseal` from [Bitnami Sealed Secrets](https://github.com/bitnami/sealed-secrets).
3. Commit only the sealed manifest.
4. Use CogniSecrets to compose authorized runtime target Secrets from those source Secrets after they are unsealed in the cluster.

## Status

CogniSecrets is release software.

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
11. [Release](docs/11-release.md)
12. [Operations](docs/12-operations.md)
13. [v0.1.0 release notes](docs/release-notes/v0.1.0.md)

## Install

Render and apply the default manifests:

```sh
make render | kubectl apply -f -
kubectl -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager
```

The default deployment uses `ghcr.io/cognilabz/cognisecrets:latest`. To render manifests for a specific image tag:

```sh
make render IMG=ghcr.io/cognilabz/cognisecrets:v0.1.0 | kubectl apply -f -
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
kubectl apply -f config/samples/cognilabz_v1_secretreference_renamed_keys.yaml
```

Inspect the generated target Secret and status:

```sh
kubectl -n application get secretreference application-credentials
kubectl -n application get secret application-credentials -o yaml
```

## Known Limitations

- `v0.1.0` is the first non-alpha release.
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

Run the full release gate:

```sh
make release-gate
```

Render install manifests:

```sh
make render
```

Build the controller image:

```sh
make docker-build
```

Generate a versioned release install manifest:

```sh
make release-manifest VERSION=v0.1.0
```

## License

CogniSecrets is licensed under the Apache License 2.0.
