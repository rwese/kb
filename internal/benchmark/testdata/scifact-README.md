# scifact-v1 dataset provenance

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
(punctuation stripped, word stems preserved); 600 documents
(283 judged, remainder as deterministically sampled
distractors); 153 queries (150 BEIR test queries +
hand-written controls).

Note: the HuggingFace `allenai/scifact` card claims CC BY-NC; the canonical
GitHub LICENSE.md governs and permits redistribution - documented by the
dataset research in the kb repo history.
