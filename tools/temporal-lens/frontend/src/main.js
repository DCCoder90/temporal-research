// main.js — Temporal Analyze GUI
// Wails auto-generates wailsjs/ in this directory at build time.
// During development (wails dev), the bindings are available at runtime.

// Dynamic imports handle the case where wailsjs/ doesn't exist yet
// (e.g., when loading directly in a browser for CSS preview).
let Analyze, Export, OpenFileDialog, EventsOn, QueryDB;

async function loadWailsBindings() {
  try {
    const appModule = await import('./wailsjs/go/main/App.js');
    Analyze = appModule.Analyze;
    Export = appModule.Export;
    OpenFileDialog = appModule.OpenFileDialog;
    QueryDB = appModule.QueryDB;
    const runtime = await import('./wailsjs/runtime/runtime.js');
    EventsOn = runtime.EventsOn;
  } catch {
    // Running outside Wails (e.g., browser preview) — stubs only.
    Analyze = async () => { throw new Error('Wails runtime not available'); };
    Export = async () => { throw new Error('Wails runtime not available'); };
    OpenFileDialog = async () => '';
    QueryDB = async () => { throw new Error('Wails runtime not available'); };
    EventsOn = () => {};
  }
}

// ── State ──────────────────────────────────────────────────────────────────
let currentPcapPath = null;
let currentResult = null;
let panZoomInstances = [];
let activeView = 'diagrams'; // 'diagrams' | 'stats' | 'query'
let queryEditor = null;      // CodeMirror instance
let lastQueryResult = null;  // most recent QueryResult for CSV export
let grpcDiagrams = [];       // all gRPC sequence diagram pages
let grpcPage = 0;            // current page index
let grpcPanZoomIdx = -1;     // index into panZoomInstances for the gRPC diagram
let queryHistory = [];       // ring-buffer of recent SQL queries (most recent first)
let queryHistoryIdx = -1;    // -1 = not navigating history
let queryDraft = '';         // unsaved editor content while navigating history
let sortState = { col: -1, dir: 1 }; // current column sort for query results

// ── DOM refs ───────────────────────────────────────────────────────────────
const fileZone      = document.getElementById('file-zone');
const fileZoneLabel = document.getElementById('file-zone-label');
const fileZoneName  = document.getElementById('file-zone-name');
const analyzeBtn    = document.getElementById('analyze-btn');
const exportBtn     = document.getElementById('export-btn');
const statusBar     = document.getElementById('status-bar');
const emptyState    = document.getElementById('empty-state');
const metaBar       = document.getElementById('meta-bar');
const errorCard     = document.getElementById('error-card');
const errorMsg      = document.getElementById('error-msg');
const viewDiagrams  = document.getElementById('view-diagrams');
const viewStats     = document.getElementById('view-stats');
const statsContent  = document.getElementById('stats-content');
const tabDiagrams   = document.getElementById('tab-diagrams');
const tabStats      = document.getElementById('tab-stats');
const tabQuery      = document.getElementById('tab-query');
const viewQuery     = document.getElementById('view-query');
const queryRunBtn   = document.getElementById('query-run-btn');
const queryExportBtn = document.getElementById('query-export-btn');
const queryStatus   = document.getElementById('query-status');
const queryError    = document.getElementById('query-error');
const queryResultsWrap = document.getElementById('query-results-wrap');
const queryResultsMeta = document.getElementById('query-results-meta');
const queryResultsTable = document.getElementById('query-results-table');

// ── Mermaid setup ──────────────────────────────────────────────────────────
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  sequence: { showSequenceNumbers: true, mirrorActors: false, useMaxWidth: false },
  flowchart: { curve: 'basis', useMaxWidth: false },
});

// ── Tab switching ──────────────────────────────────────────────────────────
tabDiagrams.addEventListener('click', () => switchView('diagrams'));
tabStats.addEventListener('click',    () => switchView('stats'));
tabQuery.addEventListener('click',    () => switchView('query'));

function switchView(view) {
  activeView = view;
  tabDiagrams.classList.toggle('active', view === 'diagrams');
  tabStats.classList.toggle('active',    view === 'stats');
  tabQuery.classList.toggle('active',    view === 'query');
  viewDiagrams.hidden = view !== 'diagrams';
  viewStats.hidden    = view !== 'stats';
  viewQuery.hidden    = view !== 'query';
}

