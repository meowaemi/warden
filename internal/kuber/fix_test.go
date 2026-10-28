package kuber

import (
	"context"
	"testing"
	"time"
	"warden/internal/control"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// statefulSet returns a StatefulSet with an init container, two containers
// and an annotation that must survive a fix.
func statefulSet() *appsv1.StatefulSet {
	two := int32(2)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "prod", UID: "uid-db", ResourceVersion: "7", Annotations: map[string]string{"keep": "me"}},
		Spec: appsv1.StatefulSetSpec{Replicas: &two, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "migrate"}},
			Containers: []corev1.Container{
				{Name: "sidecar"},
				{Name: "db", Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")},
				}},
			},
		}}},
	}
}

// fixCommand returns a command of kind for that StatefulSet.
func fixCommand(kind string) control.Command {
	return control.Command{ID: "fix-1", Kind: kind, TargetKind: control.StatefulSet, Namespace: "prod", Name: "db", TargetUID: "uid-db", ExpiresAt: time.Now().Add(time.Minute)}
}

// TestSetResources checks that resources are merged into the right container,
// the command ID goes to the object and not to the pod template, a repeat
// only reads, and init and missing containers are handled.
func TestSetResources(t *testing.T) {
	cs := fake.NewSimpleClientset(statefulSet())
	c := &Client{cs: cs}
	cmd := fixCommand(control.KindSetResources)
	cmd.Container = "db"
	cmd.Resources = &control.Resources{MemoryLimit: "256Mi", MemoryRequest: "192Mi"}

	if r := c.ExecuteCommand(context.Background(), cmd); r.Status != "applied" {
		t.Fatalf("%+v", r)
	}
	s, _ := cs.AppsV1().StatefulSets("prod").Get(context.Background(), "db", metav1.GetOptions{})
	res := s.Spec.Template.Spec.Containers[1].Resources
	if res.Limits.Memory().String() != "256Mi" || res.Requests.Memory().String() != "192Mi" || res.Requests.Cpu().String() != "250m" {
		t.Fatalf("resources not merged: %+v", res)
	}
	if s.Annotations["keep"] != "me" || s.Annotations[commandAnnotation] != "fix-1" || s.Spec.Template.Annotations[commandAnnotation] != "" {
		t.Fatalf("annotations: object %v, template %v", s.Annotations, s.Spec.Template.Annotations)
	}

	before := len(cs.Actions())
	if r := c.ExecuteCommand(context.Background(), cmd); r.Status != "applied" || len(cs.Actions()) != before+1 {
		t.Fatalf("a repeated command must only read: %+v, %d actions", r, len(cs.Actions())-before)
	}

	init := fixCommand(control.KindSetResources)
	init.ID, init.Container, init.Resources = "fix-2", "migrate", &control.Resources{CPULimit: "500m"}
	if r := c.ExecuteCommand(context.Background(), init); r.Status != "applied" {
		t.Fatalf("init container: %+v", r)
	}
	missing := fixCommand(control.KindSetResources)
	missing.ID, missing.Container, missing.Resources = "fix-3", "nope", &control.Resources{CPULimit: "1"}
	if r := c.ExecuteCommand(context.Background(), missing); r.Status != "failed" {
		t.Fatalf("missing container: %+v", r)
	}
}

// TestScale checks that replicas change without touching the pod template,
// and that a replaced object is refused.
func TestScale(t *testing.T) {
	cs := fake.NewSimpleClientset(statefulSet())
	c := &Client{cs: cs}
	cmd := fixCommand(control.KindScale)
	five := int32(5)
	cmd.Replicas = &five
	if r := c.ExecuteCommand(context.Background(), cmd); r.Status != "applied" {
		t.Fatalf("%+v", r)
	}
	s, _ := cs.AppsV1().StatefulSets("prod").Get(context.Background(), "db", metav1.GetOptions{})
	if *s.Spec.Replicas != 5 || s.Spec.Template.Annotations[commandAnnotation] != "" {
		t.Fatalf("replicas %d, template annotations %v", *s.Spec.Replicas, s.Spec.Template.Annotations)
	}
	other := cmd
	other.ID, other.TargetUID = "fix-4", "uid-old"
	if r := c.ExecuteCommand(context.Background(), other); r.Status != "failed" {
		t.Fatalf("replaced target accepted: %+v", r)
	}
}

// TestOwnerResolution checks that the owner of a pod is its Deployment
// through the ReplicaSet, its StatefulSet, or none for a bare pod.
func TestOwnerResolution(t *testing.T) {
	controller := true
	ref := func(kind, name, uid string) []metav1.OwnerReference {
		return []metav1.OwnerReference{{Kind: kind, Name: name, UID: types.UID(uid), Controller: &controller}}
	}
	failing := corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}}
	cs := fake.NewSimpleClientset(
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "api-7f", Namespace: "prod", UID: "uid-rs", OwnerReferences: ref("Deployment", "api", "uid-api")}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7f-x", Namespace: "prod", OwnerReferences: ref("ReplicaSet", "api-7f", "uid-rs")}, Status: failing},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: "prod", OwnerReferences: ref("StatefulSet", "db", "uid-db")}, Status: failing},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bare", Namespace: "prod"}, Status: failing},
	)
	inputs, err := (&Client{cs: cs}).Inputs(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"api-7f-x": "Deployment/api/uid-api", "db-0": "StatefulSet/db/uid-db", "bare": "//"}
	for _, in := range inputs {
		if got := in.OwnerKind + "/" + in.OwnerName + "/" + in.OwnerUID; got != want[in.PodName] {
			t.Errorf("%s: owner %s, want %s", in.PodName, got, want[in.PodName])
		}
	}
	if len(inputs) != 3 {
		t.Fatalf("inputs: %+v", inputs)
	}
}
