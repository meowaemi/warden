package control

import "testing"

// TestCommandValidate checks which commands Validate accepts.
func TestCommandValidate(t *testing.T) {
	three, tooMany := int32(3), int32(MaxReplicas+1)
	base := Command{ID: "c1", Namespace: "prod", Name: "api", TargetUID: "uid-1"}
	with := func(f func(*Command)) Command { c := base; f(&c); return c }
	for _, tc := range []struct {
		name string
		cmd  Command
		ok   bool
	}{
		{"restart", with(func(c *Command) { c.Kind = KindRestart }), true},
		{"restart of a StatefulSet", with(func(c *Command) { c.Kind = KindRestart; c.TargetKind = StatefulSet }), false},
		{"restart with resources", with(func(c *Command) { c.Kind = KindRestart; c.Resources = &Resources{MemoryLimit: "1Gi"} }), false},
		{"memory limit", with(func(c *Command) {
			c.Kind, c.Container, c.Resources = KindSetResources, "app", &Resources{MemoryLimit: "512Mi"}
		}), true},
		{"all resources of a DaemonSet", with(func(c *Command) {
			c.Kind, c.TargetKind, c.Container = KindSetResources, DaemonSet, "agent"
			c.Resources = &Resources{CPURequest: "100m", CPULimit: "1", MemoryRequest: "128Mi", MemoryLimit: "256Mi"}
		}), true},
		{"no container", with(func(c *Command) { c.Kind, c.Resources = KindSetResources, &Resources{MemoryLimit: "1Gi"} }), false},
		{"no resources", with(func(c *Command) { c.Kind, c.Container, c.Resources = KindSetResources, "app", &Resources{} }), false},
		{"bad quantity", with(func(c *Command) {
			c.Kind, c.Container, c.Resources = KindSetResources, "app", &Resources{MemoryLimit: "lots"}
		}), false},
		{"zero quantity", with(func(c *Command) {
			c.Kind, c.Container, c.Resources = KindSetResources, "app", &Resources{CPULimit: "0"}
		}), false},
		{"request above limit", with(func(c *Command) {
			c.Kind, c.Container, c.Resources = KindSetResources, "app", &Resources{MemoryRequest: "1Gi", MemoryLimit: "512Mi"}
		}), false},
		{"scale", with(func(c *Command) { c.Kind, c.Replicas = KindScale, &three }), true},
		{"scale to too many", with(func(c *Command) { c.Kind, c.Replicas = KindScale, &tooMany }), false},
		{"scale a DaemonSet", with(func(c *Command) { c.Kind, c.TargetKind, c.Replicas = KindScale, DaemonSet, &three }), false},
		{"scale without replicas", with(func(c *Command) { c.Kind = KindScale }), false},
		{"unknown kind", with(func(c *Command) { c.Kind = "delete" }), false},
		{"no UID", with(func(c *Command) { c.Kind = KindRestart; c.TargetUID = "" }), false},
	} {
		if err := tc.cmd.Validate(); (err == nil) != tc.ok {
			t.Errorf("%s: err=%v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}
