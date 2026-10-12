package kuber

import (
	"context"
	"path/filepath"
	"warden/internal/logger"

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
