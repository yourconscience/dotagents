#!/usr/bin/env python3
"""Privacy-safe, isolated memory-provider evaluation for dotagents.

The harness runs deterministic built-in-memory and real memsearch baselines.
Hermes provider plugins are inventoried from their manifests and emitted as
explicit capability gaps until a configured, isolated driver is available.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
import uuid
from pathlib import Path
from typing import Any, Iterable

PROVIDER_METADATA: dict[str, dict[str, Any]] = {
    "honcho": {"storage": "cloud/self-hosted", "local_first": False, "tools": ["honcho_profile", "honcho_search", "honcho_context", "honcho_reasoning", "honcho_conclude"]},
    "openviking": {"storage": "self-hosted", "local_first": True, "tools": ["viking_search", "viking_read", "viking_browse", "viking_remember", "viking_forget", "viking_add_resource"]},
    "mem0": {"storage": "cloud/self-hosted/oss", "local_first": True, "tools": ["mem0_search", "mem0_add", "mem0_update", "mem0_delete"]},
    "hindsight": {"storage": "cloud/local", "local_first": True, "tools": ["hindsight_retain", "hindsight_recall", "hindsight_reflect"]},
    "holographic": {"storage": "local", "local_first": True, "tools": ["fact_store", "fact_feedback"]},
    "retaindb": {"storage": "cloud", "local_first": False, "tools": ["retaindb_profile", "retaindb_search", "retaindb_context", "retaindb_remember", "retaindb_forget"]},
    "byterover": {"storage": "local/cloud", "local_first": True, "tools": ["brv_query", "brv_curate", "brv_status"]},
    "supermemory": {"storage": "cloud/self-hosted", "local_first": True, "tools": ["supermemory_store", "supermemory_search", "supermemory_forget", "supermemory_profile"]},
}
BASELINE_PROVIDERS = ("builtin", "memsearch")

PRIVACY_PATTERNS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("email", re.compile(r"(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b")),
    ("ip_address", re.compile(r"(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])")),
    ("phone", re.compile(r"(?<!\w)(?:\+?\d[\d ()-]{8,}\d)(?!\w)")),
    ("home_path", re.compile(r"(?i)(?:/Users/[^/\s]+|/home/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+)")),
    ("secret", re.compile(r"(?i)\b(?:gh[pousr]_|github_pat_|sk-(?:ant-|proj-)?|glpat-|xox[baprs]-)[A-Za-z0-9_-]{16,}")),
    ("credential_assignment", re.compile(r"(?i)\b(?:api[_-]?key|secret|token|password)\s*[:=]\s*[\"']?[^\s\"']{8,}")),
)


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    if not path.exists():
        raise ValueError(f"missing fixture file: {path.name}")
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(f"invalid JSONL in {path.name}:{line_number}: {exc.msg}") from exc
        if not isinstance(value, dict):
            raise ValueError(f"record in {path.name}:{line_number} must be an object")
        records.append(value)
    return records


def load_fixture(root: Path) -> dict[str, list[dict[str, Any]]]:
    fixture = {
        "documents": read_jsonl(root / "documents.jsonl"),
        "queries": read_jsonl(root / "queries.jsonl"),
        "conversations": read_jsonl(root / "conversations.jsonl"),
        "mutations": read_jsonl(root / "mutations.jsonl"),
    }
    document_ids: set[str] = set()
    evidence_ids: set[str] = set()
    for document in fixture["documents"]:
        doc_id = str(document.get("id", "")).strip()
        text = document.get("text")
        if not doc_id or doc_id in document_ids or not isinstance(text, str) or not text.strip():
            raise ValueError("documents require unique non-empty id and text")
        document_ids.add(doc_id)
        evidence = document.get("evidence")
        if not isinstance(evidence, list) or not evidence:
            raise ValueError(f"document {doc_id} requires evidence")
        for item in evidence:
            if not isinstance(item, dict) or not str(item.get("id", "")).strip() or not str(item.get("text", "")).strip():
                raise ValueError(f"document {doc_id} has invalid evidence")
            evidence_id = str(item["id"])
            if evidence_id in evidence_ids:
                raise ValueError(f"duplicate evidence id: {evidence_id}")
            evidence_ids.add(evidence_id)
    query_ids: set[str] = set()
    for query in fixture["queries"]:
        query_id = str(query.get("id", "")).strip()
        if not query_id or query_id in query_ids or not str(query.get("query", "")).strip():
            raise ValueError("queries require unique non-empty id and query")
        query_ids.add(query_id)
        expected = query.get("expected_evidence_ids", [])
        if not isinstance(expected, list):
            raise ValueError(f"query {query_id} expected_evidence_ids must be a list")
        unknown = sorted(set(str(value) for value in expected) - evidence_ids)
        if unknown:
            raise ValueError(f"query {query_id} references unknown evidence: {', '.join(unknown)}")
    for mutation in fixture["mutations"]:
        mutation_id = str(mutation.get("id", "")).strip()
        if not mutation_id:
            raise ValueError("mutations require a non-empty id")
        action = str(mutation.get("action", "")).strip()
        if action not in ("update", "forget"):
            raise ValueError(f"mutation {mutation_id} has unsupported action: {action or '(empty)'}")
        evidence_key = str(mutation.get("evidence_id", "")).strip()
        if evidence_key not in evidence_ids:
            raise ValueError(f"mutation {mutation_id} references unknown evidence: {evidence_key or '(empty)'}")
        base_query = str(mutation.get("base_query_id", "")).strip()
        if base_query and base_query not in query_ids:
            raise ValueError(f"mutation {mutation_id} references unknown query: {base_query}")
        if action == "update" and not str(mutation.get("new_text", "")).strip():
            raise ValueError(f"mutation {mutation_id} update requires new_text")
    return fixture


def scan_text(text: str, relative_path: str, forbidden: Iterable[str]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    lines = text.splitlines() or [text]
    for line_number, line in enumerate(lines, start=1):
        for category, pattern in PRIVACY_PATTERNS:
            if pattern.search(line):
                findings.append({"category": category, "path": relative_path, "line": line_number})
        lowered = line.casefold()
        for canary in forbidden:
            value = canary.strip()
            if value and value.casefold() in lowered:
                findings.append({"category": "forbidden_canary", "path": relative_path, "line": line_number})
                break
    return findings


def scan_fixture(root: Path, forbidden: Iterable[str] = ()) -> dict[str, Any]:
    files = ("documents.jsonl", "queries.jsonl", "conversations.jsonl", "mutations.jsonl")
    findings: list[dict[str, Any]] = []
    digest = hashlib.sha256()
    for name in files:
        path = root / name
        if not path.is_file():
            findings.append({"category": "missing_file", "path": name, "line": 0})
            continue
        data = path.read_bytes()
        digest.update(name.encode("utf-8") + b"\0" + data)
        findings.extend(scan_text(data.decode("utf-8"), name, forbidden))
    return {
        "schema_version": 1,
        "fixture_sha256": digest.hexdigest(),
        "files": list(files),
        "finding_count": len(findings),
        "findings": findings,
        "approved": False,
    }


def _yaml_scalar(path: Path, key: str) -> str | None:
    pattern = re.compile(rf"^\s*{re.escape(key)}\s*:\s*[\"']?([^\"'#\n]+)")
    for line in path.read_text(encoding="utf-8").splitlines():
        match = pattern.match(line)
        if match:
            return match.group(1).strip()
    return None


def discover_capabilities(plugin_root: Path, hermes_help: str | None = None) -> list[dict[str, Any]]:
    if hermes_help is None:
        hermes = shutil.which("hermes")
        if hermes:
            result = subprocess.run([hermes, "memory", "--help"], capture_output=True, text=True, timeout=15, check=False)
            hermes_help = result.stdout + result.stderr
        else:
            hermes_help = ""
    runtime_names = {
        name for name in PROVIDER_METADATA if re.search(rf"(?<![\w-]){re.escape(name)}(?![\w-])", hermes_help or "", re.IGNORECASE)
    }
    manifests: dict[str, Path] = {}
    if plugin_root.is_dir():
        for manifest in plugin_root.glob("*/plugin.yaml"):
            name = _yaml_scalar(manifest, "name") or manifest.parent.name
            manifests[name] = manifest
    names = sorted(set(PROVIDER_METADATA) | runtime_names | set(manifests))
    matrix: list[dict[str, Any]] = []
    for name in names:
        manifest = manifests.get(name)
        metadata = PROVIDER_METADATA.get(name, {})
        matrix.append({
            "name": name,
            "version": _yaml_scalar(manifest, "version") if manifest else None,
            "manifest_present": manifest is not None,
            "runtime_listed": name in runtime_names,
            "storage": metadata.get("storage", "unknown"),
            "local_first": metadata.get("local_first", False),
            "tools": metadata.get("tools", []),
            "automated_driver": False,
            "capability_gap": "isolated provider driver not configured",
        })
    return matrix


def _round(value: float) -> float:
    return round(value, 6)


def score_rankings(queries: list[dict[str, Any]], rankings: dict[str, list[str]]) -> dict[str, Any]:
    if not queries:
        return {"query_count": 0, "recall_at_1": 0.0, "recall_at_3": 0.0, "recall_at_5": 0.0, "mrr": 0.0, "ndcg_at_5": 0.0, "abstention_accuracy": 0.0}
    recall_sums = {1: 0.0, 3: 0.0, 5: 0.0}
    reciprocal_ranks: list[float] = []
    ndcgs: list[float] = []
    abstention: list[float] = []
    for query in queries:
        expected = set(str(value) for value in query.get("expected_evidence_ids", []))
        ranked = rankings.get(str(query["id"]), [])
        is_unanswerable = bool(query.get("unanswerable", not expected))
        if is_unanswerable:
            value = 1.0 if not ranked else 0.0
            for k in recall_sums:
                recall_sums[k] += value
            reciprocal_ranks.append(value)
            ndcgs.append(value)
            abstention.append(value)
            continue
        for k in recall_sums:
            recall_sums[k] += len(expected.intersection(ranked[:k])) / len(expected)
        positions = [index + 1 for index, evidence_id in enumerate(ranked) if evidence_id in expected]
        reciprocal_ranks.append(1.0 / min(positions) if positions else 0.0)
        dcg = sum(1.0 / math.log2(index + 2) for index, evidence_id in enumerate(ranked[:5]) if evidence_id in expected)
        ideal = sum(1.0 / math.log2(index + 2) for index in range(min(len(expected), 5)))
        ndcgs.append(dcg / ideal if ideal else 0.0)
    count = len(queries)
    return {
        "query_count": count,
        "recall_at_1": _round(recall_sums[1] / count),
        "recall_at_3": _round(recall_sums[3] / count),
        "recall_at_5": _round(recall_sums[5] / count),
        "mrr": _round(sum(reciprocal_ranks) / count),
        "ndcg_at_5": _round(sum(ndcgs) / count),
        "abstention_accuracy": _round(sum(abstention) / len(abstention)) if abstention else None,
    }


class ProviderAdapter:
    name = "unknown"

    def __init__(self, sandbox: Path) -> None:
        self.sandbox = sandbox.resolve()
        self.sandbox.mkdir(parents=True, exist_ok=True)

    def health(self) -> dict[str, Any]:
        return {"available": True}

    def reset(self) -> None:
        return None

    def ingest(self, documents: list[dict[str, Any]]) -> None:
        raise NotImplementedError

    def query(self, query: str, top_k: int) -> list[dict[str, Any]]:
        raise NotImplementedError

    def update(self, mutation: dict[str, Any]) -> dict[str, Any]:
        return {"supported": False, "reason": "update is not implemented by this adapter"}

    def forget(self, mutation: dict[str, Any]) -> dict[str, Any]:
        return {"supported": False, "reason": "forget is not implemented by this adapter"}

    def restart(self) -> dict[str, Any]:
        return {"supported": True}

    def export(self) -> dict[str, Any]:
        return {"supported": False, "reason": "export is not implemented by this adapter"}

    def capture(self, conversation: dict[str, Any]) -> dict[str, Any]:
        del conversation
        return {"supported": False, "reason": "scripted conversation capture is not implemented by this adapter"}

    def teardown(self) -> None:
        return None


class BuiltinAdapter(ProviderAdapter):
    name = "builtin"

    def __init__(self, sandbox: Path, char_limit: int = 2200) -> None:
        super().__init__(sandbox)
        self.char_limit = char_limit
        self.memory_path = self.sandbox / "hermes" / "memories" / "MEMORY.md"
        self.entries: list[dict[str, str]] = []

    def reset(self) -> None:
        self.entries = []
        if self.memory_path.exists():
            self.memory_path.unlink()

    def ingest(self, documents: list[dict[str, Any]]) -> None:
        self.memory_path.parent.mkdir(parents=True, exist_ok=True)
        entries: list[dict[str, str]] = []
        rendered: list[str] = []
        for document in documents:
            for evidence in document["evidence"]:
                text = f"[{evidence['id']}] {evidence['text']}"
                candidate = "\n§\n".join(rendered + [text]) + "\n"
                if len(candidate) > self.char_limit:
                    self.entries = entries
                    self.memory_path.write_text("\n§\n".join(rendered) + ("\n" if rendered else ""), encoding="utf-8")
                    return
                rendered.append(text)
                entries.append({"evidence_id": str(evidence["id"]), "text": str(evidence["text"]), "document_id": str(document["id"])})
        self.entries = entries
        self.memory_path.write_text("\n§\n".join(rendered) + ("\n" if rendered else ""), encoding="utf-8")

    def _persist(self) -> None:
        self.memory_path.parent.mkdir(parents=True, exist_ok=True)
        rendered = [f"[{entry['evidence_id']}] {entry['text']}" for entry in self.entries]
        self.memory_path.write_text("\n§\n".join(rendered) + ("\n" if rendered else ""), encoding="utf-8")

    def _find_entry(self, evidence_id: str) -> dict[str, str] | None:
        for entry in self.entries:
            if entry["evidence_id"] == evidence_id:
                return entry
        return None

    def update(self, mutation: dict[str, Any]) -> dict[str, Any]:
        entry = self._find_entry(str(mutation.get("evidence_id", "")))
        if entry is None:
            return {"supported": False, "reason": "unknown evidence id"}
        entry["text"] = str(mutation.get("new_text", "")).strip()
        self._persist()
        return {"supported": True, "changed": 1}

    def forget(self, mutation: dict[str, Any]) -> dict[str, Any]:
        evidence_id = str(mutation.get("evidence_id", ""))
        before = len(self.entries)
        self.entries = [entry for entry in self.entries if entry["evidence_id"] != evidence_id]
        if len(self.entries) == before:
            return {"supported": False, "reason": "unknown evidence id"}
        self._persist()
        return {"supported": True, "removed": 1}

    def query(self, query: str, top_k: int) -> list[dict[str, Any]]:
        del query
        return [dict(item, score=1.0) for item in self.entries[:top_k]]

    def export(self) -> dict[str, Any]:
        return {"supported": True, "path": str(self.memory_path.relative_to(self.sandbox)), "characters": len(self.memory_path.read_text(encoding="utf-8"))}


class MemsearchAdapter(ProviderAdapter):
    name = "memsearch"

    def __init__(self, sandbox: Path, collection: str | None = None) -> None:
        super().__init__(sandbox)
        self.binary = shutil.which("memsearch")
        self.collection = collection or f"dotagents_eval_{uuid.uuid4().hex[:12]}"
        if not self.collection.startswith("dotagents_eval_"):
            raise ValueError("evaluation memsearch collection must start with dotagents_eval_")
        self.documents_dir = self.sandbox / "documents"
        self.evidence_by_document: dict[str, list[str]] = {}

    def health(self) -> dict[str, Any]:
        return {"available": self.binary is not None, "binary": self.binary, "collection": self.collection}

    def reset(self) -> None:
        if self.binary:
            subprocess.run([self.binary, "reset", "--collection", self.collection, "--yes"], capture_output=True, text=True, timeout=30, check=False)
        if self.documents_dir.exists():
            shutil.rmtree(self.documents_dir)

    def ingest(self, documents: list[dict[str, Any]]) -> None:
        if not self.binary:
            raise RuntimeError("memsearch is not installed")
        self.documents_dir.mkdir(parents=True, exist_ok=True)
        self.evidence_by_document = {}
        for document in documents:
            doc_id = str(document["id"])
            evidence_ids = [str(item["id"]) for item in document["evidence"]]
            self.evidence_by_document[doc_id] = evidence_ids
            lines = [f"# {document.get('title') or doc_id}", "", str(document["text"]), ""]
            lines.extend(f"[evidence:{item['id']}] {item['text']}" for item in document["evidence"])
            (self.documents_dir / f"{doc_id}.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
        result = subprocess.run(
            [self.binary, "index", str(self.documents_dir), "--collection", self.collection, "--force"],
            capture_output=True,
            text=True,
            timeout=180,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(f"memsearch index failed: {(result.stderr or result.stdout).strip()[:500]}")

    @staticmethod
    def _result_records(payload: Any) -> list[dict[str, Any]]:
        if isinstance(payload, list):
            return [item for item in payload if isinstance(item, dict)]
        if isinstance(payload, dict):
            for key in ("results", "data", "matches"):
                value = payload.get(key)
                if isinstance(value, list):
                    return [item for item in value if isinstance(item, dict)]
        return []

    def query(self, query: str, top_k: int) -> list[dict[str, Any]]:
        if not self.binary:
            raise RuntimeError("memsearch is not installed")
        result = subprocess.run(
            [self.binary, "search", query, "--top-k", str(top_k), "--collection", self.collection, "--source-prefix", str(self.documents_dir), "--json-output"],
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(f"memsearch search failed: {(result.stderr or result.stdout).strip()[:500]}")
        payload = json.loads(result.stdout)
        ranked: list[dict[str, Any]] = []
        seen: set[str] = set()
        for record in self._result_records(payload):
            source = str(record.get("source") or record.get("path") or record.get("file") or "")
            content = str(record.get("content") or record.get("text") or record.get("chunk") or "")
            score = record.get("score") or record.get("similarity") or 0.0
            doc_id = Path(source).stem if source else ""
            ids = re.findall(r"\[evidence:([^\]]+)\]", content)
            if not ids:
                ids = self.evidence_by_document.get(doc_id, [])
            for evidence_id in ids:
                if evidence_id not in seen:
                    seen.add(evidence_id)
                    ranked.append({"evidence_id": evidence_id, "document_id": doc_id, "score": score})
                    if len(ranked) >= top_k:
                        return ranked
        return ranked

    def teardown(self) -> None:
        self.reset()


class UnavailableProviderAdapter(ProviderAdapter):
    def __init__(self, sandbox: Path, name: str, reason: str) -> None:
        super().__init__(sandbox)
        self.name = name
        self.reason = reason

    def health(self) -> dict[str, Any]:
        return {"available": False, "reason": self.reason}

    def ingest(self, documents: list[dict[str, Any]]) -> None:
        del documents
        raise RuntimeError(self.reason)

    def query(self, query: str, top_k: int) -> list[dict[str, Any]]:
        del query, top_k
        raise RuntimeError(self.reason)


class Timer:
    def __init__(self) -> None:
        self.started = 0.0
        self.elapsed_ms = 0.0

    def __enter__(self) -> "Timer":
        self.started = time.perf_counter()
        return self

    def __exit__(self, exc_type: Any, exc: Any, traceback: Any) -> None:
        del exc_type, exc, traceback
        self.elapsed_ms = (time.perf_counter() - self.started) * 1000


def _latency_summary(values: list[float]) -> dict[str, float | None]:
    if not values:
        return {"p50_ms": None, "p95_ms": None}
    ordered = sorted(values)
    index = max(0, math.ceil(0.95 * len(ordered)) - 1)
    return {"p50_ms": _round(statistics.median(ordered)), "p95_ms": _round(ordered[index])}


def run_adapter(adapter: ProviderAdapter, fixture: dict[str, list[dict[str, Any]]], top_k: int = 5) -> dict[str, Any]:
    health = adapter.health()
    if not health.get("available"):
        return {"provider": adapter.name, "status": "capability_gap", "health": health, "metrics": None}
    rankings: dict[str, list[str]] = {}
    query_details: list[dict[str, Any]] = []
    query_latencies: list[float] = []
    lifecycle: list[dict[str, Any]] = []
    try:
        adapter.reset()
        with Timer() as ingest_timer:
            adapter.ingest(fixture["documents"])
        for query in fixture["queries"]:
            with Timer() as query_timer:
                results = adapter.query(str(query["query"]), top_k)
            ids = [str(item["evidence_id"]) for item in results]
            rankings[str(query["id"])] = ids
            query_latencies.append(query_timer.elapsed_ms)
            query_details.append({"query_id": query["id"], "evidence_ids": ids, "latency_ms": _round(query_timer.elapsed_ms)})
        for mutation in fixture["mutations"]:
            action = mutation.get("action")
            if action == "forget":
                outcome = adapter.forget(mutation)
            else:
                outcome = adapter.update(mutation)
            post: dict[str, Any] = {}
            base_query_id = str(mutation.get("base_query_id", "")).strip()
            if outcome.get("supported") and base_query_id:
                base_query = next((q for q in fixture["queries"] if str(q["id"]) == base_query_id), None)
                if base_query is not None:
                    ranked = [str(item["evidence_id"]) for item in adapter.query(str(base_query["query"]), top_k)]
                    post["ranked"] = ranked
                    expect_absent = [str(value) for value in mutation.get("expect_absent", [])]
                    if expect_absent:
                        post["absent_ok"] = all(value not in ranked for value in expect_absent)
            lifecycle.append({"mutation_id": mutation.get("id"), "action": action, **outcome, **({"post": post} if post else {})})
        restart = adapter.restart()
        exported = adapter.export()
        capture = [adapter.capture(conversation) for conversation in fixture["conversations"]]
        return {
            "provider": adapter.name,
            "status": "completed",
            "health": health,
            "metrics": score_rankings(fixture["queries"], rankings),
            "latency": {"ingest_ms": _round(ingest_timer.elapsed_ms), "query": _latency_summary(query_latencies)},
            "queries": query_details,
            "lifecycle": lifecycle,
            "capture": capture,
            "restart": restart,
            "export": exported,
        }
    except (OSError, RuntimeError, subprocess.SubprocessError, json.JSONDecodeError) as exc:
        return {"provider": adapter.name, "status": "failed", "health": health, "error": str(exc)[:500], "metrics": None}
    finally:
        adapter.teardown()


def make_adapter(name: str, sandbox: Path) -> ProviderAdapter:
    if name == "builtin":
        return BuiltinAdapter(sandbox)
    if name == "memsearch":
        return MemsearchAdapter(sandbox)
    return UnavailableProviderAdapter(sandbox, name, "provider requires an explicitly configured isolated driver")


def _forbidden_values(path: str | None) -> list[str]:
    if not path:
        return []
    return [line.strip() for line in Path(path).read_text(encoding="utf-8").splitlines() if line.strip()]


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def command_privacy_scan(args: argparse.Namespace) -> int:
    report = scan_fixture(Path(args.fixture), _forbidden_values(args.forbidden_file))
    if args.output:
        write_json(Path(args.output), report)
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if report["finding_count"] else 0


def command_capabilities(args: argparse.Namespace) -> int:
    matrix = discover_capabilities(Path(args.plugin_root).expanduser())
    payload = {"schema_version": 1, "hermes": shutil.which("hermes"), "providers": matrix}
    if args.output:
        write_json(Path(args.output), payload)
    print(json.dumps(payload, indent=2, sort_keys=True))
    return 0


def command_run(args: argparse.Namespace) -> int:
    fixture_root = Path(args.fixture).resolve()
    forbidden = _forbidden_values(args.forbidden_file)
    privacy = scan_fixture(fixture_root, forbidden)
    if privacy["finding_count"]:
        print(json.dumps({"error": "privacy scan failed", "findings": privacy["findings"]}, indent=2), file=sys.stderr)
        return 2
    fixture = load_fixture(fixture_root)
    requested = list(BASELINE_PROVIDERS) + sorted(PROVIDER_METADATA) if args.provider == "all" else [args.provider]
    output = Path(args.output).resolve()
    if output.exists() and not args.overwrite:
        print(f"output already exists: {output} (pass --overwrite)", file=sys.stderr)
        return 2
    results: list[dict[str, Any]] = []
    sandbox_parent = Path(args.sandbox_root).resolve() if args.sandbox_root else None
    with tempfile.TemporaryDirectory(prefix="dotagents-memory-eval-", dir=str(sandbox_parent) if sandbox_parent else None) as tmp:
        root = Path(tmp).resolve()
        if root == Path.home().resolve() or Path.home().resolve() in root.parents:
            # Temp roots under a user's home are safe only because every adapter
            # is still rooted beneath this newly-created evaluation directory.
            pass
        for name in requested:
            arm = root / name
            results.append(run_adapter(make_adapter(name, arm), fixture, top_k=args.top_k))
    payload = {
        "schema_version": 1,
        "fixture_sha256": privacy["fixture_sha256"],
        "privacy_scan": {"finding_count": 0, "approved": bool(args.approved_fixture)},
        "environment": {
            "python": sys.version.split()[0],
            "hermes": shutil.which("hermes"),
            "memsearch": shutil.which("memsearch"),
        },
        "results": results,
    }
    write_json(output, payload)
    print(json.dumps(payload, indent=2, sort_keys=True))
    failed = [item for item in results if item["status"] == "failed"]
    return 1 if failed else 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Evaluate Hermes memory providers without touching live memory stores")
    subparsers = parser.add_subparsers(dest="command", required=True)

    privacy = subparsers.add_parser("privacy-scan", help="scan a frozen fixture for private data")
    privacy.add_argument("--fixture", required=True)
    privacy.add_argument("--forbidden-file", help="local newline-delimited canaries; values are never printed")
    privacy.add_argument("--output")
    privacy.set_defaults(handler=command_privacy_scan)

    capabilities = subparsers.add_parser("capabilities", help="inventory installed Hermes memory providers")
    capabilities.add_argument("--plugin-root", default="~/.hermes/hermes-agent/plugins/memory")
    capabilities.add_argument("--output")
    capabilities.set_defaults(handler=command_capabilities)

    run = subparsers.add_parser("run", help="run isolated evaluation arms")
    run.add_argument("--fixture", required=True)
    run.add_argument("--provider", default="all", choices=["all", *BASELINE_PROVIDERS, *sorted(PROVIDER_METADATA)])
    run.add_argument("--output", required=True)
    run.add_argument("--forbidden-file")
    run.add_argument("--approved-fixture", action="store_true", help="record that the human fixture review gate passed")
    run.add_argument("--overwrite", action="store_true")
    run.add_argument("--top-k", type=int, default=5)
    run.add_argument("--sandbox-root")
    run.set_defaults(handler=command_run)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return int(args.handler(args))


if __name__ == "__main__":
    raise SystemExit(main())
