#!/bin/sh
set -eu

normalize_tenths() {
	value="$1"
	case "$value" in
		*.*)
			int_part="${value%%.*}"
			frac_part="${value#*.}"
			;;
		*)
			int_part="$value"
			frac_part="0"
			;;
	esac

	if [ -z "$int_part" ]; then
		int_part="0"
	fi

	frac_part="$(printf '%s' "$frac_part" | cut -c1)"
	if [ -z "$frac_part" ]; then
		frac_part="0"
	fi

	printf '%s\n' "$((int_part * 10 + frac_part))"
}

format_tenths() {
	value="$1"
	case "$value" in
		*.*)
			int_part="${value%%.*}"
			frac_part="${value#*.}"
			;;
		*)
			int_part="$value"
			frac_part="0"
			;;
	esac

	frac_part="$(printf '%s' "$frac_part" | cut -c1)"
	if [ -z "$frac_part" ]; then
		frac_part="0"
	fi

	printf '%s.%s\n' "$int_part" "$frac_part"
}

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

	got_tenths="$(normalize_tenths "$coverage")"
	want_tenths="$(normalize_tenths "$threshold")"
	if [ "$got_tenths" -lt "$want_tenths" ]; then
		printf 'coverage gate failed for %s: got %s%%, need %s%%\n' \
			"$pkg" \
			"$(format_tenths "$coverage")" \
			"$(format_tenths "$threshold")" >&2
		exit 1
	fi
}

check_package ./internal/backend/xray 50
check_package ./internal/probe 60
check_package ./internal/app 65
