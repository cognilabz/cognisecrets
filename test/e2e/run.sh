#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-cognisecrets-e2e}"
IMAGE="${IMAGE:-ghcr.io/cognilabz/cognisecrets:e2e}"
KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
KUSTOMIZE="${KUSTOMIZE:-${ROOT_DIR}/bin/kustomize}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"

ctx="kind-${CLUSTER_NAME}"

cleanup() {
  if [[ "${KEEP_CLUSTER}" != "true" ]]; then
    "${KIND}" delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf '%s\n' "$*" >&2
  exit 1
}

k() {
  "${KUBECTL}" --context "${ctx}" "$@"
}

apply_yaml() {
  k apply -f -
}

wait_for_jsonpath() {
  local namespace="$1"
  local resource="$2"
  local jsonpath="$3"
  local expected="$4"

  for _ in {1..60}; do
    local value
    value="$(k -n "${namespace}" get "${resource}" -o "jsonpath=${jsonpath}" 2>/dev/null || true)"
    if [[ "${value}" == "${expected}" ]]; then
      return 0
    fi
    sleep 1
  done

  log "timed out waiting for ${namespace}/${resource} ${jsonpath}=${expected}"
  k -n "${namespace}" get "${resource}" -o yaml || true
  k -n cognisecrets-system logs deployment/cognisecrets-controller-manager --tail=200 || true
  return 1
}

wait_for_absent() {
  local namespace="$1"
  local resource="$2"

  for _ in {1..60}; do
    if ! k -n "${namespace}" get "${resource}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  log "timed out waiting for ${namespace}/${resource} to be absent"
  k -n "${namespace}" get "${resource}" -o yaml || true
  return 1
}

assert_jsonpath() {
  local namespace="$1"
  local resource="$2"
  local jsonpath="$3"
  local expected="$4"
  local value

  value="$(k -n "${namespace}" get "${resource}" -o "jsonpath=${jsonpath}")"
  if [[ "${value}" != "${expected}" ]]; then
    printf 'assertion failed for %s/%s %s: got %q want %q\n' "${namespace}" "${resource}" "${jsonpath}" "${value}" "${expected}" >&2
    exit 1
  fi
}

assert_no_secret_value_in_status() {
  local namespace="$1"
  local name="$2"
  shift 2
  local status

  status="$(k -n "${namespace}" get "secretreference/${name}" -o jsonpath='{.status.conditions[*].message}')"
  for value in "$@"; do
    if [[ -n "${value}" && "${status}" == *"${value}"* ]]; then
      printf 'status message for %s/%s leaked secret value %q\n' "${namespace}" "${name}" "${value}" >&2
      exit 1
    fi
  done
}

assert_no_secret_value_in_events() {
  local namespace="$1"
  shift
  local events

  events="$(k -n "${namespace}" get events -o jsonpath='{range .items[*]}{.message}{"\n"}{end}' 2>/dev/null || true)"
  for value in "$@"; do
    if [[ -n "${value}" && "${events}" == *"${value}"* ]]; then
      printf 'events in namespace %s leaked secret value %q\n' "${namespace}" "${value}" >&2
      exit 1
    fi
  done
}

expect_apply_failure() {
  local name="$1"
  local manifest="$2"

  if printf '%s\n' "${manifest}" | apply_yaml >/tmp/cognisecrets-e2e-apply.out 2>/tmp/cognisecrets-e2e-apply.err; then
    printf 'expected apply failure for %s\n' "${name}" >&2
    cat /tmp/cognisecrets-e2e-apply.out >&2
    exit 1
  fi
}

resource_version() {
  local namespace="$1"
  local resource="$2"
  k -n "${namespace}" get "${resource}" -o jsonpath='{.metadata.resourceVersion}'
}

install_controller() {
  log "building controller image ${IMAGE}"
  docker build -t "${IMAGE}" "${ROOT_DIR}"

  log "creating kind cluster ${CLUSTER_NAME}"
  "${KIND}" delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  "${KIND}" create cluster --name "${CLUSTER_NAME}"

  log "loading controller image"
  "${KIND}" load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"

  log "installing CogniSecrets"
  "${KUSTOMIZE}" build "${ROOT_DIR}/config/default" |
    sed "s#ghcr.io/cognilabz/cognisecrets:latest#${IMAGE}#g" |
    apply_yaml

  k -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager --timeout=120s
}

