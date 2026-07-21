package controller

import (
	"context"
	"strings"
	"testing"

	cogniv1beta1 "github.com/cognilabz/cognisecrets/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileCreatesComposedTarget(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "application-credentials", []cogniv1beta1.SecretSource{
		{
			Namespace: "shared",
			Name:      "database",
			Keys: []cogniv1beta1.SecretKeyMapping{
				{Name: "username"},
				{Name: "password", Target: "DB_PASSWORD"},
			},
		},
	})
	source := newSource("shared", "database", "application", map[string][]byte{
		"username": []byte("app"),
		"password": {0x00, 0xff, 0x10},
		"ignored":  []byte("not copied"),
	})

	reconciler := newTestReconciler(t, ref, source)
	reconcileOnce(t, ctx, reconciler, ref)

	var target corev1.Secret
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "application-credentials"}, &target); err != nil {
		t.Fatalf("expected target Secret: %v", err)
	}
	assertData(t, target.Data, map[string][]byte{
		"username":    []byte("app"),
		"DB_PASSWORD": {0x00, 0xff, 0x10},
	})
	if target.Type != corev1.SecretTypeOpaque {
		t.Fatalf("target type = %q, want %q", target.Type, corev1.SecretTypeOpaque)
	}
	if target.Labels[cogniv1beta1.ManagedByLabel] != cogniv1beta1.ManagedByValue {
		t.Fatalf("managed-by label missing: %#v", target.Labels)
	}
	if !ownedBy(&target, ref) {
		t.Fatalf("target Secret does not have controller owner reference for SecretReference")
	}
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionTrue, cogniv1beta1.ReasonSynced)
}

func TestReconcileReportsForeignTargetBeforeSourceErrors(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "missing"}})
	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "application"},
		Data:       map[string][]byte{"keep": []byte("me")},
	}
	reconciler := newTestReconciler(t, ref, foreign)

	reconcileOnce(t, ctx, reconciler, ref)

	var target corev1.Secret
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &target); err != nil {
		t.Fatalf("expected foreign target to remain: %v", err)
	}
	assertData(t, target.Data, map[string][]byte{"keep": []byte("me")})
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonTargetAlreadyExists)
}

func TestReconcileDeletesManagedTargetOnAccessDenied(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "database"}})
	source := newSource("shared", "database", "reporting", map[string][]byte{"password": []byte("secret")})
	target := newManagedTarget(ref, map[string][]byte{"password": []byte("stale")})
	reconciler := newTestReconciler(t, ref, source, target)

	reconcileOnce(t, ctx, reconciler, ref)

	var deleted corev1.Secret
	err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &deleted)
	if err == nil {
		t.Fatalf("expected managed target to be deleted")
	}
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonAccessDenied)
}

func TestReconcileReportsSourceNotFound(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "database"}})
	reconciler := newTestReconciler(t, ref)

	reconcileOnce(t, ctx, reconciler, ref)

	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonSourceNotFound)
}

func TestReconcileDeletesManagedTargetOnMissingKey(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{
		Namespace: "shared",
		Name:      "database",
		Keys:      []cogniv1beta1.SecretKeyMapping{{Name: "password"}},
	}})
	source := newSource("shared", "database", "application", map[string][]byte{"username": []byte("app")})
	target := newManagedTarget(ref, map[string][]byte{"password": []byte("stale")})
	reconciler := newTestReconciler(t, ref, source, target)

	reconcileOnce(t, ctx, reconciler, ref)

	var deleted corev1.Secret
	err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &deleted)
	if err == nil {
		t.Fatalf("expected managed target to be deleted")
	}
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonSourceKeyNotFound)
}

func TestReconcileRejectsDuplicateTargetKeyWithContributors(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{
		{
			Namespace: "shared",
			Name:      "database",
			Keys:      []cogniv1beta1.SecretKeyMapping{{Name: "password", Target: "PASSWORD"}},
		},
		{
			Namespace: "shared",
			Name:      "messaging",
			Keys:      []cogniv1beta1.SecretKeyMapping{{Name: "token", Target: "PASSWORD"}},
		},
	})
	database := newSource("shared", "database", "application", map[string][]byte{"password": []byte("a")})
	messaging := newSource("shared", "messaging", "application", map[string][]byte{"token": []byte("b")})
	reconciler := newTestReconciler(t, ref, database, messaging)

	reconcileOnce(t, ctx, reconciler, ref)

	var target corev1.Secret
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &target); err == nil {
		t.Fatalf("expected no target Secret when duplicate target key exists")
	}
	condition := readyCondition(t, reconciler.Client, ctx, ref)
	if condition.Reason != cogniv1beta1.ReasonDuplicateTargetKey {
		t.Fatalf("reason = %q, want %q", condition.Reason, cogniv1beta1.ReasonDuplicateTargetKey)
	}
	for _, want := range []string{"PASSWORD", "shared/database key password", "shared/messaging key token"} {
		if !strings.Contains(condition.Message, want) {
			t.Fatalf("message %q does not contain %q", condition.Message, want)
		}
	}
}