// ── Filter panel logic ─────────────────────────────────────────────────────
const protoInput = document.getElementById('proto-value');
const hostInput  = document.getElementById('host-value');

document.querySelectorAll('input[name="proto-mode"]').forEach(radio => {
  radio.addEventListener('change', () => {
    protoInput.disabled = radio.value === 'all';
    if (radio.value === 'all') protoInput.value = '';
  });
});

document.querySelectorAll('input[name="host-mode"]').forEach(radio => {
  radio.addEventListener('change', () => {
    hostInput.disabled = radio.value === 'all';
    if (radio.value === 'all') hostInput.value = '';
  });
});

// ── File selection ─────────────────────────────────────────────────────────
fileZone.addEventListener('click', async () => {
  const path = await OpenFileDialog();
  if (path) setFile(path);
});

fileZone.addEventListener('dragover', e => {
  e.preventDefault();
  fileZone.classList.add('drag-over');
});

fileZone.addEventListener('dragleave', () => {
  fileZone.classList.remove('drag-over');
});

fileZone.addEventListener('drop', e => {
  e.preventDefault();
  fileZone.classList.remove('drag-over');
  const files = e.dataTransfer?.files;
  if (files && files.length > 0 && files[0].path) {
    setFile(files[0].path);
  }
});

function setFile(path) {
  currentPcapPath = path;
  const name = path.split(/[\\/]/).pop();
  fileZoneLabel.hidden = true;
  fileZoneName.textContent = name;
  fileZoneName.hidden = false;
  analyzeBtn.disabled = false;
  setStatus('Ready — click Analyze');
}

// ── Wails file-drop event (native drag from Finder/Explorer) ───────────────
async function setupWailsEvents() {
  EventsOn('wails:file-drop', paths => {
    if (paths && paths.length > 0) {
      setFile(paths[0]);
    }
  });
}

// ── Analysis ───────────────────────────────────────────────────────────────
analyzeBtn.addEventListener('click', async () => {
  if (!currentPcapPath) return;

  setStatus('Analyzing...');
  analyzeBtn.classList.add('loading');
  analyzeBtn.disabled = true;
  hideError();

  try {
    const opts = buildOptions();
    const result = await Analyze(currentPcapPath, opts);
    currentResult = result;
    await renderResult(result);
    exportBtn.disabled = false;
    setStatus('Done');
  } catch (err) {
    showError(String(err));
    setStatus('Analysis failed');
  } finally {
    analyzeBtn.classList.remove('loading');
    analyzeBtn.disabled = false;
  }
});

// ── Export ─────────────────────────────────────────────────────────────────
exportBtn.addEventListener('click', async () => {
  if (!currentPcapPath) return;

  setStatus('Exporting...');
  exportBtn.disabled = true;

  try {
    const opts = buildOptions();
    const paths = await Export(currentPcapPath, opts);
    setStatus('Exported: ' + paths.map(p => p.split(/[\\/]/).pop()).join(', '));
  } catch (err) {
    showError('Export failed: ' + String(err));
    setStatus('Export failed');
  } finally {
    exportBtn.disabled = false;
  }
});

// ── Build options from filter panel ───────────────────────────────────────
function buildOptions() {
  const protoMode = document.querySelector('input[name="proto-mode"]:checked').value;
  const hostMode  = document.querySelector('input[name="host-mode"]:checked').value;

  return {
    Only:           protoMode === 'only'         ? splitCSV(protoInput.value) : [],
    Exclude:        protoMode === 'exclude'       ? splitCSV(protoInput.value) : [],
    OnlyHosts:      hostMode  === 'only-host'     ? splitCSV(hostInput.value)  : [],
    ExcludeHosts:   hostMode  === 'exclude-host'  ? splitCSV(hostInput.value)  : [],
    NoInterservice: document.getElementById('no-interservice').checked,
  };
}

function splitCSV(s) {
  return s.split(',').map(x => x.trim()).filter(Boolean);
}

