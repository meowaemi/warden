package kuber

import (
	"context"
	"testing"
	"warden/internal/rules"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func node(name string, unschedulable bool, conds ...corev1.NodeCondition) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
		Status:     corev1.NodeStatus{Conditions: conds},
	}
}

func cond(t corev1.NodeConditionType, s corev1.ConditionStatus) corev1.NodeCondition {
	return corev1.NodeCondition{Type: t, Status: s, Message: string(t) + " is " + string(s)}
}

func TestNodeProblems(t *testing.T) {
	cs := fake.NewSimpleClientset(
		node("healthy", false, cond(corev1.NodeReady, corev1.ConditionTrue), cond(corev1.NodeMemoryPressure, corev1.ConditionFalse)),
		node("down", false, cond(corev1.NodeReady, corev1.ConditionUnknown)),
		node("broken", true, cond(corev1.NodeReady, corev1.ConditionFalse), cond(corev1.NodeDiskPressure, corev1.ConditionTrue)),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "prod"},
			Spec:       corev1.PodSpec{NodeName: "broken"},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}}},
		},
	)
	c := &Client{cs: cs}
	inputs, err := c.Inputs(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]rules.Input{}
	for _, in := range inputs {
		got[in.NodeName+"/"+in.PodName+"/"+in.Reason] = in
	}
	for _, key := range []string{
		"down//" + rules.NodeUnreachable,
		"broken//" + rules.NodeNotReady,
		"broken//" + rules.DiskPressure,
		"broken//" + rules.NodeCordoned,
		"broken/app/ImagePullBackOff",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s in %v", key, got)
		}
	}
	if len(got) != 5 {
		t.Errorf("healthy node or false condition reported: %v", got)
	}
}
