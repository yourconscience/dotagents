#!/usr/bin/env python3

from __future__ import annotations

import argparse
import asyncio
import json
import os
import sqlite3
import sys
from pathlib import Path
from typing import Any

from twscrape import API, gather


DEFAULT_DB = Path(
    os.environ.get("TECHSEARCH_X_DB", "~/Workspace/techsearch/x/accounts.db")
).expanduser()


def nested_get(value: Any, *keys: str) -> Any:
    cur = value
    for key in keys:
        if isinstance(cur, dict):
            cur = cur.get(key)
        else:
            cur = getattr(cur, key, None)
        if cur is None:
            return None
    return cur


def load_payload(model: Any) -> dict[str, Any]:
    if hasattr(model, "model_dump"):
        payload = model.model_dump(mode="json")
    elif hasattr(model, "dict"):
        payload = model.dict()
    elif hasattr(model, "json"):
        payload = json.loads(model.json())
    else:
        raise TypeError(f"unsupported model type: {type(model)!r}")
    return payload


def add_tweet_url(payload: dict[str, Any]) -> dict[str, Any]:
    username = nested_get(payload, "user", "username")
    tweet_id = payload.get("id") or payload.get("id_str")
    if username and tweet_id:
        payload.setdefault("url", f"https://x.com/{username}/status/{tweet_id}")
    return payload


def add_profile_url(payload: dict[str, Any]) -> dict[str, Any]:
    username = payload.get("username")
    if username:
        payload.setdefault("url", f"https://x.com/{username}")
    return payload


def print_json(value: Any) -> None:
    json.dump(value, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


def error_payload(error: str, message: str, **extra: Any) -> dict[str, Any]:
    payload: dict[str, Any] = {"error": error, "message": message}
    payload.update(extra)
    return payload


def inspect_db(db_path: Path) -> dict[str, Any]:
    if not db_path.exists():
        return {
            "db_exists": False,
            "db_size_bytes": 0,
            "db_readable": False,
            "tables": [],
        }

    try:
        with sqlite3.connect(db_path) as conn:
            rows = conn.execute(
                "SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name"
            ).fetchall()
        tables = [row[0] for row in rows]
        return {
            "db_exists": True,
            "db_size_bytes": db_path.stat().st_size,
            "db_readable": True,
            "tables": tables,
        }
    except Exception as exc:
        return {
            "db_exists": True,
            "db_size_bytes": db_path.stat().st_size,
            "db_readable": False,
            "db_error": str(exc),
            "tables": [],
        }


def doctor_payload(db_path: Path) -> dict[str, Any]:
    db_info = inspect_db(db_path)
    return {
        "db_path": str(db_path),
        **db_info,
        "search_example": (
            "uv run --with twscrape python ~/.agents/skills/techsearch/tools/x_search.py "
            "search --query 'from:karpathy evals' --limit 5 --product Latest"
        ),
        "setup_notes": [
            "This helper expects a twscrape accounts database.",
            "Default location: ~/Workspace/techsearch/x/accounts.db",
            "Prefer cookie-backed accounts over repeated scripted logins.",
            "The doctor command only validates local DB presence and readability. It does not verify live X auth.",
            "Refresh auth manually when X starts returning login or rate-limit failures.",
        ],
        "twscrape_add_accounts_example": (
            "twscrape --db ~/Workspace/techsearch/x/accounts.db add_accounts "
            "~/Workspace/techsearch/x/accounts.txt username:password:email:email_password:_:cookies"
        ),
    }


async def run_search(
    db_path: Path, query: str, limit: int, product: str
) -> list[dict[str, Any]]:
    api = API(str(db_path), raise_when_no_account=True)
    kwargs: dict[str, Any] = {"limit": limit}
    if product != "Latest":
        kwargs["kv"] = {"product": product}
    tweets = await gather(api.search(query, **kwargs))
    return [add_tweet_url(load_payload(tweet)) for tweet in tweets]


async def run_user_by_login(db_path: Path, login: str) -> dict[str, Any]:
    api = API(str(db_path), raise_when_no_account=True)
    user = await api.user_by_login(login)
    if user is None:
        raise LookupError(f"user not found: {login}")
    return add_profile_url(load_payload(user))


def require_db(db_path: Path) -> None:
    if db_path.exists():
        return
    print_json(
        {
            "error": "missing_accounts_db",
            "message": (
                "No twscrape accounts database was found. Run the doctor command for setup guidance."
            ),
            "doctor": (
                "uv run --with twscrape python ~/.agents/skills/techsearch/tools/x_search.py doctor"
            ),
            "db_path": str(db_path),
        }
    )
    raise SystemExit(2)


async def main() -> int:
    parser = argparse.ArgumentParser(
        description="Read-only X search helper for the techsearch skill."
    )
    parser.add_argument(
        "--db",
        default=str(DEFAULT_DB),
        help="Path to the twscrape accounts DB. Default: %(default)s",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    subparsers.add_parser("doctor", help="Show expected DB path and setup guidance")

    search_parser = subparsers.add_parser("search", help="Run an X search query")
    search_parser.add_argument("--query", required=True, help="X search query")
    search_parser.add_argument(
        "--limit", type=int, default=10, help="Desired result count"
    )
    search_parser.add_argument(
        "--product",
        choices=["Latest", "Top", "Media"],
        default="Latest",
        help="X product tab to query",
    )

    user_parser = subparsers.add_parser("user", help="Fetch a user profile by login")
    user_parser.add_argument("--login", required=True, help="X login without @")

    args = parser.parse_args()
    db_path = Path(args.db).expanduser()

    if args.command == "doctor":
        print_json(doctor_payload(db_path))
        return 0

    require_db(db_path)

    if args.command == "search":
        try:
            results = await run_search(db_path, args.query, args.limit, args.product)
        except Exception as exc:
            print_json(
                error_payload(
                    "search_failed",
                    "X search failed. Refresh auth or fall back to Playwright.",
                    detail=str(exc),
                    db_path=str(db_path),
                    query=args.query,
                    product=args.product,
                )
            )
            return 3
        print_json({"query": args.query, "product": args.product, "results": results})
        return 0

    if args.command == "user":
        try:
            result = await run_user_by_login(db_path, args.login)
        except LookupError as exc:
            print_json(
                error_payload(
                    "user_not_found",
                    "No X profile matched that login.",
                    detail=str(exc),
                    login=args.login,
                )
            )
            return 4
        except Exception as exc:
            print_json(
                error_payload(
                    "user_lookup_failed",
                    "X profile lookup failed. Refresh auth or fall back to Playwright.",
                    detail=str(exc),
                    db_path=str(db_path),
                    login=args.login,
                )
            )
            return 3
        print_json(result)
        return 0

    parser.error(f"unsupported command: {args.command}")
    return 1


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