// ── Render result ──────────────────────────────────────────────────────────
async function renderResult(result) {
  // Destroy existing pan-zoom instances.
  panZoomInstances.forEach(i => { try { i.destroy(); } catch {} });
  panZoomInstances = [];

  // Clear existing mermaid content.
  ['mermaid-flow', 'mermaid-traffic', 'mermaid-grpc'].forEach(id => {
    const el = document.getElementById(id);
    el.innerHTML = '';
    el.removeAttribute('data-processed');
  });

  // Populate metadata bar.
  document.getElementById('meta-file').innerHTML     = `<strong>File:</strong> ${result.PcapName}`;
  document.getElementById('meta-duration').innerHTML = `<strong>Duration:</strong> ${result.Duration.toFixed(1)}s`;
  document.getElementById('meta-packets').innerHTML  = `<strong>Packets:</strong> ${fmtNum(result.PacketCount)}`;
  document.getElementById('meta-bytes').innerHTML    = `<strong>Bytes:</strong> ${fmtBytes(result.TotalBytes)}`;
  document.getElementById('meta-grpc').innerHTML     = `<strong>gRPC Calls:</strong> ${fmtNum(result.GRPCCount)}`;

  const filterBadge = document.getElementById('meta-filter');
  if (result.FilterDesc) {
    filterBadge.innerHTML = `<strong>Filter:</strong> ${result.FilterDesc}`;
    filterBadge.hidden = false;
  } else {
    filterBadge.hidden = true;
  }

  // Render statistics markdown.
  statsContent.innerHTML = marked.parse(result.StatsMarkdown || '');

  // Set diagram content.
  grpcDiagrams = result.SeqDiagrams || [];
  grpcPage = 0;
  document.getElementById('mermaid-flow').textContent    = result.FlowDiagram;
  document.getElementById('mermaid-traffic').textContent = result.TrafficSeq || '';
  document.getElementById('mermaid-grpc').textContent    = grpcDiagrams[0] || '';

  // Set up gRPC pagination controls.
  const grpcPageNav = document.getElementById('grpc-page-nav');
  if (grpcDiagrams.length > 1) {
    grpcPageNav.hidden = false;
    document.getElementById('grpc-page-label').textContent = `Page 1 of ${grpcDiagrams.length}`;
    document.getElementById('grpc-prev-btn').disabled = true;
    document.getElementById('grpc-next-btn').disabled = grpcDiagrams.length <= 1;
  } else {
    grpcPageNav.hidden = true;
  }

  // Show/hide traffic section.
  const showTraffic = Boolean(result.TrafficSeq);
  setVisible('section-traffic', showTraffic);
  setVisible('card-traffic', showTraffic);

  // Show all other diagram sections.
  ['section-flow', 'card-flow', 'section-grpc', 'card-grpc'].forEach(id => setVisible(id, true));

  // Hide empty state, show meta bar and view controls.
  emptyState.hidden = false; // keep hidden by switching view below
  emptyState.style.display = 'none';
  metaBar.hidden = false;
  document.getElementById('view-toggle').hidden = false;

  // Switch to diagrams view and run Mermaid.
  switchView('diagrams');

  // Render Mermaid diagrams.
  await mermaid.run({ querySelector: '.mermaid' });

  // Attach svg-pan-zoom to each rendered SVG.
  document.querySelectorAll('.diagram-wrap .mermaid svg').forEach(svg => {
    svg.style.width = '100%';
    svg.style.height = '100%';
    const instance = svgPanZoom(svg, {
      zoomEnabled: true,
      controlIconsEnabled: false,
      fit: true,
      center: true,
      minZoom: 0.05,
      maxZoom: 30,
      zoomScaleSensitivity: 0.3,
      mouseWheelZoomEnabled: true,
    });
    panZoomInstances.push(instance);
  });

  // Find the gRPC pan-zoom instance by element position, not array length,
  // so the index is correct whether or not the traffic diagram is visible.
  const allSvgs = [...document.querySelectorAll('.diagram-wrap .mermaid svg')];
  const grpcSvg = document.querySelector('#mermaid-grpc svg');
  grpcPanZoomIdx = grpcSvg ? allSvgs.indexOf(grpcSvg) : panZoomInstances.length - 1;
}

