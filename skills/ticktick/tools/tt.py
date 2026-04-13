#!/usr/bin/env python3
"""
TickTick CLI - Simple task listing tool.

Usage:
    uv run --with ~/Public/ticktick-sdk python tt.py tasks
    uv run --with ~/Public/ticktick-sdk python tt.py projects
    uv run --with ~/Public/ticktick-sdk python tt.py search "keyword"
"""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
from datetime import datetime, timedelta
from typing import Any


def format_date(dt: datetime | str | None) -> str:
    """Format datetime for display."""
    if dt is None:
        return "-"
    if isinstance(dt, str):
        try:
            dt = datetime.fromisoformat(dt.replace("Z", "+00:00"))
        except ValueError:
            return dt[:10] if len(dt) >= 10 else dt
    return dt.strftime("%Y-%m-%d")


def format_task_line(task: Any, show_project: bool = True) -> str:
    """Format a task as a single line."""
    status = "[x]" if task.status == 2 else "[ ]"
    priority_map = {0: " ", 1: "!", 3: "!!", 5: "!!!"}
    priority = priority_map.get(task.priority, " ")
    due = format_date(getattr(task, "due_date", None))
    project = f"({task.project_id[:8]})" if show_project and task.project_id else ""
    return f"{status} {priority:3} {due:10} {task.title} {project}"


async def list_tasks(
    client: Any,
    project_id: str | None = None,
    project_name: str | None = None,
    status: str | None = None,
    kind: str | None = None,
    output_format: str = "table",
) -> None:
    """List tasks with optional filters."""
    from ticktick_sdk import TaskStatus, TaskKind

    # Get all tasks
    tasks = await client.get_all_tasks()

    # Filter by project name if specified
    if project_name:
        projects = await client.list_projects()
        matching = [p for p in projects if project_name.lower() in p.name.lower()]
        if not matching:
            print(f"No project matching '{project_name}'", file=sys.stderr)
            return
        project_ids = {p.id for p in matching}
        tasks = [t for t in tasks if t.project_id in project_ids]

    # Filter by project_id
    if project_id:
        tasks = [t for t in tasks if t.project_id == project_id]

    # Filter by status
    if status == "incomplete":
        tasks = [t for t in tasks if t.status != TaskStatus.COMPLETED]
    elif status == "completed":
        tasks = [t for t in tasks if t.status == TaskStatus.COMPLETED]

    # Filter by kind (task vs note)
    if kind == "note":
        tasks = [t for t in tasks if getattr(t, "kind", None) == TaskKind.NOTE]
    elif kind == "task":
        tasks = [t for t in tasks if getattr(t, "kind", None) != TaskKind.NOTE]

    # Sort by due date
    tasks.sort(key=lambda t: (str(getattr(t, "due_date", None) or "9999-12-31"), t.title))

    if output_format == "json":
        output = []
        for t in tasks:
            output.append({
                "id": t.id,
                "title": t.title,
                "status": t.status,
                "priority": t.priority,
                "due_date": str(getattr(t, "due_date", None)),
                "project_id": t.project_id,
                "tags": getattr(t, "tags", []),
                "content": getattr(t, "content", ""),
            })
        print(json.dumps(output, indent=2, ensure_ascii=False))
    elif output_format == "markdown":
        print("| Status | Pri | Due        | Title | Project |")
        print("|--------|-----|------------|-------|---------|")
        for t in tasks:
            status_mark = "x" if t.status == 2 else " "
            pri = {1: "!", 3: "!!", 5: "!!!"}.get(t.priority, "")
            due = format_date(getattr(t, "due_date", None))
            print(f"| [{status_mark}] | {pri:3} | {due} | {t.title} | {t.project_id[:8] if t.project_id else ''} |")
    else:
        # Table format
        print(f"{'Status':<6} {'Pri':<3} {'Due':<10} {'Title':<50} {'Project':<10}")
        print("-" * 85)
        for t in tasks:
            print(format_task_line(t))

    print(f"\n{len(tasks)} tasks", file=sys.stderr)


async def list_projects(client: Any, output_format: str = "table") -> None:
    """List all projects."""
    projects = await client.list_projects()

    if output_format == "json":
        output = []
        for p in projects:
            output.append({
                "id": p.id,
                "name": p.name,
                "kind": str(getattr(p, "kind", "")),
                "color": getattr(p, "color", ""),
            })
        print(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        print(f"{'ID':<24} {'Name':<30} {'Kind':<10}")
        print("-" * 70)
        for p in projects:
            kind = str(getattr(p, "kind", ""))
            print(f"{p.id:<24} {p.name:<30} {kind:<10}")

    print(f"\n{len(projects)} projects", file=sys.stderr)


async def search_tasks(client: Any, query: str, output_format: str = "table") -> None:
    """Search tasks by keyword."""
    tasks = await client.get_all_tasks()

    # Simple keyword search in title and content
    query_lower = query.lower()
    matches = [
        t for t in tasks
        if query_lower in t.title.lower()
        or query_lower in (getattr(t, "content", "") or "").lower()
    ]

    if output_format == "json":
        output = []
        for t in matches:
            output.append({
                "id": t.id,
                "title": t.title,
                "status": t.status,
                "due_date": str(getattr(t, "due_date", None)),
                "project_id": t.project_id,
                "content": getattr(t, "content", ""),
            })
        print(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        print(f"{'Status':<6} {'Pri':<3} {'Due':<10} {'Title':<50}")
        print("-" * 75)
        for t in matches:
            print(format_task_line(t, show_project=False))

    print(f"\n{len(matches)} matches for '{query}'", file=sys.stderr)


async def main_async(args: argparse.Namespace) -> int:
    """Async main entry point."""
    from ticktick_sdk import TickTickClient

    try:
        async with TickTickClient.from_settings() as client:
            if args.command == "tasks":
                await list_tasks(
                    client,
                    project_id=args.project_id,
                    project_name=args.project,
                    status=args.status,
                    kind=args.kind,
                    output_format="json" if args.json else "markdown" if args.markdown else "table",
                )
            elif args.command == "projects":
                await list_projects(
                    client,
                    output_format="json" if args.json else "table",
                )
            elif args.command == "search":
                await search_tasks(
                    client,
                    args.query,
                    output_format="json" if args.json else "table",
                )
            else:
                print(f"Unknown command: {args.command}", file=sys.stderr)
                return 1
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

    return 0


def main() -> int:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        prog="tt",
        description="TickTick CLI - list tasks and projects",
    )

    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--markdown", action="store_true", help="Output as Markdown table")

    subparsers = parser.add_subparsers(dest="command", required=True)

    # tasks command
    tasks_parser = subparsers.add_parser("tasks", help="List tasks")
    tasks_parser.add_argument("--project", type=str, help="Filter by project name")
    tasks_parser.add_argument("--project-id", type=str, help="Filter by project ID")
    tasks_parser.add_argument("--status", choices=["incomplete", "completed"], help="Filter by status")
    tasks_parser.add_argument("--kind", choices=["task", "note"], help="Filter by kind")

    # projects command
    subparsers.add_parser("projects", help="List projects")

    # search command
    search_parser = subparsers.add_parser("search", help="Search tasks")
    search_parser.add_argument("query", type=str, help="Search query")

    args = parser.parse_args()
    return asyncio.run(main_async(args))


if __name__ == "__main__":
    sys.exit(main())
