// Package kuber reads problems from Kubernetes and applies commands to it.
package kuber

import (
	"context"
	"fmt"
	"time"
	"warden/internal/logger"
	"warden/internal/rules"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client talks to one cluster. A container that crashed less than
// recentCrash ago is still reported after it runs again.
type Client struct {
	cs          kubernetes.Interface
	recentCrash time.Duration
}

// New connects using kubeconfig and kubeContext. Empty values mean
// $KUBECONFIG or ~/.kube/config with its current context, then the
// in-cluster service account.
func New(kubeconfig, kubeContext string, recentCrash time.Duration) (*Client, error) {
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loading.ExplicitPath = kubeconfig
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{CurrentContext: kubeContext})

	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig %q context %q: %w", kubeconfig, kubeContext, err)
	}
	cfg.Timeout = 10 * time.Second

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset for %s: %w", cfg.Host, err)
	}

	if kubeContext == "" {
		if raw, err := cc.RawConfig(); err == nil {
			kubeContext = raw.CurrentContext
		}
	}
	logger.Log.Info("kube client created", "kubeconfig", kubeconfig, "context", kubeContext, "host", cfg.Host)
	return &Client{cs: cs, recentCrash: recentCrash}, nil
}

// Ping returns the API server version.
func (c *Client) Ping(ctx context.Context) (string, error) {
	v, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("reach API server: %w", err)
	}
	return v.GitVersion, nil
}

// Inputs lists the problems of all nodes and of the pods in namespace,
// all namespaces if it is empty. Each pod problem names the pod's owner.
func (c *Client) Inputs(ctx context.Context, namespace string) ([]rules.Input, error) {
	start := time.Now()
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in namespace %q after %s: %w",
			NamespaceLabel(namespace), time.Since(start).Round(time.Millisecond), err)
	}

	replicaSets, err := c.cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replica sets in namespace %q after %s: %w",
			NamespaceLabel(namespace), time.Since(start).Round(time.Millisecond), err)
	}
	owners := map[types.UID]*metav1.OwnerReference{}
	for i := range replicaSets.Items {
		rs := &replicaSets.Items[i]
		owners[rs.UID] = metav1.GetControllerOf(rs)
	}

	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes after %s: %w", time.Since(start).Round(time.Millisecond), err)
	}

	var inputs []rules.Input
	for _, n := range nodes.Items {
		inputs = append(inputs, nodeProblems(n)...)
	}
	for _, p := range pods.Items {
		first := len(inputs)
		if in, ok := unschedulable(p); ok {
			inputs = append(inputs, in)
		}
		inputs = c.appendContainers(inputs, p, p.Status.InitContainerStatuses, p.Spec.InitContainers)
		inputs = c.appendContainers(inputs, p, p.Status.ContainerStatuses, p.Spec.Containers)
		kind, name, uid := owner(p, owners)
		for i := first; i < len(inputs); i++ {
			inputs[i].OwnerKind, inputs[i].OwnerName, inputs[i].OwnerUID = kind, name, uid
		}
	}
	return inputs, nil
}

// NamespaceLabel names a namespace for logs; the empty one is "all".
func NamespaceLabel(namespace string) string {
	if namespace == metav1.NamespaceAll {
		return "all"
	}
	return namespace
}

// owner returns the workload that controls a pod. A ReplicaSet is resolved
// to its Deployment: changing the ReplicaSet would be undone.
func owner(p corev1.Pod, replicaSets map[types.UID]*metav1.OwnerReference) (kind, name, uid string) {
	ref := metav1.GetControllerOf(&p)
	if ref == nil {
		return "", "", ""
	}
	if ref.Kind == "ReplicaSet" {
		if d := replicaSets[ref.UID]; d != nil {
			ref = d
		}
	}
	return ref.Kind, ref.Name, string(ref.UID)
}

