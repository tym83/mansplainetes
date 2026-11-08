#!/usr/bin/env bash
# End-to-end test of the admission webhook chart on a live cluster (kind in
# CI). Installs the chart, then talks to the API server as people and as
# the cluster itself and checks who hears what.
#
#   hack/e2e-webhook.sh helm           # chart-generated certificate
#   hack/e2e-webhook.sh cert-manager   # certificate from cert-manager
#
# Expects kubectl and helm pointed at a disposable cluster, and the image
# IMAGE (default mansplainetes-webhook:e2e) already loaded into it.
set -euo pipefail

mode=${1:?usage: $0 helm|cert-manager}
image=${IMAGE:-mansplainetes-webhook:e2e}
release=mansplainetes
ns=mansplainetes
loud=shop
quiet=backyard
root=$(cd "$(dirname "$0")/.." && pwd)

pass=0
fail=0
ok() { echo "ok   - $*"; pass=$((pass + 1)); }
bad() { echo "FAIL - $*"; fail=$((fail + 1)); }

# expect <description> <output> <pattern>...: every pattern is in output.
expect() {
	local what=$1 out=$2
	shift 2
	for p in "$@"; do
		if grep -qiF -- "$p" <<<"$out"; then
			ok "$what: $p"
		else
			bad "$what: missing '$p'"
			echo "$out" | sed 's/^/    | /'
		fi
	done
}

# silent <description> <output>: no admission warning at all.
silent() {
	if grep -q '^Warning:' <<<"$2"; then
		bad "$1: expected no warnings"
		echo "$2" | sed 's/^/    | /'
	else
		ok "$1: no warnings"
	fi
}

# as <user> <kubectl args...>: kubectl as a person, stdout and stderr.
as() {
	local user=$1
	shift
	kubectl --as="$user" --as-group=system:authenticated "$@" 2>&1
}

repo=${image%:*}
tag=${image##*:}
chart_args=(--namespace "$ns" --create-namespace
	--set image.repository="$repo" --set image.tag="$tag" --set image.pullPolicy=Never
	--wait --timeout 3m)

echo "# install ($mode)"
if [ "$mode" = cert-manager ]; then
	kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION:-v1.16.2}/cert-manager.yaml" >/dev/null
	kubectl -n cert-manager rollout status deploy --timeout=3m
	chart_args+=(--set certManager.enabled=true)
fi
helm upgrade --install "$release" "$root/charts/mansplainetes" "${chart_args[@]}"

if [ "$mode" = cert-manager ]; then
	for _ in $(seq 60); do
		bundle=$(kubectl get validatingwebhookconfiguration "$release" -o jsonpath='{.webhooks[0].clientConfig.caBundle}')
		[ -n "$bundle" ] && break
		sleep 2
	done
	[ -n "$bundle" ] && ok "cert-manager injected the CA bundle" || bad "cert-manager did not inject the CA bundle"
fi

echo "# people and namespaces"
kubectl create namespace "$loud"
kubectl create namespace "$quiet"
kubectl label namespace "$loud" mansplainetes.dev/mode=on
# Labelled on purpose: kube-system must stay quiet anyway.
kubectl label namespace kube-system mansplainetes.dev/mode=on --overwrite
kubectl create serviceaccount robot -n kube-system
kubectl create serviceaccount argo -n "$loud"
for who in alice@example.com bob@example.com carol@example.com warmup@example.com system:e2e-robot; do
	kubectl create clusterrolebinding "e2e-${who%%@*}" --clusterrole=cluster-admin --user="$who" >/dev/null
done
kubectl create clusterrolebinding e2e-sa --clusterrole=cluster-admin \
	--serviceaccount=kube-system:robot --serviceaccount="$loud:argo" >/dev/null

# The API server may need a moment to reach a fresh webhook; a server-side
# dry run does not count against the rate limit.
for _ in $(seq 60); do
	out=$(as warmup@example.com -n "$loud" run warmup --image=busybox:1.36 --dry-run=server)
	grep -q '^Warning:' <<<"$out" && break
	sleep 2
done
expect "webhook reachable" "$out" "Warning:"

