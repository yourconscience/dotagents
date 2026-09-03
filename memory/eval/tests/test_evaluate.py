import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).parents[1] / "evaluate.py"
SPEC = importlib.util.spec_from_file_location("dotagents_memory_eval", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
EVAL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EVAL)


def two_doc_fixture():
    return {
        "documents": [
            {"id": "doc-1", "text": "alpha corpus", "evidence": [{"id": "ev-1", "text": "alpha"}]},
            {"id": "doc-2", "text": "beta corpus", "evidence": [{"id": "ev-2", "text": "beta"}]},
        ],
        "queries": [{"id": "q-1", "query": "alpha", "expected_evidence_ids": ["ev-1"]}],
        "conversations": [],
        "mutations": [],
    }


class PrivacyScannerTests(unittest.TestCase):
    def test_clean_fixture_passes_and_private_patterns_are_categorized(self):
        clean = '{"id":"doc-1","text":"The fictional project uses bounded memory."}\n'
        self.assertEqual(EVAL.scan_text(clean, "documents.jsonl", []), [])

        dirty = "email user@example.com host 192.168.1.4 path /Users/alice/project token ghp_abcdefghijklmnopqrstuvwxyz123456"
        categories = {finding["category"] for finding in EVAL.scan_text(dirty, "documents.jsonl", ["Alice"])}
        self.assertTrue({"email", "ip_address", "home_path", "secret", "forbidden_canary"} <= categories)
        for finding in EVAL.scan_text(dirty, "documents.jsonl", ["Alice"]):
            self.assertNotIn("alice", json.dumps(finding).lower())


