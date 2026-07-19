package controller

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	cogniv1alpha1 "github.com/cognilabz/cognisecrets/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const sourceSecretIndexField = ".spec.sourcesSecret"

// SecretReferenceReconciler reconciles SecretReference resources into target Secrets.
type SecretReferenceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type compositionFailure struct {
	reason  string
	message string
}

type valueContributor struct {
	sourceNamespace string
	sourceName      string
	sourceKey       string
	targetKey       string
}

// +kubebuilder:rbac:groups=cognilabz.com,resources=secretreferences,verbs=get;list;watch
// +kubebuilder:rbac:groups=cognilabz.com,resources=secretreferences/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

func (r *SecretReferenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ref cogniv1alpha1.SecretReference
	if err := r.Get(ctx, req.NamespacedName, &ref); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var target corev1.Secret
	targetKey := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	targetExists := true
	if err := r.Get(ctx, targetKey, &target); err != nil {
		if !apierrors.IsNotFound(err) {
			result, statusErr := r.setFailure(ctx, &ref, cogniv1alpha1.ReasonWriteFailed, fmt.Sprintf("failed to read target Secret %s/%s: %v", ref.Namespace, ref.Name, err))
			return result, firstErr(err, statusErr)
		}
		targetExists = false
	}

	if targetExists && !ownedBy(&target, &ref) {
		msg := fmt.Sprintf("target Secret %s/%s already exists and is not owned by SecretReference %s/%s", ref.Namespace, ref.Name, ref.Namespace, ref.Name)
		r.event(&ref, corev1.EventTypeWarning, cogniv1alpha1.ReasonTargetAlreadyExists, msg)
		return r.setFailure(ctx, &ref, cogniv1alpha1.ReasonTargetAlreadyExists, msg)
	}

	desired, failure := r.desiredTarget(ctx, &ref)
	if failure != nil {
		if targetExists {
			if err := r.Delete(ctx, &target); err != nil && !apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("failed to delete managed target Secret %s/%s after %s: %v", ref.Namespace, ref.Name, failure.reason, err)
				r.event(&ref, corev1.EventTypeWarning, cogniv1alpha1.ReasonWriteFailed, msg)
				result, statusErr := r.setFailure(ctx, &ref, cogniv1alpha1.ReasonWriteFailed, msg)
				return result, firstErr(err, statusErr)
			}
			r.event(&ref, corev1.EventTypeWarning, failure.reason, fmt.Sprintf("deleted managed target Secret %s/%s: %s", ref.Namespace, ref.Name, failure.message))
		} else {
			r.event(&ref, corev1.EventTypeWarning, failure.reason, failure.message)
		}
		return r.setFailure(ctx, &ref, failure.reason, failure.message)
	}

	if !targetExists {
		if err := r.Create(ctx, desired); err != nil {
			return r.handleTargetWriteFailure(ctx, &ref, nil, err)
		}
		r.event(&ref, corev1.EventTypeNormal, "TargetCreated", fmt.Sprintf("created target Secret %s/%s", ref.Namespace, ref.Name))
		return r.setReady(ctx, &ref)
	}

	if managedFieldsEqual(&target, desired) {
		return r.setReady(ctx, &ref)
	}

	updated := target.DeepCopy()
	applyManagedFields(updated, desired, &ref)
	if err := r.Update(ctx, updated); err != nil {
		return r.handleTargetWriteFailure(ctx, &ref, &target, err)
	}
	r.event(&ref, corev1.EventTypeNormal, "TargetUpdated", fmt.Sprintf("updated target Secret %s/%s", ref.Namespace, ref.Name))

	logger.V(1).Info("synchronized target Secret", "secret", targetKey)
	return r.setReady(ctx, &ref)
}