scenario_api_validation() {
  log "scenario: API validation"
  k create namespace api-validation

  expect_apply_failure "missing sources" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: missing-sources
  namespace: api-validation
spec: {}
YAML
)"

  expect_apply_failure "empty sources" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: empty-sources
  namespace: api-validation
spec:
  sources: []
YAML
)"

  expect_apply_failure "source without namespace" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: missing-namespace
  namespace: api-validation
spec:
  sources:
    - name: database
YAML
)"

  expect_apply_failure "source without name" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: missing-name
  namespace: api-validation
spec:
  sources:
    - namespace: shared
YAML
)"

  expect_apply_failure "empty keys" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: empty-keys
  namespace: api-validation
spec:
  sources:
    - namespace: shared
      name: database
      keys: []
YAML
)"

  expect_apply_failure "key without name" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: missing-key-name
  namespace: api-validation
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - target: PASSWORD
YAML
)"

  expect_apply_failure "unknown spec field" "$(cat <<'YAML'
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: unknown-field
  namespace: api-validation
spec:
  unsupported: true
  sources:
    - namespace: shared
      name: database
YAML
)"
}

scenario_basic_sync_and_watches() {
  log "scenario: basic sync, multi-source, no-op, and source update"
  k create namespace shared
  k create namespace basic
  k -n shared create secret generic database \
    --type=kubernetes.io/basic-auth \
    --from-literal=username=app \
    --from-literal=password=s3cr3t
  k -n shared annotate secret database \
    cognisecrets.cognilabz.com/allowed-namespaces=' basic , basic,,invalid_!,basic'
  k -n shared create secret generic messaging \
    --from-literal=token=tok \
    --from-literal=url=nats
  k -n shared annotate secret messaging \
    cognisecrets.cognilabz.com/allowed-namespaces=basic
  cat <<'YAML' | apply_yaml
apiVersion: v1
kind: Secret
metadata:
  name: binary
  namespace: shared
  annotations:
    cognisecrets.cognilabz.com/allowed-namespaces: basic
type: Opaque
data:
  bytes: AP8Q
YAML

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: all-keys
  namespace: basic
spec:
  sources:
    - namespace: shared
      name: database
YAML
  wait_for_jsonpath basic secretreference/all-keys '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  assert_jsonpath basic secret/all-keys '{.type}' Opaque
  assert_jsonpath basic secret/all-keys '{.data.username}' YXBw
  assert_jsonpath basic secret/all-keys '{.data.password}' czNjcjN0
  assert_jsonpath basic secret/all-keys '{.metadata.labels.app\.kubernetes\.io/managed-by}' cognisecrets
  assert_jsonpath basic secret/all-keys '{.metadata.ownerReferences[0].controller}' true

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: binary-copy
  namespace: basic
spec:
  sources:
    - namespace: shared
      name: binary
      keys:
        - name: bytes
          target: raw
YAML
  wait_for_jsonpath basic secretreference/binary-copy '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  assert_jsonpath basic secret/binary-copy '{.data.raw}' AP8Q

  local secret_rv_before status_rv_before
  secret_rv_before="$(resource_version basic secret/all-keys)"
  status_rv_before="$(resource_version basic secretreference/all-keys)"
  k -n shared annotate secret database e2e.cognisecrets/irrelevant=one --overwrite
  sleep 3
  assert_jsonpath basic secret/all-keys '{.metadata.resourceVersion}' "${secret_rv_before}"
  assert_jsonpath basic secretreference/all-keys '{.metadata.resourceVersion}' "${status_rv_before}"

  k -n shared create secret generic unrelated --from-literal=value=one
  sleep 3
  assert_jsonpath basic secret/all-keys '{.metadata.resourceVersion}' "${secret_rv_before}"

  k -n shared patch secret database --type=merge -p '{"data":{"username":"YXBwMg=="}}'
  wait_for_jsonpath basic secret/all-keys '{.data.username}' YXBwMg==

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: composed
  namespace: basic
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
          target: DB_USERNAME
        - name: password
          target: DB_PASSWORD
    - namespace: shared
      name: messaging
      keys:
        - name: token
        - name: token
          target: TOKEN_COPY
YAML
  wait_for_jsonpath basic secretreference/composed '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  assert_jsonpath basic secret/composed '{.data.DB_USERNAME}' YXBwMg==
  assert_jsonpath basic secret/composed '{.data.DB_PASSWORD}' czNjcjN0
  assert_jsonpath basic secret/composed '{.data.token}' dG9r
  assert_jsonpath basic secret/composed '{.data.TOKEN_COPY}' dG9r

  k -n shared patch secret messaging --type=merge -p '{"data":{"token":"dG9rMg=="}}'
  wait_for_jsonpath basic secret/composed '{.data.token}' dG9rMg==
  wait_for_jsonpath basic secret/composed '{.data.TOKEN_COPY}' dG9rMg==
}

