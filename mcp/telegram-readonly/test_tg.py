from __future__ import annotations

import importlib.util
import unittest
from importlib.machinery import SourceFileLoader
from pathlib import Path
from typing import Any
from unittest.mock import patch


ROOT = Path(__file__).resolve().parent


def _load_tg() -> Any:
    spec = importlib.util.spec_from_loader("tg_cli", SourceFileLoader("tg_cli", str(ROOT / "tg")))
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ReadMessagesTest(unittest.IsolatedAsyncioTestCase):
    async def test_read_messages_pages_oldest_to_newest(self) -> None:
        tg = _load_tg()
        calls: list[dict] = []
        pages = [
            [
                {"id": 4, "text": "four"},
                {"id": 5, "text": "five"},
            ],
            [
                {"id": 2, "text": "two"},
                {"id": 3, "text": "three"},
            ],
            [
                {"id": 1, "text": "one"},
            ],
        ]

        async def fake_call(method: str, params: dict) -> list[dict]:
            self.assertEqual(method, "get_recent_messages")
            calls.append(params)
            return pages[len(calls) - 1]

        with patch.object(tg, "_call", fake_call):
            messages = await tg._read_messages("chat", limit=5, batch_size=2)

        self.assertEqual([m["id"] for m in messages], [1, 2, 3, 4, 5])
        self.assertEqual(calls, [
            {"chat": "chat", "limit": 2},
            {"chat": "chat", "limit": 2, "offset_id": 4},
            {"chat": "chat", "limit": 1, "offset_id": 2},
        ])

    async def test_read_messages_rejects_non_paging_daemon(self) -> None:
        tg = _load_tg()
        pages = [
            [{"id": 4}, {"id": 5}],
            [{"id": 4}, {"id": 5}],
        ]

        async def fake_call(method: str, params: dict) -> list[dict]:
            return pages.pop(0)

        with patch.object(tg, "_call", fake_call):
            with self.assertRaisesRegex(RuntimeError, "paged reads"):
                await tg._read_messages("chat", limit=4, batch_size=2)


if __name__ == "__main__":
    unittest.main()
