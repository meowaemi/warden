package kuber

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
	"warden/internal/logger"
	"warden/internal/rules"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	cs *kubernetes.Clientset
}

// New creates a client from ~/.kube/config. It does not contact the cluster.
func New() (*Client, error) {
	path := filepath.Join(homedir.HomeDir(), ".kube", "config")

	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig %s: %w", path, err)
	}
	cfg.Timeout = 10 * time.Second

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset for %s: %w", cfg.Host, err)
	}

	logger.Log.Info("kube client created", "kubeconfig", path, "host", cfg.Host)
	return &Client{cs: cs}, nil
}

// Ping checks that the cluster answers by asking the API server for its
// version, and returns that version.
func (c *Client) Ping(ctx context.Context) (string, error) {
	v, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("reach API server: %w", err)
	}
	return v.GitVersion, nil
}

// Inputs lists pods in namespace and returns one input per problem found:
// a pod that cannot be scheduled, or a problematic init or regular container.
// An empty namespace (metav1.NamespaceAll) means all namespaces.
func (c *Client) Inputs(ctx context.Context, namespace string) ([]rules.Input, error) {
	start := time.Now()
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in namespace %q after %s: %w",
			NamespaceLabel(namespace), time.Since(start).Round(time.Millisecond), err)
	}

	var inputs []rules.Input
	for _, p := range pods.Items {
		if in, ok := unschedulable(p); ok {
			inputs = append(inputs, in)
		}
		inputs = appendContainers(inputs, p, p.Status.InitContainerStatuses, p.Spec.InitContainers)
		inputs = appendContainers(inputs, p, p.Status.ContainerStatuses, p.Spec.Containers)
	}
	return inputs, nil
}

// NamespaceLabel returns namespace for logging, with "all" for the empty one.
func NamespaceLabel(namespace string) string {
	if namespace == metav1.NamespaceAll {
		return "all"
	}
	return namespace
}

// unschedulable reports a pod the scheduler cannot place on any node.
// Such a pod has no container statuses yet, so it is checked separately.
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

// appendContainers adds an input for every problematic container in statuses.
// specs are the matching container definitions, used to read memory limits.
func appendContainers(inputs []rules.Input, p corev1.Pod, statuses []corev1.ContainerStatus, specs []corev1.Container) []rules.Input {
	for _, st := range statuses {
		reason, message, exitCode, since := problem(st)
		if reason == "" {
			continue
		}
		inputs = append(inputs, rules.Input{
			PodName:       p.Name,
			Namespace:     p.Namespace,
			ContainerName: st.Name,
			Reason:        reason,
			Message:       message,
			ExitCode:      exitCode,
			RestartCount:  st.RestartCount,
			Since:         since,
			MemoryLimitMi: memoryLimitMi(specs, st.Name),
		})
	}
	return inputs
}

// oomRecent is how long a container that was OOMKilled and then recovered is
// still reported, so the problem does not vanish before anyone sees it.
const oomRecent = 10 * time.Minute

// problem returns the reason, the Kubernetes message, the exit code and the
// time Kubernetes recorded for the container's problem, or an empty reason if
// the container is healthy. The time is zero when Kubernetes has none.
//
// Kubernetes keeps the last termination until the container stops again, so
// an OOM kill stays there long after the container has recovered. It counts
// as a problem only while the container is not ready or for oomRecent after
// the kill.
func problem(st corev1.ContainerStatus) (string, string, int32, time.Time) {
	if t := st.LastTerminationState.Terminated; t != nil && t.Reason == "OOMKilled" &&
		(!st.Ready || time.Since(t.FinishedAt.Time) < oomRecent) {
		return t.Reason, t.Message, t.ExitCode, t.FinishedAt.Time
	}
	if w := st.State.Waiting; w != nil && w.Reason != "ContainerCreating" {
		return w.Reason, w.Message, 0, time.Time{}
	}
	if t := st.State.Terminated; t != nil && t.Reason != "Completed" {
		return t.Reason, t.Message, t.ExitCode, t.FinishedAt.Time
	}
	return "", "", 0, time.Time{}
}

// memoryLimitMi returns the memory limit of the named container in MiB,
// or 0 if it has none.
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
