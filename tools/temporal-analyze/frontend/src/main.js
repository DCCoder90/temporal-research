// main.js — Temporal Analyze GUI
// Wails auto-generates wailsjs/ in this directory at build time.
// During development (wails dev), the bindings are available at runtime.

// Dynamic imports handle the case where wailsjs/ doesn't exist yet
// (e.g., when loading directly in a browser for CSS preview).
let Analyze, Export, OpenFileDialog, EventsOn;

async function loadWailsBindings() {
  try {
    const appModule = await import('./wailsjs/go/main/App.js');
    Analyze = appModule.Analyze;
    Export = appModule.Export;
    OpenFileDialog = appModule.OpenFileDialog;
    const runtime = await import('./wailsjs/runtime/runtime.js');
    EventsOn = runtime.EventsOn;
  } catch {
    // Running outside Wails (e.g., browser preview) — stubs only.
    Analyze = async () => { throw new Error('Wails runtime not available'); };
    Export = async () => { throw new Error('Wails runtime not available'); };
    OpenFileDialog = async () => '';
    EventsOn = () => {};
  }
}

// ── State ──────────────────────────────────────────────────────────────────
let currentPcapPath = null;
let currentResult = null;
let panZoomInstances = [];
let activeView = 'diagrams'; // 'diagrams' | 'stats'

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

function switchView(view) {
  activeView = view;
  tabDiagrams.classList.toggle('active', view === 'diagrams');
  tabStats.classList.toggle('active',    view === 'stats');
  viewDiagrams.hidden = view !== 'diagrams';
  viewStats.hidden    = view !== 'stats';
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
  document.getElementById('mermaid-flow').textContent    = result.FlowDiagram;
  document.getElementById('mermaid-traffic').textContent = result.TrafficSeq || '';
  document.getElementById('mermaid-grpc').textContent    = result.SeqDiagram;

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
}

// ── Zoom controls ──────────────────────────────────────────────────────────
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

// ── Init ───────────────────────────────────────────────────────────────────
(async () => {
  await loadWailsBindings();
  await setupWailsEvents();
  // Hide view toggle until first analysis completes.
  document.getElementById('view-toggle').hidden = true;
})();
