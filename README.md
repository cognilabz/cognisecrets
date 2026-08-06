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

CogniSecrets fills this gap by allowing applications to consume secrets from one or multiple encrypted source secrets while keeping everything Kubernetes-native.

### Features

- 🔐 Keep encrypted secrets in Git
- 🚀 No external Vault required
- 🔄 Compose a Secret from multiple source Secrets
- 📦 Kubernetes-native controller
- ⚡ Designed for GitOps workflows
- ✅ Works perfectly with Argo CD
- 🔑 Never stores plaintext secrets itself

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
- integrate seamlessly with Argo CD

---

## Comparison

| Feature | CogniSecrets | Sealed Secrets | External Secrets | HashiCorp Vault |
|----------|--------------|----------------|------------------|-----------------|
| GitOps native | ✅ | ✅ | ✅ | ⚠️ |
| Secret encryption | Uses Sealed Secrets | ✅ | ❌ | ✅ |
| Secret composition | ✅ | ❌ | Limited | Limited |
| External backend required | ❌ | ❌ | ✅ | ✅ |
| Additional infrastructure | ❌ | ❌ | Depends | ✅ |

---

# Installation

```bash
kubectl apply -k manifests
```

---

# Quick Start

## 1. Create a local Secret

> **Do not commit this file to Git.**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault
  namespace: dev

stringData:
  username: test-user
  password: test-password
```

---

## 2. Seal the Secret

```bash
kubeseal -o yaml < secrets.yaml > sealed-secrets.yaml
rm secrets.yaml
```

`kubeseal` is provided by Bitnami Sealed Secrets.

The generated `sealed-secrets.yaml` can safely be committed to Git.

---

## 3. Create a SecretReference

```yaml
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

---

## 4. Apply the manifests

```bash
kubectl apply -f sealed-secrets.yaml
kubectl apply -f secretreference.yaml
```

CogniSecrets automatically creates the target Kubernetes `Secret`.

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

# Development

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

# Documentation

1. Product overview
2. Design principles
3. API specification
4. Controller specification
5. Security model
6. Lifecycle
7. Error catalog
8. Conformance tests
9. Roadmap
10. End-to-end testing
11. Release process
12. Operations
13. Release notes

See the `docs/` directory for details.

---

# Roadmap

Planned improvements can be found in

```
docs/09-roadmap.md
```

Contributions and feature requests are welcome.

---

# License

CogniSecrets is licensed under the Apache License 2.0.