echo "# a person in a labelled namespace hears him"
greedy() {
	cat <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: greedy
spec:
  replicas: 1
  selector:
    matchLabels: {app: greedy}
  template:
    metadata:
      labels: {app: greedy}
    spec:
      containers:
        - name: web
          image: nginx:latest
EOF
}
out=$(greedy | as alice@example.com -n "$loud" apply -f -)
expect "alice creates a deployment" "$out" \
	"Warning:" \
	"you're creating deployment/greedy in $loud" \
	"PMS: Probably My Setup. deployment/greedy runs a single replica." \
	'container "web" runs nginx:latest, the latest tag.' \
	'container "web" has no memory limit.' \
	"...and 2 more things"

out=$(as alice@example.com -n "$loud" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: probeless
spec:
  containers:
    - name: app
      image: nginx:1.27
      resources:
        requests: {cpu: 10m, memory: 16Mi}
        limits: {memory: 32Mi}
EOF
)
expect "alice creates a pod without probes" "$out" \
	'container "app" has no readiness probe.' \
	'container "app" has no liveness probe.'

echo "# rate limit: the same thing to the same person once per window"
out=$(as alice@example.com -n "$loud" label deploy greedy tier=one)
expect "first update" "$out" "you're changing metadata.labels of deployment/greedy"
out=$(as alice@example.com -n "$loud" label deploy greedy tier=two --overwrite)
silent "second update by the same person" "$out"
out=$(as bob@example.com -n "$loud" scale deploy greedy --replicas=3)
expect "another person scales the same deployment" "$out" "Scaling from 1 to 3 replicas was my idea"

echo "# nobody else hears him"
out=$(greedy | as alice@example.com -n "$quiet" apply -f -)
silent "unlabelled namespace" "$out"
out=$(as alice@example.com -n kube-system run quiet --image=nginx:latest)
silent "kube-system namespace" "$out"
out=$(as system:e2e-robot -n "$loud" run robot --image=nginx:latest)
silent "system: user" "$out"
out=$(kubectl --as=system:serviceaccount:kube-system:robot -n "$loud" run kube-system-sa --image=nginx:latest 2>&1)
silent "kube-system service account" "$out"
out=$(kubectl --as=system:serviceaccount:$loud:argo -n "$loud" run argo --image=nginx:latest 2>&1)
silent "service account (serviceAccounts=false)" "$out"

if [ "$mode" = helm ]; then
	echo "# helm upgrade keeps the generated certificate"
	sum() { kubectl -n "$ns" get secret "$release-tls" -o jsonpath="{.data.$1}" | sha256sum | cut -d' ' -f1; }
	crt=$(sum 'tls\.crt')
	ca=$(sum 'ca\.crt')
	bundle=$(kubectl get validatingwebhookconfiguration "$release" -o jsonpath='{.webhooks[0].clientConfig.caBundle}')
	helm upgrade "$release" "$root/charts/mansplainetes" "${chart_args[@]}" --set quietFor=11m
	# quietFor changes the pod template: wait for the new pod, or the next
	# request can reach no webhook at all (failurePolicy Ignore) and stay silent.
	kubectl -n "$ns" rollout status deploy "$release" --timeout=2m
	[ "$crt" = "$(sum 'tls\.crt')" ] && ok "tls.crt kept across upgrade" || bad "tls.crt changed on upgrade"
	[ "$ca" = "$(sum 'ca\.crt')" ] && ok "ca.crt kept across upgrade" || bad "ca.crt changed on upgrade"
	[ "$bundle" = "$(kubectl get validatingwebhookconfiguration "$release" -o jsonpath='{.webhooks[0].clientConfig.caBundle}')" ] &&
		ok "caBundle kept across upgrade" || bad "caBundle changed on upgrade"
	out=$(as carol@example.com -n "$loud" run after-upgrade --image=nginx:latest)
	expect "after upgrade" "$out" "you're creating pod/after-upgrade in $loud"
fi

echo "# webhook down: requests pass (failurePolicy Ignore)"
kubectl -n "$ns" scale deploy "$release" --replicas=0
kubectl -n "$ns" wait --for=delete pod -l app.kubernetes.io/instance="$release" --timeout=2m
if out=$(as carol@example.com -n "$loud" run while-down --image=nginx:latest); then
	ok "request allowed while the webhook is down"
else
	bad "request refused while the webhook is down"
fi
silent "webhook down" "$out"
kubectl -n "$ns" scale deploy "$release" --replicas=1
kubectl -n "$ns" rollout status deploy "$release" --timeout=2m

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
