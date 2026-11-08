# Mansplainetes chart

Server mode for [Mansplainetes](https://github.com/tym83/mansplainetes): a
validating admission webhook that mansplains every change back to the person
who made it, whatever client they used. It is a satire of mansplaining. He is
the joke, not you.

He talks through admission warnings, the `Warning:` lines that kubectl, k9s,
Lens and Argo CD already show. He always allows the request, so the cluster
behaves exactly as it would without him:

```text
$ kubectl apply -f web.yaml
Warning: Well, actually, you're changing spec.replicas, spec.template of deployment/web in shop. I'd have changed fewer things.
Warning: Scaling from 1 to 3 replicas was my idea, by the way.
Warning: PMS: Probably My Setup. container "web" runs nginx, with no tag, which means latest. Latest is the best version. Everyone knows that.
Warning: PMS: Probably My Setup. container "web" has no memory limit. Limits are for people who don't trust their code.
deployment.apps/web configured
```

Every fact in a PMS hint is read from the object being applied: no memory
limit, no resource requests, an image with no tag or `:latest` and no digest,
no readiness or liveness probe (not for Jobs), a Deployment or StatefulSet
with a single replica. The opinions are his.

## Install

```bash
helm install mansplainetes oci://ghcr.io/tym83/charts/mansplainetes \
  --version <version> --namespace mansplainetes --create-namespace
kubectl label namespace shop mansplainetes.dev/mode=on
```

Nothing happens until a namespace is labelled.

## What he never does

- Change, deny or delay a request. The webhook always allows, has
  `failurePolicy: Ignore` and `sideEffects: None`, and times out after
  3 seconds.
- Talk to the cluster itself: nodes, controllers and every other `system:`
  identity pass untouched. Service accounts (Argo CD, Flux, operators) pass
  too, unless `serviceAccounts=true`; those in `kube-system` always do.
- Look at system namespaces (`kube-*` and the release namespace), nodes,
  leases, events or Secrets.
- Repeat himself: the same object and operation is mansplained to the same
  person at most once per `quietFor` (10 minutes). A `--dry-run=server` is
  mansplained without using that up.
- Remember anything beyond that window, or call the API server. The webhook
  has no RBAC and no service account token.

## Values

| Value | Default | Meaning |
| --- | --- | --- |
| `namespaceLabel.key` / `.value` | `mansplainetes.dev/mode` / `on` | Label that opts a namespace in. |
| `excludedNamespaces` | `kube-system`, `kube-public`, `kube-node-lease` | Never, labelled or not. The release namespace is always added. |
| `rules` | pods, services, workloads, their `scale` | What he comments on. |
| `quietFor` | `10m` | Rate limit per person and object. |
| `serviceAccounts` | `false` | Talk to service accounts too. |
| `off` | `false` | Silence him without uninstalling. |
| `certManager.enabled` | `false` | Use cert-manager for TLS; otherwise the chart generates a CA once and keeps it. |
| `image.repository` / `.tag` | `ghcr.io/tym83/mansplainetes-webhook` / appVersion | Image. |

Silence a namespace with `kubectl label namespace <ns> mansplainetes.dev/mode-`.
