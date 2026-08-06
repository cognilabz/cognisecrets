# CogniSecrets

CogniSecrets is a Kubernetes controller that composes and synchronizes a target `Secret` from one or more existing source `Secret` objects. In combination with kubeseal it can be used as a simple git-based vault.

## Installation

```sh
kubectl apply -k manifests
```

## Best Practice (GitOps)

1. create a local `secrets.yaml` file - but never publish the plain secrets file to your git-repos!

   ```sh
    apiVersion: v1
    kind: Secret
    metadata:
      name: vault
      namespace: dev
    stringData:
      username: test-user
      password: test-password
   ```

2. seal the secret and remove local secrets-file (or at least add it to .gitignore).
   ```sh
    kubeseal -o yaml < secrets.yaml > sealed-secrets.yaml
    rm secrets.yaml
   ```
   *`kubeseal` is provided by [Bitnami Sealed Secrets](https://github.com/bitnami/sealed-secrets).*

3. create the SecretReference manifest "secretreferences.yaml"

   ```sh
    apiVersion: cognilabz.com/v1
    kind: SecretReference
    metadata:
      name: mysecret
    spec:
      type: Opaque
      sources:
        - name: vault
          namespace: dev
          keys:
            - name: username
              target: DB_USERNAME
            - name: password
              target: DB_PASSWORD
   ```

4. apply sealed-secret and secretreference manifests

   ```sh
    kubectl apply -f sealed-secrets.yaml
    kubectl apply -f secretreferences.yaml
   ```

SealedSecrets and SecretReferences can and should be pushed to git.

If you use ArgoCD then add the following custom-resource-definiton [argocd-cm.yaml](docs/argocd-cm.yaml).

More examples can be found in `config/samples/`.

## Development

```sh
make tools
make generate manifests
make test

# run the local kind E2E smoke suite:
make e2e

# run the full release gate:
make release-gate

# render install manifests:
make render

# build the controller image:
make docker-build

# generate versioned release install manifests:
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
13. [v0.2.6 release notes](docs/release-notes/v0.2.6.md)

## License

CogniSecrets is licensed under the Apache License 2.0.
