#!/usr/bin/env bash
# Create the 9 CI/PR state-machine labels used by the SniShaper bots.
# Idempotent: gh label create --force updates color/description on re-run.
#
# Usage: labels.sh [REPO]
# Repo resolution order: argument > LABELS_REPO > GITHUB_REPOSITORY >
# SnishaperTeam/SniShaper.
set -euo pipefail

REPO="${1:-${LABELS_REPO:-${GITHUB_REPOSITORY:-SnishaperTeam/SniShaper}}}"

create_label() {
  gh label create "$1" \
    --repo "$REPO" \
    --color "$2" \
    --description "$3" \
    --force
}

# Names, colors and descriptions follow the label table in
# spec/ci-pr-bots-automation/plan.md.
create_label "cla-pending"        "d4c5f9" "Contributor agreement pending signature (policy-bot)"
create_label "cla-signed"         "0e8a16" "Contributor agreement signed (policy-bot)"
create_label "ci-running"         "fbca04" "PR CI running (syntax-bot)"
create_label "ci-failed"          "d73a4a" "PR CI failed (syntax-bot)"
create_label "ci-passed"          "0e8a16" "PR CI passed (syntax-bot)"
create_label "syntax-review"      "1d76db" "CI result review completed (syntax-bot)"
create_label "reviewers-assigned" "5319e7" "Reviewers assigned (policy-bot)"
create_label "changes-requested"  "d93f0b" "Changes requested (CI failure or reviewer review)"
create_label "approved"           "0e8a16" "Approved by reviewer (policy-bot)"

echo "All 9 labels ensured on ${REPO}."