scenario_authorization_failures() {
  log "scenario: authorization failures"
  k create namespace auth
  k -n shared create secret generic no-annotation --from-literal=value=secret
  k -n shared create secret generic empty-annotation --from-literal=value=secret
  k -n shared annotate secret empty-annotation cognisecrets.cognilabz.com/allowed-namespaces=''
  k -n shared create secret generic wildcard --from-literal=value=secret
  k -n shared annotate secret wildcard cognisecrets.cognilabz.com/allowed-namespaces='*'

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: no-annotation
  namespace: auth
spec:
  sources:
    - namespace: shared
      name: no-annotation
---
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: empty-annotation
  namespace: auth
spec:
  sources:
    - namespace: shared
      name: empty-annotation
---
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: wildcard
  namespace: auth
spec:
  sources:
    - namespace: shared
      name: wildcard
YAML
  wait_for_jsonpath auth secretreference/no-annotation '{.status.conditions[?(@.type=="Ready")].reason}' AccessDenied
  wait_for_jsonpath auth secretreference/empty-annotation '{.status.conditions[?(@.type=="Ready")].reason}' AccessDenied
  wait_for_jsonpath auth secretreference/wildcard '{.status.conditions[?(@.type=="Ready")].reason}' AccessDenied
  wait_for_absent auth secret/no-annotation
  wait_for_absent auth secret/empty-annotation
  wait_for_absent auth secret/wildcard
  assert_no_secret_value_in_status auth no-annotation secret
  assert_no_secret_value_in_status auth empty-annotation secret
  assert_no_secret_value_in_status auth wildcard secret
}

scenario_fail_closed_and_recovery() {
  log "scenario: fail-closed deletion and recovery"
  k create namespace recovery
  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: late-source
  namespace: recovery
spec:
  sources:
    - namespace: shared
      name: late-source
YAML
  wait_for_jsonpath recovery secretreference/late-source '{.status.conditions[?(@.type=="Ready")].reason}' SourceNotFound
  k -n shared create secret generic late-source --from-literal=value=late
  k -n shared annotate secret late-source cognisecrets.cognilabz.com/allowed-namespaces=recovery
  wait_for_jsonpath recovery secretreference/late-source '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  wait_for_jsonpath recovery secret/late-source '{.data.value}' bGF0ZQ==

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: missing-key
  namespace: recovery
spec:
  sources:
    - namespace: shared
      name: late-source
      keys:
        - name: missing
YAML
  wait_for_jsonpath recovery secretreference/missing-key '{.status.conditions[?(@.type=="Ready")].reason}' SourceKeyNotFound
  wait_for_absent recovery secret/missing-key
  k -n shared patch secret late-source --type=merge -p '{"data":{"missing":"cmVjb3ZlcmVk"}}'
  wait_for_jsonpath recovery secretreference/missing-key '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  wait_for_jsonpath recovery secret/missing-key '{.data.missing}' cmVjb3ZlcmVk

  k -n shared delete secret late-source
  wait_for_jsonpath recovery secretreference/late-source '{.status.conditions[?(@.type=="Ready")].reason}' SourceNotFound
  wait_for_absent recovery secret/late-source
}

