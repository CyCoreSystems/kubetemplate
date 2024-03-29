# KubeTemplate

KubeTemplate (package `kubetemplate`) is a library to facilitate the
construction of reactive application configurations when running inside
Kubernetes.

It provides a common Go-based templating engine to construct application
configuration files using a number of kubernetes resources as sources.

## Usage

A templating engine can be used on any number of templates, but before calling
`Render()` on a template, you should run that template through a `Learn()`
cycle.

Thus, the usual process is to run each template through a `Learn()` whenever it
is added or changes, and then call `Render()` any number of times.
Note that `Learn()` takes out a lock on the entire engine, so you should not
attempt to run it concurrently.

The network portion of this tool relies on a netdiscover instance.
See [NetDiscover](https://github.com/CyCoreSystems/netdiscover) for more
information.
At a minimum, just create one with `discover.NewDiscoverer()`.

## Templates

Values for the templates may come from a number of sources:

  - [ConfigMap](#configmap-and-secret) / [Secret](#configmap-and-secret)
  - [Environment Variables](#environment-variables)
  - [Service](#service)
  - [ServiceIP](#serviceip)
  - [Endpoints](#endpoints) (of a Service)
  - [EndpointIPs](#endpoint-ips) (of a Service)
  - [Network](#network-data)

### ConfigMap and Secret

To obtain ConfigMap or Secret entries, KubeTemplate will use the Kubernetes API to
attempt to pull in the ConfigMap/Secret and key requested. 

**Format**:  `{{.ConfigMap "<name>" "<namespace>" "<key>"}}` 

**Format**:  `{{.Secret "<name>" "<namespace>" "<key>"}}` 

The provided namespace _may_ be `""` if both the ConfigMap/Secret is in the same
namespace as the Pod _and_ the `POD_NAMESPACE` environment variable is properly
set.

The ConfigMap/Secret will be monitored by the engine, and if it is updated, the
configuration files will be regenerated, and a reload will be performed.

Note that this will likely require an RBAC entry to allow the `ServiceAccount`
under which the engine is running to access the referenced ConfigMap.

**NOTE**: it is generally easier to use the standard kubernetes Pod methods to
translate ConfigMap and Secret values to environment variables.  However, doing
so does not currently result in changes to those referent ConfigMaps and Secrets
being propagated to running applications.  Therefore, the choice between using
these dynamic references and the native kubernetes environment variable bindings
is left to the user.


### Environment Variables

**Format**: `{{.Env "<name>"}}`

It is useful to note that IP addresses of services within the same namespace
will automatically be populated as environment variables by kubernetes.  These
will be of the form `<SERVICE_NAME>_SERVICE_HOST`.  For instance, the IP of a
service named "kamailio" will be stored in the environment variable
`KAMAILIO_SERVICE_HOST`.  This is a normal, default feature of all kubernetes
containers.  See the [documentation](https://kubernetes.io/docs/concepts/services-networking/service/) for more information.

### Service

Data from a kubernetes Service may be obtained using the Kubernetes API.

**Format**: `{{.Service "<name>" "<namespace>]"}}`

The provided namespace _may_ be `""` if both the Service is in the same
namespace as the Pod _and_ the `POD_NAMESPACE` environment variable is properly
set.

The value returned here is the Kubernetes
[Service](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#service-v1-core).
Keep in mind that Go uses PascalCase for the fields, so "clusterIP" becomes
"ClusterIP".

For example, to get the ClusterIP of a service named "kamailio" in the "voip"
namespace:

`{{ with .Service "kamailio" "voip"}}{{.Spec.ClusterIP}}{{end}}`

Note that the IP address of a service within the same namespace can be obtained
more simply by environment variable, as described above.

### ServiceIP

Since the most common reason to probe a service is to retrieve its ClusterIP, we
have also included a macro which does just that.

**Format**: `{{.ServiceIP "<name>" "<namespace>]"}}`

This works as Service, but instead of returning a structure, it just returns the
ClusterIP of the Service, as a string.


### Endpoints

Data from the kubernetes Endpoints of a Service may be obtained using the
Kubernetes API.

**Format**: `{{.Service "<name>" "<namespace>"}}`

The provided namespace _may_ be `""` if both the Service is in the same
namespace as the Pod _and_ the `POD_NAMESPACE` environment variable is properly
set.

The value returned is the Kubernetes [Endpoints](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#endpoints-v1-core).

The Endpoints will be monitored by the engine, and if it is updated, the
reload channel will be signaled.

This is usually used to obtain the dynamic set of proxy servers, but since the
most common reason to do this is to obtain the set of IPs for endpoints of a
service, we provide a second helper function just for that.

### Endpoint IPs

One of the most common pieces of dynamic data to retrieve is the set of IPs for
the endpoints of a service.  Therefore, to simplify the relatively tedious
iteration of these directly from the Endpoints spec, we provide the EndpointIPs
macro, which returns the list of IPs of all Endpoints of the given service
name.

**Format**: `{{.EndpointIPs "<name>" "<namespace>"}}`

The provided namespace _may_ be `""` if both the Service is in the same
namespace as the Pod _and_ the `POD_NAMESPACE` environment variable is properly
set.

Using this is then easy.  For example, to create a PJSIP endpoint from the set
of proxy servers running as the "kamailio" service:

`pjsip.d/proxies.conf`:

```ini
[proxies]
type=endpoint
transport=k8s-internal-ipv4-external-media
context=from-proxies
disallow=all
allow=ulaw
aors=proxies
rtp_symmetric=yes

[proxies]
type=aor
{{range .EndpointIPs "kamailio"}}
contact=sip:{{.}}
{{end}}

[proxies]
type=identify
endpoint=proxies
{{range .EndpointIPs "kamailio"}}
match={{.}}
{{end}}

[proxies]
type=acl
deny=0.0.0.0/0.0.0.0
{{range .EndpointIPs "kamailio"}}
permit={{.}}
{{end}}
```

The Endpoints IPs will be monitored by the engine, and if they are updated, the
configuration files will be regenerated, and a reload will be performed.


### Network data

The IP addresses for the running Pod are made available, as well.

**Format**: `{{.Network "<kind>"}}`

The available data kinds correspond to the data available from
[NetDiscover](https://github.com/CyCoreSystems/netdiscover):

  - "hostname"
  - "privatev4"
  - "publicv4"
  - "publicv6"


