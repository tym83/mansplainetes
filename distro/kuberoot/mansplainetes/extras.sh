# shellcheck shell=sh disable=SC2154 # $OUT and $r come from build-rootfs.sh
# Sourced by kuberoot's build-rootfs.sh: Mansplainetes on the node.
# hack/kuberoot.sh builds mansplainetes-boot into $OUT/mansplainetes.
install -m 0755 "$OUT/mansplainetes/mansplainetes-boot" "$r/usr/bin/mansplainetes-boot"
