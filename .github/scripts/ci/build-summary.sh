#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "${SCRIPT_DIR}/../bots/common.sh" ]; then
  source "${SCRIPT_DIR}/../bots/common.sh"
fi

SUMMARY_FILE="${GITHUB_STEP_SUMMARY:-}"
if [ -n "$SUMMARY_FILE" ] && command -v cygpath >/dev/null 2>&1; then
  case "$SUMMARY_FILE" in
    [A-Za-z]:\\*) SUMMARY_FILE="$(cygpath -u "$SUMMARY_FILE")" ;;
  esac
fi

CAPTION=".github/scripts/ci/build-summary.sh"

usage() {
  cat <<'EOF'
build-summary.sh - CI summary generator for the SniShaper workflows

Usage:
  build-summary.sh record --target TEXT [--project NAME] [--tasks TEXT]
                           [--runner NAME] [--outcome STATUS] [--started ISO8601]
                           [--row "LABEL=PATH"]... [--dir PATH]...
  build-summary.sh report [--run-id ID] [--repo OWNER/NAME] [--title TEXT]

record  writes the build table plus expandable output/toolchain details
        of the current job into the job summary.
report  writes a run-wide overview of every build job, the run artifacts
        and failed steps into the job summary.
EOF
}

emit_line() {
  printf '%s\n' "$1"
  if [ -n "$SUMMARY_FILE" ]; then
    printf '%s\n' "$1" >>"$SUMMARY_FILE" 2>/dev/null || true
  fi
}

outcome_icon() {
  case "$1" in
    success) printf ':white_check_mark:' ;;
    cancelled) printf ':no_entry_sign:' ;;
    skipped) printf ':heavy_minus_sign:' ;;
    *) printf ':x:' ;;
  esac
}

human_size() {
  awk -v b="${1:-0}" 'BEGIN {
    if (b == "") { print "-"; exit }
    u = "B"; v = b / 1
    if (v >= 1024) { u = "KB"; v = v / 1024 }
    if (u == "KB" && v >= 1024) { u = "MB"; v = v / 1024 }
    if (u == "MB" && v >= 1024) { u = "GB"; v = v / 1024 }
    printf "%.1f %s", v, u
  }'
}

resolve_file() {
  local pattern="$1" candidate
  case "$pattern" in
    *'*'* | *'?'*)
      for candidate in $pattern; do
        if [ -f "$candidate" ]; then
          printf '%s' "$candidate"
          return 0
        fi
      done
      return 1
      ;;
  esac
  if [ -f "$pattern" ]; then
    printf '%s' "$pattern"
    return 0
  fi
  return 1
}

duration_human() {
  local start="$1" end="$2" s e d
  if [ -z "$start" ] || [ -z "$end" ]; then
    printf '%s' "-"
    return 0
  fi
  s="$(date -u -d "$start" +%s 2>/dev/null || true)"
  e="$(date -u -d "$end" +%s 2>/dev/null || true)"
  if [ -z "$s" ] || [ -z "$e" ] || [ "$e" -lt "$s" ]; then
    printf '%s' "-"
    return 0
  fi
  d=$((e - s))
  if [ "$d" -ge 3600 ]; then
    printf '%dh %dm' $((d / 3600)) $(((d % 3600) / 60))
  elif [ "$d" -ge 60 ]; then
    printf '%dm %ds' $((d / 60)) $((d % 60))
  else
    printf '%ds' "$d"
  fi
}

go_version_detect() {
  if command -v go >/dev/null 2>&1; then
    go version 2>/dev/null | awk '{ print $3 }'
  fi
}

node_version_detect() {
  if command -v node >/dev/null 2>&1; then
    node --version 2>/dev/null | sed 's/^/node /'
  fi
}

go_version_from_mod() {
  local mod="${1:-go.mod}"
  if [ -f "$mod" ]; then
    awk '/^go[[:space:]]+/ { print "go" $2; exit }' "$mod"
  fi
}

