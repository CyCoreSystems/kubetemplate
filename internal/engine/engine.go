package engine

import (
	"context"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"sync"
	"time"

	"github.com/CyCoreSystems/netdiscover/discover"
	"k8s.io/client-go/kubernetes"
)

// KubeAPITimeout is the amount of time to wait for the kubernetes API to respond before failing
var KubeAPITimeout = 10 * time.Second

func init() {
	rand.Seed(time.Now().Unix())
}

type Engine struct {
	namespaces map[string]*Namespace

	kc kubernetes.Interface

	disc discover.Discoverer

	changes chan struct{}

	stop chan struct{}

	stopFunc sync.Once

	defaultResync time.Duration

	learning bool

	learnMutex sync.RWMutex
}

// New returns a new Engine.
func New(kc kubernetes.Interface, disc discover.Discoverer, defaultResync time.Duration) (*Engine, error) {
	return &Engine{
		namespaces:    make(map[string]*Namespace),
		kc:            kc,
		disc:          disc,
		changes:       make(chan struct{}, 1),
		stop:          make(chan struct{}),
		defaultResync: defaultResync,
	}, nil
}

// addNamespace adds a namespace to the engine.
// NOTE: This should never be called unless the learning mutex lock is held.
func (e *Engine) addNamespace(name string) *Namespace {
	n, ok := e.namespaces[name]
	if !ok {
		n := NewNamespace(name, e.kc, e.defaultResync, e.changes)

		go n.Run(e.stop)

		e.namespaces[name] = n
	}

	return n
}

func (e *Engine) namespace(name string) *Namespace {
	n, ok := e.namespaces[name]
	if !ok {
		return nil
	}

	return n
}

func (e *Engine) render(out io.Writer, in io.Reader, p *DataProvider) error {
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	t, err := template.New("t").Parse(string(buf))
	if err != nil {
		return err
	}

	return t.Execute(out, p)
}

// Learn processes the given input as a template, recording any discovered resources to the resource monitors.
func (e *Engine) Learn(in io.Reader) error {
	e.learnMutex.Lock()
	defer e.learnMutex.Unlock()

	e.learning = true
	defer func() {
		e.learning = false
	}()

	return e.render(io.Discard, in, &DataProvider{
		learning: true,
		e:        e,
	})
}

// Render processes the given input as a template, writing it to the provided output
func (e *Engine) Render(out io.Writer, in io.Reader) error {

	e.learnMutex.RLock()
	defer e.learnMutex.RUnlock()

	return e.render(out, in, &DataProvider{
		learning: false,
		e:        e,
	})
}

// Close shuts down the engine
func (e *Engine) Close() error {
	e.stopFunc.Do(func() {
		close(e.stop)
	})

	return nil
}

// Wait waits until a change is detected.
func (e *Engine) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-e.changes:
	}
}
