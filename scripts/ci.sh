#!/usr/bin/env bash
set -euo pipefail

repo_root=$(pwd)
work_dir=$(mktemp -d "${RUNNER_TEMP:-/tmp}/gooo-module-linker.XXXXXX")
artifact_dir="$repo_root/ci-artifacts"
generated_dir="$artifact_dir/generated"
mkdir -p "$generated_dir"

declare -a metric_names=(compile build test conformance integration)
declare -A metric_ms
declare -A metric_rss

measure() {
  local name=$1
  shift
  local start end
  start=$(date +%s%3N)
  /usr/bin/time -f '%M' -o "$work_dir/$name.rss" "$@"
  end=$(date +%s%3N)
  metric_ms[$name]=$((end - start))
  metric_rss[$name]=$(tr -d '[:space:]' < "$work_dir/$name.rss")
}

case "$(go env GOVERSION)" in
  go1.27.*) ;;
  *) echo "Go 1.27 is required" >&2; exit 1 ;;
esac

measure compile go test -run '^$' ./...
measure build bash -c "go build -o '$work_dir/gooo-module-linker' ./cmd/gooo-module-linker && go build -o '$work_dir/gooo-conformance' ./cmd/gooo-conformance"
measure test go test ./...
measure conformance "$work_dir/gooo-conformance" -policy meta/module-linker.gooo -manifest meta/conformance.gooo -root . -out "$work_dir/conformance.json"
measure integration bash -c "'$work_dir/gooo-module-linker' -policy meta/module-linker.gooo -out-ir '$generated_dir/linked.ir.json' -out-go '$generated_dir/generated_link.go' fixtures/modules/app.gooo fixtures/modules/feature.gooo fixtures/modules/core.gooo && go build -o '$work_dir/generated-link' '$generated_dir/generated_link.go' && '$work_dir/generated-link' >/dev/null"

cp "$work_dir/conformance.json" "$artifact_dir/conformance.json"

go_files=$(find . -type f -name '*.go' -not -path './.git/*' -not -path './ci-artifacts/*' | wc -l | tr -d '[:space:]')
gooo_files=$(find . -type f -name '*.gooo' -not -path './.git/*' -not -path './ci-artifacts/*' | wc -l | tr -d '[:space:]')
go_lines=$(find . -type f -name '*.go' -not -path './.git/*' -not -path './ci-artifacts/*' -print0 | while IFS= read -r -d '' file; do wc -l < "$file"; done | awk '{sum += $1} END {print sum + 0}')
gooo_lines=$(find . -type f -name '*.gooo' -not -path './.git/*' -not -path './ci-artifacts/*' -print0 | while IFS= read -r -d '' file; do wc -l < "$file"; done | awk '{sum += $1} END {print sum + 0}')
subdirectories=$(find . -mindepth 1 -type d -not -path './.git/*' -not -path './ci-artifacts' -not -path './ci-artifacts/*' | wc -l | tr -d '[:space:]')
regular_files=$(find . -type f -not -path './.git/*' -not -path './ci-artifacts/*' -not -path './README.md' | wc -l | tr -d '[:space:]')
generated_count=$(find "$generated_dir" -type f | wc -l | tr -d '[:space:]')
generated_bytes=$(find "$generated_dir" -type f -print0 | xargs -0 wc -c | awk 'END {print $1 + 0}')
peak_rss=0
for name in "${metric_names[@]}"; do
  if (( metric_rss[$name] > peak_rss )); then peak_rss=${metric_rss[$name]}; fi
done

export GO_FILES=$go_files GO_LINES=$go_lines GOOO_FILES=$gooo_files GOOO_LINES=$gooo_lines
export SUBDIRECTORIES=$subdirectories REGULAR_FILES=$regular_files
export GENERATED_COUNT=$generated_count GENERATED_BYTES=$generated_bytes PEAK_RSS=$peak_rss
export COMPILE_MS=${metric_ms[compile]} COMPILE_RSS=${metric_rss[compile]}
export BUILD_MS=${metric_ms[build]} BUILD_RSS=${metric_rss[build]}
export TEST_MS=${metric_ms[test]} TEST_RSS=${metric_rss[test]}
export CONFORMANCE_MS=${metric_ms[conformance]} CONFORMANCE_RSS=${metric_rss[conformance]}
export INTEGRATION_MS=${metric_ms[integration]} INTEGRATION_RSS=${metric_rss[integration]}

python3 - "$work_dir/conformance.json" "$artifact_dir/gooo-evidence.json" <<'PY'
import json
import os
import pathlib
import sys

conformance = json.loads(pathlib.Path(sys.argv[1]).read_text())
evidence_path = pathlib.Path(sys.argv[2])

metrics = {
    "inventory": {
        "go_files": int(os.environ["GO_FILES"]),
        "go_physical_lines": int(os.environ["GO_LINES"]),
        "gooo_files": int(os.environ["GOOO_FILES"]),
        "gooo_physical_lines": int(os.environ["GOOO_LINES"]),
        "subdirectories": int(os.environ["SUBDIRECTORIES"]),
        "regular_files": int(os.environ["REGULAR_FILES"]),
        "root_readme_excluded": True,
    },
    "generated_artifacts": {
        "count": int(os.environ["GENERATED_COUNT"]),
        "bytes": int(os.environ["GENERATED_BYTES"]),
    },
    "runtime": {
        "compile": {"wall_ms": int(os.environ["COMPILE_MS"]), "peak_rss_kib": int(os.environ["COMPILE_RSS"])},
        "build": {"wall_ms": int(os.environ["BUILD_MS"]), "peak_rss_kib": int(os.environ["BUILD_RSS"])},
        "test": {"wall_ms": int(os.environ["TEST_MS"]), "peak_rss_kib": int(os.environ["TEST_RSS"])},
        "conformance": {"wall_ms": int(os.environ["CONFORMANCE_MS"]), "peak_rss_kib": int(os.environ["CONFORMANCE_RSS"])},
        "integration": {"wall_ms": int(os.environ["INTEGRATION_MS"]), "peak_rss_kib": int(os.environ["INTEGRATION_RSS"])},
        "peak_rss_kib": int(os.environ["PEAK_RSS"]),
    },
    "tests": {
        "total": int(conformance["total"]),
        "selected": int(conformance["selected"]),
        "executed": int(conformance["executed"]),
        "reused": int(conformance["reused"]),
        "failed": int(conformance["failed"]),
        "unknown": int(conformance["unknown"]),
    },
}

evidence = {
    "schema": "gooo.release-evidence/v1",
    "repository": "kimjooyoon/gooo-module-linker",
    "contract": {
        "path": "meta/module-linker.gooo",
        "authority": "metacode",
        "status_precedence": conformance["precedence"],
        "unknown_fields": ["stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"],
    },
    "conformance": conformance,
    "metrics": metrics,
}
evidence_path.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n")
PY