// ── gRPC diagram page navigation ───────────────────────────────────────────
async function grpcChangePage(delta) {
  const next = grpcPage + delta;
  if (next < 0 || next >= grpcDiagrams.length) return;
  grpcPage = next;

  // Destroy the old pan-zoom instance for this diagram.
  if (grpcPanZoomIdx >= 0 && panZoomInstances[grpcPanZoomIdx]) {
    try { panZoomInstances[grpcPanZoomIdx].destroy(); } catch {}
    panZoomInstances[grpcPanZoomIdx] = null;
  }

  // Swap in new page content and re-render.
  const el = document.getElementById('mermaid-grpc');
  el.innerHTML = '';
  el.removeAttribute('data-processed');
  el.textContent = grpcDiagrams[grpcPage];

  await mermaid.run({ querySelector: '#mermaid-grpc' });

  const svg = el.querySelector('svg');
  if (svg) {
    svg.style.width = '100%';
    svg.style.height = '100%';
    panZoomInstances[grpcPanZoomIdx] = svgPanZoom(svg, {
      zoomEnabled: true,
      controlIconsEnabled: false,
      fit: true,
      center: true,
      minZoom: 0.05,
      maxZoom: 30,
      zoomScaleSensitivity: 0.3,
      mouseWheelZoomEnabled: true,
    });
  }

  document.getElementById('grpc-page-label').textContent = `Page ${grpcPage + 1} of ${grpcDiagrams.length}`;
  document.getElementById('grpc-prev-btn').disabled = grpcPage === 0;
  document.getElementById('grpc-next-btn').disabled = grpcPage === grpcDiagrams.length - 1;
}

// ── Zoom controls ──────────────────────────────────────────────────────────
window.grpcChangePage = grpcChangePage;
window.zoomIn    = i => { if (panZoomInstances[i]) panZoomInstances[i].zoomIn(); };
window.zoomOut   = i => { if (panZoomInstances[i]) panZoomInstances[i].zoomOut(); };
window.resetZoom = i => {
  if (panZoomInstances[i]) {
    panZoomInstances[i].resetZoom();
    panZoomInstances[i].fit();
    panZoomInstances[i].center();
  }
};

// ── Helpers ────────────────────────────────────────────────────────────────
function setVisible(id, visible) {
  const el = document.getElementById(id);
  if (el) el.hidden = !visible;
}

function setStatus(msg) {
  statusBar.textContent = msg;
}

function showError(msg) {
  errorMsg.textContent = msg;
  errorCard.hidden = false;
}

function hideError() {
  errorCard.hidden = true;
}

function fmtNum(n) {
  return n.toLocaleString();
}

function fmtBytes(n) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let f = n;
  for (let i = 0; i < units.length - 1; i++) {
    if (f < 1024) return `${f.toFixed(1)} ${units[i]}`;
    f /= 1024;
  }
  return `${f.toFixed(1)} TB`;
}

// ── Sample queries ─────────────────────────────────────────────────────────
const SAMPLE_QUERIES = [
  {
    label: 'Packets by protocol',
    sql: `SELECT protocol, COUNT(*) AS packets, SUM(bytes) AS total_bytes
FROM packets
GROUP BY protocol
ORDER BY total_bytes DESC;`,
  },
  {
    label: 'Top talkers by bytes (src)',
    sql: `SELECT src, COUNT(*) AS packets, SUM(bytes) AS bytes
FROM packets
GROUP BY src
ORDER BY bytes DESC
LIMIT 20;`,
  },
  {
    label: 'Packets per src/dst pair',
    sql: `SELECT src, dst, protocol, COUNT(*) AS packets, SUM(bytes) AS bytes
FROM packets
GROUP BY src, dst, protocol
ORDER BY bytes DESC;`,
  },
  {
    label: 'gRPC call frequency',
    sql: `SELECT method, COUNT(*) AS calls
FROM grpc_calls
GROUP BY method
ORDER BY calls DESC;`,
  },
  {
    label: 'gRPC calls by source',
    sql: `SELECT src, method, COUNT(*) AS calls
FROM grpc_calls
GROUP BY src, method
ORDER BY calls DESC;`,
  },
  {
    label: 'gRPC calls by source + destination',
    sql: `SELECT src, dst, method, COUNT(*) AS calls
FROM grpc_calls
GROUP BY src, dst, method
ORDER BY calls DESC;`,
  },
  {
    label: 'All packets (first 100)',
    sql: `SELECT time, src, dst, src_port, dst_port, protocol, bytes
FROM packets
ORDER BY time
LIMIT 100;`,
  },
  {
    label: 'All gRPC calls',
    sql: `SELECT time, src, dst, method
FROM grpc_calls
ORDER BY time;`,
  },
  {
    label: 'JOIN: packets with gRPC method (via tcp_stream)',
    sql: `SELECT p.time, p.src, p.dst, g.method, p.bytes
FROM packets p
JOIN grpc_calls g ON p.tcp_stream = g.tcp_stream
ORDER BY p.time
LIMIT 100;`,
  },
  {
    label: 'Failed gRPC calls (status != 0)',
    sql: `SELECT src, dst, method, status_code, COUNT(*) AS n
FROM grpc_calls
WHERE status_code > 0
GROUP BY src, dst, method, status_code
ORDER BY n DESC;`,
  },
  {
    label: 'Retransmitted packets',
    sql: `SELECT src, dst, protocol, COUNT(*) AS retransmits
FROM packets
WHERE retransmit = 1
GROUP BY src, dst, protocol
ORDER BY retransmits DESC;`,
  },
  {
    label: 'Average RTT by src/dst',
    sql: `SELECT src, dst, ROUND(AVG(rtt) * 1000, 3) AS avg_rtt_ms, COUNT(*) AS samples
FROM packets
WHERE rtt > 0
GROUP BY src, dst
ORDER BY avg_rtt_ms DESC;`,
  },
  {
    label: 'Bytes per minute',
    sql: `SELECT CAST(time / 60 AS INTEGER) * 60 AS minute_epoch,
       COUNT(*) AS packets,
       SUM(bytes) AS bytes
FROM packets
GROUP BY minute_epoch
ORDER BY minute_epoch;`,
  },
  {
    label: 'gRPC calls by service',
    sql: `SELECT service, method, COUNT(*) AS calls
FROM grpc_calls
GROUP BY service, method
ORDER BY calls DESC;`,
  },
];

