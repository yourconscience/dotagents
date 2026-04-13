#!/usr/bin/env python3
"""
TickTick CLI - Read-only task and project listing.

Uses pyticktick V1 API (official OAuth2). V2 features are not used.

Usage:
    uv run --with pyticktick python tt.py projects
    uv run --with pyticktick python tt.py tasks [--project NAME] [--json]
    uv run --with pyticktick python tt.py search "keyword"
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import warnings

warnings.filterwarnings("ignore")

try:
    from loguru import logger as _loguru_logger
    _loguru_logger.remove()
except ImportError:
    pass


def make_client():
    """Create pyticktick Client from TICKTICK_* env vars (V1-only)."""
    from pyticktick import Client

    client_id = os.environ.get("TICKTICK_CLIENT_ID")
    client_secret = os.environ.get("TICKTICK_CLIENT_SECRET")
    access_token = os.environ.get("TICKTICK_ACCESS_TOKEN")

    missing = [k for k, v in {
        "TICKTICK_CLIENT_ID": client_id,
        "TICKTICK_CLIENT_SECRET": client_secret,
        "TICKTICK_ACCESS_TOKEN": access_token,
    }.items() if not v]
    if missing:
        print(f"Missing env vars: {', '.join(missing)}", file=sys.stderr)
        sys.exit(2)

    # TickTick enforces token expiration server-side. Pass a far-future value
    # to satisfy pyticktick's local validation. Run `ticktick-sdk auth` or
    # pyticktick signon to refresh when the server rejects the token.
    far_future = 4102444800  # 2100-01-01

    return Client(
        v1_client_id=client_id,
        v1_client_secret=client_secret,
        v1_token={"value": access_token, "expiration": far_future},
    )


def list_projects(output_format: str) -> None:
    client = make_client()
    projects = client.get_projects_v1().root

    if output_format == "json":
        out = [{"id": p.id, "name": p.name, "kind": p.kind, "color": p.color} for p in projects]
        print(json.dumps(out, ensure_ascii=False, indent=2))
        return

    print(f"{'ID':<26} {'Name':<30} {'Kind':<10}")
    print("-" * 70)
    for p in projects:
        print(f"{p.id:<26} {p.name:<30} {p.kind or '':<10}")
    print(f"\n{len(projects)} projects", file=sys.stderr)


def _iter_project_tasks(client, project_filter: str | None):
    """Yield (project, task) pairs for projects matching filter."""
    projects = client.get_projects_v1().root
    if project_filter:
        projects = [p for p in projects if project_filter.lower() in p.name.lower()]
    for project in projects:
        try:
            data = client.get_project_with_data_v1(project.id)
        except Exception as e:
            print(f"Warning: failed to fetch {project.name}: {e}", file=sys.stderr)
            continue
        for task in data.tasks:
            yield project, task


def _task_dict(project, task) -> dict:
    return {
        "id": task.id,
        "title": task.title,
        "status": "completed" if task.status else "incomplete",
        "priority": task.priority,
        "project": project.name,
        "project_id": project.id,
        "content": task.content or "",
        "due_date": task.due_date,
    }


def _print_task_table(results: list) -> None:
    print(f"{'Status':<6} {'Pri':<3} {'Due':<10} {'Project':<15} {'Title'}")
    print("-" * 90)
    for project, task in results:
        status = "[x]" if task.status else "[ ]"
        pri = {1: "!", 3: "!!", 5: "!!!"}.get(task.priority, "")
        due = str(task.due_date)[:10] if task.due_date else "-"
        print(f"{status:<6} {pri:<3} {due:<10} {project.name[:14]:<15} {task.title}")


def _sort_results(results: list) -> list:
    return sorted(results, key=lambda x: (str(x[1].due_date or "9999-12-31"), x[1].title))


def list_tasks(project_filter: str | None, status_filter: str | None, output_format: str) -> None:
    client = make_client()
    results = list(_iter_project_tasks(client, project_filter))

    if status_filter == "incomplete":
        results = [(p, t) for p, t in results if not t.status]
    elif status_filter == "completed":
        results = [(p, t) for p, t in results if t.status]

    results = _sort_results(results)

    if output_format == "json":
        print(json.dumps([_task_dict(p, t) for p, t in results], ensure_ascii=False, indent=2))
        return

    _print_task_table(results)
    print(f"\n{len(results)} tasks", file=sys.stderr)


def search_tasks(query: str, output_format: str) -> None:
    client = make_client()
    query_lower = query.lower()
    matches = [
        (p, t) for p, t in _iter_project_tasks(client, None)
        if query_lower in t.title.lower() or query_lower in (t.content or "").lower()
    ]
    matches = _sort_results(matches)

    if output_format == "json":
        print(json.dumps([_task_dict(p, t) for p, t in matches], ensure_ascii=False, indent=2))
        return

    _print_task_table(matches)
    print(f"\n{len(matches)} matches for '{query}'", file=sys.stderr)


def main() -> int:
    parser = argparse.ArgumentParser(prog="tt", description="TickTick CLI - list tasks and projects")
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    sub = parser.add_subparsers(dest="command", required=True)

    tasks_p = sub.add_parser("tasks", help="List tasks")
    tasks_p.add_argument("--project", type=str, help="Filter by project name substring")
    tasks_p.add_argument("--status", choices=["incomplete", "completed"], help="Filter by status")

    sub.add_parser("projects", help="List projects")

    search_p = sub.add_parser("search", help="Search tasks by keyword")
    search_p.add_argument("query", type=str)

    args = parser.parse_args()
    fmt = "json" if args.json else "table"

    try:
        if args.command == "projects":
            list_projects(fmt)
        elif args.command == "tasks":
            list_tasks(args.project, args.status, fmt)
        elif args.command == "search":
            search_tasks(args.query, fmt)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
