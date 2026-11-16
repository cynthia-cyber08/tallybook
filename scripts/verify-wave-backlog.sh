#!/usr/bin/env bash
# Verify the wave backlog programmatically. Exits non-zero on any failure.
set -uo pipefail
cd "$(dirname "$0")/.."
BACKLOG="${BACKLOG:-$(ls -1 docs/WAVE*_BACKLOG.json | sort -V | tail -1)}"
fail=0
chk() { if [ "$2" = "$3" ]; then echo "  PASS  $1 ($2)"; else echo "  FAIL  $1: expected $3, got $2"; fail=1; fi; }

echo "=== 1. counts per complexity tier and exact point total ==="
python3 - "$BACKLOG" <<'PY'
import json, sys, collections
import importlib.util as _u
_s = _u.spec_from_file_location("g", "scripts/gen-wave-backlog.py")
_g = _u.module_from_spec(_s); _s.loader.exec_module(_g)
P = _g.POINTS  # single source of truth for the ladder
iss = json.load(open(sys.argv[1]))
t = collections.Counter(i["complexity"] for i in iss)
for tier in ("high", "medium", "low"):
    print(f"  {tier:<7} x {t.get(tier,0):>3}  @ {P[tier]:>3} = {t.get(tier,0)*P[tier]:>6}")
print(f"  {'TOTAL':<7} x {len(iss):>3}        = {sum(P[i['complexity']] for i in iss):>6}")
PY

echo
echo "=== 2. no duplicate titles within the backlog ==="
dupes=$(jq -r '.[].title' "$BACKLOG" | sort | uniq -d | wc -l | tr -d ' ')
chk "duplicate titles inside backlog" "$dupes" "0"

echo
echo "=== 3. reconciliation against live issues ==="
# This backlog's issues may already be filed. A collision with an issue we
# filed from THIS backlog is expected; a collision with anything older is not.
# WAVE_FIRST_ISSUE is the first issue number this wave created.
WAVE_FIRST_ISSUE="${WAVE_FIRST_ISSUE:-13}"
jq -r '.[].title' "$BACKLOG" | sort > /tmp/wave_want.txt
gh issue list --repo Tallybook-Org/tallybook --state all --limit 1000 \
   --json number,title -q '.[] | [(.number|tostring), .title] | @tsv' > /tmp/wave_live.tsv
awk -F'\t' -v k="$WAVE_FIRST_ISSUE" '$1 <  k {print $2}' /tmp/wave_live.tsv | sort > /tmp/wave_older.txt
awk -F'\t' -v k="$WAVE_FIRST_ISSUE" '$1 >= k {print $2}' /tmp/wave_live.tsv | sort > /tmp/wave_wave.txt
gh issue list --repo Tallybook-Org/tallybook-contracts --state all --limit 1000 \
   --json title -q '.[].title' | sort >> /tmp/wave_older.txt
sort -u /tmp/wave_older.txt -o /tmp/wave_older.txt

filed=$(comm -12 /tmp/wave_want.txt /tmp/wave_wave.txt | wc -l | tr -d ' ')
pending=$(comm -23 /tmp/wave_want.txt /tmp/wave_wave.txt | wc -l | tr -d ' ')
orphan=$(comm -13 /tmp/wave_want.txt /tmp/wave_wave.txt | wc -l | tr -d ' ')
echo "  already filed from this backlog: $filed"
echo "  in backlog, not yet filed:       $pending"
if [ "$pending" -gt 0 ]; then comm -23 /tmp/wave_want.txt /tmp/wave_wave.txt | sed 's/^/      + /'; fi
chk "wave issues on GitHub that are NOT in this backlog" "$orphan" "0"
[ "$orphan" -ne 0 ] && comm -13 /tmp/wave_want.txt /tmp/wave_wave.txt
collide=$(comm -12 /tmp/wave_want.txt /tmp/wave_older.txt | wc -l | tr -d ' ')
chk "titles colliding with a PRE-EXISTING issue" "$collide" "0"
[ "$collide" -ne 0 ] && comm -12 /tmp/wave_want.txt /tmp/wave_older.txt

echo
echo "=== 4. required fields present and in range ==="
python3 - "$BACKLOG" <<'PY'
import json, sys, os
iss = json.load(open(sys.argv[1]))
bad = 0
for n, i in enumerate(iss, 1):
    for f in ("title","complexity","area","description","why","criteria","guidance"):
        if not i.get(f): print(f"  FAIL  issue {n}: missing {f}"); bad += 1
    if not 3 <= len(i.get("criteria",[])) <= 5: print(f"  FAIL  issue {n}: {len(i['criteria'])} criteria"); bad += 1
    if not 2 <= len(i.get("guidance",[])) <= 4: print(f"  FAIL  issue {n}: {len(i['guidance'])} guidance"); bad += 1
print(f"  PASS  all {len(iss)} issues have every required field, criteria 3-5, guidance 2-4" if not bad
      else f"  {bad} field problems")
sys.exit(1 if bad else 0)
PY
[ $? -ne 0 ] && fail=1

echo
echo "=== 5. good_first_issue only on trivial/low, each with a real entry point ==="
python3 - "$BACKLOG" <<'PY'
import json, sys, os
iss = json.load(open(sys.argv[1])); bad = 0
gfi = [i for i in iss if i.get("good_first_issue")]
for i in gfi:
    if i["complexity"] != "low":
        print(f"  FAIL  '{i['title'][:50]}' is good_first_issue but complexity {i['complexity']}"); bad += 1
    ep = i.get("entry_point")
    if not ep: print(f"  FAIL  '{i['title'][:50]}' has no entry_point"); bad += 1
    elif not os.path.exists(ep): print(f"  FAIL  entry_point does not exist: {ep}"); bad += 1
print(f"  PASS  {len(gfi)} good-first-issues, all complexity:low, all entry points exist" if not bad else "")
sys.exit(1 if bad else 0)
PY
[ $? -ne 0 ] && fail=1

echo
echo "=== 6. every cited file:line in every issue body resolves ==="
python3 - "$BACKLOG" <<'PY'
import json, sys, re, os
iss = json.load(open(sys.argv[1]))
pat = re.compile(r'\b((?:cmd|internal|scripts|docs|\.github)/[\w./-]+\.(?:go|sql|yml|md)):(\d+)\b')
checked = bad = 0
for i in iss:
    for text in [i["why"], i["description"]] + i["criteria"] + i["guidance"]:
        for path, line in pat.findall(text):
            checked += 1
            if not os.path.exists(path):
                print(f"  FAIL  {path} does not exist"); bad += 1; continue
            n = sum(1 for _ in open(path))
            if int(line) > n:
                print(f"  FAIL  {path}:{line} beyond EOF ({n} lines)"); bad += 1
print(f"  PASS  {checked} file:line citations checked, all resolve" if not bad else f"  {bad} of {checked} citations broken")
sys.exit(1 if bad else 0)
PY
[ $? -ne 0 ] && fail=1

echo
if [ "$fail" -eq 0 ]; then echo "ALL CHECKS PASSED"; else echo "SOME CHECKS FAILED"; fi
exit "$fail"
