# CogniSecrets

> Compose Kubernetes Secrets from existing Secrets across namespaces with explicit authorization.

CogniSecrets is a minimal Kubernetes controller that composes and synchronizes one target `Secret` from one or more existing source `Secret` objects.

It adds exactly one capability to Kubernetes: explicit, authorized Secret composition across namespace boundaries.

## Status

CogniSecrets is currently in the specification phase. The API, controller behavior, security model, and conformance tests are defined before implementation begins.

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

- [Product overview](docs/01-product-overview.md)

## License

A license will be selected before the first public release.
