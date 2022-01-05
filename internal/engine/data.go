package engine

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrNoNamespace indicates that the requested namespace is not being monitored.
// This should not happen unless the given template has not been Learned yet.
var ErrNoNamespace = errors.New("namespace not monitored (did you call Learn()?)")

type DataProvider struct {
	learning bool
	e        *Engine
}

func (p *DataProvider) getOrCreateNamespace(namespace string) *Namespace {
	n := p.e.namespace(namespace)
	if n == nil {
		n = p.e.addNamespace(namespace)
	}

	return n
}

// ConfigMap returns a kubernetes ConfigMap value
func (p *DataProvider) ConfigMap(name, namespace, key string) (string, error) {

	if p.learning {
		n := p.getOrCreateNamespace(namespace)

		n.ConfigMaps.Add(name, key)
	}

	n := p.e.namespace(namespace)
	if n == nil {
		return "", ErrNoNamespace
	}

	data, err := p.e.kc.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	out, ok := data.Data[key]
	if !ok {
		return "", nil
	}

	return out, nil
}

// Secret returns a kubernetes Secret value
func (p *DataProvider) Secret(name, namespace, key string) (out string, err error) {
	b64Data, err := p.SecretBinary(name, namespace, key)
	if err != nil {
		return "", err
	}

	var data []byte

	_, err = base64.StdEncoding.Decode(data, b64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret value: %w", err)
	}

	return string(data), nil
}

// SecretBinary returns a kubernetes Secret binary value
func (p *DataProvider) SecretBinary(name, namespace, key string) (out []byte, err error) {
	if p.learning {
		n := p.getOrCreateNamespace(namespace)

		n.Secrets.Add(name, key)
	}

	n := p.e.namespace(namespace)
	if n == nil {
		return nil, ErrNoNamespace
	}

	secret, err := p.e.kc.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	b64Data, ok := secret.Data[key]
	if !ok {
		return nil, nil
	}

	return b64Data, nil
}

// Env returns the value of an environment variable
func (p *DataProvider) Env(name string) string {
	return os.Getenv(name)
}

// Service returns a kubernetes Service
func (p *DataProvider) Service(name, namespace string) (s *v1.Service, err error) {
	if p.learning {
		n := p.getOrCreateNamespace(namespace)

		n.Services.Add(name)
	}

	n := p.e.namespace(namespace)
	if n == nil {
		return nil, ErrNoNamespace
	}

	return p.e.kc.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// ServiceIP returns a kubernetes Service's ClusterIP
func (p *DataProvider) ServiceIP(name, namespace string) (string, error) {
	service, err := p.Service(name, namespace)
	if err != nil {
		return "", nil
	}

	return service.Spec.ClusterIP, nil
}

// Endpoints returns the Endpoints for the given Service
func (p *DataProvider) Endpoints(name, namespace string) (ep *v1.Endpoints, err error) {
	if p.learning {
		n := p.getOrCreateNamespace(namespace)

		n.Endpoints.Add(name)
	}

	n := p.e.namespace(namespace)
	if n == nil {
		return nil, ErrNoNamespace
	}

	return p.e.kc.CoreV1().Endpoints(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// EndpointIPs returns the set of IP addresses for the given Service's endpoints.
func (p *DataProvider) EndpointIPs(name, namespace string) (out []string, err error) {
	eps, err := p.Endpoints(name, namespace)
	if err != nil {
		return nil, err
	}

	for _, ss := range eps.Subsets {
		for _, addr := range ss.Addresses {
			out = append(out, addr.IP)
		}
	}

	return
}

// Network retrieves network information about the running Pod.
func (p *DataProvider) Network(kind string) (string, error) {
	switch strings.ToLower(kind) {
	case "hostname":
		return p.e.disc.Hostname()
	case "privateipv4", "privatev4":
		ip, err := p.e.disc.PrivateIPv4()
		return ip.String(), err
	case "publicipv4", "publicv4":
		ip, err := p.e.disc.PublicIPv4()
		return ip.String(), err
	case "publicipv6", "publicv6":
		ip, err := p.e.disc.PublicIPv6()
		return ip.String(), err
	default:
		return "", errors.New("unhandled kind")
	}
}
