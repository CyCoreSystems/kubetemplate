package resource

import (
	"context"
	"strings"
	"sync"

	"github.com/ericchiang/k8s"
)

// mu protects the resource prototypes list
var mu sync.RWMutex

// list contains the set of resource types which are supported by this templating engine
var list = map[string]Prototyper{}

// Prototyper implements a kubernetes resource to be handled by this
// templating engine.  It needs to be able to instantiate a kubernetes resource
// of its matching type, as well as create a stateful implementation of that
// resource which can indicate when a change of its salient details occurs.
type Prototyper interface {

	// Object returns a new empty object of this resource type
	Object() k8s.Resource

	// Changeable returns a Changeable for this resource type
	Changeable(ctx context.Context, kc *k8s.Client, namespace, name string) (Changeable, error)
}

// A Changeable resource is a stateful kubernetes resource which tracks old and present states, making it able to indicate when a change occurs.
type Changeable interface {

	// Name returns the name of this resource
	Name() string

	// Namespace returns the namespace of this resource
	Namespace() string

	// Kind returns the name of the resource type
	Kind() string

	// Changed indicates whether the resource has materially changed
	Changed(ctx context.Context, kc *k8s.Client) (bool, error)
}

// AddPrototyper adds a prototyper for the given resource type to the list of supported resources of the engine
func AddPrototyper(kind string, p Prototyper) {
	mu.Lock()
	list[strings.ToLower(kind)] = p
	mu.Unlock()
}

// GetPrototyper returns a prototyper for the given resource
func GetPrototyper(kind string) (Prototyper, bool) {
	mu.RLock()
	p, ok := list[strings.ToLower(kind)]
	mu.RUnlock()
	return p, ok
}

// List returns a list of the supported resource kinds
func List() (ret []string) {
	for k := range list {
		ret = append(ret, k)
	}
	return
}
