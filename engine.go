package kubetemplate

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/CyCoreSystems/kubetemplate/resource"
	"github.com/CyCoreSystems/netdiscover/discover"
	"github.com/ericchiang/k8s"
	v1 "github.com/ericchiang/k8s/apis/core/v1"
)

// KubeAPITimeout is the amount of time to wait for the kubernetes API to respond before failing
var KubeAPITimeout = 10 * time.Second

func init() {
	rand.Seed(time.Now().Unix())
}

// Engine provides the template rendering engine
type Engine struct {
	kc *k8s.Client

	disc discover.Discoverer

	reload chan error

	firstRenderCompleted bool

	//watchers map[string]*k8s.Watcher
	watchers []*Watcher

	mu sync.RWMutex
}

// AddWatched adds a namespace/name/kind tuple to the engine, to be watched
func (e *Engine) AddWatched(ctx context.Context, kind, namespace, name string, opts ...k8s.Option) error {

	p, ok := resource.GetPrototyper(kind)
	if !ok {
		return fmt.Errorf("unhandled resource type %s", kind)
	}
	// If the watcher already exists for this namespace+kind, just add our name to the list of watched entities
	e.mu.RLock()
	for _, w := range e.watchers {
		if w.kind == kind && w.namespace == namespace {
			e.mu.RUnlock()
			return w.Add(ctx, e.kc, namespace, name)
		}
	}
	e.mu.RUnlock()

	kubeWatcher, err := e.kc.Watch(ctx, namespace, p.Object())
	if err != nil {
		return err
	}

	w := &Watcher{
		w:          kubeWatcher,
		kind:       kind,
		namespace:  namespace,
		prototyper: p,
	}
	if err = w.Add(ctx, e.kc, namespace, name); err != nil {
		return err
	}

	e.mu.Lock()
	e.watchers = append(e.watchers, w)
	e.mu.Unlock()

	go func() {
		for {
			obj := p.Object()
			if _, err = w.w.Next(obj); err != nil {
				e.failure(err)
				return
			}

			// Only reload if this resource has one of the names we are monitoring
			for _, r := range w.resources {
				if r.Name() == obj.GetMetadata().GetName() {
					e.reload <- nil
					break
				}
			}
		}
	}()

	return nil
}

// Watcher wraps the kubernetes watcher to filter changes to just the names of our interest.
type Watcher struct {
	w *k8s.Watcher

	kind string

	namespace string

	prototyper resource.Prototyper

	resources []resource.Changeable

	mu sync.Mutex
}

// Add adds a resource to the existing watcher
func (w *Watcher) Add(ctx context.Context, kc *k8s.Client, namespace, name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, r := range w.resources {
		if r.Name() == name {
			return nil
		}
	}

	res, err := w.prototyper.Changeable(ctx, kc, namespace, name)
	if err != nil {
		return err
	}

	w.resources = append(w.resources, res)
	return nil
}

// Close shuts down the watcher
func (w *Watcher) Close() error {
	if w.w != nil {
		return w.w.Close()
	}
	return nil
}

// NewEngine returns a new rendering engine.  The supplied reloadChan servces
// as an indicator to reload and as a return channel for errors.  If `nil` is
// passed down the channel, a reload is requested.  If an error is passed down,
// the Engine has died and must be restarted.
func NewEngine(reloadChan chan error, disc discover.Discoverer) *Engine {
	return &Engine{
		disc:   disc,
		reload: reloadChan,
	}
}

// Close shuts down the template engine
func (e *Engine) Close() {

	e.mu.Lock()
	for _, w := range e.watchers {
		w.Close() // nolint
	}
	e.watchers = nil
	e.mu.Unlock()

}

func (e *Engine) connectKube() (err error) {
	if e.kc != nil {
		return
	}

	e.kc, err = k8s.NewInClusterClient()

	return
}

func (e *Engine) failure(err error) {
	e.Close()

	if e.reload != nil {
		e.reload <- err
	}
}

// FirstRenderComplete should be called after the first render is completed.
// This eliminates some extra kubernetes API checking for resource watching.
func (e *Engine) FirstRenderComplete(ok bool) {
	e.firstRenderCompleted = ok
}

