#!/usr/bin/env bash
set -euo pipefail

COV_DIR="coverage"
MERGED="${COV_DIR}/coverage.out"

mkdir -p "${COV_DIR}"

ALL_PKGS=$(go list ./... 2>/dev/null \
    | grep -Ev '/mocks?($|/)' \
    | awk 'NF' || true)

TEST_PKGS=$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... 2>/dev/null \
    | grep -Ev '/mocks?($|/)' \
    | awk 'NF' || true)

if [[ -z "${ALL_PKGS}" ]]; then
    echo "No packages found; writing empty coverage file"
    echo "mode: set" > "${MERGED}"
    echo "Coverage written to ${MERGED} (no packages)"
    exit 0
fi

if [[ -z "${TEST_PKGS}" ]]; then
    echo "No packages with tests found; writing empty coverage file"
    echo "mode: set" > "${MERGED}"
    echo "Coverage written to ${MERGED} (no test packages)"
    exit 0
fi

# Enforce one-to-one package test coverage for all non-mock packages.
MISSING_TEST_PKGS=$(comm -23 <(echo "${ALL_PKGS}" | sort) <(echo "${TEST_PKGS}" | sort) || true)
if [[ -n "${MISSING_TEST_PKGS}" ]]; then
    echo "ERROR: missing tests for non-mock packages:"
    echo "${MISSING_TEST_PKGS}"
    exit 1
fi

echo "Running tests for all non-mock packages with native package coverage..."
# shellcheck disable=SC2086
go test -coverprofile="${MERGED}" ${TEST_PKGS}

echo "Coverage written to ${MERGED}"