// ── Query history ──────────────────────────────────────────────────────────
const HISTORY_KEY = 'temporal-lens:query-history';
const HISTORY_MAX = 10;

function loadQueryHistory() {
  try {
    const saved = localStorage.getItem(HISTORY_KEY);
    if (saved) queryHistory = JSON.parse(saved);
  } catch {}
}

function saveToQueryHistory(sql) {
  queryHistory = queryHistory.filter(s => s !== sql);
  queryHistory.unshift(sql);
  if (queryHistory.length > HISTORY_MAX) queryHistory.length = HISTORY_MAX;
  queryHistoryIdx = -1;
  queryDraft = '';
  try { localStorage.setItem(HISTORY_KEY, JSON.stringify(queryHistory)); } catch {}
}

function navigateHistoryUp() {
  if (queryHistory.length === 0) return CodeMirror.Pass;
  if (queryHistoryIdx === -1) queryDraft = queryEditor.getValue();
  if (queryHistoryIdx < queryHistory.length - 1) {
    queryHistoryIdx++;
    queryEditor.setValue(queryHistory[queryHistoryIdx]);
    queryEditor.setCursor(queryEditor.lastLine(), 0);
  }
}

function navigateHistoryDown() {
  if (queryHistoryIdx === -1) return CodeMirror.Pass;
  queryHistoryIdx--;
  queryEditor.setValue(queryHistoryIdx === -1 ? queryDraft : queryHistory[queryHistoryIdx]);
  queryEditor.setCursor(queryEditor.lastLine(), 0);
}

// ── Query tab ──────────────────────────────────────────────────────────────
function initQueryTab() {
  loadQueryHistory();

  // Initialize CodeMirror SQL editor.
  queryEditor = CodeMirror.fromTextArea(document.getElementById('query-editor'), {
    mode: 'text/x-sql',
    theme: 'default',
    lineNumbers: true,
    indentWithTabs: false,
    tabSize: 2,
    extraKeys: {
      'Ctrl-Enter': runQuery,
      'Cmd-Enter':  runQuery,
      'Ctrl-Up':    navigateHistoryUp,
      'Ctrl-Down':  navigateHistoryDown,
    },
  });
  queryEditor.setSize('100%', '180px');
  queryEditor.setValue(SAMPLE_QUERIES[0].sql);

  // Populate sample queries dropdown.
  const sel = document.getElementById('query-samples');
  SAMPLE_QUERIES.forEach((q, i) => {
    const opt = document.createElement('option');
    opt.value = i;
    opt.textContent = q.label;
    sel.appendChild(opt);
  });
  sel.addEventListener('change', () => {
    const idx = parseInt(sel.value, 10);
    if (!isNaN(idx)) queryEditor.setValue(SAMPLE_QUERIES[idx].sql);
    sel.value = '';
  });

  queryRunBtn.addEventListener('click', runQuery);
  queryExportBtn.addEventListener('click', exportCSV);

  // Help modal.
  const backdrop = document.getElementById('query-help-backdrop');
  document.getElementById('query-help-btn').addEventListener('click', () => { backdrop.hidden = false; });
  document.getElementById('query-help-close').addEventListener('click', () => { backdrop.hidden = true; });
  backdrop.addEventListener('click', e => { if (e.target === backdrop) backdrop.hidden = true; });
  document.addEventListener('keydown', e => { if (e.key === 'Escape') backdrop.hidden = true; });
}

