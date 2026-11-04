'use strict';
// All DOM text is set with textContent / createElement (no innerHTML) so
// server-provided strings can never inject markup.
const $ = (id) => document.getElementById(id);
const STEPS = ['Target', 'Server', 'Layout', 'Scan', 'Install'];
let step = 0, env = {}, scanned = false, hostKeyOK = '';

async function api(path, body) {
  const opt = body === undefined ? {} : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
  const r = await fetch(path, opt);
  const j = await r.json().catch(() => ({}));
  if (!r.ok && !j.errors && !j.error) j.error = 'HTTP ' + r.status;
  return j;
}
function el(tag, props, ...kids) {
  const e = document.createElement(tag);
  Object.assign(e, props || {});
  for (const k of kids) e.append(k);
  return e;
}
function target() { return document.querySelector('input[name=target]:checked').value; }

function sietchRows() { return [...document.querySelectorAll('#sietches .srow')]; }
function addSietch(name, mode) {
  const n = sietchRows().length + 1;
  const row = el('div', { className: 'srow' });
  const inp = el('input', { value: name || 'Sietch ' + n, maxLength: 32 });
  const sel = el('select');
  for (const m of ['PVE', 'PVP']) sel.append(el('option', { value: m, textContent: m }));
  sel.value = mode || 'PVE';
  const del = el('button', { type: 'button', textContent: 'Remove' });
  del.onclick = () => { if (sietchRows().length > 1) { row.remove(); estimate(); } };
  inp.oninput = estimate;
  row.append(inp, sel, del);
  $('sietches').append(row);
  estimate();
}

function config() {
  const t = target();
  const dd = +$('dd').value;
  return {
    config: {
      target: t,
      ssh: { host: $('ssh-host').value.trim(), port: +$('ssh-port').value || 22, user: $('ssh-user').value.trim(),
        auth: $('ssh-auth').value, keyPath: $('ssh-key').value.trim(), hostKeyFingerprint: hostKeyOK },
      wslDistro: t === 'local' && $('wsl-distro').value ? $('wsl-distro').value : '',
      installDir: $('installdir').value.trim(),
      webPort: +$('webport').value, serverTitle: $('title').value.trim(), region: $('region').value,
      sietches: sietchRows().map((r) => ({ name: r.children[0].value.trim(), mode: r.children[1].value })),
      deepDesert: { instances: dd, thirdRole: dd === 3 ? $('dd3').value : '' },
      applyTuning: $('tuning').checked,
    },
    secrets: { sshPassword: $('ssh-password').value, sshPassphrase: $('ssh-passphrase').value, funcomToken: $('token').value },
    experimentalCombat: $('combat').checked,
    acceptDisclaimer: $('accept').checked,
  };
}

async function estimate() {
  const r = await api('/api/validate', config());
  $('ram-est').textContent = r.requiredRamGiB ? 'Estimated RAM needed for this layout: about ' + r.requiredRamGiB + ' GB (heuristic - the scan checks your server).' : '';
}

function show(n) {
  step = n;
  document.querySelectorAll('section.panel').forEach((s) => (s.hidden = +s.dataset.step !== n));
  const nav = $('steps'); nav.replaceChildren(...STEPS.map((s, i) => el('span', { className: i === n ? 'on' : '', textContent: i + 1 + '. ' + s })));
  $('back').disabled = n === 0 || (n === 4 && running);
  $('next').hidden = n === 4;
  $('errors').textContent = '';
}

async function next() {
  const r = await api('/api/validate', config());
  const errs = (r.errors || []).filter((e) => relevant(e));
  if (errs.length) { $('errors').textContent = errs.join(' | '); return; }
  if (step === 1 && !$('token').value.trim()) { $('errors').textContent = 'Enter your Funcom token.'; return; }
  if (step === 3 && !scanned) { $('errors').textContent = 'Run the scan first.'; return; }
  show(step + 1);
  if (step === 4) summary();
}
function relevant(e) {
  const pre = e.split(':')[0].split(' ')[0];
  const byStep = { 0: ['ssh:', 'target', 'installDir', 'wslDistro'], 1: ['webPort', 'serverTitle', 'region'], 2: ['sietch', 'deepDesert', 'at'] };
  const keys = byStep[step] || [];
  return keys.some((k) => e.startsWith(k)) && pre !== '';
}

