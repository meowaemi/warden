package kuber

import (
	"context"
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"warden/internal/control"
)

// TestExecuteCommandPreconditionsAndOutcomes runs a restart against a fake API
// server. The patch must check the UID and resourceVersion and keep existing
// annotations; the status depends on the target and the server's reply.
func TestExecuteCommandPreconditionsAndOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, uid, marker string
		expired           bool
		patchCode         int
		want              string
		writes            int
	}{
		{name: "accepted", uid: "uid-1", patchCode: 200, want: "applied", writes: 1},
		{name: "replacement target", uid: "uid-other", patchCode: 200, want: "failed"},
		{name: "duplicate marker", uid: "uid-1", marker: "command-1", patchCode: 200, want: "applied"},
		{name: "expired", uid: "uid-1", expired: true, want: "expired"},
		{name: "lost write confirmation", uid: "uid-1", patchCode: 500, want: "unknown", writes: 1},
		{name: "forbidden write", uid: "uid-1", patchCode: 403, want: "failed", writes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, reads := 0, 0
			d := appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"}, ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test", UID: "uid-1", ResourceVersion: "42"}}
			d.UID = types.UID(tc.uid)
			d.Spec.Template.Annotations = map[string]string{commandAnnotation: tc.marker, "existing": "keep"}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case "GET":
					reads++
					_ = json.NewEncoder(w).Encode(d)
				case "PATCH":
					writes++
					var ops []map[string]any
					if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
						t.Error(err)
					}
					if len(ops) != 3 || ops[0]["path"] != "/metadata/uid" || ops[1]["value"] != "42" {
						t.Errorf("missing UID/version conditions: %#v", ops)
					}
					a := ops[2]["value"].(map[string]any)
					if a["existing"] != "keep" || a[commandAnnotation] != "command-1" {
						t.Errorf("bad annotations %#v", a)
					}
					if tc.patchCode != 200 {
						w.WriteHeader(tc.patchCode)
						_ = json.NewEncoder(w).Encode(metav1.Status{Status: "Failure", Code: int32(tc.patchCode), Reason: metav1.StatusReasonUnknown, Message: "simulated failure"})
						return
					}
					_ = json.NewEncoder(w).Encode(d)
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
			}))
			defer ts.Close()
			cs, err := kubernetes.NewForConfig(&rest.Config{Host: ts.URL})
			if err != nil {
				t.Fatal(err)
			}
			c := &Client{cs: cs}
			deadline := time.Now().Add(time.Minute)
			if tc.expired {
				deadline = time.Now().Add(-time.Second)
			}
			got := c.ExecuteCommand(context.Background(), control.Command{ID: "command-1", Kind: "restart_deployment", Namespace: "test", Name: "app", TargetUID: "uid-1", ExpiresAt: deadline})
			if got.Status != tc.want || writes != tc.writes {
				t.Errorf("status=%s writes=%d; want %s writes=%d", got.Status, writes, tc.want, tc.writes)
			}
			if tc.expired && reads != 0 {
				t.Fatal("expired command contacted Kubernetes")
			}
		})
	}
}
