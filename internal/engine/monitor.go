package engine

import (
	"bytes"
	"crypto/sha256"
	"sort"
	"strconv"
	"sync"
	"time"

	coreapiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Namespace describes a namespace of Resource Monitors
// This is used to house namespace-limited informer factories.
type Namespace struct {
	name string

	f informers.SharedInformerFactory

	ConfigMaps *ConfigMapMonitor
	Endpoints  *EndpointsMonitor
	Secrets    *SecretMonitor
	Services   *ServiceMonitor
}

func NewNamespace(name string, kc kubernetes.Interface, defaultResync time.Duration, changes chan<- struct{}) *Namespace {
	f := informers.NewSharedInformerFactoryWithOptions(kc, defaultResync)

	n := &Namespace{
		name:       name,
		f:          f,
		ConfigMaps: NewConfigMapMonitor(name, changes, f),
		Endpoints:  NewEndpointsMonitor(name, changes, f),
		Secrets:    NewSecretMonitor(name, changes, f),
		Services:   NewServiceMonitor(name, changes, f),
	}

	return n
}

// Run the namespace monitor.
func (n *Namespace) Run(stop <-chan struct{}) {
	if n == nil {
		return
	}

	type svcFunc func(<-chan struct{})

	svcs := []svcFunc{
		n.ConfigMaps.Run,
		n.Endpoints.Run,
		n.Secrets.Run,
		n.Services.Run,
	}

	wg := new(sync.WaitGroup)

	for _, s := range svcs {
		wg.Add(1)

		go func(f svcFunc) {
			f(stop)
			wg.Done()
		}(s)
	}

	wg.Wait()
}

type baseMonitor struct {
	namespace string
	changes   chan<- struct{}
}

func (m *baseMonitor) triggerChange() {
	select {
	case m.changes <- struct{}{}:
	default:
	}
}

func (m *baseMonitor) ResourceMonitor() {
}

type ConfigMapMonitor struct {
	*baseMonitor

	keys map[string][]string

	i corev1.ConfigMapInformer
}

func NewConfigMapMonitor(namespace string, changes chan<- struct{}, f informers.SharedInformerFactory) *ConfigMapMonitor {
	return &ConfigMapMonitor{
		baseMonitor: &baseMonitor{
			namespace: namespace,
			changes:   changes,
		},
		keys: make(map[string][]string),
		i:    f.Core().V1().ConfigMaps(),
	}
}

// Filter implements a FilterFunc for cache.FilteringResourceEventHandler.
func (m *ConfigMapMonitor) Filter(obj interface{}) bool {
	o, ok := obj.(*coreapiv1.ConfigMap)
	if !ok {
		return false
	}

	for k := range m.keys {
		if o.GetName() == k {
			return true
		}
	}

	return false
}

// Add adds a ConfigMap and key to be monitored.
func (m *ConfigMapMonitor) Add(name, key string) {
	interestingKeys, ok := m.keys[name]
	if !ok {
		m.keys[name] = []string{key}

		return
	}

	for _, k := range interestingKeys {
		if k == key {
			return
		}
	}

	m.keys[name] = append(interestingKeys, key)
}

// Get returns a specific ConfigMap.
func (m *ConfigMapMonitor) Get(name, key string) (*coreapiv1.ConfigMap, error) {
	return m.i.Lister().ConfigMaps(m.namespace).Get(name)
}

func (m *ConfigMapMonitor) Run(stop <-chan struct{}) {
	m.i.Informer().AddEventHandler(&cache.FilteringResourceEventHandler{
		FilterFunc: m.Filter,
		Handler:    m,
	})

	m.i.Informer().Run(stop)
}

// OnUpdate implements cache.ResourceEventHandler
func (m *ConfigMapMonitor) OnUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*coreapiv1.ConfigMap)
	if !ok {
		return
	}

	cur, ok := newObj.(*coreapiv1.ConfigMap)
	if !ok {
		return
	}

	interestingKeys, ok := m.keys[cur.GetName()]
	if !ok {
		return
	}

	for _, k := range interestingKeys {
		a, ok := old.Data[k]
		if !ok {
			m.triggerChange()

			return
		}

		b := cur.Data[k]
		if !ok {
			m.triggerChange()

			return
		}

		if a != b {
			m.triggerChange()
		}
	}
}

