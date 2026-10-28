package kuber

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"warden/internal/control"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// commandAnnotation holds the ID of the last applied command; finding it
// means the command is already applied. A restart writes it to the pod
// template, which triggers the rollout; other commands write it to the
// workload's own metadata to avoid an extra rollout.
const commandAnnotation = "warden.command-id"

// workload is a Deployment, StatefulSet or DaemonSet as ExecuteCommand needs it.
type workload struct {
	uid             string
	resourceVersion string
	annotations     map[string]string
	template        corev1.PodTemplateSpec
	patch           func(ctx context.Context, data []byte) error
}

// workload reads the target object.
func (c *Client) workload(ctx context.Context, kind, namespace, name string) (workload, error) {
	apps := c.cs.AppsV1()
	get, opts := metav1.GetOptions{}, metav1.PatchOptions{}
	switch kind {
	case control.Deployment:
		d, err := apps.Deployments(namespace).Get(ctx, name, get)
		if err != nil {
			return workload{}, err
		}
		return workload{string(d.UID), d.ResourceVersion, d.Annotations, d.Spec.Template, func(ctx context.Context, data []byte) error {
			_, err := apps.Deployments(namespace).Patch(ctx, name, types.JSONPatchType, data, opts)
			return err
		}}, nil
	case control.StatefulSet:
		s, err := apps.StatefulSets(namespace).Get(ctx, name, get)
		if err != nil {
			return workload{}, err
		}
		return workload{string(s.UID), s.ResourceVersion, s.Annotations, s.Spec.Template, func(ctx context.Context, data []byte) error {
			_, err := apps.StatefulSets(namespace).Patch(ctx, name, types.JSONPatchType, data, opts)
			return err
		}}, nil
	case control.DaemonSet:
		d, err := apps.DaemonSets(namespace).Get(ctx, name, get)
		if err != nil {
			return workload{}, err
		}
		return workload{string(d.UID), d.ResourceVersion, d.Annotations, d.Spec.Template, func(ctx context.Context, data []byte) error {
			_, err := apps.DaemonSets(namespace).Patch(ctx, name, types.JSONPatchType, data, opts)
			return err
		}}, nil
	}
	return workload{}, fmt.Errorf("unsupported target kind %q", kind)
}

// ExecuteCommand applies a command. It patches only the object the operator
// chose (same UID) and only if nobody changed it since the read (same
// resourceVersion). A failure that may still have been applied is unknown.
func (c *Client) ExecuteCommand(ctx context.Context, cmd control.Command) control.Result {
	result := control.Result{CommandID: cmd.ID, FinishedAt: time.Now().UTC()}
	finish := func(status, note string) control.Result {
		result.Status = status
		result.Note = note
		result.FinishedAt = time.Now().UTC()
		return result
	}
	if err := cmd.Validate(); err != nil {
		return finish("failed", "unsupported or incomplete command: "+err.Error())
	}
	if !time.Now().Before(cmd.ExpiresAt) {
		return finish("expired", "deadline passed before reading target")
	}
	w, err := c.workload(ctx, cmd.Workload(), cmd.Namespace, cmd.Name)
	if err != nil {
		return finish("failed", "target read failed; no write attempted: "+err.Error())
	}
	if w.uid != cmd.TargetUID {
		return finish("failed", "target UID changed; refusing to modify a different "+cmd.Workload())
	}

	ops := []map[string]any{
		{"op": "test", "path": "/metadata/uid", "value": w.uid},
		{"op": "test", "path": "/metadata/resourceVersion", "value": w.resourceVersion},
	}
	var done string
	switch cmd.Kind {
	case control.KindRestart:
		if w.template.Annotations[commandAnnotation] == cmd.ID {
			return finish("applied", "this command annotation is already present; no second patch sent")
		}
		ops = append(ops, map[string]any{"op": "add", "path": "/spec/template/metadata/annotations", "value": marked(w.template.Annotations, cmd.ID)})
		done = "pod-template annotation updated; rollout/readiness has not been verified"
	case control.KindSetResources:
		if w.annotations[commandAnnotation] == cmd.ID {
			return finish("applied", "this command annotation is already present; no second patch sent")
		}
		path, current, ok := container(w.template, cmd.Container)
		if !ok {
			return finish("failed", "container "+cmd.Container+" not found in the pod template; no write attempted")
		}
		ops = append(ops,
			map[string]any{"op": "test", "path": path + "/name", "value": cmd.Container},
			map[string]any{"op": "add", "path": path + "/resources", "value": withResources(current, *cmd.Resources)},
			map[string]any{"op": "add", "path": "/metadata/annotations", "value": marked(w.annotations, cmd.ID)},
		)
		done = "container resources updated; rollout/readiness has not been verified"
	case control.KindScale:
		if w.annotations[commandAnnotation] == cmd.ID {
			return finish("applied", "this command annotation is already present; no second patch sent")
		}
		ops = append(ops,
			map[string]any{"op": "add", "path": "/spec/replicas", "value": *cmd.Replicas},
			map[string]any{"op": "add", "path": "/metadata/annotations", "value": marked(w.annotations, cmd.ID)},
		)
		done = fmt.Sprintf("replicas set to %d; readiness has not been verified", *cmd.Replicas)
	}

	if !time.Now().Before(cmd.ExpiresAt) {
		return finish("expired", "deadline passed before write")
	}
	patch, _ := json.Marshal(ops)
	if err = w.patch(ctx, patch); err != nil {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) || apierrors.IsConflict(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
			return finish("failed", "Kubernetes rejected the change: "+err.Error())
		}
		return finish("unknown", "write confirmation unavailable; inspect target annotation before issuing another command: "+err.Error())
	}
	return finish("applied", done)
}

// marked copies annotations and adds the command ID.
func marked(annotations map[string]string, id string) map[string]string {
	out := map[string]string{}
	for k, v := range annotations {
		out[k] = v
	}
	out[commandAnnotation] = id
	return out
}

// container finds a container, init containers included, and returns its
// JSON patch path and current resources.
func container(t corev1.PodTemplateSpec, name string) (string, corev1.ResourceRequirements, bool) {
	for i, ct := range t.Spec.Containers {
		if ct.Name == name {
			return fmt.Sprintf("/spec/template/spec/containers/%d", i), ct.Resources, true
		}
	}
	for i, ct := range t.Spec.InitContainers {
		if ct.Name == name {
			return fmt.Sprintf("/spec/template/spec/initContainers/%d", i), ct.Resources, true
		}
	}
	return "", corev1.ResourceRequirements{}, false
}

// withResources returns current with the values set in r replaced.
func withResources(current corev1.ResourceRequirements, r control.Resources) corev1.ResourceRequirements {
	out := *current.DeepCopy()
	set := func(list *corev1.ResourceList, name corev1.ResourceName, value string) {
		if value == "" {
			return
		}
		if *list == nil {
			*list = corev1.ResourceList{}
		}
		(*list)[name] = resource.MustParse(value)
	}
	set(&out.Requests, corev1.ResourceCPU, r.CPURequest)
	set(&out.Limits, corev1.ResourceCPU, r.CPULimit)
	set(&out.Requests, corev1.ResourceMemory, r.MemoryRequest)
	set(&out.Limits, corev1.ResourceMemory, r.MemoryLimit)
	return out
}