// nodeProblems reports a node that is not ready, unreachable, under
// resource pressure or cordoned.
func nodeProblems(n corev1.Node) []rules.Input {
	var inputs []rules.Input
	for _, cond := range n.Status.Conditions {
		reason := ""
		switch {
		case cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionFalse:
			reason = rules.NodeNotReady
		case cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionUnknown:
			reason = rules.NodeUnreachable
		case cond.Type == corev1.NodeMemoryPressure && cond.Status == corev1.ConditionTrue:
			reason = rules.MemoryPressure
		case cond.Type == corev1.NodeDiskPressure && cond.Status == corev1.ConditionTrue:
			reason = rules.DiskPressure
		case cond.Type == corev1.NodePIDPressure && cond.Status == corev1.ConditionTrue:
			reason = rules.PIDPressure
		}
		if reason == "" {
			continue
		}
		inputs = append(inputs, rules.Input{
			NodeName: n.Name,
			Reason:   reason,
			Message:  cond.Message,
			Since:    cond.LastTransitionTime.Time,
		})
	}
	if n.Spec.Unschedulable {
		inputs = append(inputs, rules.Input{NodeName: n.Name, Reason: rules.NodeCordoned})
	}
	return inputs
}

// unschedulable reports a pod the scheduler cannot place.
func unschedulable(p corev1.Pod) (rules.Input, bool) {
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodScheduled &&
			cond.Status == corev1.ConditionFalse &&
			cond.Reason == corev1.PodReasonUnschedulable {
			return rules.Input{
				PodName:   p.Name,
				Namespace: p.Namespace,
				Reason:    cond.Reason,
				Message:   cond.Message,
				Since:     cond.LastTransitionTime.Time,
			}, true
		}
	}
	return rules.Input{}, false
}

// appendContainers adds the problems of the pod's failing containers.
func (c *Client) appendContainers(inputs []rules.Input, p corev1.Pod, statuses []corev1.ContainerStatus, specs []corev1.Container) []rules.Input {
	for _, st := range statuses {
		pr, ok := c.problem(st)
		if !ok {
			continue
		}
		inputs = append(inputs, rules.Input{
			NodeName:      p.Spec.NodeName,
			PodName:       p.Name,
			Namespace:     p.Namespace,
			ContainerName: st.Name,
			Reason:        pr.reason,
			Message:       pr.message,
			ExitCode:      pr.exitCode,
			RestartCount:  st.RestartCount,
			Since:         pr.since,
			Recovered:     pr.recovered,
			MemoryLimitMi: memoryLimitMi(specs, st.Name),
		})
	}
	return inputs
}

// containerProblem is why a container fails.
type containerProblem struct {
	reason    string
	message   string
	exitCode  int32
	since     time.Time
	recovered bool
}

// problem reports whether a container is failing and why. A crash loop
// is reported by the cause of the last crash, not by the back-off.
func (c *Client) problem(st corev1.ContainerStatus) (containerProblem, bool) {
	last := st.LastTerminationState.Terminated
	failed := last != nil && last.Reason != "Completed"

	switch {
	case st.State.Waiting != nil:
		w := st.State.Waiting
		switch {
		case w.Reason == "ContainerCreating":
			return containerProblem{}, false
		case w.Reason == "CrashLoopBackOff" && failed:
			return terminated(last, w.Message), true
		default:
			return containerProblem{reason: w.Reason, message: w.Message}, true
		}

	case st.State.Terminated != nil:
		if st.State.Terminated.Reason == "Completed" {
			return containerProblem{}, false
		}
		return terminated(st.State.Terminated, ""), true

	case st.State.Running != nil && failed && time.Since(last.FinishedAt.Time) < c.recentCrash:
		pr := terminated(last, "")
		pr.recovered = true
		return pr, true
	}

	return containerProblem{}, false
}

// terminated describes a terminated state; its own message wins over message.
func terminated(t *corev1.ContainerStateTerminated, message string) containerProblem {
	if t.Message != "" {
		message = t.Message
	}
	return containerProblem{
		reason:   t.Reason,
		message:  message,
		exitCode: t.ExitCode,
		since:    t.FinishedAt.Time,
	}
}

// memoryLimitMi returns a container's memory limit in MiB, 0 if it has none.
func memoryLimitMi(specs []corev1.Container, container string) int {
	for _, c := range specs {
		if c.Name != container {
			continue
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			return int(q.Value() / (1024 * 1024))
		}
	}
	return 0
}
