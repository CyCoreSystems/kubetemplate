package resource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sort"
	"strconv"

	"github.com/ericchiang/k8s"
	v1 "github.com/ericchiang/k8s/apis/core/v1"
)

func init() {
	AddPrototyper("Endpoints", &endpointsPrototyper{})
}

// endpointsPrototyper defines the resource processor for a Endpoints
type endpointsPrototyper struct{}

func (p *endpointsPrototyper) Object() k8s.Resource {
	return new(v1.Endpoints)
}

func (p *endpointsPrototyper) Changeable(ctx context.Context, kc *k8s.Client, namespace, name string) (Changeable, error) {
	ep := new(v1.Endpoints)

	if err := kc.Get(ctx, namespace, name, ep); err != nil {
		return nil, err
	}

	return &watchedEndpoints{
		namespace: namespace,
		name:      name,
		r:         ep,
	}, nil
}

type watchedEndpoints struct {
	namespace string

	name string

	r *v1.Endpoints
}

func (w *watchedEndpoints) Name() string {
	return w.name
}

func (w *watchedEndpoints) Namespace() string {
	return w.namespace
}

func (w *watchedEndpoints) Kind() string {
	return "Endpoints"
}

func (w *watchedEndpoints) Changed(ctx context.Context, kc *k8s.Client) (bool, error) {
	ep := new(v1.Endpoints)

	if err := kc.Get(ctx, w.namespace, w.name, ep); err != nil {
		return false, err
	}

	old := w.r
	w.r = ep

	// Determine if any of the important aspects of this Endpoints have changed

	if len(ep.GetSubsets()) != len(old.GetSubsets()) {
		return true, nil
	}

	h1, err := subsetsHash(ep)
	if err != nil {
		return false, err
	}
	h2, err := subsetsHash(old)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(h1, h2) {
		return true, nil
	}

	return false, nil
}

func subsetsHash(ep *v1.Endpoints) ([]byte, error) {
	var addrs []string
	var ports []string

	for _, ss := range ep.GetSubsets() {
		for _, p := range ss.GetPorts() {
			ports = append(ports, p.GetName()+strconv.Itoa(int(p.GetPort())))
		}

		for _, a := range ss.GetAddresses() {
			addrs = append(addrs, a.GetNodeName()+a.GetIp())
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
	return h.Sum(nil), nil
}
