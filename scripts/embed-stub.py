#!/usr/bin/env python3
"""Deterministic embedder speaking Ollama's /api/embeddings.

Two modes:

1. Topic-dictionary mode (default): embeddings are hand-built concept
   histograms, so cosine similarity measures topic overlap of the known
   stems. Pipeline testing and demos for the paraphrase fixture only.
2. TF-IDF mode (`--corpus corpus.jsonl`): the embedder fits document
   frequencies over a kb dataset JSONL (or BEIR corpus.jsonl), then embeds
   any text as an L2-normalized sublinear TF-IDF vector. Cosine similarity
   is then classic vector-space ranking: weak, bag-of-words "semantics",
   but a real, describable IR model to compare against BM25.

Run numbers from either mode are synthetic stand-ins, not neural-embedding
quality. Ollama lives elsewhere; this server mimics only /api/embeddings.

Usage: scripts/embed-stub.py [--corpus FILE] [port]
"""
import json
import math
import re
import sys
from collections import Counter
from http.server import BaseHTTPRequestHandler, HTTPServer

# Surface forms -> topic. Hybrid fixtures can rely on these stems.
CONCEPTS = {
	"NET": ["wireless", "signal", "modulation", "throughput", "channel"],
	"DNS": ["dns", "resolvers", "recursive", "lookups", "resolution"],
	"QPERF": ["queries", "analytic", "column", "zone", "planner", "segments", "scans"],
	"LOCKS": ["writers", "locks", "serialize", "transactions", "batch", "counter"],
	"IMG": ["images", "pulls", "registry", "mirror", "bases"],
	"CRASH": ["pod", "restarting", "readiness", "dependency", "endpoint", "grace"],
	"SECRETS": ["tokens", "rotation", "revision", "leaks", "credentials", "vault", "revoke"],
	"AUTH": ["password", "guessing", "login", "attempts", "factors", "privileged", "usernames"],
	"ALERT": ["outages", "monitor", "objectives", "budgets", "paging", "symptoms"],
	"METRICS": ["series", "identifier", "aggregate", "cardinality", "storage"],
}
WORD = re.compile(r"[a-z]+")
DIM = {name: i for i, name in enumerate(sorted(CONCEPTS))}
LOOKUP = {w: DIM[c] for c, words in CONCEPTS.items() for w in words}


def tokenize(text):
	return WORD.findall(text.lower())


class Tfidf:
	"""Sublinear TF-IDF over the corpus vocabulary; unseen tokens are
	dropped (idf unknown). L2-normalized output."""

	def __init__(self, path):
		docs = 0
		df = Counter()
		with open(path) as f:
			for line in f:
				r = json.loads(line)
				text = r.get("text") or r.get("content") or ""
				if not text:
					continue
				docs += 1
				for term in set(tokenize(text)):
					df[term] += 1
		self.docs = docs
		self.idf = {t: math.log((docs + 1) / (n + 1)) + 1.0 for t, n in df.items()}
		self.vocab = sorted(self.idf)

	def embed(self, text):
		tf = Counter(tokenize(text))
		vals = {}
		for term, count in tf.items():
			idf = self.idf.get(term)
			if idf is not None:
				vals[term] = (1.0 + math.log(count)) * idf
		norm = math.sqrt(sum(v * v for v in vals.values()))
		return {} if norm == 0 else {t: v / norm for t, v in vals.items()}


def topic_embed(text):
	vec = [0.0] * len(DIM)
	for w in tokenize(text):
		if (dim := LOOKUP.get(w)) is not None:
			vec[dim] += 1.0
	norm = math.sqrt(sum(v * v for v in vec))
	if norm > 0:
		vec = [v / norm for v in vec]
	return vec


TFIDF = None


class Handler(BaseHTTPRequestHandler):
	def do_POST(self):
		if self.path != "/api/embeddings":
			self.send_error(404)
			return
		length = int(self.headers.get("Content-Length", 0))
		req = json.loads(self.rfile.read(length))
		if TFIDF is not None:
			# Sparse dict; dense-list encoding since the client expects an array.
			vec = TFIDF.embed(req.get("prompt", ""))
			emb = [vec.get(term, 0.0) for term in TFIDF.vocab]
		else:
			emb = topic_embed(req.get("prompt", ""))
		body = json.dumps({"embedding": emb}).encode()
		self.send_response(200)
		self.send_header("Content-Type", "application/json")
		self.send_header("Content-Length", str(len(body)))
		self.end_headers()
		self.wfile.write(body)

	def log_message(self, *args):
		pass


def main():
	args = sys.argv[1:]
	corpus = None
	if args and args[0] == "--corpus":
		corpus = args[1]
		args = args[2:]
	port = int(args[0]) if args else 11434
	if corpus:
		global TFIDF
		TFIDF = Tfidf(corpus)
		print(f"embed-stub tf-idf mode over {TFIDF.docs} docs, vocab {len(TFIDF.vocab)}")
	print(f"embed-stub listening on :{port}", flush=True)
	HTTPServer(("127.0.0.1", port), Handler).serve_forever()


if __name__ == "__main__":
	main()
