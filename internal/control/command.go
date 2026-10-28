package control

import (
	"errors"
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Command kinds.
const (
	KindRestart      = "restart_deployment"
	KindSetResources = "set_resources"
	KindScale        = "scale"
)

// Workload kinds a command can target; an empty TargetKind means Deployment.
const (
	Deployment  = "Deployment"
	StatefulSet = "StatefulSet"
	DaemonSet   = "DaemonSet"
)

// MaxReplicas caps a scale command.
const MaxReplicas = 1000

// dnsLabel matches a container name.
var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Workload returns the target kind, Deployment by default.
func (c Command) Workload() string {
	if c.TargetKind == "" {
		return Deployment
	}
	return c.TargetKind
}

// Validate checks that a command is complete and supported. The center
// checks it before queueing and the agent again before executing.
func (c Command) Validate() error {
	if !identifier.MatchString(c.ID) {
		return errors.New("invalid command ID")
	}
	if !validName(c.Namespace) || !validName(c.Name) || !identifier.MatchString(c.TargetUID) {
		return errors.New("invalid target namespace, name or UID")
	}
	switch c.Kind {
	case KindRestart:
		if c.Workload() != Deployment {
			return errors.New("restart supports only Deployments")
		}
		if c.Container != "" || c.Resources != nil || c.Replicas != nil {
			return errors.New("restart takes no container, resources or replicas")
		}
	case KindSetResources:
		if w := c.Workload(); w != Deployment && w != StatefulSet && w != DaemonSet {
			return fmt.Errorf("unsupported target kind %q", w)
		}
		if len(c.Container) > 63 || !dnsLabel.MatchString(c.Container) {
			return errors.New("invalid container name")
		}
		if c.Replicas != nil {
			return errors.New("set_resources takes no replicas")
		}
		return validResources(c.Resources)
	case KindScale:
		if w := c.Workload(); w != Deployment && w != StatefulSet {
			return fmt.Errorf("cannot scale %q", w)
		}
		if c.Replicas == nil || *c.Replicas < 0 || *c.Replicas > MaxReplicas {
			return fmt.Errorf("replicas must be from 0 to %d", MaxReplicas)
		}
		if c.Container != "" || c.Resources != nil {
			return errors.New("scale takes no container or resources")
		}
	default:
		return fmt.Errorf("unsupported command kind %q", c.Kind)
	}
	return nil
}

// validResources requires at least one positive quantity and no request above its limit.
func validResources(r *Resources) error {
	if r == nil || *r == (Resources{}) {
		return errors.New("set at least one resource")
	}
	pairs := []struct{ name, request, limit string }{
		{"cpu", r.CPURequest, r.CPULimit},
		{"memory", r.MemoryRequest, r.MemoryLimit},
	}
	for _, p := range pairs {
		req, err := quantity(p.name, p.request)
		if err != nil {
			return err
		}
		lim, err := quantity(p.name, p.limit)
		if err != nil {
			return err
		}
		if p.request != "" && p.limit != "" && req.Cmp(lim) > 0 {
			return fmt.Errorf("%s request exceeds its limit", p.name)
		}
	}
	return nil
}

// quantity parses a positive quantity; an empty value is zero.
func quantity(name, value string) (resource.Quantity, error) {
	if value == "" {
		return resource.Quantity{}, nil
	}
	q, err := resource.ParseQuantity(value)
	if err != nil || q.Sign() <= 0 {
		return q, fmt.Errorf("invalid %s quantity %q", name, value)
	}
	return q, nil
}
