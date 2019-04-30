package resource

import (
	"context"

	"github.com/ericchiang/k8s"
	v1 "github.com/ericchiang/k8s/apis/core/v1"
)

func init() {
	AddPrototyper("ConfigMap", &configMapPrototyper{})
}

// configMapPrototyper defines the resource processor for a ConfigMap
type configMapPrototyper struct{}

// Object implements ResourcePrototyper
func (p *configMapPrototyper) Object() k8s.Resource {
	return new(v1.ConfigMap)
}

// Changeable implements ResourcePrototyper
func (p *configMapPrototyper) Changeable(ctx context.Context, kc *k8s.Client, namespace, name string) (Changeable, error) {
	c := new(v1.ConfigMap)

	if err := kc.Get(ctx, namespace, name, c); err != nil {
		return nil, err
	}

	return &watchedConfigMap{
		namespace: namespace,
		name:      name,
		r:         c,
	}, nil
}

type watchedConfigMap struct {
	namespace string

	name string

	r *v1.ConfigMap
}

func (w *watchedConfigMap) Name() string {
	return w.name
}

func (w *watchedConfigMap) Namespace() string {
	return w.namespace
}

func (w *watchedConfigMap) Kind() string {
	return "ConfigMap"
}

func (w *watchedConfigMap) Changed(ctx context.Context, kc *k8s.Client) (bool, error) {
	c := new(v1.ConfigMap)

	if err := kc.Get(ctx, w.namespace, w.name, c); err != nil {
		return false, err
	}

	old := w.r
	w.r = c

	// Determine if any of the important aspects of this ConfigMap have changed
	for k, v := range c.GetData() {
		oldVal, ok := old.GetData()[k]
		if !ok {
			// new key
			return true, nil
		}
		if oldVal != v {
			return true, nil
		}
	}

	return false, nil
}
