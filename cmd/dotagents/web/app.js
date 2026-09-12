const $ = (selector) => document.querySelector(selector);
const yaml = $('#yaml');
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
function esc(value) { return String(value ?? '').replace(/[&<>"']/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char])); }
function renderLinks(ui) {
  const links = pick(ui, 'Links','links') || [];
  $('#links').replaceChildren(...links.map((link) => { const a = document.createElement('a'); a.textContent = pick(link,'Name','name'); a.href = pick(link,'URL','url'); a.target = '_top'; return a; }));
}
function field(path, value, kind = 'text') {
  const disabled = state.read_only ? 'disabled' : '';
  if (kind === 'checkbox') return `<input type="checkbox" data-edit-path="${esc(path)}" data-edit-kind="checkbox" ${value ? 'checked' : ''} ${disabled}>`;
  return `<input type="${kind}" data-edit-path="${esc(path)}" value="${esc(value)}" ${disabled}>`;
}
function renderStructured(config) {
  const agents = pick(config, 'Agents','agents') || [];
  const servers = pick(config, 'MCPServers','mcp_servers') || [];
  const hooks = pick(config, 'Hooks','hooks') || [];
  const links = pick(pick(config, 'UI','ui'), 'Links','links') || [];
  const rows = [];
  rows.push(`<div class="ledger-row"><span class="key">version</span><span class="value">${field('/version', pick(config,'Version','version'), 'number')}</span><small>shared schema</small></div>`);
  for (const agent of agents) {
    const name = pick(agent,'Name','name');
    rows.push(`<div class="ledger-row"><span class="key">agent · ${esc(name)}</span><span class="value">${field(`/agents/${name}/skill_root`, pick(agent,'SkillRoot','skill_root') || '')}</span><small>${field(`/agents/${name}/enabled`, !!pick(agent,'Enabled','enabled'), 'checkbox')} enabled · skill root</small></div>`);
    rows.push(`<div class="ledger-row subrow"><span class="key">agent root</span><span class="value">${field(`/agents/${name}/agent_root`, pick(agent,'AgentRoot','agent_root') || '')}</span><small>${field(`/agents/${name}/role_model`, pick(agent,'RoleModel','role_model') || '')}</small></div>`);
  }
  for (const server of servers) {
    const name = pick(server,'Name','name');
    rows.push(`<div class="ledger-row"><span class="key">MCP · ${esc(name)}</span><span class="value">${field(`/mcp_servers/${name}/command`, pick(server,'Command','command') || '')}</span><small>${field(`/mcp_servers/${name}/enabled`, !!pick(server,'Enabled','enabled'), 'checkbox')} enabled · command</small></div>`);
  }
  for (const hook of hooks) {
    const name = pick(hook,'Name','name');
    rows.push(`<div class="ledger-row"><span class="key">hook · ${esc(name)}</span><span class="value">${field(`/hooks/${name}/command`, pick(hook,'Command','command') || '')}</span><small>${field(`/hooks/${name}/enabled`, !!pick(hook,'Enabled','enabled'), 'checkbox')} enabled · ${field(`/hooks/${name}/event`, pick(hook,'Event','event') || '')}</small></div>`);
  }
  links.forEach((link, index) => rows.push(`<div class="ledger-row"><span class="key">link · ${field(`/ui/links/${index}/name`, pick(link,'Name','name') || '')}</span><span class="value">${field(`/ui/links/${index}/url`, pick(link,'URL','url') || '')}</span><small>navigation</small></div>`));
  const ledger = $('#structured'); const template = document.createElement('template');
  template.innerHTML = rows.join('');
  ledger.replaceChildren(...template.content.children);
  $('#structured').querySelectorAll('[data-edit-path]').forEach((input) => input.addEventListener('change', () => stageStructuredEdit(input)));
}
async function stageStructuredEdit(input) {
  const value = input.dataset.editKind === 'checkbox' ? input.checked : (input.type === 'number' ? Number(input.value) : input.value);
  pendingOperations.set(input.dataset.editPath, {op:'set', path:input.dataset.editPath, value});
  try {
    const result = await api('/api/config/validate', {method:'POST', body:JSON.stringify({layer, operations:[...pendingOperations.values()]})});
    yaml.value = result.raw_yaml;
    $('#diff').textContent = result.diff || '(no changes)';
    setStatus('Change staged. Review the YAML diff, then save.', 'ok');
  } catch (error) {
    pendingOperations.delete(input.dataset.editPath);
    setStatus(error.message, 'error');
  }
}
function render() {
  const config = state.typed_config;
  $('#heading').textContent = layer[0].toUpperCase() + layer.slice(1) + (layer === 'effective' ? ' merge' : ' YAML');
  renderStructured(config);
  renderLinks(state.effective_ui);
  $('#source-meta').textContent = state.paths[layer === 'effective' ? 'shared' : layer] || '';
  yaml.value = state.raw_yaml || '';
  yaml.readOnly = state.read_only;
  renderLinks(state.effective_ui);
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
  try { const result = await api('/api/config/validate', {method:'POST', body:JSON.stringify({layer, raw_yaml:yaml.value})}); $('#diff').textContent = result.diff || '(no changes)'; setStatus('YAML and typed config are valid.', 'ok'); }
  catch (error) { setStatus(error.message, 'error'); }
}
async function review() {
  $('#diff').textContent = state ? (await api('/api/config/validate', {method:'POST', body:JSON.stringify({layer, raw_yaml:yaml.value})})).diff || '(no changes)' : '';
}
async function save() {
  try { const result = await api('/api/config/raw', {method:'PUT', body:JSON.stringify({layer, expected_revision:state.revision, raw_yaml:yaml.value})}); $('#diff').textContent = result.diff || '(no changes)'; await load(layer); setStatus('Saved canonical YAML. Sync remains separate.', 'ok'); }
  catch (error) { setStatus(error.message, 'error'); }
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
