#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-cognisecrets-e2e}"
IMAGE="${IMAGE:-ghcr.io/cognilabz/cognisecrets:e2e}"
KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
KUSTOMIZE="${KUSTOMIZE:-${ROOT_DIR}/bin/kustomize}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"

cleanup() {
  if [[ "${KEEP_CLUSTER}" != "true" ]]; then
    "${KIND}" delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

log() {
  printf '==> %s\n' "$*"
}

wait_for_jsonpath() {
  local namespace="$1"
  local resource="$2"
  local jsonpath="$3"
  local expected="$4"

  for _ in {1..60}; do
    local value
    value="$("${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n "${namespace}" get "${resource}" -o "jsonpath=${jsonpath}" 2>/dev/null || true)"
    if [[ "${value}" == "${expected}" ]]; then
      return 0
    fi
    sleep 1
  done

  log "timed out waiting for ${namespace}/${resource} ${jsonpath}=${expected}"
  "${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n "${namespace}" get "${resource}" -o yaml || true
  "${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n cognisecrets-system logs deployment/cognisecrets-controller-manager --tail=200 || true
  return 1
}

wait_for_absent() {
  local namespace="$1"
  local resource="$2"

  for _ in {1..60}; do
    if ! "${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n "${namespace}" get "${resource}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  log "timed out waiting for ${namespace}/${resource} to be absent"
  "${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n "${namespace}" get "${resource}" -o yaml || true
  return 1
}

apply_yaml() {
  "${KUBECTL}" --context "kind-${CLUSTER_NAME}" apply -f -
}

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

"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n cognisecrets-system rollout status deployment/cognisecrets-controller-manager --timeout=120s

log "scenario: selected key sync and authorization revoke"
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" create namespace shared
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" create namespace application
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n shared create secret generic database \
  --from-literal=username=app \
  --from-literal=password=s3cr3t
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n shared annotate secret database \
  cognisecrets.cognilabz.com/allowed-namespaces=application
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" apply -f "${ROOT_DIR}/config/samples/cognilabz_v1alpha1_secretreference_renamed_keys.yaml"

wait_for_jsonpath application secretreference/application-credentials '{.status.conditions[?(@.type=="Ready")].reason}' Synced
wait_for_jsonpath application secret/application-credentials '{.metadata.labels.app\.kubernetes\.io/managed-by}' cognisecrets
wait_for_jsonpath application secret/application-credentials '{.data.DB_USERNAME}' YXBw
wait_for_jsonpath application secret/application-credentials '{.data.DB_PASSWORD}' czNjcjN0

"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n shared annotate secret database \
  cognisecrets.cognilabz.com/allowed-namespaces=reporting --overwrite
wait_for_jsonpath application secretreference/application-credentials '{.status.conditions[?(@.type=="Ready")].reason}' AccessDenied
wait_for_absent application secret/application-credentials

log "scenario: foreign target conflict"
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" create namespace conflict
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n conflict create secret generic existing --from-literal=keep=value
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n shared annotate secret database \
  cognisecrets.cognilabz.com/allowed-namespaces=conflict --overwrite
cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1alpha1
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

log "scenario: managed-source rejection"
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" create namespace chain
"${KUBECTL}" --context "kind-${CLUSTER_NAME}" -n shared annotate secret database \
  cognisecrets.cognilabz.com/allowed-namespaces=chain --overwrite
cat <<'YAML' | apply_yaml
apiVersion: cognilabz.com/v1alpha1
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
apiVersion: cognilabz.com/v1alpha1
kind: SecretReference
metadata:
  name: chained
  namespace: application
spec:
  sources:
    - namespace: chain
      name: generated
YAML
wait_for_jsonpath application secretreference/chained '{.status.conditions[?(@.type=="Ready")].reason}' ManagedSourceRejected
wait_for_absent application secret/chained

log "E2E smoke suite passed"
