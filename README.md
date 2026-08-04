# CogniSecrets

CogniSecrets is a minimal Kubernetes controller that composes and synchronizes a target `Secret` from one or more existing source `Secret` objects.

## Installation

```sh
kubectl apply -k manifests
```

## Best Practice (GitOps)

1. create "secrets.yaml"
2. kubeseal -o yaml < secrets.yaml > sealed-secrets.yaml
3. create "congisecrets.yaml"
4. apply both "sealed-secrets.yaml" and "cognisecrets.yaml" to Kubernetes.

Sealed-secrets and cognisecrets can be pushed to git. But never publish the plain secrets file!

*`kubeseal` is provided by [Bitnami Sealed Secrets](https://github.com/bitnami/sealed-secrets).*

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

Generate versioned release install manifests:

```sh
make release-manifest VERSION=v0.1.0
```

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

## License

CogniSecrets is licensed under the Apache License 2.0.