// OnAdd implements cache.ResourceEventHandler
func (m *ConfigMapMonitor) OnAdd(obj interface{}) {
	m.triggerChange()
}

// OnDelete implements cache.ResourceEventHandler
func (m *ConfigMapMonitor) OnDelete(obj interface{}) {
	m.triggerChange()
}

type SecretMonitor struct {
	*baseMonitor

	keys map[string][]string

	i corev1.SecretInformer
}

func NewSecretMonitor(namespace string, changes chan<- struct{}, f informers.SharedInformerFactory) *SecretMonitor {
	return &SecretMonitor{
		baseMonitor: &baseMonitor{
			namespace: namespace,
			changes:   changes,
		},
		keys: make(map[string][]string),
		i:    f.Core().V1().Secrets(),
	}
}

// Filter implements a FilterFunc for cache.FilteringResourceEventHandler.
func (m *SecretMonitor) Filter(obj interface{}) bool {
	o, ok := obj.(*coreapiv1.Secret)
	if !ok {
		return false
	}

	for k := range m.keys {
		if o.GetName() == k {
			return true
		}
	}

	return false
}

// Add adds a ConfigMap and key to be monitored.
func (m *SecretMonitor) Add(name, key string) {
	interestingKeys, ok := m.keys[name]
	if !ok {
		m.keys[name] = []string{key}

		return
	}

	for _, k := range interestingKeys {
		if k == key {
			return
		}
	}

	m.keys[name] = append(interestingKeys, key)
}

// Get returns a specific Secret.
func (m *SecretMonitor) Get(name, key string) (*coreapiv1.Secret, error) {
	return m.i.Lister().Secrets(m.namespace).Get(name)
}

func (m *SecretMonitor) Run(stop <-chan struct{}) {
	m.i.Informer().AddEventHandler(&cache.FilteringResourceEventHandler{
		FilterFunc: m.Filter,
		Handler:    m,
	})

	m.i.Informer().Run(stop)
}

// OnUpdate implements cache.ResourceEventHandler
func (m *SecretMonitor) OnUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*coreapiv1.Secret)
	if !ok {
		return
	}

	cur, ok := newObj.(*coreapiv1.Secret)
	if !ok {
		return
	}

	interestingKeys, ok := m.keys[cur.GetName()]
	if !ok {
		return
	}

	for _, k := range interestingKeys {
		a, ok := old.Data[k]
		if !ok {
			m.triggerChange()

			return
		}

		b, ok := cur.Data[k]
		if !ok {
			m.triggerChange()

			return
		}

		if !bytes.Equal(a, b) {
			m.triggerChange()

			return
		}
	}
}

// OnAdd implements cache.ResourceEventHandler
func (m *SecretMonitor) OnAdd(obj interface{}) {
	m.triggerChange()
}

// OnDelete implements cache.ResourceEventHandler
func (m *SecretMonitor) OnDelete(obj interface{}) {
	m.triggerChange()
}

type ServiceMonitor struct {
	*baseMonitor

	keys []string

	i corev1.ServiceInformer
}

func NewServiceMonitor(namespace string, changes chan<- struct{}, f informers.SharedInformerFactory) *ServiceMonitor {
	return &ServiceMonitor{
		baseMonitor: &baseMonitor{
			namespace: namespace,
			changes:   changes,
		},
		i: f.Core().V1().Services(),
	}
}

// Filter implements a FilterFunc for cache.FilteringResourceEventHandler.
func (m *ServiceMonitor) Filter(obj interface{}) bool {
	o, ok := obj.(*coreapiv1.Service)
	if !ok {
		return false
	}

	for _, k := range m.keys {
		if o.GetName() == k {
			return true
		}
	}

	return false
}

// Add adds a Service to be monitored.
func (m *ServiceMonitor) Add(name string) {
	for _, k := range m.keys {
		if k == name {
			return
		}
	}

	m.keys = append(m.keys, name)
}

