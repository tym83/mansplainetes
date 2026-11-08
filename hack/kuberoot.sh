#!/usr/bin/env bash
# Builds kuberoot as the distribution Mansplainetes: the sources of kuberoot
# with distros/mansplainetes added (edge's profile with mansplainetes-boot as
# the first service of every role, and its program in the image), driven by
# kuberoot's own hack/hetzner.sh in a builder of the tenant mansplainetes.
# Nothing heavy runs here: binaries, the image and the media are built in the
# builder. Nothing in kuberoot's core changes.
#
#   hack/kuberoot.sh build amd64
#   hack/kuberoot.sh bundle amd64
#   hack/kuberoot.sh media amd64 [kernel args]
#   hack/kuberoot.sh node <name> amd64
#   hack/kuberoot.sh console <name>
#
#   KUBEROOT_SRC  a checkout of kuberoot (default: ../kuberoot)
#   KUBEROOT_REF  what of it to build (default: the commit in .upstream-version)
#   NS            the builder's namespace (default: tenant-mansplainetes)
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SRC=${KUBEROOT_SRC:-$ROOT/../kuberoot}
REF=${KUBEROOT_REF:-$(cat "${ROOT}/.upstream-version")}
NS=${NS:-tenant-mansplainetes}
export KUBECONFIG=${KUBECONFIG_WORKSHOP:-$HOME/workshop.txt}
CTX=${KUBE_CONTEXT:-admin@workshop}

work=$(mktemp -d)
trap 'rm -rf "${work}"' EXIT

git -C "${SRC}" archive "${REF}" | tar -x -C "${work}"
# hetzner.sh ships what git sees
git -C "${work}" init -q

d="${work}/distros/mansplainetes"
mkdir -p "${d}/addons"
python3 "${ROOT}/distro/kuberoot/mansplainetes/profile.py" "${work}/distros/edge/profile.yaml" > "${d}/profile.yaml"
cp "${work}"/distros/edge/addons/* "${d}/addons/"
cp "${ROOT}/distro/kuberoot/mansplainetes/extras.sh" "${d}/"

if [ "${1:-}" = build ]; then
  arch=${2:?arch required}
  # The narrator, where extras.sh takes it from.
  b() { kubectl --context "$CTX" -n "$NS" exec -i builder -c tools -- "$@"; }
  (cd "${ROOT}" && git ls-files -co --exclude-standard -z |
    COPYFILE_DISABLE=1 tar --null -T - -czf -) |
    b sh -c 'rm -rf /work/src/mansplainetes && mkdir -p /work/src/mansplainetes && tar -xzf - -C /work/src/mansplainetes'
  b sh -c "set -e; cd /work/src/mansplainetes; out=/work/out/${arch}/mansplainetes; mkdir -p \$out
    CGO_ENABLED=0 GOARCH=${arch} go build -trimpath -o \$out/mansplainetes-boot ./cmd/mansplainetes-boot
    ls -la \$out"
fi

KUBEROOT_NS="${NS}" KUBEROOT_DISTRO=mansplainetes "${work}/hack/hetzner.sh" "$@"
