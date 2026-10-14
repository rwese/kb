#!/usr/bin/env bash
# Run one dataset through the ranking matrix (bm25 baseline plus a set of
# hybrid weight balances) and print a cross-configuration comparison table.
#
# Usage: scripts/bench-matrix.sh DATASET [--top-k N] [--outdir DIR]
#
# Hybrid runs need the configured embedder (config `embedder: ollama` with
# Ollama serving the model). Failed configurations are reported but do not
# abort the matrix; reports are kept for `kb bench diff` on same-config
# re-runs.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$REPO_ROOT/bin/kb"
TOP_K=3
OUTDIR=""
DATASET=""

while [ $# -gt 0 ]; do
	case "$1" in
	--top-k)
		TOP_K="$2"
		shift 2
		;;
	--outdir)
		OUTDIR="$2"
		shift 2
		;;
	*)
		if [ -z "$DATASET" ]; then
			DATASET="$1"
		else
			echo "usage: $0 DATASET [--top-k N] [--outdir DIR]" >&2
			exit 2
		fi
		shift
		;;
	esac
done

if [ -z "$DATASET" ]; then
	echo "usage: $0 DATASET [--top-k N] [--outdir DIR]" >&2
	exit 2
fi

if [ ! -x "$BIN" ]; then
	echo "building $BIN"
	(cd "$REPO_ROOT" && CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -tags sqlite_fts5 -o bin/kb ./cmd/kb)
fi

if [ -z "$OUTDIR" ]; then
	STEM="$(basename "$DATASET")"
	STEM="${STEM%.*}"
	OUTDIR="$REPO_ROOT/bench-results/${STEM}-$(date +%Y%m%d-%H%M%S)"
fi
mkdir -p "$OUTDIR"

# Matrix: name|algorithm|bm25_weight|semantic_weight. Weights are inert for
# bm25 and omitted from its report (see bench CLI).
MATRIX=(
	"bm25|bm25||"
	"hybrid-0.7-0.3|hybrid|0.7|0.3"
	"hybrid-0.5-0.5|hybrid|0.5|0.5"
	"hybrid-0.3-0.7|hybrid|0.3|0.7"
	"hybrid-0.1-0.9|hybrid|0.1|0.9"
	"hybrid-0.0-1.0|hybrid|0.0|1.0"
)

echo "dataset: $DATASET (top_k=$TOP_K)"
echo "reports: $OUTDIR"
echo

declare -a REPORTS=()
for entry in "${MATRIX[@]}"; do
	IFS='|' read -r name algorithm bm25w semw <<<"$entry"
	out="$OUTDIR/$name.json"
	args=(bench run --dataset "$DATASET" --top-k "$TOP_K" --algorithm "$algorithm")
	if [ -n "$bm25w" ]; then
		args+=("--bm25-weight" "$bm25w" "--semantic-weight" "$semw")
	fi
	printf '%-18s ' "$name"
	if "$BIN" "${args[@]}" >"$out"; then
		echo "ok ($out)"
		REPORTS+=("$out")
	else
		echo "FAILED (kept stderr above; no report)"
		rm -f "$out"
	fi
done

if [ "${#REPORTS[@]}" -eq 0 ]; then
	echo "no successful runs" >&2
	exit 1
fi

echo
python3 - "${REPORTS[@]}" <<'PY'
import json
import sys

rows = []
for path in sys.argv[1:]:
    with open(path) as f:
        r = json.load(f)
    rank = r["config"]["ranking"]
    label = rank["name"]
    for unit in ("article", "entry"):
        agg = r["aggregate"].get(unit)
        if not agg:
            continue
        rows.append({
            "config": label,
            "unit": unit,
            "precision": agg["precision"],
            "recall": agg["recall"],
            "mrr": agg["mrr"],
            "ndcg": agg["ndcg"],
            "hit1": agg["hit1_rate"],
        })

METRICS = ("precision", "recall", "mrr", "ndcg", "hit1")
HEADER = f"{'config':<28} {'unit':<8}" + "".join(f" {m:>10}" for m in METRICS)
print(HEADER)
print("-" * len(HEADER))

# Best value per (unit, metric) across configs, marked with '*'.
best = {}
for row in rows:
    for m in METRICS:
        key = (row["unit"], m)
        best[key] = max(best.get(key, float("-inf")), row[m])

for row in rows:
    cells = []
    for m in METRICS:
        mark = "*" if row[m] == best[(row["unit"], m)] and len(rows) > 1 else " "
        cells.append(f"{row[m]:>9.3f}{mark}")
    print(f"{row['config']:<28} {row['unit']:<8}" + "".join(f" {c}" for c in cells))

print()
print("* best per (unit, metric); top_k per run shown in reports")
PY
