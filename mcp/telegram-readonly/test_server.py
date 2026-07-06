from __future__ import annotations

import importlib.util
import tempfile
import unittest
from importlib.machinery import SourceFileLoader
from pathlib import Path
from types import SimpleNamespace
from typing import Any


ROOT = Path(__file__).resolve().parent


def _load_server() -> Any:
    spec = importlib.util.spec_from_loader("tg_server", SourceFileLoader("tg_server", str(ROOT / "server.py")))
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ServerHelpersTest(unittest.TestCase):
    def test_media_kind_marks_common_media_types(self) -> None:
        server = _load_server()

        cases = [
            ("voice", SimpleNamespace(video_note=False, voice=True, video=False, gif=False, sticker=False, audio=False, photo=False, document=False)),
            ("video", SimpleNamespace(video_note=False, voice=False, video=True, gif=False, sticker=False, audio=False, photo=False, document=False)),
            ("photo", SimpleNamespace(video_note=False, voice=False, video=False, gif=False, sticker=False, audio=False, photo=True, document=False)),
        ]

        for expected, msg in cases:
            with self.subTest(expected=expected):
                self.assertEqual(server._media_kind(msg), expected)

    def test_message_to_dict_marks_video_note(self) -> None:
        server = _load_server()
        msg = SimpleNamespace(
            id=42,
            date=None,
            sender_id=7,
            sender=SimpleNamespace(first_name="Alice", last_name=None, username="alice", id=7),
            message="",
            media=object(),
            reply_to=None,
            file=SimpleNamespace(name="round.mp4", mime_type="video/mp4"),
            video_note=True,
            voice=False,
            video=False,
            gif=False,
            sticker=False,
            audio=False,
            photo=False,
            document=False,
        )

        data = server._message_to_dict(msg)

        self.assertEqual(data["media_kind"], "video_note")
        self.assertEqual(data["file_name"], "round.mp4")
        self.assertEqual(data["mime_type"], "video/mp4")

    def test_message_to_dict_skips_media_fields_without_media(self) -> None:
        server = _load_server()
        msg = SimpleNamespace(
            id=43,
            date=None,
            sender_id=7,
            sender=None,
            message="hello",
            media=None,
            reply_to=None,
            file=SimpleNamespace(name="ignored.mp4", mime_type="video/mp4"),
            video_note=True,
        )

        data = server._message_to_dict(msg)

        self.assertFalse(data["has_media"])
        self.assertIsNone(data["media_kind"])
        self.assertIsNone(data["file_name"])
        self.assertIsNone(data["mime_type"])

    def test_download_target_creates_directory_output(self) -> None:
        server = _load_server()
        with tempfile.TemporaryDirectory() as tmp:
            target = server._download_target(f"{tmp}/circles/")
            self.assertTrue(target.exists())
            self.assertTrue(target.is_dir())

    def test_download_target_preserves_file_output(self) -> None:
        server = _load_server()
        with tempfile.TemporaryDirectory() as tmp:
            target = server._download_target(f"{tmp}/circles/video-note.mp4")
            self.assertEqual(target.name, "video-note.mp4")
            self.assertTrue(target.parent.exists())


if __name__ == "__main__":
    unittest.main()
