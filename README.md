# faas-idler

Scale OpenFaaS functions to zero replicas after a period of inactivity

> Premise: functions (Deployments) can be scaled to 0/0 replicas from 1/1 or N/N replicas when they are not receiving traffic. Traffic is observed from Prometheus metrics collected in the OpenFaaS API Gateway.

![](./docs/faas-idler.png)

Scaling to zero requires an "un-idler" or a blocking HTTP proxy which can reverse the process when incoming requests attempt to access a given function. This is done through the OpenFaaS API Gateway through which every incoming call passes - see [Add feature: scale from zero to 1 replicas #685](https://github.com/openfaas/faas/pull/685).

faas-idler is implemented as a controller which polls Prometheus metrics on a regular basis and tries to reconcile a desired condition - i.e. zero replicas -> scale down API call.

## Building

The build requires Docker and builds a local Docker image.

```
TAG=0.1.1 make build
TAG=0.1.1 make push
```

## Usage

### Quick start

Swarm:

```
docker stack deploy func -c docker-compose.yml
```

Kubernetes

```
kubectl apply -f faas-idler-dep.yml
```

### Configuration

* Environmental variables:

On Kubernetes the `gateway_url` needs to contain the suffix of the namespace you picked at deploy time. This is usually `.openfaas` and is pre-configured with a default.

Try using the ClusterIP/Cluster Service instead and port 8080.

`gateway_url` - URL for faas-provider
`prometheus_host` - host for Prometheus
`prometheus_port` - port for Prometheus
`inactivity_duration` - i.e. `10m` (Golang duration)
`reconcile_interval` - i.e. `30s` (default value)


* Command-line args

`-dry-run` - don't send scaling event 

How it works:

`gateway_function_invocation_total` is measured for activity over `duration` i.e. `1h` of inactivity (or no HTTP requests)

## Logs

You can view the logs to show reconciliation in action.

```
kubectl logs -n openfaas -f deploy/faas-idler
```

