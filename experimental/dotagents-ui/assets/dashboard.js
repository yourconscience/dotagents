const state = { filter: "all", query: "", cards: [] };

function text(value) {
  return value === undefined || value === null || value === "" ? "-" : String(value);
}

function node(tag, className, value) {
  const el = document.createElement(tag);
  if (className) el.className = className;
  if (value !== undefined) el.textContent = value;
  return el;
}

function metric(label, value) {
  const el = node("div", "metric");
  el.append(node("strong", "", text(value)));
  el.append(node("span", "meta", label));
  return el;
}

function item(kind, title, description, meta, commands) {
  const copyItems = Array.isArray(commands) ? commands.filter(Boolean) : (commands ? [commands] : []);
  const el = node("article", "item");
  el.dataset.kind = kind;
  el.dataset.search = [kind, title, description, meta].concat(copyItems).filter(Boolean).join(" ").toLowerCase();
  const header = node("header");
  header.append(node("h2", "", text(title)));
  header.append(node("span", "badge", kind));
  el.append(header);
  if (description) el.append(node("p", "", description));
  if (meta) el.append(node("div", "path", meta));
  for (const command of copyItems) {
    const row = node("div", "copy-row");
    row.append(node("code", "", command));
    const button = node("button", "", "Copy");
    button.type = "button";
    button.dataset.copy = command;
    button.addEventListener("click", copyText);
    row.append(button);
    el.append(row);
  }
  return el;
}

async function copyText(event) {
  const button = event.currentTarget;
  const value = button.dataset.copy || "";
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(value);
    } else {
      const area = document.createElement("textarea");
      area.value = value;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.append(area);
      area.select();
      const copied = document.execCommand("copy");
      area.remove();
      if (!copied) throw new Error("copy command failed");
    }
    button.textContent = "Copied";
    setTimeout(() => { button.textContent = "Copy"; }, 2000);
  } catch (error) {
    button.textContent = "Copy failed";
    setTimeout(() => { button.textContent = "Copy"; }, 2000);
  }
}

function applyCatalogFilters() {
  const query = state.query.trim().toLowerCase();
  for (const card of state.cards) {
    const kindMatch = state.filter === "all" || card.dataset.kind === state.filter;
    const queryMatch = query === "" || card.dataset.search.includes(query);
    card.classList.toggle("hidden", !(kindMatch && queryMatch));
  }
}

function track(container, cards) {
  container.replaceChildren(...cards);
  state.cards.push(...cards);
}

async function loadJSON(path) {
  const response = await fetch(path);
  if (!response.ok) throw new Error(path + " returned " + response.status);
  return response.json();
}

async function render() {
  const [overview, skills, agents, mcps, hooks, examples] = await Promise.all([
    loadJSON("/api/overview"),
    loadJSON("/api/skills"),
    loadJSON("/api/agents"),
    loadJSON("/api/mcps"),
    loadJSON("/api/hooks"),
    loadJSON("/api/examples")
  ]);
  state.overview = overview;

  const metrics = document.getElementById("overview-metrics");
  metrics.replaceChildren(
    metric("Branch", overview.branch),
    metric("Commit", overview.commit),
    metric("Managed", overview.sync.managed),
    metric("Missing", overview.sync.missing),
    metric("Drifted", overview.sync.drifted),
    metric("Conflicts", overview.sync.conflicts),
    metric("MCPs", overview.mcp_count),
    metric("Hooks", overview.hook_count)
  );

  state.cards = [];
  document.getElementById("agent-status").replaceChildren(...(overview.agents || []).map((agent) =>
    item("status", agent.name, agent.detected ? "Detected" : "Binary not detected", agent.skill_root, agent.synced ? "synced" : "needs sync")
  ));
  track(document.getElementById("skills-list"), (skills || []).map((skill) =>
    item("skill", skill.name, skill.description, skill.path, skill.command)
  ));
  track(document.getElementById("agents-list"), (agents || []).map((agent) =>
    item("agent", agent.name, agent.description, [agent.path, [agent.model, agent.effort].filter(Boolean).join(" / ")].filter(Boolean).join(" | "))
  ));
  track(document.getElementById("mcps-list"), (mcps || []).map((mcp) =>
    item("mcp", mcp.name, (mcp.agents || []).join(", "), mcp.path, mcp.command + " " + (mcp.args || []).join(" "))
  ));
  track(document.getElementById("hooks-list"), (hooks || []).map((hook) =>
    item("hook", hook.name, "memory hook", hook.path)
  ));

  const exampleState = document.getElementById("examples-state");
  const warnings = examples.warnings || [];
  if (warnings.length > 0) {
    exampleState.textContent = "Loaded with " + warnings.length + " example warning(s).";
    exampleState.className = "warn";
  } else if ((examples.examples || []).length === 0) {
    exampleState.textContent = "No examples configured yet.";
  } else {
    exampleState.textContent = "Loaded " + examples.examples.length + " example card(s).";
  }
  track(document.getElementById("examples-list"), (examples.examples || []).map((example) =>
    item("example", example.title || example.name, example.description, example.path, [example.command, example.prompt])
  ));
  applyCatalogFilters();
}

document.getElementById("catalog-search").addEventListener("input", (event) => {
  state.query = event.target.value;
  applyCatalogFilters();
});
document.getElementById("catalog-filters").addEventListener("click", (event) => {
  const button = event.target.closest("button[data-filter]");
  if (!button) return;
  state.filter = button.dataset.filter;
  for (const current of document.querySelectorAll("#catalog-filters button")) {
    current.setAttribute("aria-pressed", String(current === button));
  }
  applyCatalogFilters();
});

render().catch((error) => {
  document.getElementById("overview-metrics").replaceChildren(node("p", "warn", error.message));
});