function summary() {
  const c = config().config;
  const rows = [
    ['Target', c.target === 'ssh' ? c.ssh.user + '@' + c.ssh.host + ':' + c.ssh.port : 'This computer'],
    ['Server', c.serverTitle + ' (' + c.region + '), Web UI port ' + c.webPort],
    ['Sietches', c.sietches.map((s) => s.name + ' [' + s.mode + ']').join(', ')],
    ['Deep Desert', c.deepDesert.instances ? c.deepDesert.instances + ' instance(s)' : 'upstream default'],
  ];
  const d = $('summary'); d.replaceChildren(...rows.map(([k, v]) => el('div', {}, el('b', { textContent: k + ': ' }), document.createTextNode(v))));
}

async function scan() {
  $('scan').disabled = true; $('scan-out').textContent = 'Scanning...'; $('errors').textContent = '';
  const r = await api('/api/scan', config());
  $('scan').disabled = false;
  if (r.hostKeyRequired) { askHostKey(r); return; }
  if (r.error) { $('scan-out').replaceChildren(el('div', { className: 'f error', textContent: r.error }), ...(r.diagnosis || []).map(diagCard)); return; }
  scanned = true;
  const h = r.host, rep = r.report;
  const out = [el('div', {}, el('b', { textContent: 'Target: ' }), document.createTextNode(r.target + ' - ' + (h.Distro || h.OS) + ', ' + (h.CPUModel || '') + ', ' + h.CPUThreads + ' threads, ' + (h.MemTotalKB / 1048576).toFixed(1) + ' GB RAM, ' + (h.DiskFreeKB / 1048576).toFixed(0) + ' GB free, AVX2 ' + (h.AVX2 ? 'yes' : 'NO')))];
  out.push(el('div', {}, document.createTextNode('Layout needs about ' + rep.requiredRamGiB + ' GB; this host fits up to ' + rep.maxSietches + ' Sietch(es) without Deep Desert.')));
  for (const f of rep.findings || []) out.push(el('div', { className: 'f ' + f.level, textContent: f.level.toUpperCase() + ': ' + f.message }));
  if (rep.ok === false) out.push(el('div', { className: 'f error', textContent: 'Blocking problems found - fix them or choose another target.' })), scanned = false;
  $('scan-out').replaceChildren(...out);
  if (rep.wslConfig) { $('wslcfg').hidden = false; $('wslcfg').textContent = '# %UserProfile%\\.wslconfig\n' + rep.wslConfig; }
}

function askHostKey(r) {
  const b = $('hostkey'); b.hidden = false;
  const ok = el('button', { type: 'button', textContent: 'Trust this key', className: 'primary' });
  ok.onclick = () => { hostKeyOK = r.fingerprint; b.hidden = true; scan(); };
  b.replaceChildren(el('b', { textContent: r.mismatch ? 'WARNING: the server key CHANGED.' : 'First connection to this server.' }),
    el('div', { textContent: 'SSH host key fingerprint: ' + r.fingerprint }),
    el('div', { className: 'muted', textContent: 'Compare it with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server before trusting it.' }), r.mismatch ? document.createTextNode('') : ok);
  $('scan-out').textContent = '';
}

function diagCard(d, confirmId) {
  const c = el('div', { className: 'diag' });
  c.append(el('h4', { textContent: d.title }), el('span', { className: 'risk', textContent: 'Fix type: ' + d.risk }), el('p', { textContent: d.explain }));
  for (const a of d.actions || []) c.append(el('div', { className: 'f info', textContent: a.description }));
  if (d.docUrl) c.append(el('a', { href: d.docUrl, target: '_blank', rel: 'noopener noreferrer', textContent: 'Documentation' }));
  if (confirmId) {
    const y = el('button', { type: 'button', textContent: 'Apply fix', className: 'primary' });
    const n = el('button', { type: 'button', textContent: 'Decline' });
    const send = (v) => { api('/api/confirm', { id: confirmId, approve: v }); y.disabled = n.disabled = true; };
    y.onclick = () => send(true); n.onclick = () => send(false);
    c.append(el('div', {}, y, n));
  }
  return c;
}

