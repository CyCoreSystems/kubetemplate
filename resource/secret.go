package resource

import (
	"bytes"
	"context"

	"github.com/ericchiang/k8s"
	v1 "github.com/ericchiang/k8s/apis/core/v1"
)

func init() {
	AddPrototyper("Secret", &secretPrototyper{})
}

// secretPrototyper defines the resource processor for a Secret
type secretPrototyper struct{}

// Object implements ResourcePrototyper
func (p *secretPrototyper) Object() k8s.Resource {
	return new(v1.Secret)
}

// Changeable implements ResourcePrototyper
func (p *secretPrototyper) Changeable(ctx context.Context, kc *k8s.Client, namespace, name string) (Changeable, error) {
	s := new(v1.Secret)

	if err := kc.Get(ctx, namespace, name, s); err != nil {
		return nil, err
	}

	return &watchedSecret{
		namespace: namespace,
		name:      name,
		r:         s,
	}, nil
}

type watchedSecret struct {
	namespace string

	name string

	r *v1.Secret
}

func (w *watchedSecret) Name() string {
	return w.name
}

func (w *watchedSecret) Namespace() string {
	return w.namespace
}

func (w *watchedSecret) Kind() string {
	return "Secret"
}

func (w *watchedSecret) Changed(ctx context.Context, kc *k8s.Client) (bool, error) {
	s := new(v1.Secret)

	if err := kc.Get(ctx, w.namespace, w.name, s); err != nil {
		return false, err
	}

	old := w.r
	w.r = s

	// Determine if any of the important aspects of this ConfigMap have changed
	for k, v := range s.GetData() {
		oldVal, ok := old.GetData()[k]
		if !ok {
			// new key
			return true, nil
		}
		if !bytes.Equal(oldVal, v) {
			return true, nil
		}
	}

	return false, nil
}
