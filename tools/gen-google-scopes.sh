#!/bin/sh
# Emit the google-cloud purpose block for sources.yaml.
#
# Google's scope values become purpose keys, which are public API and become
# prefix list names the reconciler treats as immutable identity. They are
# therefore committed and reviewed rather than generated at build time: the
# coverage check in checkGoogleScopeCoverage fails the build when Google adds
# or renames a region, and this script produces the replacement block so nobody
# hand-types 48 lines.
#
# Usage: tools/gen-google-scopes.sh   then paste over the purposes: block.
set -eu
curl -fsS https://www.gstatic.com/ipranges/cloud.json | python3 -c '
import json, sys
d = json.load(sys.stdin)["prefixes"]
scopes = sorted({p["scope"] for p in d if p.get("scope")})
print("          - {key: all, direction: egress, aggregate: true, select: \"*\"}")
w = max(len(s) for s in scopes) + 1
for s in scopes:
    key = s + ","
    print("          - {key: %-*s direction: egress, select: \"scope=%s\"}" % (w, key, s))
'
