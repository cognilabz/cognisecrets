# CogniSecrets

> **GitOps-native secret composition for Kubernetes.**

CogniSecrets is a Kubernetes controller that builds application `Secret` objects from one or more existing Kubernetes `Secret` resources.

When used together with **Bitnami Sealed Secrets**, CogniSecrets provides a lightweight Git-based secret vault without requiring HashiCorp Vault, External Secrets, or any other external secret backend.

---

## Why CogniSecrets?

Managing secrets in GitOps environments is often a trade-off:

- HashiCorp Vault requires additional infrastructure.
- External Secrets requires an external secret backend.
- Sealed Secrets encrypts secrets, but does not help composing or reusing them.

CogniSecrets fills this gap by allowing applications to consume Secrets derived from one or multiple encrypted SealedSecret manifests while keeping everything Kubernetes-native.

### Features

- 🔐 Keep encrypted secrets in Git
- 🚀 No external Vault required
- 🔄 Compose a Secret from multiple source Secrets
- 📦 Kubernetes-native controller
- ⚡ Designed for GitOps workflows
- ✅ Argo CD friendly with custom health checks
- 🔑 Does not introduce its own secret store

---

## Architecture

```
                    Git Repository
                          │
                SealedSecret manifests
                          │
                          ▼
                Sealed Secrets Controller
                          │
                          ▼
                 Source Kubernetes Secrets
                          │
                          ▼
                    CogniSecrets
                          │
                          ▼
              Application Kubernetes Secret
```

Applications only consume the generated target `Secret`. Sensitive values remain encrypted in Git and are decrypted only inside the Kubernetes cluster by Sealed Secrets.

---

## When should I use CogniSecrets?

CogniSecrets is a good fit when you want to:

- manage secrets with GitOps
- avoid operating HashiCorp Vault
- reuse the same secret values across multiple applications
- compose application secrets from different source secrets
- use Argo CD with a custom health check

---

## Comparison

| Feature | CogniSecrets | Sealed Secrets | External Secrets | HashiCorp Vault |
|----------|--------------|----------------|------------------|-----------------|
| GitOps native | ✅ | ✅ | ✅ | ⚠️ |
| Secret encryption | Via Sealed Secrets | ✅ | ❌ | ✅ |
| Secret composition | ✅ | ❌ | Not primary focus | Not primary focus |
| External backend required | ❌ | ❌ | ✅ | ✅ |
| Additional infrastructure | ❌ | ❌ | Depends | ✅ |

---

## Installation

### Prerequisites

- A Kubernetes cluster
- `kubectl`
- `kubeseal`
- Bitnami Sealed Secrets installed in the cluster

### From a release manifest

```bash
kubectl apply -f https://github.com/cognilabz/cognisecrets/releases/latest/download/cognisecrets.yaml
```

### From a checkout of this repository

```bash
kubectl apply -k manifests
```

---

## Quick Start

### 1. Create a local Secret

> **Do not commit this file to Git.**

Save this as `secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault
  namespace: dev
  annotations:
    cognisecrets.cognilabz.com/allowed-namespaces: dev

stringData:
  username: test-user
  password: test-password
```

---

### 2. Seal the Secret

```bash
kubeseal -o yaml < secrets.yaml > sealed-secrets.yaml
rm secrets.yaml
```

`kubeseal` is provided by Bitnami Sealed Secrets.

The generated `sealed-secrets.yaml` can safely be committed to Git.

---

### 3. Create a SecretReference

Save this as `secretreference.yaml`:

```yaml
apiVersion: cognilabz.com/v1
kind: SecretReference

metadata:
  name: mysecret
  namespace: dev

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

---

### 4. Apply the manifests

```bash
kubectl create namespace dev --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f sealed-secrets.yaml
kubectl apply -f secretreference.yaml
```

CogniSecrets automatically creates the target Kubernetes `Secret`.

Verify the result:

```bash
kubectl -n dev get secretreference mysecret
kubectl -n dev get secret mysecret
```

---

## GitOps

Both

- `SealedSecret`
- `SecretReference`

are intended to be stored in Git.

A typical GitOps workflow looks like this:

```
Developer
      │
      ▼
Create Secret
      │
      ▼
Seal Secret
      │
      ▼
Commit SealedSecret + SecretReference
      │
      ▼
Git Repository
      │
      ▼
Argo CD
      │
      ▼
Cluster
      │
      ▼
CogniSecrets
      │
      ▼
Application Secret
```

For Argo CD users, add the custom health definition found in:

```
docs/argocd-cm.yaml
```

---

## Examples

Additional examples are available in:

```
config/samples/
```

---

## Development

```bash
make tools

make generate manifests

make test

# Local Kind end-to-end tests
make e2e

# Complete release validation
make release-gate

# Render installation manifests
make render

# Build controller image
make docker-build

# Generate versioned release manifests
make release-manifest VERSION=v0.2.6
```

---

## Documentation

1. [Product overview](docs/01-product-overview.md)
2. [Design principles](docs/02-design-principles.md)
3. [API specification](docs/03-api-specification.md)
4. [Controller specification](docs/04-controller-specification.md)
5. [Security model](docs/05-security-model.md)
6. [Lifecycle](docs/06-lifecycle.md)
7. [Error catalog](docs/07-error-catalog.md)
8. [Conformance tests](docs/08-conformance-test-specification.md)
9. [Roadmap](docs/09-roadmap.md)
10. [End-to-end testing](docs/10-e2e-test-concept.md)
11. [Release process](docs/11-release.md)
12. [Operations](docs/12-operations.md)
13. [v0.2.6 release notes](docs/release-notes/v0.2.6.md)

Contributions and feature requests are welcome.

---

## License

CogniSecrets is licensed under the Apache License 2.0.