func (r *SecretReferenceReconciler) desiredTarget(ctx context.Context, ref *cogniv1alpha1.SecretReference) (*corev1.Secret, *compositionFailure) {
	data := map[string][]byte{}
	contributors := map[string]valueContributor{}

	for _, sourceRef := range ref.Spec.Sources {
		var source corev1.Secret
		key := types.NamespacedName{Namespace: sourceRef.Namespace, Name: sourceRef.Name}
		if err := r.Get(ctx, key, &source); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fail(cogniv1alpha1.ReasonSourceNotFound, "source Secret %s/%s does not exist", sourceRef.Namespace, sourceRef.Name)
			}
			return nil, fail(cogniv1alpha1.ReasonWriteFailed, "failed to read source Secret %s/%s: %v", sourceRef.Namespace, sourceRef.Name, err)
		}

		if isManagedSource(&source) {
			return nil, fail(cogniv1alpha1.ReasonManagedSourceRejected, "source Secret %s/%s is managed by CogniSecrets", sourceRef.Namespace, sourceRef.Name)
		}

		if !allowsNamespace(source.Annotations[cogniv1alpha1.AuthorizationAnnotation], ref.Namespace) {
			return nil, fail(cogniv1alpha1.ReasonAccessDenied, "source Secret %s/%s does not authorize namespace %s", sourceRef.Namespace, sourceRef.Name, ref.Namespace)
		}

		mappings := resolvedMappings(sourceRef, source.Data)
		for _, mapping := range mappings {
			value, ok := source.Data[mapping.Name]
			if !ok {
				return nil, fail(cogniv1alpha1.ReasonSourceKeyNotFound, "source Secret %s/%s does not contain key %s", sourceRef.Namespace, sourceRef.Name, mapping.Name)
			}
			targetKey := mapping.Target
			if targetKey == "" {
				targetKey = mapping.Name
			}
			current := valueContributor{
				sourceNamespace: sourceRef.Namespace,
				sourceName:      sourceRef.Name,
				sourceKey:       mapping.Name,
				targetKey:       targetKey,
			}
			if previous, exists := contributors[targetKey]; exists {
				return nil, fail(cogniv1alpha1.ReasonDuplicateTargetKey, "target key %s is produced by %s/%s key %s and %s/%s key %s", targetKey, previous.sourceNamespace, previous.sourceName, previous.sourceKey, current.sourceNamespace, current.sourceName, current.sourceKey)
			}
			contributors[targetKey] = current
			data[targetKey] = append([]byte(nil), value...)
		}
	}

	targetType := ref.Spec.Type
	if targetType == "" {
		targetType = corev1.SecretTypeOpaque
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Labels: map[string]string{
				cogniv1alpha1.ManagedByLabel: cogniv1alpha1.ManagedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef(ref)},
		},
		Type: targetType,
		Data: data,
	}
	return secret, nil
}

func resolvedMappings(source cogniv1alpha1.SecretSource, data map[string][]byte) []cogniv1alpha1.SecretKeyMapping {
	if len(source.Keys) > 0 {
		return source.Keys
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mappings := make([]cogniv1alpha1.SecretKeyMapping, 0, len(keys))
	for _, key := range keys {
		mappings = append(mappings, cogniv1alpha1.SecretKeyMapping{Name: key, Target: key})
	}
	return mappings
}

func allowsNamespace(annotation, namespace string) bool {
	for _, item := range strings.Split(annotation, ",") {
		item = strings.TrimSpace(item)
		if item == "" || item == "*" || !isNamespaceName(item) {
			continue
		}
		if item == namespace {
			return true
		}
	}
	return false
}

func isNamespaceName(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	return isDNSLabel(value)
}

func isDNSLabel(value string) bool {
	if value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isManagedSource(secret *corev1.Secret) bool {
	for _, ref := range secret.OwnerReferences {
		if ref.Controller != nil && *ref.Controller && ref.APIVersion == cogniv1alpha1.GroupVersion.String() && ref.Kind == "SecretReference" {
			return true
		}
	}
	return false
}

func ownedBy(secret *corev1.Secret, ref *cogniv1alpha1.SecretReference) bool {
	for _, ownerRef := range secret.OwnerReferences {
		if ownerRef.Controller != nil && *ownerRef.Controller && ownerRef.APIVersion == cogniv1alpha1.GroupVersion.String() && ownerRef.Kind == "SecretReference" && ownerRef.UID == ref.UID {
			return true
		}
	}
	return false
}

func ownerRef(ref *cogniv1alpha1.SecretReference) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion: cogniv1alpha1.GroupVersion.String(),
		Kind:       "SecretReference",
		Name:       ref.Name,
		UID:        ref.UID,
		Controller: &controller,
	}
}

func managedFieldsEqual(current, desired *corev1.Secret) bool {
	if current.Type != desired.Type || !reflect.DeepEqual(current.Data, desired.Data) {
		return false
	}
	if current.Labels[cogniv1alpha1.ManagedByLabel] != cogniv1alpha1.ManagedByValue {
		return false
	}
	return hasOwnerRef(current.OwnerReferences, desired.OwnerReferences[0])
}

func applyManagedFields(target, desired *corev1.Secret, ref *cogniv1alpha1.SecretReference) {
	target.Type = desired.Type
	target.Data = copySecretData(desired.Data)
	if target.Labels == nil {
		target.Labels = map[string]string{}
	}
	target.Labels[cogniv1alpha1.ManagedByLabel] = cogniv1alpha1.ManagedByValue
	target.OwnerReferences = withOwnOwnerRef(target.OwnerReferences, ownerRef(ref))
}

func copySecretData(data map[string][]byte) map[string][]byte {
	copied := make(map[string][]byte, len(data))
	for key, value := range data {
		copied[key] = append([]byte(nil), value...)
	}
	return copied
}