classify_job() {
  local name="$1"
  JOB_PIPELINE=""
  JOB_TARGET=""
  JOB_RUNNER=""
  if [[ "$name" =~ ^GUI[[:space:]]+([^/]+)/([^[:space:]]+)[[:space:]]+on[[:space:]]+(.+)$ ]]; then
    JOB_PIPELINE="GUI"
    JOB_TARGET="${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
    JOB_RUNNER="${BASH_REMATCH[3]}"
  elif [[ "$name" =~ ^darwin[[:space:]]+([^[:space:]]+)[[:space:]]+on[[:space:]]+(.+)$ ]]; then
    JOB_PIPELINE="CLI"
    JOB_TARGET="darwin/${BASH_REMATCH[1]}"
    JOB_RUNNER="${BASH_REMATCH[2]}"
  elif [[ "$name" =~ ^CLI[[:space:]]+\((.+)\)$ ]]; then
    JOB_PIPELINE="CLI"
    JOB_TARGET="${BASH_REMATCH[1]}"
  elif [[ "$name" =~ ^Build[[:space:]]+Windows[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="GUI"
    JOB_TARGET="windows (MSIX + installers)"
  elif [[ "$name" =~ ^Build[[:space:]]+Linux/([^[:space:]]+)[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="GUI"
    JOB_TARGET="linux/${BASH_REMATCH[1]}"
  elif [[ "$name" =~ ^Build[[:space:]]+Linux[[:space:]]+packages/([^[:space:]]+)[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="Packages"
    JOB_TARGET="linux/${BASH_REMATCH[1]}"
  elif [[ "$name" =~ ^Build[[:space:]]+macOS/([^[:space:]]+)[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="macOS"
    JOB_TARGET="darwin/${BASH_REMATCH[1]}"
  elif [[ "$name" =~ ^Build[[:space:]]+CLI[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="CLI"
    JOB_TARGET="windows + linux + darwin"
  elif [[ "$name" =~ ^packages[[:space:]]+([^[:space:]]+)[[:space:]]+on[[:space:]]+(.+)$ ]]; then
    JOB_PIPELINE="Packages"
    JOB_TARGET="linux/${BASH_REMATCH[1]}"
    JOB_RUNNER="${BASH_REMATCH[2]}"
  elif [[ "$name" =~ ^AAR[[:space:]]*\+[[:space:]]+APK/AAB ]]; then
    JOB_PIPELINE="Android"
    JOB_TARGET="android"
  elif [[ "$name" =~ ^test[[:space:]]*\((.+)\)$ ]]; then
    JOB_PIPELINE="Test"
    JOB_TARGET="${BASH_REMATCH[1]}"
  fi
}

cmd_record() {
  local project="SniShaper" target="" tasks="" runner="" outcome="success" started="" cache_hit=""
  local rows=() dirs=()
  while [ $# -gt 0 ]; do
    case "$1" in
      --project) project="${2:-}"; shift 2 ;;
      --target) target="${2:-}"; shift 2 ;;
      --tasks) tasks="${2:-}"; shift 2 ;;
      --runner) runner="${2:-}"; shift 2 ;;
      --outcome) outcome="${2:-}"; shift 2 ;;
      --started) started="${2:-}"; shift 2 ;;
      --cache-hit) cache_hit="${2:-}"; shift 2 ;;
      --row) rows+=("${2:-}"); shift 2 ;;
      --dir) dirs+=("${2:-}"); shift 2 ;;
      *) shift ;;
    esac
  done
  if [ -z "$target" ]; then
    echo "record: --target is required" >&2
    return 2
  fi

  local gover nver icon build_time
  gover="$(go_version_detect)"
  [ -n "$gover" ] || gover="$(go_version_from_mod)"
  [ -n "$gover" ] || gover="-"
  nver="$(node_version_detect)"
  [ -n "$nver" ] || nver="-"
  icon="$(outcome_icon "$outcome")"
  if [ -n "$started" ]; then
    build_time="$(duration_human "$started" "$(date -u +%Y-%m-%dT%H:%M:%SZ)")"
  else
    build_time="-"
  fi
  [ -n "$tasks" ] || tasks="-"

  emit_line "### ${project} Builds · ${target} <sub><em>captured by ${CAPTION}</em></sub>"
  emit_line ""
  emit_line "| Build Target | Requested Tasks | Go Version | Build Time | Build Outcome | Artifact |"
  emit_line "| --- | --- | --- | --- | --- | --- |"
  local row label path resolved
  for row in ${rows[@]+"${rows[@]}"}; do
    [ -n "$row" ] || continue
    label="${row%%=*}"
    path="${row#*=}"
    resolved="$(resolve_file "$path" || true)"
    if [ -n "$resolved" ]; then
      emit_line "| ${label} | ${tasks} | ${gover} | ${build_time} | ${icon} | \`${resolved}\` · $(human_size "$(wc -c <"$resolved" | tr -d ' ')") |"
    else
      emit_line "| ${label} | ${tasks} | ${gover} | ${build_time} | ${icon} | \`${path}\` · missing |"
    fi
  done
  if [ "${#rows[@]}" -eq 0 ]; then
    emit_line "| ${target} | ${tasks} | ${gover} | ${build_time} | ${icon} | - |"
  fi

  local dir total=0 count_file f fsize
  for dir in ${dirs[@]+"${dirs[@]}"}; do
    [ -d "$dir" ] || continue
    total=0
    count_file=0
    emit_line ""
    emit_line "<details>"
    emit_line "<summary>Output directory: <code>${dir}</code> — Expand for more details</summary>"
    emit_line ""
    emit_line "| File | Size |"
    emit_line "| --- | --- |"
    while IFS= read -r f; do
      fsize="$(wc -c <"$f" | tr -d ' ')"
      total=$((total + fsize))
      count_file=$((count_file + 1))
      emit_line "| ${f} | $(human_size "$fsize") |"
    done < <(find "$dir" -type f 2>/dev/null | sort)
    emit_line ""
    emit_line "${count_file} files · $(human_size "$total") total"
    emit_line "</details>"
  done

  local cache_lines=() entry cname cval
  if [ -n "$cache_hit" ]; then
    case "$cache_hit" in
      true)
        cache_lines+=("| Actions cache (setup-go) | restored from cache |")
        ;;
      false)
        cache_lines+=("| Actions cache (setup-go) | not restored (miss) |")
        ;;
      *)
        cache_lines+=("| Actions cache (setup-go) | ${cache_hit} |")
        ;;
    esac
  fi
  emit_line ""
  emit_line "<details>"
  emit_line "<summary>Toolchain &amp; caches — Expand for more details</summary>"
  emit_line ""
  emit_line "| Item | Value |"
  emit_line "| --- | --- |"
  emit_line "| Runner | ${runner:--} |"
  emit_line "| Go | ${gover} |"
  emit_line "| Node | ${nver} |"
  if [ "${#cache_lines[@]}" -gt 0 ]; then
    local cline
    for cline in "${cache_lines[@]}"; do
      emit_line "$cline"
    done
  fi
  emit_line "</details>"
  emit_line ""
}

cmd_report() {
  local run_id="" repo="" title="SniShaper CI Summary"
  while [ $# -gt 0 ]; do
    case "$1" in
      --run-id) run_id="${2:-}"; shift 2 ;;
      --repo) repo="${2:-}"; shift 2 ;;
      --title) title="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  [ -n "$run_id" ] || run_id="${GITHUB_RUN_ID:-}"
  [ -n "$repo" ] || repo="${GH_REPO:-${GITHUB_REPOSITORY:-}}"
  if [ -z "$run_id" ] || [ -z "$repo" ]; then
    echo "report: run id or repository unknown, skipping the overview" >&2
    return 0
  fi
  if ! command -v gh >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
    echo "report: gh or jq unavailable, skipping the overview" >&2
    return 0
  fi

  local jobs_file body count page=1
  jobs_file="$(mktemp)"
  while [ "$page" -le 10 ]; do
    body="$(gh api "repos/${repo}/actions/runs/${run_id}/jobs?per_page=100&page=${page}" 2>/dev/null)" || break
    count="$(jq -r '.jobs | length' <<<"$body" 2>/dev/null || echo 0)"
    [ "$count" -eq 0 ] && break
    jq -c '.jobs[]' <<<"$body" 2>/dev/null | tr -d '\r' >>"$jobs_file" || true
    page=$((page + 1))
  done
  awk '!seen[$0]++' "$jobs_file" > "${jobs_file}.uniq" && mv "${jobs_file}.uniq" "$jobs_file"
  if [ ! -s "$jobs_file" ]; then
    rm -f "$jobs_file"
    echo "report: no job data available for run ${run_id}" >&2
    return 0
  fi

  local gover
  gover="$(go_version_from_mod)"
  [ -n "$gover" ] || gover="-"

  emit_line "### ${title} <sub><em>captured by ${CAPTION}</em></sub>"
  emit_line ""
  emit_line "| Build | Target | Runner | Go Version | Build Time | Build Outcome | Job |"
  emit_line "| --- | --- | --- | --- | --- | --- | --- |"

  local total=0 passed=0 failed=0 skipped=0
  local name conclusion url started completed runner failed_steps icon dur
  while IFS=$'\t' read -r name conclusion url started completed runner failed_steps; do
    [ -n "$name" ] || continue
    classify_job "$name"
    [ -n "$JOB_PIPELINE" ] || continue
    total=$((total + 1))
    case "$conclusion" in
      success)
        passed=$((passed + 1))
        icon=":white_check_mark:"
        ;;
      skipped)
        skipped=$((skipped + 1))
        icon=":heavy_minus_sign:"
        ;;
      cancelled)
        skipped=$((skipped + 1))
        icon=":no_entry_sign:"
        ;;
      *)
        failed=$((failed + 1))
        icon=":x:"
        ;;
    esac
    dur="$(duration_human "$started" "$completed")"
    emit_line "| ${JOB_PIPELINE} | ${JOB_TARGET} | ${JOB_RUNNER:-$runner} | ${gover} | ${dur} | ${icon} | [job](${url}) |"
  done < <(jq -r '[ (.name // ""), (.conclusion // ""), (.html_url // ""), (.started_at // ""), (.completed_at // ""), ((.labels // []) | if length > 0 then .[0] else (.runner_name // "") end), ((.steps // []) | map(select(.conclusion == "failure") | .name) | join("; ")) ] | @tsv' "$jobs_file" 2>/dev/null | tr -d '\r')

  emit_line ""
  emit_line "**${total} builds · ${passed} passed · ${failed} failed · ${skipped} skipped**"

  if [ "$failed" -gt 0 ]; then
    emit_line ""
    emit_line "#### Failed steps"
    emit_line ""
    local fname furl fsteps
    while IFS=$'\t' read -r fname furl fsteps; do
      [ -n "$fname" ] || continue
      classify_job "$fname"
      [ -n "$JOB_PIPELINE" ] || continue
      [ -n "$fsteps" ] || continue
      emit_line "- **${JOB_PIPELINE} ${JOB_TARGET}** — ${fsteps//;/, } — [job](${furl})"
    done < <(jq -r '[ (.name // ""), (.html_url // ""), ((.steps // []) | map(select(.conclusion == "failure") | .name) | join("; ")) ] | @tsv' "$jobs_file" 2>/dev/null | tr -d '\r')
  fi

  local artifacts
  artifacts="$(gh api "repos/${repo}/actions/runs/${run_id}/artifacts?per_page=100" 2>/dev/null || true)"
  if [ -n "$artifacts" ]; then
    local artifact_count
    artifact_count="$(jq -r '.artifacts | length' <<<"$artifacts" 2>/dev/null || echo 0)"
    if [ "$artifact_count" -gt 0 ]; then
      emit_line ""
      emit_line "#### Artifacts"
      emit_line ""
      emit_line "| Artifact | Size |"
      emit_line "| --- | --- |"
      local artifact_name artifact_size
      while IFS=$'\t' read -r artifact_name artifact_size; do
        emit_line "| ${artifact_name} | $(human_size "$artifact_size") |"
      done < <(jq -r '.artifacts[] | [.name, (.size_in_bytes | tostring)] | @tsv' <<<"$artifacts" 2>/dev/null | tr -d '\r')
    fi
  fi

  local caches
  caches="$(gh api "repos/${repo}/actions/caches?per_page=100" 2>/dev/null || true)"
  if [ -n "$caches" ]; then
    local cache_count
    cache_count="$(jq -r '.actions_caches | length' <<<"$caches" 2>/dev/null || echo 0)"
    if [ "$cache_count" -gt 0 ]; then
      emit_line ""
      emit_line "#### Caches <sub><em>actions/cache entries on ${repo}</em></sub>"
      emit_line ""
      emit_line "| Cache Key | Size | Created | Last Accessed |"
      emit_line "| --- | --- | --- | --- |"
      local cache_key cache_size cache_created cache_accessed cache_total
      cache_total=0
      while IFS=$'\t' read -r cache_key cache_size cache_created cache_accessed; do
        cache_total=$((cache_total + cache_size))
        emit_line "| \`${cache_key}\` | $(human_size "$cache_size") | ${cache_created%%T*} | ${cache_accessed%%T*} |"
      done < <(jq -r '.actions_caches | sort_by(-.size_in_bytes) | .[] | [.key, (.size_in_bytes | tostring), (.created_at // "-"), (.last_accessed_at // "-")] | @tsv' <<<"$caches" 2>/dev/null | tr -d '\r')
      emit_line ""
      emit_line "${cache_count} caches · $(human_size "$cache_total") total"
    fi
  fi
  rm -f "$jobs_file"
  emit_line ""
}

main() {
  local mode="${1:-}"
  if [ $# -gt 0 ]; then
    shift
  fi
  case "$mode" in
    record) cmd_record "$@" ;;
    report) cmd_report "$@" ;;
    *)
      usage
      exit 2
      ;;
  esac
}

main "$@"
