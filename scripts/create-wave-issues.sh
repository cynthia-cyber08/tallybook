#!/usr/bin/env bash
# Create the wave backlog's issues on GitHub from docs/WAVE2_BACKLOG.json.
#
# Dry-run by default: prints exactly what it would create and exits 0 without
# touching GitHub. Pass --apply to actually create issues.
#
# Safe to re-run. `gh issue create` is not idempotent, so this script reads
# every existing issue title first (open and closed) and skips any exact
# match. A run interrupted halfway is resumed by re-running the same command:
# the issues already created are skipped as duplicates.
set -euo pipefail

cd "$(dirname "$0")/.."

BACKLOG="${BACKLOG:-$(ls -1 docs/WAVE*_BACKLOG.json | sort -V | tail -1)}"
PACE_SECONDS="${PACE_SECONDS:-3}"          # GitHub's secondary content-creation limit returns a
                          # bare 403, not a documented rate-limit error, so pace
                          # creations rather than relying on detecting it.
APPLY=0
LIMIT=0                   # 0 = no limit. --limit N stops after N creations, for
                          # a staged rollout: create a few, read one back from
                          # GitHub to confirm the body and labels rendered, then
                          # re-run without --limit to do the rest.

while [ $# -gt 0 ]; do
  arg="$1"
  case "$arg" in
    --apply) APPLY=1 ;;
    --limit) shift; LIMIT="${1:-0}"
             case "$LIMIT" in ''|*[!0-9]*) echo "--limit needs a number" >&2; exit 2 ;; esac ;;
    -h|--help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg (use --apply to create, or no arguments to dry-run)" >&2; exit 2 ;;
  esac
  shift
done

command -v gh  >/dev/null || { echo "gh is required but not on PATH" >&2; exit 1; }
command -v jq  >/dev/null || { echo "jq is required but not on PATH" >&2; exit 1; }
[ -f "$BACKLOG" ] || { echo "backlog not found: $BACKLOG" >&2; exit 1; }

# --- validate before anything else; refuse to touch GitHub if malformed ------
echo "validating $BACKLOG ..."
if ! python3 scripts/gen-wave-backlog.py; then
  echo "REFUSING to continue: the backlog is malformed. Nothing was sent to GitHub." >&2
  exit 1
fi

REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
TOTAL=$(jq 'length' "$BACKLOG")
echo

# --- read every existing title so a re-run never duplicates -----------------
echo "reading existing issue titles from $REPO ..."
EXISTING=$(mktemp); BODY=$(mktemp); trap 'rm -f "$EXISTING" "$BODY"' EXIT
gh issue list --repo "$REPO" --state all --limit 1000 --json title -q '.[].title' > "$EXISTING"
echo "  $(wc -l < "$EXISTING" | tr -d ' ') existing issues found"

# --- label existence check (a missing label makes gh issue create fail) -----
HAVE_LABELS=$(mktemp); trap 'rm -f "$EXISTING" "$BODY" "$HAVE_LABELS"' EXIT
gh label list --repo "$REPO" --limit 200 --json name -q '.[].name' > "$HAVE_LABELS"
{ jq -r '.[] | "complexity: \(.complexity)"' "$BACKLOG" | sort -u; echo "good first issue"; } |
while IFS= read -r l; do
  grep -Fxq "$l" "$HAVE_LABELS" || { echo "label missing on $REPO: '$l' - create it first" >&2; exit 1; }
done || exit 1

type_label_for() {  # derive the repo's type:* label from the conventional-commit prefix
  case "$1" in
    fix*)              echo "bug" ;;
    test*)             echo "type: test" ;;
    refactor*)         echo "type: refactor" ;;
    docs*)             echo "type: docs" ;;
    ci*|chore*|feat*)  echo "type: feature" ;;
    *)                 echo "" ;;
  esac
}

created=0; skipped=0; index=0

if [ "$APPLY" -eq 0 ]; then
  echo; echo "=== DRY RUN — nothing will be created. Re-run with --apply to create. ==="
fi
echo

while [ "$index" -lt "$TOTAL" ]; do
  title=$(jq -r ".[$index].title" "$BACKLOG")
  complexity=$(jq -r ".[$index].complexity" "$BACKLOG")
  gfi=$(jq -r ".[$index].good_first_issue // false" "$BACKLOG")
  index=$((index + 1))

  if grep -Fxq "$title" "$EXISTING"; then
    echo "SKIP   already exists: $title"
    skipped=$((skipped + 1))
    continue
  fi

  labels="complexity: $complexity"
  t=$(type_label_for "$title"); [ -n "$t" ] && labels="$labels,$t"
  [ "$gfi" = "true" ] && labels="$labels,good first issue"

  python3 scripts/wave-issue-body.py "$BACKLOG" "$((index - 1))" > "$BODY"

  if [ "$APPLY" -eq 0 ]; then
    echo "WOULD CREATE  [$labels]"
    echo "              $title"
    continue
  fi

  echo "CREATE $title"
  if ! url=$(gh issue create --repo "$REPO" --title "$title" \
             --body-file "$BODY" --label "$labels" 2>&1); then
    echo >&2
    echo "FAILED while creating: $title" >&2
    echo "  gh said: $url" >&2
    echo "  created $created issue(s) before this failure; $skipped skipped as existing." >&2
    echo "  Re-run the SAME command to resume — created issues are skipped by title." >&2
    exit 1
  fi
  echo "       -> $url"
  created=$((created + 1))
  if [ "$LIMIT" -gt 0 ] && [ "$created" -ge "$LIMIT" ]; then
    echo
    echo "stopping at --limit $LIMIT. $created created, $skipped skipped."
    echo "Read one back to confirm it rendered, then re-run without --limit to"
    echo "create the remaining $((TOTAL - skipped - created)); these are skipped by title."
    exit 0
  fi
  sleep "$PACE_SECONDS"
done

echo
if [ "$APPLY" -eq 0 ]; then
  echo "dry run complete: $((TOTAL - skipped)) would be created, $skipped already exist (of $TOTAL)."
  echo "re-run with --apply to create them."
else
  echo "done: $created created, $skipped skipped as already existing (of $TOTAL in the backlog)."
  echo "Now reconcile against the live issues in BOTH directions rather than trusting this tally:"
  echo "  gh issue list --state all --limit 1000 --json title -q '.[].title' | sort > /tmp/live.txt"
  echo "  jq -r '.[].title' $BACKLOG | sort > /tmp/want.txt"
  echo "  comm -23 /tmp/want.txt /tmp/live.txt   # in backlog, NOT on GitHub"
  echo "  comm -13 /tmp/want.txt /tmp/live.txt   # on GitHub, not in this backlog"
fi
