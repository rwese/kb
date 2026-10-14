#!/usr/bin/env python3
"""Convert the BEIR-packaged SciFact subset into the kb dataset JSONL format.

Deterministic: doc selection, id remapping, and query subsampling are pure
functions of the input files, so re-running against the same BEIR zip yields
byte-identical output (the benchmark's sha-256 bindings depend on that).

Usage:
  scripts/make-scifact-dataset.py <beir-scifact-dir> \
      [--docs N] [--queries N]

Input: the unpacked BEIR scifact directory (corpus.jsonl, queries.jsonl,
qrels/test.tsv). Binary gold qrels (score 1 = directly addresses the claim,
supporting or refuting) map to grade 3, matching the other fixtures'
judgment style.

Output:
  internal/benchmark/testdata/scifact-v1.jsonl
  internal/benchmark/testdata/scifact-README.md   (license/attribution, once)

Licensing: corpus abstracts are ODC-By 1.0 (S2ORC), annotations CC BY 4.0 -
see the generated README before committing.
"""
import argparse
import hashlib
import json
import os
import sys
from collections import Counter

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT_JSONL = os.path.join(REPO, "internal/benchmark/testdata/scifact-v1.jsonl")
OUT_README = os.path.join(REPO, "internal/benchmark/testdata/scifact-README.md")
CONTROL_QUERIES = [
	("q-ctrl-audit", "quarterly audit filing deadlines for corporate taxes", "article"),
	("q-ctrl-cafe", "best neighborhood espresso beans to brew at home", "article"),
	("q-ctrl-garden", "repotting an overgrown monstera without root damage", "article"),
]


def fts_safe(text):
	"""BEIR claims go raw into FTS5 MATCH; strip anything that parses as
	query syntax (quotes, dots, hyphens, apostrophes, commas). Word stems are
	preserved, so lexical matching is not distorted. Recorded in the README."""
	cleaned = "".join(c if c.isalnum() or c == "_" else " " for c in text)
	return " ".join(cleaned.split())


def hexids(doc_id, taken_docs, taken_entries):
	"""Map an arbitrary BEIR doc id to (entry, suffix), both 6 lowercase hex,
	deterministically and collision-free."""
	for salt in ("", "a", "b", "c", "d"):
		h = hashlib.md5((salt + doc_id).encode()).hexdigest()
		entry, suffix = h[:6], h[6:12]
		if entry in taken_entries or f"{entry}-{suffix}" in taken_docs:
			continue
		taken_docs.add(f"{entry}-{suffix}")
		taken_entries.add(entry)
		return entry, suffix
	raise RuntimeError("id space exhausted")


