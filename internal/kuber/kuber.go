package kuber

import (
	"context"
	"fmt"
	"time"
	"warden/internal/logger"
	"warden/internal/rules"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	cs          kubernetes.Interface
	recentCrash time.Duration
}

// New connects to the cluster described by kubeconfig and kubeContext. An empty
// kubeconfig means the usual lookup: $KUBECONFIG, then ~/.kube/config; an
// empty context means the kubeconfig's current one. If no kubeconfig is
// found, the in-cluster service account of the pod is used, so an agent
// deployed inside its own cluster needs no flags.
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
func (c *Client) Ping(ctx context.Context) (string, error) {
	v, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("reach API server: %w", err)
	}
	return v.GitVersion, nil
}
func (c *Client) Inputs(ctx context.Context, namespace string) ([]rules.Input, error) {
	start := time.Now()
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in namespace %q after %s: %w",
			NamespaceLabel(namespace), time.Since(start).Round(time.Millisecond), err)
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
		if in, ok := unschedulable(p); ok {
			inputs = append(inputs, in)
		}
		inputs = c.appendContainers(inputs, p, p.Status.InitContainerStatuses, p.Spec.InitContainers)
		inputs = c.appendContainers(inputs, p, p.Status.ContainerStatuses, p.Spec.Containers)
	}
	return inputs, nil
}
func NamespaceLabel(namespace string) string {
	if namespace == metav1.NamespaceAll {
		return "all"
	}
	return namespace
}
// nodeProblems returns the problems of one node (server): its Ready
// condition, resource pressure and cordoning. Nodes are cluster-wide, so they
// are checked whatever namespace warden watches.
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

type containerProblem struct {
	reason    string
	message   string
	exitCode  int32
	since     time.Time // zero if Kubernetes has no time for it
	recovered bool      // the container failed but is running again
}

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