scenario_conflicts_and_ownership() {
  log "scenario: conflicts and ownership"
  k create namespace conflict
  k -n conflict create secret generic existing --from-literal=keep=value
  k -n conflict label secret existing app.kubernetes.io/managed-by=cognisecrets
  k -n shared annotate secret database cognisecrets.cognilabz.com/allowed-namespaces=basic,conflict --overwrite
  k -n shared annotate secret messaging cognisecrets.cognilabz.com/allowed-namespaces=basic,conflict --overwrite
  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: existing
  namespace: conflict
spec:
  sources:
    - namespace: shared
      name: database
YAML
  wait_for_jsonpath conflict secretreference/existing '{.status.conditions[?(@.type=="Ready")].reason}' TargetAlreadyExists
  wait_for_jsonpath conflict secret/existing '{.data.keep}' dmFsdWU=

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: duplicate-cross-source
  namespace: conflict
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
          target: DUP
    - namespace: shared
      name: messaging
      keys:
        - name: token
          target: DUP
---
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: duplicate-one-source
  namespace: conflict
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
          target: DUP
        - name: password
          target: DUP
YAML
  wait_for_jsonpath conflict secretreference/duplicate-cross-source '{.status.conditions[?(@.type=="Ready")].reason}' DuplicateTargetKey
  wait_for_jsonpath conflict secretreference/duplicate-one-source '{.status.conditions[?(@.type=="Ready")].reason}' DuplicateTargetKey
  wait_for_absent conflict secret/duplicate-cross-source
  wait_for_absent conflict secret/duplicate-one-source
  k -n conflict patch secretreference duplicate-cross-source --type=merge -p '{"spec":{"sources":[{"namespace":"shared","name":"database","keys":[{"name":"username","target":"DB_USER"}]},{"namespace":"shared","name":"messaging","keys":[{"name":"token","target":"TOKEN"}]}]}}'
  wait_for_jsonpath conflict secretreference/duplicate-cross-source '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  wait_for_jsonpath conflict secret/duplicate-cross-source '{.data.DB_USER}' YXBwMg==
  wait_for_jsonpath conflict secret/duplicate-cross-source '{.data.TOKEN}' dG9rMg==
}