def main():
	ap = argparse.ArgumentParser()
	ap.add_argument("beir_dir")
	ap.add_argument("--docs", type=int, default=600, help="total documents (judged + distractors)")
	ap.add_argument("--queries", type=int, default=150)
	ap.add_argument("--seed-sort", default="")
	args = ap.parse_args()

	# Test-split qrels: binary golds.
	qrels = {}
	with open(os.path.join(args.beir_dir, "qrels", "test.tsv")) as f:
		next(f)
		for line in f:
			qid, cid, score = line.rstrip("\n").split("\t")
			if int(score) > 0:
				qrels.setdefault(qid, []).append(cid)
	judged_docs = sorted({cid for golds in qrels.values() for cid in golds})

	# Corpus: id -> (title, text)
	corpus = {}
	with open(os.path.join(args.beir_dir, "corpus.jsonl")) as f:
		for line in f:
			d = json.loads(line)
			corpus[d["_id"]] = (d.get("title") or "", d.get("text") or "")
	missing = [d for d in judged_docs if d not in corpus]
	if missing:
		sys.exit(f"qrels reference {len(missing)} docs absent from corpus, e.g. {missing[:3]}")

	# Distractors: deterministic round-robin over md5 order, judged docs first.
	pool = sorted((d for d in corpus if d not in judged_docs),
		key=lambda d: hashlib.md5(d.encode()).hexdigest())
	target_dods = judged_docs + pool[: max(0, args.docs - len(judged_docs))]

	# Queries with a retrievable gold doc inside the kept doc set, stable order.
	query_texts = {}
	with open(os.path.join(args.beir_dir, "queries.jsonl")) as f:
		for line in f:
			q = json.loads(line)
			query_texts[q["_id"]] = q.get("text") or ""
	test_qids = sorted((q for q in qrels if q in query_texts),
		key=lambda q: hashlib.md5(q.encode()).hexdigest())
	kept_qids = test_qids[: args.queries]

	# Remap ids.
	taken_docs, taken_entries = set(), set()
	entrymap = {}   # beir doc id -> entry id
	docmap = {}     # beir doc id -> kb document id
	for old in target_dods:
		entry, suffix = hexids(old, taken_docs, taken_entries)
		entrymap[old] = entry
		docmap[old] = f"{entry}-{suffix}"

	wrote_queries = 0
	with open(OUT_JSONL, "w") as out:
		out.write(json.dumps({"type": "dataset", "version": 1, "id": "scifact-v1"}) + "\n")
		seen_entries = set()
		for old in target_dods:
			entry = entrymap[old]
			if entry not in seen_entries:
				seen_entries.add(entry)
				out.write(json.dumps({"type": "entry", "id": entry,
					"title": corpus[old][0] or corpus[old][1][:60], "tags": "scifact"}) + "\n")
			out.write(json.dumps({"type": "document", "id": docmap[old],
				"entry_id": entry, "title": corpus[old][0] or corpus[old][1][:60],
				"content": corpus[old][1]}) + "\n")
		for qid in kept_qids:
			wrote_queries += 1
			out.write(json.dumps({"type": "query", "id": f"q-{qid}",
				"text": fts_safe(query_texts[qid]), "unit": "article",
				"relevant": [{"id": docmap[cid], "grade": 3} for cid in qrels[qid]]}) + "\n")
		for cid, text, unit in CONTROL_QUERIES:
			wrote_queries += 1
			out.write(json.dumps({"type": "query", "id": cid, "text": text,
				"unit": unit}) + "\n")

	# Attribution README (write once; never overwrite manual edits).
	if not os.path.exists(OUT_README):
		with open(OUT_README, "w") as f:
			f.write(f"""# scifact-v1 dataset provenance

Generated from the BEIR-packaged SciFact subset by
`scripts/make-scifact-dataset.py`; do not edit `scifact-v1.jsonl` by hand
(it is deterministic and may be sha-256-bound).

- Source: https://public.ukp.informatik.tu-darmstadt.de/thakur/BEIR/datasets/scifact.zip
- Corpus abstracts: ODC-By 1.0 (part of Semantic Scholar S2ORC); see
  https://github.com/allenai/scifact LICENSE.md.
- Claims / relevance annotations: CC BY 4.0.
- Citation: Wadden et al., "Fact or Fiction: Verifying Scientific Claims with
  Weak Supervision", EMNLP 2020. If used, also cite S2ORC and BEIR
  (Thakur et al., NeurIPS 2021).

Transformations: binary gold qrels mapped to grade 3; ids remapped to the kb
6-hex scheme; query texts sanitized for FTS5 MATCH compatibility
(punctuation stripped, word stems preserved); {len(target_dods)} documents
({len(judged_docs)} judged, remainder as deterministically sampled
distractors); {wrote_queries} queries ({len(kept_qids)} BEIR test queries +
hand-written controls).

Note: the HuggingFace `allenai/scifact` card claims CC BY-NC; the canonical
GitHub LICENSE.md governs and permits redistribution - documented by the
dataset research in the kb repo history.
""")

	print(f"wrote {OUT_JSONL}: {len(target_dods)} docs, {wrote_queries} queries")
	print(f"judged docs: {len(judged_docs)}, BEIR queries kept: {len(kept_qids)}")


if __name__ == "__main__":
	main()
