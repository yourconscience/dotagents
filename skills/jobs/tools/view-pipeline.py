#!/usr/bin/env python3
# /// script
# dependencies = ["pyyaml"]
# ///
"""Render opportunities.yaml as a readable markdown pipeline view.

Usage:
  uv run view-pipeline.py          # print to stdout
  uv run view-pipeline.py --open   # write to /tmp/pipeline.md and open in cmux
"""
import subprocess
import sys
from pathlib import Path
import yaml

DATA = Path(__file__).resolve().parent.parent / "data" / "opportunities.yaml"
OUT = Path("/tmp/pipeline.md")


def render():
    entries = yaml.safe_load(DATA.read_text()) or []

    needs_action = [e for e in entries if e.get("stage") == "needs-action"]
    active = [e for e in entries if e.get("stage") == "active"]
    new = [e for e in entries if e.get("stage") == "new"]
    waiting = [e for e in entries if e.get("stage") == "waiting"]
    low = [e for e in entries if e.get("stage") == "low-priority"]
    closed = [e for e in entries if e.get("stage") == "closed"]

    lines = []
    p = lines.append

    p("# Job Pipeline\n")

    if needs_action:
        p("## Needs Action\n")
        for e in needs_action:
            p(f"### {e.get('company', '?')} - {e.get('role') or 'TBD'}")
            _detail(p, e)
        p("")

    if active:
        p("## Active Interviews\n")
        for e in active:
            p(f"### {e.get('company', '?')} - {e.get('role') or 'TBD'}")
            _detail(p, e)
        p("")

    if waiting:
        p("## Waiting for Response\n")
        for e in waiting:
            company = e.get("company", "?")
            role = e.get("role") or "TBD"
            na = e.get("next_action") or ""
            last = e.get("last_contact_date") or "?"
            p(f"- **{company}** ({role}) - last contact {last}. {na}")
        p("")

    if new:
        p("## Not Yet Applied\n")
        p("| Company | Role | Location | Next Step |")
        p("|---|---|---|---|")
        for e in new:
            company = e.get("company", "?")
            role = (e.get("role") or "TBD")[:45]
            loc = e.get("location") or ("Remote" if e.get("remote") else "?")
            na = (e.get("next_action") or "")[:70]
            p(f"| {company} | {role} | {loc} | {na} |")
        p("")

    if low:
        p("## Low Priority\n")
        for e in low:
            company = e.get("company", "?")
            role = (e.get("role") or "TBD")[:50]
            reason = ""
            rf = e.get("red_flag")
            if rf:
                reason = f" ({rf})"
            p(f"- **{company}**: {role}{reason}")
        p("")

    p("---\n")
    p(f"Active: {len(active)} | Needs action: {len(needs_action)} | "
      f"Waiting: {len(waiting)} | New: {len(new)} | "
      f"Low: {len(low)} | Closed: {len(closed)}")

    return "\n".join(lines)


def _detail(p, e):
    loc = e.get("location") or ("Remote" if e.get("remote") else "?")
    comp = e.get("comp") or ""
    contact = e.get("contact") or ""
    last = e.get("last_contact_date") or ""
    na = e.get("next_action") or ""
    due = e.get("next_action_due") or ""

    parts = [f"**Location:** {loc}"]
    if comp:
        parts.append(f"**Comp:** {comp}")
    if contact:
        parts.append(f"**Contact:** {contact.split(';')[0].strip()}")
    if last:
        parts.append(f"**Last contact:** {last}")
    p(f"  {' | '.join(parts)}")

    if na:
        due_str = f" (due {due})" if due else ""
        p(f"  **Next:** {na}{due_str}")
    p("")


if __name__ == "__main__":
    md = render()
    if "--open" in sys.argv:
        OUT.write_text(md)
        subprocess.run(["cmux", "markdown", "open", str(OUT)])
    else:
        print(md)
