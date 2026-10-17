#!/usr/bin/env bash
# CI failure diagnosis for the SniShaper syntax-bot.
#
# Usage: ci-diagnose.sh PR_NUMBER RUN_ID
#
# Fetches the failed jobs of a workflow run, classifies the first error in
# each job's log per the error-classification table in
# spec/ci-pr-bots-automation/plan.md, parses the job name for
# pipeline / platform / arch, and prints a five-element Markdown diagnosis
# (pipeline, platform/arch, error type, file:line, error summary + log
# links) on stdout.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

PR_NUMBER="${1:?usage: ci-diagnose.sh PR_NUMBER RUN_ID}"
RUN_ID="${2:?usage: ci-diagnose.sh PR_NUMBER RUN_ID}"
RUN_URL="https://github.com/${GH_REPO}/actions/runs/${RUN_ID}"

# --- error classification (plan.md error table) ----------------------------

first_match() {
  grep -Eim1 "$1" "$2" || true
}

_classify_log() {
  # Sets DIAG_TYPE and DIAG_LINE from the job log at $1.
  local log_file="$1" match
  match="$(first_match 'syntax error|expected .* found|unexpected token' "$log_file")"
  if [ -n "$match" ]; then
    DIAG_TYPE="Syntax error"
    DIAG_LINE="$match"
    return
  fi
  match="$(first_match 'build failed|undefined:|cannot use' "$log_file")"
  if [ -n "$match" ]; then
    DIAG_TYPE="Build error"
    DIAG_LINE="$match"
    return
  fi
  match="$(first_match 'cannot find package|cannot find module|npm ERR!|404 .* registry' "$log_file")"
  if [ -n "$match" ]; then
    DIAG_TYPE="Dependency error"
    DIAG_LINE="$match"
    return
  fi
  # Loose "compile" family last, so ordinary build progress lines cannot
  # shadow a sharper match above.
  match="$(first_match 'compile' "$log_file")"
  if [ -n "$match" ]; then
    DIAG_TYPE="Build error"
    DIAG_LINE="$match"
    return
  fi
  DIAG_TYPE="Other failure"
  DIAG_LINE="$(first_match 'error' "$log_file")"
  if [ -z "$DIAG_LINE" ]; then
    DIAG_LINE="(no error line found in the job log)"
  fi
}

# --- location extraction ----------------------------------------------------

_extract_loc() {
  # Extract "path/file.ext:line" (or "file.ext(line,col)") from a log line.
  local line="$1" loc
  loc="$(printf '%s' "$line" | grep -Eo "[^ \"\`':;|]+\.[A-Za-z0-9]{1,5}:[0-9]+" | head -1 || true)"
  if [ -z "$loc" ]; then
    loc="$(printf '%s' "$line" | grep -Eo "[^ \"\`':;|]+\.[A-Za-z0-9]{1,5}\([0-9]+,[0-9]+\)" | head -1 || true)"
  fi
  if [ -n "$loc" ]; then
    # Strip CI workspace prefixes for readability.
    loc="$(printf '%s' "$loc" | sed -e 's|.*/work/[^/]*/[^/]*/||' -e 's|.*\\a\\[^\\]*\\[^\\]*\\||')"
    printf '%s' "$loc"
  else
    printf '%s' "-"
  fi
}

# --- job-name parsing -------------------------------------------------------

_parse_job_name() {
  # Sets PIPELINE / PLATFORM / ARCH from the job display name.
  local name="$1"
  PIPELINE="unknown"
  PLATFORM="unknown"
  ARCH="unknown"
  if [[ "$name" =~ ^(GUI)[[:space:]]([^/]+)/([^[:space:]]+) ]]; then
    PIPELINE="GUI"
    PLATFORM="${BASH_REMATCH[2]}"
    ARCH="${BASH_REMATCH[3]}"
  elif [[ "$name" =~ ^(CLI)[[:space:]]([^/]+)/([^[:space:]]+) ]]; then
    PIPELINE="CLI"
    PLATFORM="${BASH_REMATCH[2]}"
    ARCH="${BASH_REMATCH[3]}"
  elif [[ "$name" =~ ^(CLI).*5[[:space:]]targets ]]; then
    PIPELINE="CLI"
    PLATFORM="windows+linux"
    ARCH="x64"
  fi
}

# --- main -------------------------------------------------------------------

main() {
  local jobs_json tsv
  jobs_json="$(gh_retry gh api "repos/${GH_REPO}/actions/runs/${RUN_ID}/jobs")"
  tsv="$(jq -r '.jobs[] | select(.conclusion == "failure") | [.id, .name, .html_url] | @tsv' <<<"$jobs_json")"

  echo "## 🔍 CI failure diagnosis"
  echo
  echo "PR: #${PR_NUMBER} • Run: [#${RUN_ID}](${RUN_URL})"
  echo

  local n=0 job_id job_name job_url log_file loc
  while IFS=$'\t' read -r job_id job_name job_url; do
    if [ -z "$job_id" ]; then
      continue
    fi
    n=$((n + 1))
    _parse_job_name "$job_name"
    log_file="$(mktemp)"
    if gh_retry gh api "repos/${GH_REPO}/actions/jobs/${job_id}/logs" >"$log_file" 2>/dev/null; then
      _classify_log "$log_file"
    else
      DIAG_TYPE="Other failure"
      DIAG_LINE="(job log unavailable)"
    fi
    rm -f "$log_file"
    loc="$(_extract_loc "$DIAG_LINE")"

    echo "### ${n}. ${job_name}"
    echo
    echo "- **Pipeline / Platform / Arch:** ${PIPELINE} / ${PLATFORM} / ${ARCH}"
    echo "- **Error type:** ${DIAG_TYPE}"
    echo "- **Location:** \`${loc}\`"
    echo "- **Summary:** \`${DIAG_LINE:0:200}\`"
    echo "- **Logs:** [failed job](${job_url}) | [full run](${RUN_URL})"
    echo
  done <<<"$tsv"

  if [ "$n" -eq 0 ]; then
    echo "No failed jobs found for run [#${RUN_ID}](${RUN_URL}); see the run log for details."
    echo
  fi
}

main
