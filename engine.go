package kubetemplate

import (
	"context"
	"io"
	"time"

	"github.com/CyCoreSystems/kubetemplate/internal/engine"

	"github.com/CyCoreSystems/netdiscover/discover"
	"k8s.io/client-go/kubernetes"
)

// Engine implements a Kubernetes-based templating engine.
type Engine interface {
	// Wait watches for changes, returning when a change is detected.
	Wait(ctx context.Context)

	// Close shuts down the engine.
	Close() error

	// Learn prcoesses the given input template for resources to be monitored, adding each of those to the engine cache.
	// Learn is additive:  it can be called multiple times with different inputs to amalgamate at complete set.
	// This is useful when multiple templates will be used with the same rendering engine.
	// DefaultNamespace specifies a namespace to translate "" into, if desired.
	Learn(in io.Reader, defaultNamespace string) error

	// Render process the given input as a template, writing it to the provided output.
	// Make sure Learn has already been called for the given template.
	// DefaultNamespace specifies a namespace to translate "" into, if desired.
	Render(out io.Writer, in io.Reader, defaultNamespace string) error
}

// NewEngine returns a new rendering engine.
func NewEngine(kc kubernetes.Interface, disc discover.Discoverer, defaultResync time.Duration) (Engine, error) {
	return engine.New(kc, disc, defaultResync)
}