func TestReconcileRejectsManagedSourceByOwnerReference(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "database"}})
	sourceOwner := newReference("shared", "database", nil)
	source := newSource("shared", "database", "application", map[string][]byte{"password": []byte("secret")})
	source.OwnerReferences = []metav1.OwnerReference{ownerRef(sourceOwner)}
	reconciler := newTestReconciler(t, ref, source)

	reconcileOnce(t, ctx, reconciler, ref)

	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonManagedSourceRejected)
}

func TestReconcilePreservesUnmanagedTargetMetadata(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "database"}})
	source := newSource("shared", "database", "application", map[string][]byte{"password": []byte("fresh")})
	target := newManagedTarget(ref, map[string][]byte{"password": []byte("stale")})
	target.Labels["custom"] = "label"
	target.Annotations = map[string]string{"custom": "annotation"}
	target.Finalizers = []string{"example.com/finalizer"}
	target.OwnerReferences = append(target.OwnerReferences, metav1.OwnerReference{
		APIVersion: "example.com/v1",
		Kind:       "Other",
		Name:       "owner",
		UID:        types.UID("other"),
	})
	reconciler := newTestReconciler(t, ref, source, target)

	reconcileOnce(t, ctx, reconciler, ref)

	var updated corev1.Secret
	if err := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &updated); err != nil {
		t.Fatalf("expected updated target: %v", err)
	}
	assertData(t, updated.Data, map[string][]byte{"password": []byte("fresh")})
	if updated.Labels["custom"] != "label" || updated.Annotations["custom"] != "annotation" {
		t.Fatalf("unmanaged metadata not preserved: labels=%#v annotations=%#v", updated.Labels, updated.Annotations)
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != "example.com/finalizer" {
		t.Fatalf("finalizers not preserved: %#v", updated.Finalizers)
	}
	if len(updated.OwnerReferences) != 2 {
		t.Fatalf("owner references not preserved: %#v", updated.OwnerReferences)
	}
}

func TestReconcileReplacesManagedTargetWhenTypeChanges(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{
		Namespace: "shared",
		Name:      "database",
		Keys: []cogniv1beta1.SecretKeyMapping{
			{Name: "username"},
			{Name: "password"},
		},
	}})
	ref.Spec.Type = corev1.SecretTypeBasicAuth
	source := newSource("shared", "database", "application", map[string][]byte{
		"username": []byte("app"),
		"password": []byte("secret"),
	})
	target := newManagedTarget(ref, map[string][]byte{"username": []byte("stale")})
	reconciler := newTestReconciler(t, ref, source, target)

	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !result.Requeue {
		t.Fatalf("expected reconcile to requeue after deleting immutable type")
	}
	var deleted corev1.Secret
	if getErr := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &deleted); getErr == nil {
		t.Fatalf("expected old typed target to be deleted")
	}

	reconcileOnce(t, ctx, reconciler, ref)

	var replaced corev1.Secret
	if getErr := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &replaced); getErr != nil {
		t.Fatalf("expected replacement target: %v", getErr)
	}
	if replaced.Type != corev1.SecretTypeBasicAuth {
		t.Fatalf("target type = %q, want %q", replaced.Type, corev1.SecretTypeBasicAuth)
	}
	assertData(t, replaced.Data, map[string][]byte{
		"username": []byte("app"),
		"password": []byte("secret"),
	})
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionTrue, cogniv1beta1.ReasonSynced)
}

