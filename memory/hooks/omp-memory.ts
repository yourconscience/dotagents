// dotagents OMP/Pi memory extension.
//
// OMP/Pi exposes no session-end hook of its own, so this extension captures a
// basic_memory digest on `agent_end` by piping an OMP-tagged payload into the
// shared session-end dispatch (memory/hooks/session-end.sh), which classifies
// `agent: "omp"` and writes the same local digest Claude and Codex use.
//
// Manual wiring (dotagents does not install OMP extensions automatically):
//   cp ~/.agents/memory/hooks/omp-memory.ts ~/.omp/agent/extensions/
// Override the hook location with DOTAGENTS_MEMORY_HOOKS if your dotagents
// checkout is not at ~/.agents.
//
// Reminder: automatic digests are a net, not a replacement. Capture deliberate
// cross-harness facts explicitly with:  rem add -src omp "<fact>"
import { spawn } from "node:child_process";
import * as os from "node:os";
import * as path from "node:path";
import type { AgentEndEvent, ExtensionAPI, ExtensionContext } from "@oh-my-pi/pi-coding-agent";

function hooksDir(): string {
  return process.env.DOTAGENTS_MEMORY_HOOKS || path.join(os.homedir(), ".agents", "memory", "hooks");
}

function digestMessages(event: AgentEndEvent): Array<{ role: string; content: unknown }> {
  const messages: Array<{ role: string; content: unknown }> = [];
  for (const message of event.messages || []) {
    if (!message || typeof message !== "object") continue;
    const typed = message as { role?: unknown; content?: unknown };
    if (typeof typed.role !== "string" || typed.content == null) continue;
    messages.push({ role: typed.role, content: typed.content });
  }
  return messages;
}

function captureDigest(ctx: ExtensionContext, event: AgentEndEvent): void {
  const sessionId = ctx.sessionManager.getSessionId();
  if (!sessionId) return;
  const cwd = ctx.cwd || process.cwd();
  const payload = JSON.stringify({
    agent: "omp",
    hook_event_name: "SessionEnd",
    session_id: sessionId,
    cwd,
    messages: digestMessages(event),
  });
  try {
    const child = spawn(path.join(hooksDir(), "session-end.sh"), [], {
      env: { ...process.env, DOTAGENTS_MEMORY_SOURCE: "omp" },
      stdio: ["pipe", "ignore", "ignore"],
      detached: true,
    });
    child.on("error", () => {});
    child.unref();
    child.stdin.on("error", () => {});
    child.stdin.end(payload);
  } catch (_) {
    // Best-effort: never let memory capture break the session.
  }
}

export default function dotagentsOmpMemoryExtension(api: ExtensionAPI) {
  api.on("agent_end", async (event, ctx) => {
    captureDigest(ctx, event);
  });
}