func hasOwnerRef(ownerRefs []metav1.OwnerReference, desired metav1.OwnerReference) bool {
	for _, ownerRef := range ownerRefs {
		if ownerRef.APIVersion == desired.APIVersion && ownerRef.Kind == desired.Kind && ownerRef.Name == desired.Name && ownerRef.UID == desired.UID && ownerRef.Controller != nil && *ownerRef.Controller {
			return true
		}
	}
	return false
}

func withOwnOwnerRef(ownerRefs []metav1.OwnerReference, desired metav1.OwnerReference) []metav1.OwnerReference {
	out := make([]metav1.OwnerReference, 0, len(ownerRefs)+1)
	for _, ownerRef := range ownerRefs {
		if ownerRef.APIVersion == desired.APIVersion && ownerRef.Kind == desired.Kind && ownerRef.UID == desired.UID {
			continue
		}
		out = append(out, ownerRef)
	}
	out = append(out, desired)
	return out
}

func fail(reason, format string, args ...any) *compositionFailure {
	return &compositionFailure{reason: reason, message: fmt.Sprintf(format, args...)}
}

func (r *SecretReferenceReconciler) handleTargetWriteFailure(ctx context.Context, ref *cogniv1alpha1.SecretReference, current *corev1.Secret, err error) (ctrl.Result, error) {
	reason := cogniv1alpha1.ReasonWriteFailed
	if apierrors.IsInvalid(err) {
		reason = cogniv1alpha1.ReasonTargetRejected
		if current != nil {
			if deleteErr := r.Delete(ctx, current); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				msg := fmt.Sprintf("failed to delete managed target Secret %s/%s after TargetRejected: %v", ref.Namespace, ref.Name, deleteErr)
				r.event(ref, corev1.EventTypeWarning, cogniv1alpha1.ReasonWriteFailed, msg)
				result, statusErr := r.setFailure(ctx, ref, cogniv1alpha1.ReasonWriteFailed, msg)
				return result, firstErr(deleteErr, statusErr)
			}
		}
	}
	msg := fmt.Sprintf("failed to write target Secret %s/%s: %v", ref.Namespace, ref.Name, err)
	r.event(ref, corev1.EventTypeWarning, reason, msg)
	result, statusErr := r.setFailure(ctx, ref, reason, msg)
	return result, firstErr(err, statusErr)
}

func (r *SecretReferenceReconciler) setReady(ctx context.Context, ref *cogniv1alpha1.SecretReference) (ctrl.Result, error) {
	return r.setCondition(ctx, ref, metav1.Condition{
		Type:               cogniv1alpha1.ReadyConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             cogniv1alpha1.ReasonSynced,
		Message:            "Target Secret is synchronized",
		ObservedGeneration: ref.Generation,
	})
}

func (r *SecretReferenceReconciler) setFailure(ctx context.Context, ref *cogniv1alpha1.SecretReference, reason, message string) (ctrl.Result, error) {
	return r.setCondition(ctx, ref, metav1.Condition{
		Type:               cogniv1alpha1.ReadyConditionType,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: ref.Generation,
	})
}

func (r *SecretReferenceReconciler) setCondition(ctx context.Context, ref *cogniv1alpha1.SecretReference, condition metav1.Condition) (ctrl.Result, error) {
	current := ref.DeepCopy()
	meta.SetStatusCondition(&ref.Status.Conditions, condition)
	if reflect.DeepEqual(current.Status.Conditions, ref.Status.Conditions) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, ref); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *SecretReferenceReconciler) event(ref *cogniv1alpha1.SecretReference, eventType, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(ref, eventType, reason, message)
	}
}

func (r *SecretReferenceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &cogniv1alpha1.SecretReference{}, sourceSecretIndexField, func(obj client.Object) []string {
		ref := obj.(*cogniv1alpha1.SecretReference)
		keys := make([]string, 0, len(ref.Spec.Sources))
		seen := map[string]struct{}{}
		for _, source := range ref.Spec.Sources {
			key := source.Namespace + "/" + source.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		return keys
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cogniv1alpha1.SecretReference{}).
		Owns(&corev1.Secret{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.referencesForSource)).
		Complete(r)
}

func (r *SecretReferenceReconciler) referencesForSource(ctx context.Context, obj client.Object) []reconcile.Request {
	var refs cogniv1alpha1.SecretReferenceList
	indexKey := obj.GetNamespace() + "/" + obj.GetName()
	if err := r.List(ctx, &refs, client.MatchingFields{sourceSecretIndexField: indexKey}); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(refs.Items))
	for _, ref := range refs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}})
	}
	return requests
}

func firstErr(primary, secondary error) error {
	if primary != nil {
		return primary
	}
	return secondary
}

var _ reconcile.Reconciler = &SecretReferenceReconciler{}