func TestTargetRejectedDeletesExistingManagedTarget(t *testing.T) {
	ctx := context.Background()
	ref := newReference("application", "database", []cogniv1beta1.SecretSource{{Namespace: "shared", Name: "database"}})
	target := newManagedTarget(ref, map[string][]byte{"password": []byte("stale")})
	reconciler := newTestReconciler(t, ref, target)
	err := apierrors.NewInvalid(schema.GroupKind{Group: "", Kind: "Secret"}, "database", field.ErrorList{
		field.Required(field.NewPath("data").Key(".dockerconfigjson"), "required for kubernetes.io/dockerconfigjson"),
	})

	_, gotErr := reconciler.handleTargetWriteFailure(ctx, ref, target, err)
	if gotErr == nil {
		t.Fatalf("expected target rejection error to be returned")
	}
	var deleted corev1.Secret
	if getErr := reconciler.Get(ctx, types.NamespacedName{Namespace: "application", Name: "database"}, &deleted); getErr == nil {
		t.Fatalf("expected managed target to be deleted after TargetRejected")
	}
	assertReady(t, reconciler.Client, ctx, ref, metav1.ConditionFalse, cogniv1beta1.ReasonTargetRejected)
}

func TestAllowsNamespaceParsing(t *testing.T) {
	if !allowsNamespace(" , INVALID_!, application,application", "application") {
		t.Fatalf("expected application to be allowed")
	}
	if allowsNamespace("*, reporting", "application") {
		t.Fatalf("wildcard must not authorize access")
	}
	if allowsNamespace("invalid_!", "application") {
		t.Fatalf("invalid namespace entry must be ignored")
	}
}

func newTestReconciler(t *testing.T, objects ...client.Object) *SecretReferenceReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := cogniv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add CogniSecrets scheme: %v", err)
	}
	return &SecretReferenceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&cogniv1beta1.SecretReference{}).
			WithObjects(objects...).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(32),
	}
}

func reconcileOnce(t *testing.T, ctx context.Context, reconciler *SecretReferenceReconciler, ref *cogniv1beta1.SecretReference) {
	t.Helper()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
}

func newReference(namespace, name string, sources []cogniv1beta1.SecretSource) *cogniv1beta1.SecretReference {
	return &cogniv1beta1.SecretReference{
		TypeMeta: metav1.TypeMeta{
			APIVersion: cogniv1beta1.GroupVersion.String(),
			Kind:       "SecretReference",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			UID:        uuid.NewUUID(),
			Generation: 1,
		},
		Spec: cogniv1beta1.SecretReferenceSpec{
			Type:    corev1.SecretTypeOpaque,
			Sources: sources,
		},
	}
}

func newSource(namespace, name, allowedNamespaces string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				cogniv1beta1.AuthorizationAnnotation: allowedNamespaces,
			},
		},
		Data: copySecretData(data),
		Type: corev1.SecretTypeDockerConfigJson,
	}
}

func newManagedTarget(ref *cogniv1beta1.SecretReference, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Labels: map[string]string{
				cogniv1beta1.ManagedByLabel: cogniv1beta1.ManagedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef(ref)},
		},
		Data: copySecretData(data),
		Type: corev1.SecretTypeOpaque,
	}
}

func assertReady(t *testing.T, c client.Client, ctx context.Context, ref *cogniv1beta1.SecretReference, status metav1.ConditionStatus, reason string) {
	t.Helper()
	condition := readyCondition(t, c, ctx, ref)
	if condition.Status != status || condition.Reason != reason {
		t.Fatalf("Ready = %s/%s, want %s/%s; message=%q", condition.Status, condition.Reason, status, reason, condition.Message)
	}
	if condition.ObservedGeneration != ref.Generation {
		t.Fatalf("observedGeneration = %d, want %d", condition.ObservedGeneration, ref.Generation)
	}
}

func readyCondition(t *testing.T, c client.Client, ctx context.Context, ref *cogniv1beta1.SecretReference) metav1.Condition {
	t.Helper()
	var current cogniv1beta1.SecretReference
	if err := c.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &current); err != nil {
		t.Fatalf("get SecretReference: %v", err)
	}
	for _, condition := range current.Status.Conditions {
		if condition.Type == cogniv1beta1.ReadyConditionType {
			return condition
		}
	}
	t.Fatalf("Ready condition not found: %#v", current.Status.Conditions)
	return metav1.Condition{}
}

func assertData(t *testing.T, got, want map[string][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("data length = %d, want %d; got %#v", len(got), len(want), got)
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			t.Fatalf("missing data key %q", key)
		}
		if string(gotValue) != string(wantValue) {
			t.Fatalf("data[%q] = %#v, want %#v", key, gotValue, wantValue)
		}
	}
}
