package kuber

import (
	"context"
	"path/filepath"
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

func New() (*Client, error) {
	path := filepath.Join(homedir.HomeDir(), ".kube", "config")

	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{cs: cs}, nil
}

func (c *Client) PrintPods(ctx context.Context, namespace string) error {

	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, p := range pods.Items {
		logger.Log.Info(
			"pod status",
			"name", p.Name,
			"phase", p.Status.Phase,
		)

		for _, st := range p.Status.ContainerStatuses {
			logger.Log.Info(
				"container status",
				"name", st.Name,
				"restarts", st.RestartCount,
			)

			if w := st.State.Waiting; w != nil {
				logger.Log.Warn(
					"container waiting",
					"name", st.Name,
					"reason", w.Reason,
				)
			}

			if t := st.LastTerminationState.Terminated; t != nil {
				logger.Log.Error(
					"container terminated",
					"name", st.Name,
					"reason", t.Reason,
					"exit_code", t.ExitCode,
				)
			}
		}
	}

	return nil
}

func (c *Client) Inputs(ctx context.Context, namespace string) ([]rules.Input, error) {
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var inputs []rules.Input
	for _, p := range pods.Items {
		for _, st := range p.Status.ContainerStatuses {
			reason, exitCode := problem(st)
			if reason == "" {
				continue
			}
			inputs = append(inputs, rules.Input{
				PodName:       p.Name,
				Namespace:     p.Namespace,
				ContainerName: st.Name,
				Reason:        reason,
				ExitCode:      exitCode,
				RestartCount:  st.RestartCount,
				MemoryLimitMi: memoryLimitMi(p, st.Name),
			})
		}
	}
	return inputs, nil
}

func problem(st corev1.ContainerStatus) (string, int32) {
	if t := st.LastTerminationState.Terminated; t != nil && t.Reason == "OOMKilled" {
		return t.Reason, t.ExitCode
	}
	if w := st.State.Waiting; w != nil && w.Reason != "ContainerCreating" {
		return w.Reason, 0
	}
	if t := st.State.Terminated; t != nil && t.Reason != "Completed" {
		return t.Reason, t.ExitCode
	}
	return "", 0
}

func memoryLimitMi(p corev1.Pod, container string) int {
	for _, c := range p.Spec.Containers {
		if c.Name != container {
			continue
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			return int(q.Value() / (1024 * 1024))
		}
	}
	return 0
}
