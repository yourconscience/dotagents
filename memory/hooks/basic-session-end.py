#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "lib"))

from basic_memory import session_end


if __name__ == "__main__":
    raise SystemExit(session_end())