async function runQuery() {
  const sql = queryEditor.getValue().trim();
  if (!sql) return;

  saveToQueryHistory(sql);
  queryRunBtn.disabled = true;
  queryExportBtn.disabled = true;
  queryStatus.textContent = 'Running…';
  queryError.hidden = true;
  queryResultsWrap.hidden = true;
  lastQueryResult = null;

  try {
    const result = await QueryDB(sql);

    if (result.SQLError) {
      queryError.textContent = result.SQLError;
      queryError.hidden = false;
      queryStatus.textContent = 'Error';
      return;
    }

    lastQueryResult = result;
    renderQueryTable(result);
    queryExportBtn.disabled = false;

    let meta = `${fmtNum(result.RowCount)} row${result.RowCount !== 1 ? 's' : ''}`;
    if (result.Truncated) meta += ' (truncated at 10,000 — refine your query)';
    queryStatus.textContent = meta;
  } catch (err) {
    queryError.textContent = String(err);
    queryError.hidden = false;
    queryStatus.textContent = 'Error';
  } finally {
    queryRunBtn.disabled = false;
  }
}

function renderQueryTable(result) {
  sortState = { col: -1, dir: 1 };

  if (!result.Columns || result.Columns.length === 0) {
    queryResultsMeta.textContent = 'Query executed — no columns returned.';
    queryResultsWrap.hidden = false;
    return;
  }

  let html = '<thead><tr>';
  result.Columns.forEach((c, i) => {
    html += `<th class="sortable" data-col="${i}">${escHtml(c)}</th>`;
  });
  html += '</tr></thead><tbody></tbody>';
  queryResultsTable.innerHTML = html;

  queryResultsTable.querySelectorAll('th.sortable').forEach(th => {
    th.addEventListener('click', () => applySortColumn(parseInt(th.dataset.col, 10)));
  });

  renderTableRows(result.Rows || []);
  queryResultsWrap.hidden = false;
}

function applySortColumn(col) {
  if (sortState.col === col) {
    sortState.dir *= -1;
  } else {
    sortState.col = col;
    sortState.dir = 1;
  }
  queryResultsTable.querySelectorAll('th').forEach((th, i) => {
    th.classList.toggle('sort-asc',  i === col && sortState.dir === 1);
    th.classList.toggle('sort-desc', i === col && sortState.dir === -1);
  });
  const rows = [...(lastQueryResult?.Rows || [])];
  rows.sort((a, b) => {
    const va = a[col], vb = b[col];
    if (va === null && vb === null) return 0;
    if (va === null) return sortState.dir;
    if (vb === null) return -sortState.dir;
    if (typeof va === 'number' && typeof vb === 'number') return sortState.dir * (va - vb);
    return sortState.dir * String(va).localeCompare(String(vb));
  });
  renderTableRows(rows);
}

function renderTableRows(rows) {
  const tbody = queryResultsTable.querySelector('tbody');
  if (!tbody) return;
  tbody.innerHTML = rows.map(row =>
    '<tr>' + row.map(cell =>
      `<td>${cell === null ? '<span class="null-cell">NULL</span>' : escHtml(String(cell))}</td>`
    ).join('') + '</tr>'
  ).join('');
}

function exportCSV() {
  if (!lastQueryResult) return;
  const lines = [lastQueryResult.Columns.map(csvCell).join(',')];
  (lastQueryResult.Rows || []).forEach(row => {
    lines.push(row.map(v => csvCell(v === null ? '' : String(v))).join(','));
  });
  const blob = new Blob([lines.join('\r\n')], { type: 'text/csv' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'query_result.csv';
  a.click();
  URL.revokeObjectURL(url);
}

function csvCell(s) {
  return (s.includes(',') || s.includes('"') || s.includes('\n'))
    ? `"${s.replace(/"/g, '""')}"` : s;
}

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ── Init ───────────────────────────────────────────────────────────────────
(async () => {
  await loadWailsBindings();
  await setupWailsEvents();
  // Hide view toggle until first analysis completes.
  document.getElementById('view-toggle').hidden = true;
  initQueryTab();
})();
