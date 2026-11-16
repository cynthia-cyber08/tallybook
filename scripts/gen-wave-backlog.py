#!/usr/bin/env python3
"""Regenerate docs/WAVE<N>_BACKLOG.md from docs/WAVE<N>_BACKLOG.json.

The JSON is the source of truth. The Markdown is never hand-edited: run
`make backlog` (or this script) after any change to the JSON.

Also validates the backlog, and exits non-zero if anything is malformed —
scripts/create-wave-issues.sh calls this first and refuses to touch GitHub
if it fails.
"""
import json, sys, os, re, collections

# Point values per complexity tier. Set by the project maintainer, whose
# discretion this is; a 1.7x uniform scale-up of the original 200/150/100,
# preserving the 2:1.5:1 tier ratio. Points are wave-level accounting and
# deliberately do NOT appear in rendered issue bodies: repricing the ladder
# must never require editing an already-filed issue.
POINTS = {"high": 340, "medium": 255, "low": 165}
REQUIRED = ("title", "complexity", "area", "description", "why", "criteria", "guidance")
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# Which wave to operate on. Defaults to the highest-numbered WAVE*_BACKLOG.json
# present, so adding a wave needs no edit here; override with WAVE=n.
def _latest_wave():
    import glob
    ns = []
    for f in glob.glob(os.path.join(REPO_ROOT, "docs/WAVE*_BACKLOG.json")):
        m = re.search(r"WAVE(\d+)_BACKLOG\.json$", f)
        if m:
            ns.append(int(m.group(1)))
    return str(max(ns)) if ns else "2"

WAVE = os.environ.get("WAVE") or _latest_wave()


def validate(issues):
    """Return a list of human-readable problems; empty means valid."""
    problems = []
    seen = collections.Counter(i.get("title", "") for i in issues)
    for n, issue in enumerate(issues, 1):
        tag = f"issue {n} ({issue.get('title', '<no title>')[:60]!r})"
        for field in REQUIRED:
            if not issue.get(field):
                problems.append(f"{tag}: missing or empty field {field!r}")
        if issue.get("complexity") not in POINTS:
            problems.append(f"{tag}: complexity must be one of {sorted(POINTS)}")
        crit, guid = issue.get("criteria", []), issue.get("guidance", [])
        if not 3 <= len(crit) <= 5:
            problems.append(f"{tag}: needs 3-5 criteria, has {len(crit)}")
        if not 2 <= len(guid) <= 4:
            problems.append(f"{tag}: needs 2-4 guidance notes, has {len(guid)}")
        gfi = issue.get("good_first_issue", False)
        if gfi and issue.get("complexity") != "low":
            problems.append(f"{tag}: good_first_issue is only valid on a trivial/low issue")
        if gfi and not issue.get("entry_point"):
            problems.append(f"{tag}: good_first_issue requires an entry_point")
        ep = issue.get("entry_point")
        if ep and not os.path.exists(os.path.join(REPO_ROOT, ep)):
            problems.append(f"{tag}: entry_point {ep!r} does not exist in the repo")
        if seen[issue.get("title", "")] > 1:
            problems.append(f"{tag}: duplicate title within the backlog")
    return problems


def body(issue):
    """The GitHub issue body, in the narrative shape MAINTAINER.md section 6 requires."""
    out = [f"## What's wrong\n\n{issue['why']}\n",
           f"## Proposed fix\n\n{issue['description']}\n",
           "## Acceptance criteria\n"]
    out += [f"- [ ] {c}" for c in issue["criteria"]]
    out += ["\n## Guidance\n"]
    out += [f"- {g}" for g in issue["guidance"]]
    if issue.get("entry_point"):
        out.append(f"\n**Entry point:** `{issue['entry_point']}`")
    out += ["\n---\n",
            f"*Area: `{issue['area']}` · complexity: `{issue['complexity']}`. Filed as part of "
            f"the wave {WAVE} backlog (`docs/WAVE{WAVE}_BACKLOG.json`); every issue in it cites a file and "
            f"line verified to exist at the time of filing. Point values live in the backlog, "
            f"not here, so the ladder can be repriced without editing filed issues.*"]
    return "\n".join(out)


def markdown(issues):
    tiers = collections.Counter(i["complexity"] for i in issues)
    total = sum(POINTS[i["complexity"]] for i in issues)
    L = ["<!-- GENERATED FILE - DO NOT EDIT BY HAND.",
         f"     Source of truth: docs/WAVE{WAVE}_BACKLOG.json",
         "     Regenerate with: python3 scripts/gen-wave-backlog.py  (or `make backlog`)",
         "     Any hand edit here is lost on the next regeneration. -->",
         "", f"# Wave {WAVE} backlog", "",
         f"**{len(issues)} issues · {total} points.**", "",
         "| Tier | Label | Count | Points each | Subtotal |",
         "|---|---|---:|---:|---:|"]
    for tier in ("high", "medium", "low"):
        c = tiers.get(tier, 0)
        L.append(f"| {tier} | `complexity: {tier}` | {c} | {POINTS[tier]} | {c * POINTS[tier]} |")
    L += [f"| **total** | | **{len(issues)}** | | **{total}** |", "",
          "Complexity uses this repository's existing `complexity: low|medium|high` labels "
          "(see `gh label list`); `trivial` in the wave brief maps to `low`. Grades describe "
          "real scope, per `MAINTAINER.md` section 2 — they are not adjusted to reach a point "
          "total.", "",
          "`area` is taken from the commit scopes `CONTRIBUTING.md` already defines, since the "
          "repository has no `area:` labels. It appears in each issue body rather than as a "
          "label.", "", "---", ""]
    for n, i in enumerate(issues, 1):
        gfi = " · `good first issue`" if i.get("good_first_issue") else ""
        L += [f"## {n}. {i['title']}", "",
              f"`complexity: {i['complexity']}` · {POINTS[i['complexity']]} points · "
              f"area `{i['area']}`{gfi}", "",
              f"**What's wrong.** {i['why']}", "",
              f"**Proposed fix.** {i['description']}", "", "**Acceptance criteria**", ""]
        L += [f"{k}. {c}" for k, c in enumerate(i["criteria"], 1)]
        L += ["", "**Guidance**", ""]
        L += [f"- {g}" for g in i["guidance"]]
        if i.get("entry_point"):
            L += ["", f"**Entry point:** `{i['entry_point']}`"]
        L += ["", "---", ""]
    return "\n".join(L) + "\n"


def main():
    src = os.path.join(REPO_ROOT, f"docs/WAVE{WAVE}_BACKLOG.json")
    issues = json.load(open(src))
    problems = validate(issues)
    if problems:
        print(f"backlog is INVALID ({len(problems)} problems):", file=sys.stderr)
        for p in problems:
            print(f"  - {p}", file=sys.stderr)
        return 1
    dst = os.path.join(REPO_ROOT, f"docs/WAVE{WAVE}_BACKLOG.md")
    open(dst, "w").write(markdown(issues))
    tiers = collections.Counter(i["complexity"] for i in issues)
    total = sum(POINTS[i["complexity"]] for i in issues)
    print(f"backlog valid: {len(issues)} issues, {dict(tiers)}, {total} points")
    print(f"wrote {os.path.relpath(dst, REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
