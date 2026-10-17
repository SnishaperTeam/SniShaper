#!/usr/bin/env bash
# Shared library for the SniShaper CI/PR bots (policy-bot & syntax-bot).
# Source-only; do not execute directly.
#
# Expects GH_TOKEN (or GITHUB_TOKEN) to carry the identity that performs
# the write operations, and GH_REPO / GITHUB_REPOSITORY to name the target
# repository.

: "${GH_REPO:=${GITHUB_REPOSITORY:-SnishaperTeam/SniShaper}}"
export GH_REPO

# ---------------------------------------------------------------------------
# Exemption list (single source of truth).
# Hit as PR author: skip the CLA gate. Hit as comment author: never trigger
# the agreement detection (prevents bot self-trigger loops).
# ---------------------------------------------------------------------------
EXEMPT_LOGINS=(
  "dependabot[bot]"
  "github-actions[bot]"
  "renovate[bot]"
  "snishaper-policy-bot[bot]"
  "snishaper-syntax-review-bot[bot]"
  "SnishaperTeam-ReleaseBot"
)

is_exempt() {
  local login="$1" e
  for e in "${EXEMPT_LOGINS[@]}"; do
    if [ "$e" = "$login" ]; then
      return 0
    fi
  done
  return 1
}

# ---------------------------------------------------------------------------
# gh_retry CMD... — run a gh command with exponential backoff (1s / 4s / 16s,
# 3 retries). A command that keeps failing fails the step, so the workflow
# shows a visible red X instead of failing silently.
# ---------------------------------------------------------------------------
gh_retry() {
  local delay
  if "$@"; then
    return 0
  fi
  for delay in 1 4 16; do
    sleep "$delay"
    if "$@"; then
      return 0
    fi
  done
  return 1
}

# ---------------------------------------------------------------------------
# sync_labels PR_NUMBER ADD_CSV REMOVE_CSV
#
# Delta sync of the PR state labels: adds only labels missing on the issue,
# removes only labels that are present. Adding a label from a
# mutual-exclusion group also removes the group's stale siblings
# ({cla-pending, cla-signed}, {ci-running, ci-failed, ci-passed} and
# {changes-requested, approved}); a new CI round (adding ci-running) also
# clears syntax-review, changes-requested and approved (new commits
# invalidate the previous review state). Callers still pass explicit
# removals, so repeated events converge without label flicker.
# ---------------------------------------------------------------------------
_label_group_peers() {
  case "$1" in
    cla-pending | cla-signed)
      echo "cla-pending cla-signed"
      ;;
    ci-running | ci-failed | ci-passed)
      echo "ci-running ci-failed ci-passed"
      ;;
    changes-requested | approved)
      echo "changes-requested approved"
      ;;
    *)
      echo ""
      ;;
  esac
}

_csv_to_array() {
  # $1 = output array name, $2 = comma-separated input
  local IFS=','
  read -r -a "$1" <<< "$2"
}

sync_labels() {
  local pr="$1"
  local add_csv="${2:-}"
  local remove_csv="${3:-}"
  local current
  current="$(gh_retry gh api "repos/${GH_REPO}/issues/${pr}/labels" --jq '.[].name' | paste -sd, -)"

  # Expand removals with mutual-exclusion peers of every added label.
  local adds=() rems=() label peer
  _csv_to_array adds "$add_csv"
  _csv_to_array rems "$remove_csv"
  for label in "${adds[@]}"; do
    if [ -n "$label" ]; then
      for peer in $(_label_group_peers "$label"); do
        if [ "$peer" != "$label" ]; then
          rems+=("$peer")
        fi
      done
      if [ "$label" = "ci-running" ]; then
        rems+=("syntax-review" "changes-requested" "approved")
      fi
    fi
  done

  local to_add=() to_remove=() a r
  for a in "${adds[@]}"; do
    if [ -n "$a" ]; then
      case ",$current," in
        *",$a,"*) ;;
        *) to_add+=("$a") ;;
      esac
    fi
  done
  for r in "${rems[@]}"; do
    if [ -n "$r" ]; then
      case ",$current," in
        *",$r,"*) to_remove+=("$r") ;;
      esac
    fi
  done

  if [ "${#to_add[@]}" -gt 0 ]; then
    gh_retry gh issue edit "$pr" --repo "$GH_REPO" \
      --add-label "$(IFS=','; echo "${to_add[*]}")" >/dev/null
  fi
  if [ "${#to_remove[@]}" -gt 0 ]; then
    gh_retry gh issue edit "$pr" --repo "$GH_REPO" \
      --remove-label "$(IFS=','; echo "${to_remove[*]}")" >/dev/null
  fi
}

# ---------------------------------------------------------------------------
# bot_comment PR_NUMBER ANCHOR BODY
#
# Upsert the PR conversation comment containing ANCHOR: edit the existing
# comment if one matches, create it otherwise. Keeps exactly one comment
# per anchor per PR (idempotent upsert by HTML anchor).
# ---------------------------------------------------------------------------
bot_comment() {
  local pr="$1"
  local anchor="$2"
  local body="$3"
  local comment_id
  comment_id="$(gh_retry gh api "repos/${GH_REPO}/issues/${pr}/comments" --paginate \
    --jq "map(select(.body | contains(\"$anchor\")))[0].id // empty" | head -1)"
  if [ -n "$comment_id" ]; then
    gh_retry gh api -X PATCH "repos/${GH_REPO}/issues/comments/${comment_id}" \
      -f body="$body" >/dev/null
  else
    gh_retry gh api -X POST "repos/${GH_REPO}/issues/${pr}/comments" \
      -f body="$body" >/dev/null
  fi
}