// ConfigMap returns a kubernetes ConfigMap
func (e *Engine) ConfigMap(name string, namespace string, key string) (out string, err error) {
	c := new(v1.ConfigMap)

	if err = e.connectKube(); err != nil {
		return
	}

	namespace, err = getNamespace(namespace)
	if err != nil {
		return
	}

	ctx, cancel := boundedContext()
	defer cancel()

	if err = e.kc.Get(ctx, namespace, name, c); err != nil {
		return
	}

	v, ok := c.GetData()[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in ConfigMap %s[%s]", key, name, namespace)
	}
	out = v

	if e.firstRenderCompleted {
		return
	}

	err = e.AddWatched(context.Background(), "ConfigMap", namespace, name)
	return
}

// Secret returns a kubernetes Secret
func (e *Engine) Secret(name string, namespace string, key string) (out string, err error) {
	s := new(v1.Secret)

	if err = e.connectKube(); err != nil {
		return
	}

	namespace, err = getNamespace(namespace)
	if err != nil {
		return
	}

	ctx, cancel := boundedContext()
	defer cancel()

	if err = e.kc.Get(ctx, namespace, name, s); err != nil {
		return
	}

	v, ok := s.GetData()[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in Secret %s[%s]", key, name, namespace)
	}

	b, err := base64.StdEncoding.DecodeString(string(v))
	if err != nil {
		return "", fmt.Errorf("failed to decode secret value %s: %v", key, err)
	}
	out = string(b)

	if e.firstRenderCompleted {
		return
	}

	err = e.AddWatched(context.Background(), "Secret", namespace, name)
	return
}

// Env returns the value of an environment variable
func (e *Engine) Env(name string) string {
	return os.Getenv(name)
}

// Service returns a kubernetes Service
func (e *Engine) Service(name, namespace string) (s *v1.Service, err error) {
	s = new(v1.Service)

	if err = e.connectKube(); err != nil {
		return
	}

	namespace, err = getNamespace(namespace)
	if err != nil {
		return
	}

	ctx, cancel := boundedContext()
	defer cancel()

	if err = e.kc.Get(ctx, namespace, name, s); err != nil {
		return
	}

	if e.firstRenderCompleted {
		return
	}

	err = e.AddWatched(context.Background(), "Service", namespace, name)
	return
}

// ServiceIP returns a kubernetes Service's ClusterIP
func (e *Engine) ServiceIP(name, namespace string) (string, error) {
	s, err := e.Service(name, namespace)
	if err != nil {
		return "", err
	}

	return s.GetSpec().GetClusterIP(), nil
}

// Endpoints returns the Endpoints for the given Service
func (e *Engine) Endpoints(name, namespace string) (ep *v1.Endpoints, err error) {
	ep = new(v1.Endpoints)

	if err = e.connectKube(); err != nil {
		return
	}

	namespace, err = getNamespace(namespace)
	if err != nil {
		return
	}

	ctx, cancel := boundedContext()
	defer cancel()

	if err = e.kc.Get(ctx, namespace, name, ep); err != nil {
		return
	}

	if e.firstRenderCompleted {
		return
	}

	err = e.AddWatched(context.Background(), "Endpoints", namespace, name)
	return
}

// EndpointIPs returns the set of IP addresses for the given Service's endpoints.
func (e *Engine) EndpointIPs(name, namespace string) (out []string, err error) {
	var ep *v1.Endpoints

	ep, err = e.Endpoints(name, namespace)
	if err != nil {
		return
	}

	for _, ss := range ep.GetSubsets() {
		for _, addr := range ss.GetAddresses() {
			out = append(out, addr.GetIp())
		}
	}
	return

}

// Network retrieves network information about the running Pod.
func (e *Engine) Network(kind string) (string, error) {

	kind = strings.ToLower(kind)
	switch kind {
	case "hostname":
		return e.disc.Hostname()
	case "privateipv4":
		ip, err := e.disc.PrivateIPv4()
		return ip.String(), err
	case "privatev4":
		ip, err := e.disc.PrivateIPv4()
		return ip.String(), err
	case "publicipv4":
		ip, err := e.disc.PublicIPv4()
		return ip.String(), err
	case "publicv4":
		ip, err := e.disc.PublicIPv4()
		return ip.String(), err
	case "publicipv6":
		ip, err := e.disc.PublicIPv6()
		return ip.String(), err
	case "publicv6":
		ip, err := e.disc.PublicIPv6()
		return ip.String(), err
	default:
		return "", errors.New("unhandled kind")
	}
}

func boundedContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), KubeAPITimeout)
}

func getNamespace(name string) (out string, err error) {
	out = name
	if out == "" {
		out = os.Getenv("POD_NAMESPACE")
	}
	if out == "" {
		err = errors.New("failed to determine namespace")
	}
	return
}

func watcherName(names ...string) string {
	return strings.Join(names, ".")
}