let running = false, es = null;
const stepEls = {};
function onEvent(e) {
  const log = $('log');
  if (e.type === 'log') { log.append(document.createTextNode(e.msg + '\n')); log.scrollTop = log.scrollHeight; }
  else if (e.type === 'step') {
    let li = stepEls[e.step];
    if (!li) { li = el('li'); stepEls[e.step] = li; $('progress').append(li); }
    li.className = e.status; li.textContent = e.msg || e.step;
  } else if (e.type === 'diagnosis') { $('assistant').append(diagCard(e.diagnosis)); }
  else if (e.type === 'confirm') { $('assistant').append(diagCard(e.diagnosis, e.confirmId)); }
  else if (e.type === 'fix') { log.append(document.createTextNode('[assistant] ' + e.msg + '\n')); }
  else if (e.type === 'error') { running = false; $('errors').textContent = e.msg; $('install').disabled = false; $('cancel').hidden = true; $('back').disabled = false; }
  else if (e.type === 'done') { running = false; $('errors').textContent = ''; $('cancel').hidden = true; log.append(document.createTextNode('\n' + e.msg + '\n')); const c = config().config; $('assistant').append(el('div', { className: 'diag' }, el('h4', { textContent: 'Done - open the Web UI' }), el('p', { textContent: 'http://<server-address>:' + c.webPort + ' - sign in with the admin password stored in runtime/secrets/admin-web-password.txt on the server. Open the ports from the documentation in your firewall/router.' }))); }
}

async function install() {
  $('install').disabled = true; $('back').disabled = true; $('log').textContent = ''; $('assistant').replaceChildren(); $('progress').replaceChildren();
  for (const k in stepEls) delete stepEls[k];
  const r = await api('/api/install', config());
  if (r.errors || r.error) { $('errors').textContent = (r.errors || [r.error]).join(' | '); $('install').disabled = false; $('back').disabled = false; return; }
  running = true; $('cancel').hidden = false;
  if (es) es.close();
  es = new EventSource('/api/events');
  es.onmessage = (m) => onEvent(JSON.parse(m.data));
}

async function init() {
  env = await api('/api/env');
  $('ver').textContent = 'v' + env.version + ' - ' + (env.platformName || '');
  $('region').replaceChildren(...env.regions.map((r) => el('option', { value: r, textContent: r })));
  $('keys').replaceChildren(...(env.sshKeys || []).map((k) => el('option', { value: k })));
  if (env.sshKeys && env.sshKeys[0]) $('ssh-key').value = env.sshKeys[0];
  const loc = document.querySelector('input[value=local]');
  $('local-info').textContent = env.local && env.local.reason ? env.local.reason : '';
  if (!env.local || !env.local.supported) loc.disabled = true;
  if (env.local && env.local.distros && env.local.distros.length) {
    $('wsl-distro').replaceChildren(...env.local.distros.map((d) => el('option', { value: d, textContent: d })));
  }
  const upd = () => {
    const t = target();
    $('ssh-box').hidden = t !== 'ssh';
    $('wsl-box').hidden = !(t === 'local' && env.platform === 'windows');
    const a = $('ssh-auth').value;
    $('row-key').hidden = a !== 'key'; $('row-pass').hidden = a !== 'key'; $('row-pw').hidden = a !== 'password';
    $('row-third').hidden = $('dd').value !== '3';
    scanned = false; hostKeyOK = hostKeyOK;
    estimate();
  };
  document.querySelectorAll('input[name=target]').forEach((r) => (r.onchange = upd));
  $('ssh-auth').onchange = upd; $('dd').onchange = upd;
  for (const id of ['ssh-host', 'ssh-port', 'ssh-user']) $(id).onchange = () => { scanned = false; hostKeyOK = ''; };
  $('add-sietch').onclick = () => addSietch();
  $('back').onclick = () => show(step - 1);
  $('next').onclick = next; $('scan').onclick = scan; $('install').onclick = install;
  $('cancel').onclick = () => api('/api/cancel', {});
  $('accept').onchange = () => { $('install').disabled = !$('accept').checked || running; };
  addSietch('Sietch 1', 'PVE');
  upd(); show(0);
}
init();