class FixtureTests(unittest.TestCase):
    def test_fixture_load_validates_evidence_references(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "documents.jsonl").write_text(
                json.dumps({"id": "doc-1", "text": "Fact.", "evidence": [{"id": "ev-1", "text": "Fact."}]}) + "\n",
                encoding="utf-8",
            )
            (root / "queries.jsonl").write_text(
                json.dumps({"id": "q-1", "query": "What fact?", "expected_evidence_ids": ["missing"]}) + "\n",
                encoding="utf-8",
            )
            (root / "conversations.jsonl").write_text("", encoding="utf-8")
            (root / "mutations.jsonl").write_text("", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "unknown evidence"):
                EVAL.load_fixture(root)

    def test_fixture_load_validates_mutation_references(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "documents.jsonl").write_text(
                json.dumps({"id": "doc-1", "text": "Fact.", "evidence": [{"id": "ev-1", "text": "Fact."}]}) + "\n",
                encoding="utf-8",
            )
            (root / "queries.jsonl").write_text(
                json.dumps({"id": "q-1", "query": "What fact?", "expected_evidence_ids": ["ev-1"]}) + "\n",
                encoding="utf-8",
            )
            (root / "conversations.jsonl").write_text("", encoding="utf-8")
            (root / "mutations.jsonl").write_text(
                json.dumps({"id": "m-1", "action": "forget", "document_id": "doc-1", "evidence_id": "ev-1", "base_query_id": "nope"}) + "\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "unknown query"):
                EVAL.load_fixture(root)


class ScoringTests(unittest.TestCase):
    def test_ranking_metrics_and_abstention_are_deterministic(self):
        scores = EVAL.score_rankings(
            [
                {"id": "q-1", "expected_evidence_ids": ["ev-2"], "unanswerable": False},
                {"id": "q-2", "expected_evidence_ids": [], "unanswerable": True},
            ],
            {"q-1": ["ev-1", "ev-2"], "q-2": []},
        )
        self.assertEqual(scores["query_count"], 2)
        self.assertEqual(scores["recall_at_1"], 0.5)
        self.assertEqual(scores["recall_at_3"], 1.0)
        self.assertEqual(scores["mrr"], 0.75)
        self.assertEqual(scores["abstention_accuracy"], 1.0)


class BuiltinAdapterTests(unittest.TestCase):
    def test_builtin_adapter_is_bounded_and_uses_no_live_home(self):
        fixture = two_doc_fixture()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            adapter = EVAL.BuiltinAdapter(root, char_limit=30)
            adapter.ingest(fixture["documents"])
            memory_path = root / "hermes" / "memories" / "MEMORY.md"
            self.assertTrue(memory_path.exists())
            self.assertLessEqual(len(memory_path.read_text(encoding="utf-8")), 30)
            result = adapter.query("alpha", top_k=5)
            self.assertEqual(result[0]["evidence_id"], "ev-1")
            self.assertFalse(any(str(Path.home()) in str(path) for path in root.rglob("*")))

    def test_builtin_adapter_drops_entries_that_exceed_budget(self):
        fixture = two_doc_fixture()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            adapter = EVAL.BuiltinAdapter(root, char_limit=8)
            adapter.ingest(fixture["documents"])
            self.assertEqual(adapter.entries, [])
            self.assertEqual((root / "hermes" / "memories" / "MEMORY.md").read_text(encoding="utf-8"), "")

    def test_builtin_adapter_lifecycle_update_and_forget(self):
        fixture = two_doc_fixture()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            adapter = EVAL.BuiltinAdapter(root, char_limit=400)
            adapter.ingest(fixture["documents"])
            update = adapter.update({"evidence_id": "ev-1", "new_text": "alpha revised"})
            self.assertTrue(update["supported"])
            texts = [entry["text"] for entry in adapter.entries]
            self.assertIn("alpha revised", texts)
            forget = adapter.forget({"evidence_id": "ev-2"})
            self.assertTrue(forget["supported"])
            self.assertEqual([entry["evidence_id"] for entry in adapter.entries], ["ev-1"])
            unknown = adapter.forget({"evidence_id": "missing"})
            self.assertFalse(unknown["supported"])


class MemsearchAdapterTests(unittest.TestCase):
    def test_memsearch_adapter_requires_eval_collection_prefix(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, "dotagents_eval_"):
                EVAL.MemsearchAdapter(Path(tmp), collection="ai")


class RunnerTests(unittest.TestCase):
    def test_run_adapter_records_capability_gap_for_unconfigured_provider(self):
        fixture = two_doc_fixture()
        with tempfile.TemporaryDirectory() as tmp:
            result = EVAL.run_adapter(EVAL.make_adapter("honcho", Path(tmp)), fixture)
        self.assertEqual(result["status"], "capability_gap")
        self.assertIsNone(result["metrics"])

    def test_run_adapter_executes_mutation_post_checks(self):
        fixture = two_doc_fixture()
        fixture["mutations"] = [
            {
                "id": "m-1",
                "action": "forget",
                "document_id": "doc-2",
                "evidence_id": "ev-2",
                "base_query_id": "q-1",
                "expect_absent": ["ev-2"],
            }
        ]
        with tempfile.TemporaryDirectory() as tmp:
            result = EVAL.run_adapter(EVAL.BuiltinAdapter(Path(tmp)), fixture)
        self.assertEqual(result["status"], "completed")
        post = result["lifecycle"][0]["post"]
        self.assertTrue(post["absent_ok"])
        self.assertNotIn("ev-2", post["ranked"])


class CapabilityTests(unittest.TestCase):
    def test_discovery_records_manifest_and_runtime_availability(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            provider = root / "holographic"
            provider.mkdir()
            (provider / "plugin.yaml").write_text("name: holographic\nversion: 0.1.0\n", encoding="utf-8")
            matrix = EVAL.discover_capabilities(root, hermes_help="Available providers: holographic, honcho")
            by_name = {item["name"]: item for item in matrix}
            self.assertEqual(by_name["holographic"]["version"], "0.1.0")
            self.assertTrue(by_name["holographic"]["runtime_listed"])
            self.assertTrue(by_name["honcho"]["runtime_listed"])
            self.assertFalse(by_name["honcho"]["manifest_present"])


if __name__ == "__main__":
    unittest.main()
