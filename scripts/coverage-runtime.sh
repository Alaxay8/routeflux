#!/bin/sh
set -eu

check_package() {
	pkg="$1"
	threshold="$2"

	printf 'checking coverage for %s\n' "$pkg"
	if output="$(go test -cover "$pkg" 2>&1)"; then
		:
	else
		status="$?"
		printf '%s\n' "$output"
		printf 'go test failed for %s\n' "$pkg" >&2
		exit "$status"
	fi
	printf '%s\n' "$output"

	coverage="$(printf '%s\n' "$output" | sed -n 's/.*coverage: \([0-9.][0-9.]*\)% of statements.*/\1/p' | tail -n 1)"
	if [ -z "$coverage" ]; then
		printf 'could not determine coverage for %s\n' "$pkg" >&2
		exit 1
	fi

	awk -v pkg="$pkg" -v got="$coverage" -v want="$threshold" 'BEGIN {
		if ((got + 0) < (want + 0)) {
			printf("coverage gate failed for %s: got %.1f%%, need %.1f%%\n", pkg, got, want) > "/dev/stderr"
			exit 1
		}
	}'
}

check_package ./internal/backend/xray 50
check_package ./internal/probe 60
check_package ./internal/app 65
