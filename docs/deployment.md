---
title: Deployment
---

# Deployment

The examples in this guide use small defaults. Measure real memory and CPU use
with your IdP and traffic before raising or lowering the limits.

## Container image

`deploy/Dockerfile` uses pinned Go and Distroless base-image digests. The final
image has no shell, runs as UID 65532, and contains only the static proxy binary.
The proxy does not need a writable filesystem.

Build it from the repository root:

```sh
docker build --file deploy/Dockerfile --tag idmux-proxy:local .
```

For production, use a published IdMux image pinned by its immutable registry
digest. The production examples fix the image name and require
`IDMUX_IMAGE_DIGEST` to contain a published `sha256:...` digest. Do not use a
tag such as `latest` or copy the example placeholder as a real digest. The
release workflow scans both platform images, promotes those same image archives,
and publishes SBOM and provenance attestations.

## Docker Compose

`deploy/docker-compose.production.yml` is for a single host with an existing TLS
ingress. The ingress and IdMux must share the external `edge` network; the IdMux
container is not published directly to the host. It also joins a separate
network for the upstream IdP. A Compose bridge network groups containers; it is
not an outbound destination allowlist. Use a host firewall or egress gateway to
limit outbound traffic to the IdP.

Set `IDMUX_IMAGE_DIGEST` to a published `sha256:...` digest,
`IDMUX_UPSTREAM_URL` to an HTTPS IdP URL, `IDMUX_TRUSTED_ORIGINS` to the
browser-facing HTTPS origin, and
`IDMUX_ENCRYPTION_KEY_FILE` to a secret file created by your secret manager.
Validate the digest before starting Compose:

```sh
sh deploy/validate-image-digest.sh "$IDMUX_IMAGE_DIGEST"
docker compose --file deploy/docker-compose.production.yml config
```

The file must contain the base64-encoded 32-byte key. Do not put the key in a
committed file or shell history. The example drops all Linux
capabilities, prevents privilege escalation, uses a read-only root filesystem,
and applies process, memory, and CPU limits.

Because file-based Compose secrets use a host file, check that the secret is
readable by container UID 65532 on the target Docker host. Keep its host
permissions limited to that runtime identity; do not make it world-readable.

The image has no shell or HTTP client, so it cannot run a shell-based Compose
health check. Use the Kubernetes probes below or monitor the HTTP health endpoint
from the host or ingress platform.

## Kubernetes

The manifests in `deploy/kubernetes/` create a restricted namespace, a
two-replica Deployment, an internal Service, and a default-deny NetworkPolicy.
They are examples; change the IdP URL, browser origin, ingress selectors, DNS
labels, and IdP egress address to match your cluster.

Create the `idmux-session` Secret through your cluster's secret manager. Its
`key` value must contain the base64-encoded 32-byte encryption key. The pod
mounts the Secret read-only and reads it through `IDMUX_ENCRYPTION_KEY_FILE`.
Do not commit a Kubernetes Secret containing a live key.

The production deployment file is a template. Render it with a real immutable
image digest, then review the output before applying it:

```sh
export IDMUX_IMAGE_DIGEST='sha256:<published-digest>'
sh deploy/validate-image-digest.sh "$IDMUX_IMAGE_DIGEST"
envsubst '${IDMUX_IMAGE_DIGEST}' < deploy/kubernetes/production.yaml.tmpl > /tmp/idmux-production.yaml
kubectl diff -f /tmp/idmux-production.yaml
kubectl apply -f /tmp/idmux-production.yaml
kubectl apply -f deploy/kubernetes/network-policy.yaml
```

Replace the example IdP egress address `203.0.113.10/32` in the NetworkPolicy.
It is a documentation-only address and intentionally will not reach a real IdP.
For an IdP inside the cluster, use a namespace and pod selector instead. For an
external IdP, use stable, narrow IP ranges or an egress gateway / CNI policy that
supports FQDN rules. Do not allow `0.0.0.0/0` just to make DNS or IdP access work.
The policy assumes CoreDNS labels `k8s-app: kube-dns` and ingress-nginx labels;
verify these selectors in your cluster. NetworkPolicy controls reachability; it
does not encrypt traffic. A CNI plugin must enforce NetworkPolicy. Kubernetes
combines matching NetworkPolicies additively, so review every policy that
selects these pods; another policy may allow more traffic. This example does
not remove permissions granted by other policies.

The Service is internal and speaks HTTP. Terminate public TLS at a trusted
ingress and restrict ingress-to-pod traffic with the policy. If policy requires
encryption on that hop, use a service mesh or an ingress that supports mTLS;
IdMux does not currently serve TLS itself. IdMux strips client-provided
`Forwarded`, `X-Forwarded-*`, and `X-Real-IP` headers. Do not rely on those
headers to carry trusted client data through IdMux. The ingress must terminate
browser TLS and must not expose the proxy's internal port publicly.

The sidecar example is a container fragment, not a complete Pod. Replace its
image placeholder with the published image pinned by digest; Kubernetes does
not expand shell variables in an image field. Add the `idmux-key` Secret volume
and set `fsGroup: 65532` on the Pod so the process can read the mounted key.
This mode is useful when an application in the same Pod must call IdMux at
`127.0.0.1:9000`. The sidecar is not reachable by the browser on its own. Public
browser traffic must still enter through the ingress or gateway that serves the
IdP's public origin. Use the shared Deployment and Service for that pattern.

NetworkPolicy selects whole Pods, not individual containers. The supplied
policy selects the standalone Deployment. For a sidecar, create or adapt a
policy for the application's Pod labels and keep the application's required
network access. Do not assume this standalone policy protects a sidecar Pod.

Restrict `GET /__idmux/metrics` to trusted monitoring systems. The endpoint
shares the HTTP port, so NetworkPolicy cannot filter it by path; use ingress
rules, a separate internal scrape path, or an authenticated monitoring proxy.
The health and readiness endpoints have no application authentication. Keep
them internal to platform probes. The sessions endpoint is intended for the
account picker; expose it only on the expected application origin and do not
treat its Origin/Referer checks as user authentication. Configure the ingress
not to publish other control paths.

## Limits and probes

The Go HTTP server limits headers to 32 KiB and applies read, write, and idle
timeouts. `IDMUX_UPSTREAM_TIMEOUT` defaults to 45 seconds and cancels an IdP
request that takes longer, including a response body that stalls. It can be set
from 1 second to 5 minutes; the server write timeout leaves time for the proxy
to return an error after cancellation. If the IdP has already sent response
headers, the proxy cannot replace the status; the client may receive a truncated
response when the timeout expires. The request body limit defaults to 1 MiB and
returns HTTP 413 when the request body exceeds the limit. A declared
oversized body is rejected before forwarding. For chunked bodies, the proxy
stops forwarding when the cap is reached; the IdP may receive a partial body and
must reject the incomplete request. Set the limit only as high as the largest
login or protocol form needs.

The Kubernetes startup probe allows up to one minute for process startup. The
liveness probe checks process health, and readiness controls Service traffic.
The readiness endpoint does not check IdP availability. The example requests
64 MiB and limits the container to 128 MiB; adjust after
measuring. A memory limit that is too low can cause an OOM kill.
