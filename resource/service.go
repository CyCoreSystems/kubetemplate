package resource

import (
	"context"

	"github.com/ericchiang/k8s"
	v1 "github.com/ericchiang/k8s/apis/core/v1"
)

func init() {
	AddPrototyper("Service", &servicePrototyper{})
}

// servicePrototyper defines the resource processor for a Service
type servicePrototyper struct{}

// Object implements ResourcePrototyper
func (p *servicePrototyper) Object() k8s.Resource {
	return new(v1.Service)
}

// Changeable implements ResourcePrototyper
func (p *servicePrototyper) Changeable(ctx context.Context, kc *k8s.Client, namespace, name string) (Changeable, error) {
	s := new(v1.Service)

	if err := kc.Get(ctx, namespace, name, s); err != nil {
		return nil, err
	}

	return &watchedService{
		namespace: namespace,
		name:      name,
		r:         s,
	}, nil
}

type watchedService struct {
	namespace string

	name string

	r *v1.Service
}

func (w *watchedService) Name() string {
	return w.name
}

func (w *watchedService) Namespace() string {
	return w.namespace
}

func (w *watchedService) Kind() string {
	return "Service"
}

func (w *watchedService) Changed(ctx context.Context, kc *k8s.Client) (bool, error) {
	s := new(v1.Service)

	if err := kc.Get(ctx, w.namespace, w.name, s); err != nil {
		return false, err
	}

	old := w.r
	w.r = s

	// Determine if any of the important aspects of this Service have changed

	// Test service IP address
	if s.GetSpec().GetClusterIP() != old.GetSpec().GetClusterIP() {
		return true, nil
	}

	// Test service Ports
	for _, p := range s.GetSpec().GetPorts() {
		for _, oldPort := range old.GetSpec().GetPorts() {
			if p.GetName() == oldPort.GetName() {
				if p.GetPort() != oldPort.GetPort() {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