// Get returns a specific Service.
func (m *ServiceMonitor) Get(name string) (*coreapiv1.Service, error) {
	return m.i.Lister().Services(m.namespace).Get(name)
}

func (m *ServiceMonitor) Run(stop <-chan struct{}) {
	m.i.Informer().AddEventHandler(&cache.FilteringResourceEventHandler{
		FilterFunc: m.Filter,
		Handler:    m,
	})

	m.i.Informer().Run(stop)
}

// OnUpdate implements cache.ResourceEventHandler
func (m *ServiceMonitor) OnUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*coreapiv1.Service)
	if !ok {
		return
	}

	cur, ok := newObj.(*coreapiv1.Service)
	if !ok {
		return
	}

	a, _ := old.Spec.Marshal()

	b, _ := cur.Spec.Marshal()

	if !bytes.Equal(a, b) {
		m.triggerChange()
	}
}

// OnAdd implements cache.ResourceEventHandler
func (m *ServiceMonitor) OnAdd(obj interface{}) {
	m.triggerChange()
}

// OnDelete implements cache.ResourceEventHandler
func (m *ServiceMonitor) OnDelete(obj interface{}) {
	m.triggerChange()
}

type EndpointsMonitor struct {
	*baseMonitor

	keys []string

	i corev1.EndpointsInformer
}

func NewEndpointsMonitor(namespace string, changes chan<- struct{}, f informers.SharedInformerFactory) *EndpointsMonitor {
	return &EndpointsMonitor{
		baseMonitor: &baseMonitor{
			namespace: namespace,
			changes:   changes,
		},
		i: f.Core().V1().Endpoints(),
	}
}

// Filter implements a FilterFunc for cache.FilteringResourceEventHandler.
func (m *EndpointsMonitor) Filter(obj interface{}) bool {
	o, ok := obj.(*coreapiv1.Endpoints)
	if !ok {
		return false
	}

	for _, k := range m.keys {
		if o.GetName() == k {
			return true
		}
	}

	return false
}

// Add adds an Endpoint set to be monitored.
func (m *EndpointsMonitor) Add(name string) {
	for _, k := range m.keys {
		if k == name {
			return
		}
	}

	m.keys = append(m.keys, name)
}

// Get returns a specific Endpoint.
func (m *EndpointsMonitor) Get(name string) (*coreapiv1.Endpoints, error) {
	return m.i.Lister().Endpoints(m.namespace).Get(name)
}

func (m *EndpointsMonitor) Run(stop <-chan struct{}) {
	m.i.Informer().AddEventHandler(&cache.FilteringResourceEventHandler{
		FilterFunc: m.Filter,
		Handler:    m,
	})

	m.i.Informer().Run(stop)
}

// OnUpdate implements cache.ResourceEventHandler
func (m *EndpointsMonitor) OnUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*coreapiv1.Endpoints)
	if !ok {
		return
	}

	cur, ok := newObj.(*coreapiv1.Endpoints)
	if !ok {
		return
	}

	if !bytes.Equal(endpointsSubsetHash(old), endpointsSubsetHash(cur)) {
		m.triggerChange()
	}
}

// OnAdd implements cache.ResourceEventHandler
func (m *EndpointsMonitor) OnAdd(obj interface{}) {
	m.triggerChange()
}

// OnDelete implements cache.ResourceEventHandler
func (m *EndpointsMonitor) OnDelete(obj interface{}) {
	m.triggerChange()
}

func endpointsSubsetHash(ep *coreapiv1.Endpoints) []byte {
	var addrs []string
	var ports []string

	for _, ss := range ep.Subsets {
		for _, p := range ss.Ports {
			ports = append(ports, p.Name+strconv.Itoa(int(p.Port)))
		}

		for _, a := range ss.Addresses {
			addrs = append(addrs, a.IP)
		}
	}

	sort.Strings(addrs)
	sort.Strings(ports)

	h := sha256.New()
	for _, p := range ports {
		h.Write([]byte(p))
	}
	for _, a := range addrs {
		h.Write([]byte(a))
	}
	return h.Sum(nil)
}