scenario_metadata_lifecycle_and_restart() {
  log "scenario: metadata preservation, lifecycle changes, delete, and restart"
  k create namespace lifecycle
  k -n shared annotate secret database cognisecrets.cognilabz.com/allowed-namespaces=basic,conflict,lifecycle --overwrite
  k -n shared annotate secret messaging cognisecrets.cognilabz.com/allowed-namespaces=basic,conflict,lifecycle --overwrite
  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: mutable
  namespace: lifecycle
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
          target: USERNAME
YAML
  wait_for_jsonpath lifecycle secretreference/mutable '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  k -n lifecycle annotate secret mutable custom=annotation
  k -n lifecycle label secret mutable custom=label
  k -n lifecycle patch secret mutable --type=merge -p '{"metadata":{"finalizers":["example.com/finalizer"]}}'
  k -n lifecycle patch secretreference mutable --type=merge -p '{"spec":{"sources":[{"namespace":"shared","name":"database","keys":[{"name":"username","target":"username"},{"name":"password","target":"password"}]}]}}'
  wait_for_jsonpath lifecycle secret/mutable '{.data.password}' czNjcjN0
  assert_jsonpath lifecycle secret/mutable '{.type}' Opaque
  assert_jsonpath lifecycle secret/mutable '{.metadata.annotations.custom}' annotation
  assert_jsonpath lifecycle secret/mutable '{.metadata.labels.custom}' label
  assert_jsonpath lifecycle secret/mutable '{.metadata.finalizers[0]}' example.com/finalizer

  k -n lifecycle patch secret mutable --type=json -p='[{"op":"remove","path":"/metadata/finalizers/0"}]'
  k -n lifecycle patch secretreference mutable --type=merge -p '{"spec":{"sources":[{"namespace":"shared","name":"messaging","keys":[{"name":"url","target":"URL"}]}]}}'
  wait_for_jsonpath lifecycle secret/mutable '{.data.URL}' bmF0cw==

  k -n lifecycle delete secretreference mutable
  wait_for_absent lifecycle secret/mutable

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: type-change
  namespace: lifecycle
spec:
  sources:
    - namespace: shared
      name: messaging
      keys:
        - name: url
YAML
  wait_for_jsonpath lifecycle secretreference/type-change '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  type_change_uid_before="$(k -n lifecycle get secret type-change -o jsonpath='{.metadata.uid}')"
  k -n lifecycle patch secretreference type-change --type=merge -p '{"spec":{"type":"kubernetes.io/basic-auth","sources":[{"namespace":"shared","name":"database","keys":[{"name":"username"},{"name":"password"}]}]}}'
  wait_for_jsonpath lifecycle secret/type-change '{.type}' kubernetes.io/basic-auth
  wait_for_jsonpath lifecycle secretreference/type-change '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  assert_jsonpath lifecycle secret/type-change '{.data.username}' YXBwMg==
  assert_jsonpath lifecycle secret/type-change '{.data.password}' czNjcjN0
  type_change_uid_after="$(k -n lifecycle get secret type-change -o jsonpath='{.metadata.uid}')"
  if [[ "${type_change_uid_before}" == "${type_change_uid_after}" ]]; then
    fail "expected type-change Secret to be replaced when immutable type changed"
  fi

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: target-rejected
  namespace: lifecycle
spec:
  sources:
    - namespace: shared
      name: database
      keys:
        - name: username
YAML
  wait_for_jsonpath lifecycle secretreference/target-rejected '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  k -n lifecycle patch secretreference target-rejected --type=merge -p '{"spec":{"type":"kubernetes.io/dockerconfigjson","sources":[{"namespace":"shared","name":"database","keys":[{"name":"username"}]}]}}'
  wait_for_jsonpath lifecycle secretreference/target-rejected '{.status.conditions[?(@.type=="Ready")].reason}' TargetRejected
  wait_for_absent lifecycle secret/target-rejected
  assert_no_secret_value_in_status lifecycle target-rejected app2 s3cr3t
  assert_no_secret_value_in_events lifecycle app2 s3cr3t

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: restart
  namespace: lifecycle
spec:
  sources:
    - namespace: shared
      name: messaging
      keys:
        - name: url
YAML
  wait_for_jsonpath lifecycle secretreference/restart '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  k -n lifecycle patch secret restart --type=merge -p '{"data":{"url":"c3RhbGU="}}'
  k -n cognisecrets-system rollout restart deployment/cognisecrets-controller-manager
  k -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager --timeout=120s
  wait_for_jsonpath lifecycle secret/restart '{.data.url}' bmF0cw==
}

scenario_chain_prevention() {
  log "scenario: chain prevention"
  k create namespace chain
  k -n shared annotate secret database cognisecrets.cognilabz.com/allowed-namespaces=basic,conflict,lifecycle,chain --overwrite
  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: generated
  namespace: chain
spec:
  sources:
    - namespace: shared
      name: database
YAML
  wait_for_jsonpath chain secretreference/generated '{.status.conditions[?(@.type=="Ready")].reason}' Synced

  cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1
kind: SecretReference
metadata:
  name: chained
  namespace: basic
spec:
  sources:
    - namespace: chain
      name: generated
YAML
  wait_for_jsonpath basic secretreference/chained '{.status.conditions[?(@.type=="Ready")].reason}' ManagedSourceRejected
  wait_for_absent basic secret/chained

  k -n basic patch secretreference chained --type=merge -p '{"spec":{"sources":[{"namespace":"shared","name":"database"}]}}'
  wait_for_jsonpath basic secretreference/chained '{.status.conditions[?(@.type=="Ready")].reason}' Synced
  k -n basic patch secretreference chained --type=merge -p '{"spec":{"sources":[{"namespace":"chain","name":"generated"}]}}'
  wait_for_jsonpath basic secretreference/chained '{.status.conditions[?(@.type=="Ready")].reason}' ManagedSourceRejected
  wait_for_absent basic secret/chained
}

install_controller
scenario_api_validation
scenario_basic_sync_and_watches
scenario_authorization_failures
scenario_fail_closed_and_recovery
scenario_conflicts_and_ownership
scenario_metadata_lifecycle_and_restart
scenario_chain_prevention

log "E2E conformance smoke suite passed"
