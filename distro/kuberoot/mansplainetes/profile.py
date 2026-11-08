"""Turns kuberoot's edge profile into Mansplainetes': the same node, with
mansplainetes-boot as the first service of every role. It narrates the
node's boot on the console from kinit's own service list (read-only, over
kinit's control socket) and then keeps watching, so kinit never counts it as
restarting. Nothing in kuberoot's core changes.

Usage: profile.py EDGE_PROFILE > PROFILE"""
import sys

text = open(sys.argv[1]).read()
if "\n  name: edge\n" not in text:
    sys.exit("profile of edge changed: no metadata.name to replace")
text = text.replace("\n  name: edge\n", "\n  name: mansplainetes\n", 1)

services = "\n      services:\n"
if text.count(services) != 2:
    sys.exit("profile of edge changed: expected the services of two roles")
narrator = """      - name: mansplainetes-boot
        args: [/usr/bin/mansplainetes-boot, --name=mansplainetes-boot]
"""
text = text.replace(services, services + narrator)
sys.stdout.write(text)
