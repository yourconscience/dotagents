const $ = (selector) => document.querySelector(selector);
let layer = 'shared';
let state = null;
let plan = null;
const pendingOperations = new Map();
const baseURL = new URL(window.location.pathname.endsWith('/') ? window.location.pathname : `${window.location.pathname}/`, window.location.origin);

function csrf() {
  return document.cookie.split('; ').find((item) => item.startsWith('dotagents_csrf='))?.split('=')[1] || '';
}
function setStatus(message, kind = '') {
  const node = $('#status'); node.textContent = message; node.className = `status ${kind}`;
}
async function api(path, options = {}) {
  const headers = {'Accept':'application/json', ...(options.body ? {'Content-Type':'application/json'} : {}), ...(options.method && options.method !== 'GET' ? {'X-Dotagents-CSRF':csrf()} : {})};
  const response = await fetch(new URL(path.replace(/^\//, ''), baseURL), {...options, headers});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error?.message || `request failed (${response.status})`);
  return body;
}
function pick(object, ...keys) { for (const key of keys) if (object && object[key] !== undefined) return object[key]; return undefined; }
function renderLinks(ui) {
  const links = pick(ui, 'Links','links') || [];
  $('#links').replaceChildren(...links.map((link) => {
    const anchor = document.createElement('a');
    anchor.textContent = pick(link,'Name','name');
    anchor.href = pick(link,'URL','url');
    anchor.target = '_top';
    return anchor;
  }));
}
function toggle(path, value, labelText) {
  const label = document.createElement('label');
  label.className = 'toggle';
  const input = document.createElement('input');
  input.type = 'checkbox';
  input.checked = !!value;
  input.dataset.editPath = path;
  input.disabled = state.read_only;
  label.append(input, document.createTextNode(labelText));
  return label;
}
function row(keyLabel, description, controls) {
  const div = document.createElement('div');
  div.className = 'ledger-row';
  const summary = document.createElement('div');
  const key = document.createElement('strong');
  key.className = 'key';
  key.textContent = keyLabel;
  const detail = document.createElement('small');
  detail.textContent = description;
  summary.append(key, detail);
  const choices = document.createElement('div');
  choices.className = 'choices';
  choices.append(...controls);
  div.append(summary, choices);
  return div;
}
function section(label) {
  const heading = document.createElement('h2');
  heading.className = 'ledger-section';
  heading.textContent = label;
  return heading;
}
function renderStructured(config) {
  const agents = pick(config, 'Agents','agents') || [];
  const servers = pick(config, 'MCPServers','mcp_servers') || [];
  const hooks = pick(config, 'Hooks','hooks') || [];
  const links = pick(pick(config, 'UI','ui'), 'Links','links') || [];
  const rows = [];
  if (agents.length) rows.push(section('Agents'));
  for (const agent of agents) {
    const name = pick(agent,'Name','name');
    const roots = [pick(agent,'SkillRoot','skill_root'), pick(agent,'AgentRoot','agent_root')].filter(Boolean).join(' · ');
    rows.push(row(name, roots, [toggle(`/agents/${name}/enabled`, pick(agent,'Enabled','enabled'), 'Enabled')]));
  }
  if (servers.length) rows.push(section('MCP servers'));
  for (const server of servers) {
    const name = pick(server,'Name','name');
    const targets = pick(server,'Agents','agents') || [];
    rows.push(row(name, targets.length ? `Targets: ${targets.join(', ')}` : 'No targets', [toggle(`/mcp_servers/${name}/enabled`, pick(server,'Enabled','enabled'), 'Enabled')]));
  }
  if (hooks.length) rows.push(section('Hooks'));
  for (const hook of hooks) {
    const name = pick(hook,'Name','name');
    const event = pick(hook,'Event','event');
    rows.push(row(name, event ? `Event: ${event}` : 'No event', [toggle(`/hooks/${name}/enabled`, pick(hook,'Enabled','enabled'), 'Enabled')]));
  }
  if (links.length) rows.push(section('Navigation'));
  links.forEach((link) => rows.push(row(pick(link,'Name','name'), pick(link,'URL','url'), [])));
  if (!rows.length) {
    const empty = document.createElement('p');
    empty.className = 'empty';
    empty.textContent = 'No selectable settings in this layer.';
    rows.push(empty);
  }
  const ledger = $('#structured');
  ledger.replaceChildren(...rows);
  ledger.querySelectorAll('[data-edit-path]').forEach((input) => input.addEventListener('change', () => stageStructuredEdit(input)));
}
async function stageStructuredEdit(input) {
  pendingOperations.set(input.dataset.editPath, {op:'set', path:input.dataset.editPath, value:input.checked});
  try {
    const result = await api('/api/config/validate', {method:'POST', body:JSON.stringify({layer, operations:[...pendingOperations.values()]})});
    $('#diff').textContent = result.diff || '(no changes)';
    setStatus('Change staged. Review the diff, then save.', 'ok');
  } catch (error) {
    pendingOperations.delete(input.dataset.editPath);
    setStatus(error.message, 'error');
  }
}
function render() {
  const config = state.typed_config;
  $('#heading').textContent = layer[0].toUpperCase() + layer.slice(1) + (layer === 'effective' ? ' merge' : ' configuration');
  renderStructured(config);
  renderLinks(state.effective_ui);
  $('#source-meta').textContent = state.paths[layer === 'effective' ? 'shared' : layer] || '';
  $('#revision').textContent = state.revision ? state.revision.slice(0, 12) : '';
  $('#save').disabled = state.read_only;
  $('#msave').disabled = state.read_only;
}
async function load(nextLayer = layer) {
  layer = nextLayer;
  pendingOperations.clear();
  document.querySelectorAll('.source').forEach((node) => node.classList.toggle('active', node.dataset.layer === layer));
  try { state = await api(`/api/state?layer=${encodeURIComponent(layer)}`); render(); setStatus(state.read_only ? 'Effective merge is read-only.' : 'Loaded canonical YAML.'); }
  catch (error) { setStatus(error.message, 'error'); }
}
async function validate() {
  try {
    const result = await api('/api/config/validate', {method:'POST', body:JSON.stringify({layer, operations:[...pendingOperations.values()]})});
    $('#diff').textContent = result.diff || '(no changes)';
    setStatus('Selected settings are valid.', 'ok');
  } catch (error) { setStatus(error.message, 'error'); }
}
async function review() {
  await validate();
}
async function save() {
  if (!pendingOperations.size) { setStatus('No changes to save.'); return; }
  try {
    const result = await api('/api/config', {method:'PATCH', body:JSON.stringify({layer, expected_revision:state.revision, operations:[...pendingOperations.values()]})});
    $('#diff').textContent = result.diff || '(no changes)';
    await load(layer);
    setStatus('Saved canonical configuration. Sync remains separate.', 'ok');
  } catch (error) { setStatus(error.message, 'error'); }
}
async function previewSync() {
  try { const result = await api('/api/sync/preview', {method:'POST', body:'{}'}); plan = result; $('#plan').textContent = JSON.stringify(result.plan, null, 2); $('#apply').disabled = false; setStatus(`Sync preview ready: ${result.digest.slice(0,12)}.`, 'ok'); }
  catch (error) { setStatus(error.message, 'error'); }
}
async function applySync() {
  if (!plan || !confirm('Apply this sync plan to native harnesses?')) return;
  try { await api('/api/sync/apply', {method:'POST', body:JSON.stringify({expected_revision:plan.revision, plan_digest:plan.digest, confirmed_destructive:plan.plan.destructive || []})}); setStatus('Sync applied.', 'ok'); $('#apply').disabled = true; }
  catch (error) { setStatus(error.message, 'error'); }
}
document.querySelectorAll('.source').forEach((node) => node.addEventListener('click', () => load(node.dataset.layer)));
$('#validate').addEventListener('click', validate); $('#mvalidate').addEventListener('click', validate);
$('#review').addEventListener('click', review); $('#mreview').addEventListener('click', review);
$('#save').addEventListener('click', save); $('#msave').addEventListener('click', save);
$('#preview').addEventListener('click', previewSync); $('#apply').addEventListener('click', applySync);
$('#settings').addEventListener('click', () => { layer = 'local'; load('local'); });
load();
