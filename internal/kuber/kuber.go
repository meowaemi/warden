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
	cs          *kubernetes.Clientset
	recentCrash time.Duration
}

// New creates a client from ~/.kube/config. It does not contact the cluster.
// A container that failed and is running again is still reported for
// recentCrash after its last failure.
func New(recentCrash time.Duration) (*Client, error) {
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
	return &Client{cs: cs, recentCrash: recentCrash}, nil
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
		inputs = c.appendContainers(inputs, p, p.Status.InitContainerStatuses, p.Spec.InitContainers)
		inputs = c.appendContainers(inputs, p, p.Status.ContainerStatuses, p.Spec.Containers)
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
func (c *Client) appendContainers(inputs []rules.Input, p corev1.Pod, statuses []corev1.ContainerStatus, specs []corev1.Container) []rules.Input {
	for _, st := range statuses {
		pr, ok := c.problem(st)
		if !ok {
			continue
		}
		inputs = append(inputs, rules.Input{
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

// containerProblem is what problem found about one container.
type containerProblem struct {
	reason    string
	message   string
	exitCode  int32
	since     time.Time // zero if Kubernetes has no time for it
	recovered bool      // the container failed but is running again
}

// problem reports whether the container has a problem and describes it.
//
// A container that does not run is a problem for as long as it does not run.
// A container that failed and is running again is still reported, marked as
// recovered, until recentCrash has passed since its last failure.
//
// While a container keeps crashing, Kubernetes switches it between a
// terminated state and Waiting with CrashLoopBackOff. In both cases the
// reason of the last termination (Error, OOMKilled, ...) is reported, so the
// problem keeps the same reason instead of flapping between the two.
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

// terminated describes a termination. message replaces the termination's own
// message when the termination has none.
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
