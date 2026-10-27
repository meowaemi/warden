package kuber

import (
	"context"
	"encoding/json"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
	"warden/internal/control"
)

const commandAnnotation = "warden.command-id"

func (c *Client) ExecuteCommand(ctx context.Context, cmd control.Command) control.Result {
	result := control.Result{CommandID: cmd.ID, FinishedAt: time.Now().UTC()}
	finish := func(status, note string) control.Result {
		result.Status = status
		result.Note = note
		result.FinishedAt = time.Now().UTC()
		return result
	}
	if cmd.ID == "" || cmd.Kind != "restart_deployment" || cmd.Namespace == "" || cmd.Name == "" || cmd.TargetUID == "" {
		return finish("failed", "unsupported or incomplete command")
	}
	if !time.Now().Before(cmd.ExpiresAt) {
		return finish("expired", "deadline passed before reading target")
	}
	d, err := c.cs.AppsV1().Deployments(cmd.Namespace).Get(ctx, cmd.Name, metav1.GetOptions{})
	if err != nil {
		return finish("failed", "target read failed; no write attempted: "+err.Error())
	}
	if string(d.UID) != cmd.TargetUID {
		return finish("failed", "target UID changed; refusing to modify a different Deployment")
	}
	if d.Spec.Template.Annotations[commandAnnotation] == cmd.ID {
		return finish("applied", "this command annotation is already present; no second patch sent")
	}
	if !time.Now().Before(cmd.ExpiresAt) {
		return finish("expired", "deadline passed before write")
	}
	annotations := map[string]string{}
	for k, v := range d.Spec.Template.Annotations {
		annotations[k] = v
	}
	annotations[commandAnnotation] = cmd.ID
	patch, _ := json.Marshal([]map[string]any{
		{"op": "test", "path": "/metadata/uid", "value": string(d.UID)},
		{"op": "test", "path": "/metadata/resourceVersion", "value": d.ResourceVersion},
		{"op": "add", "path": "/spec/template/metadata/annotations", "value": annotations},
	})
	_, err = c.cs.AppsV1().Deployments(cmd.Namespace).Patch(ctx, cmd.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) || apierrors.IsConflict(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
			return finish("failed", "Kubernetes rejected the change: "+err.Error())
		}
		return finish("unknown", "write confirmation unavailable; inspect target annotation before issuing another command: "+err.Error())
	}
	return finish("applied", "pod-template annotation updated; rollout/readiness has not been verified")
}
