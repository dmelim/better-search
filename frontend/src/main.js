import './style.css';

import {
  CodeXml,
  ExternalLink,
  File,
  Folder,
  FolderOpen,
  LoaderCircle,
  RefreshCw,
  Search,
  createIcons,
} from 'lucide';
import {
  OpenInVSCode,
  OpenPath,
  Rescan,
  RevealPath,
  Search as SearchEntries,
  Status,
} from '../wailsjs/go/main/App';

const lucideIcons = {
  CodeXml,
  ExternalLink,
  File,
  Folder,
  FolderOpen,
  LoaderCircle,
  RefreshCw,
  Search,
};

document.querySelector('#app').innerHTML = `
  <main class="shell">
    <section class="workspace">
      <section class="surface">
        <div class="query-panel">
          <div class="querybar">
            <label class="query-input" for="search-input">
              <span class="query-icon" data-lucide="search"></span>
              <span class="query-label">Search</span>
              <input id="search-input" class="search-input" type="text" autocomplete="off" spellcheck="false" placeholder="Type a file, folder, extension, or path fragment">
            </label>
            <button class="rescan-button" id="rescan-btn" type="button">
              <span data-lucide="refresh-cw"></span>
              <span>Rescan</span>
            </button>
          </div>

          <section class="shortcut-strip" aria-label="Keyboard shortcuts">
            <p class="section-label">Keys</p>
            <div class="shortcut-list">
              <p><kbd>/</kbd><span>Focus search</span></p>
              <p><kbd>Enter</kbd><span>Open selected</span></p>
              <p><kbd>Up/Down</kbd><span>Move through matches</span></p>
            </div>
          </section>

          <div class="filterbar">
            <div class="filter-group filter-group-wide">
              <span class="filter-label">Scope</span>
              <div class="filter-toggle filter-toggle-drives" id="drive-filters"></div>
            </div>

            <div class="filter-group">
              <span class="filter-label">Type</span>
              <div class="filter-toggle">
                <button class="filter-chip active" type="button" data-type-filter="all">Both</button>
                <button class="filter-chip" type="button" data-type-filter="files">Files</button>
                <button class="filter-chip" type="button" data-type-filter="folders">Folders</button>
              </div>
            </div>

            <label class="filter-field" for="folder-filter">
              <span class="filter-label">Path Focus</span>
              <input id="folder-filter" class="filter-text" type="text" autocomplete="off" spellcheck="false" placeholder="Optional folder path">
            </label>

            <label class="filter-field filter-field-compact" for="extension-filter">
              <span class="filter-label">Extension</span>
              <input id="extension-filter" class="filter-text" type="text" autocomplete="off" spellcheck="false" placeholder=".pdf or js">
            </label>

            <button class="clear-filters-button" id="clear-filters-btn" type="button">Clear filters</button>
          </div>
        </div>

        <div class="results-shell">
          <div class="results-head">
            <div>
              <p class="section-label">Matches</p>
              <p class="results-note" id="results-note">Type to search the active index.</p>
            </div>
          </div>
          <div class="results-viewport">
            <div class="results" id="results"></div>
          </div>
        </div>
      </section>
    </section>
  </main>
  <div class="flash" id="flash"></div>
`;

const state = {
  query: '',
  drives: [],
  filters: {
    typeFilter: 'all',
    folderPrefix: '',
    extension: '',
  },
  status: null,
  results: [],
  selectedIndex: -1,
  searchTimer: null,
  isSearching: false,
  isRescanning: false,
};

const els = {
  resultsNote: document.getElementById('results-note'),
  results: document.getElementById('results'),
  search: document.getElementById('search-input'),
  folderFilter: document.getElementById('folder-filter'),
  extensionFilter: document.getElementById('extension-filter'),
  driveFilters: document.getElementById('drive-filters'),
  clearFilters: document.getElementById('clear-filters-btn'),
  rescan: document.getElementById('rescan-btn'),
  flash: document.getElementById('flash'),
  typeFilters: Array.from(document.querySelectorAll('[data-type-filter]')),
};

const nf = new Intl.NumberFormat();
const df = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
});

function init() {
  els.search.addEventListener('input', onSearchInput);
  els.search.addEventListener('keydown', onInputKeyDown);
  els.folderFilter.addEventListener('input', onFilterInput);
  els.extensionFilter.addEventListener('input', onFilterInput);
  els.clearFilters.addEventListener('click', onClearFilters);
  els.rescan.addEventListener('click', onRescan);
  els.typeFilters.forEach((node) => {
    node.addEventListener('click', onTypeFilterClick);
  });
  document.addEventListener('keydown', onGlobalKeyDown);

  refreshStatus();
  setInterval(refreshStatus, 1800);
  els.search.focus();
  render();
}

async function refreshStatus() {
  try {
    state.status = await Status();
    state.drives = deriveDriveRoots(state.status?.roots || []);
    render();
  } catch (error) {
    flash('Could not load scan status.');
  }
}

function onSearchInput(event) {
  state.query = event.target.value.trim();
  state.selectedIndex = -1;
  scheduleSearch();
}

function onFilterInput(event) {
  const value = event.target.value.trim();
  if (event.target === els.folderFilter) {
    state.filters.folderPrefix = value;
  } else if (event.target === els.extensionFilter) {
    state.filters.extension = value;
  }

  state.selectedIndex = -1;
  scheduleSearch();
}

function onTypeFilterClick(event) {
  const nextValue = event.currentTarget.dataset.typeFilter || 'all';
  if (state.filters.typeFilter === nextValue) {
    return;
  }

  state.filters.typeFilter = nextValue;
  state.selectedIndex = -1;
  scheduleSearch();
}

function onDriveFilterClick(event) {
  const nextValue = event.currentTarget.dataset.driveFilter || '';
  if (state.filters.folderPrefix === nextValue) {
    return;
  }

  state.filters.folderPrefix = nextValue;
  els.folderFilter.value = nextValue;
  state.selectedIndex = -1;
  scheduleSearch();
}

function onClearFilters() {
  state.filters = {
    typeFilter: 'all',
    folderPrefix: '',
    extension: '',
  };
  els.folderFilter.value = '';
  els.extensionFilter.value = '';
  state.selectedIndex = -1;
  scheduleSearch();
}

function scheduleSearch() {
  if (state.searchTimer) {
    clearTimeout(state.searchTimer);
  }

  if (!state.query) {
    state.searchTimer = null;
    state.isSearching = false;
    state.results = [];
    render();
    return;
  }

  state.searchTimer = setTimeout(runSearch, 120);
  render();
}

async function runSearch() {
  if (!state.query) {
    state.searchTimer = null;
    state.results = [];
    state.isSearching = false;
    render();
    return;
  }

  state.searchTimer = null;
  state.isSearching = true;
  render();

  try {
    state.results = await SearchEntries({
      query: state.query,
      limit: 75,
      typeFilter: state.filters.typeFilter,
      folderPrefix: state.filters.folderPrefix,
      extension: state.filters.extension,
    });
    state.selectedIndex = state.results.length ? 0 : -1;
  } catch (error) {
    flash('Search failed.');
  } finally {
    state.isSearching = false;
    render();
  }
}

async function onRescan() {
  try {
    state.isRescanning = true;
    render();
    await Rescan();
    await refreshStatus();

    if (state.query) {
      await runSearch();
    } else {
      state.results = [];
      state.selectedIndex = -1;
    }
  } catch (error) {
    flash('Rescan could not be started.');
  } finally {
    state.isRescanning = false;
    render();
  }
}

async function openPath(path) {
  try {
    await OpenPath(path);
  } catch (error) {
    flash('Could not open item.');
  }
}

async function revealPath(path) {
  try {
    await RevealPath(path);
  } catch (error) {
    flash('Could not reveal item in Explorer.');
  }
}

async function openInVSCode(path) {
  try {
    await OpenInVSCode(path);
  } catch (error) {
    flash('Could not open item in VS Code.');
  }
}

function onInputKeyDown(event) {
  if (!state.results.length) return;

  if (event.key === 'ArrowDown') {
    event.preventDefault();
    state.selectedIndex = Math.min(state.selectedIndex + 1, state.results.length - 1);
    render();
    scrollSelectedIntoView();
  } else if (event.key === 'ArrowUp') {
    event.preventDefault();
    state.selectedIndex = Math.max(state.selectedIndex - 1, 0);
    render();
    scrollSelectedIntoView();
  } else if (event.key === 'Enter' && state.selectedIndex >= 0) {
    event.preventDefault();
    openPath(state.results[state.selectedIndex].path);
  }
}

function onGlobalKeyDown(event) {
  if (event.key === '/') {
    const tag = document.activeElement?.tagName;
    if (tag !== 'INPUT' && tag !== 'TEXTAREA') {
      event.preventDefault();
      els.search.focus();
      els.search.select();
    }
  }
}

function render() {
  renderFilterControls();
  renderResults();
  renderIcons();
}

function renderFilterControls() {
  els.typeFilters.forEach((node) => {
    node.classList.toggle('active', node.dataset.typeFilter === state.filters.typeFilter);
  });

  renderDriveFilters();
  els.clearFilters.disabled = !hasActiveFilters();
}

function renderDriveFilters() {
  const driveButtons = ['']
    .concat(state.drives)
    .map((drive) => `
      <button
        class="filter-chip ${isDriveActive(drive) ? 'active' : ''}"
        type="button"
        data-drive-filter="${escapeAttr(drive)}"
      >${escapeHtml(drive || 'All drives')}</button>
    `)
    .join('');

  els.driveFilters.innerHTML = driveButtons;
  els.driveFilters.querySelectorAll('[data-drive-filter]').forEach((node) => {
    node.addEventListener('click', onDriveFilterClick);
  });
}

function renderResults() {
  const filterSuffix = formatFilterSuffix();

  if (!state.query) {
    els.resultsNote.textContent = filterSuffix
      ? `Type to search everywhere${filterSuffix}.`
      : 'Type to search everywhere.';
    els.results.innerHTML = `
      <div class="empty">
        <h3 class="empty-title">One search field. One result stream.</h3>
        <p class="empty-copy">${escapeHtml(filterSuffix
          ? `Filters are ready. Add a query to search${filterSuffix}.`
          : 'Search stays global by default. Use drive chips only when you want to focus the scan.'
        )}</p>
      </div>
    `;
    return;
  }

  if (state.isSearching && !state.results.length) {
    els.resultsNote.textContent = `Searching for "${state.query}"${filterSuffix}`;
    els.results.innerHTML = `
      <div class="empty">
        <div class="empty-icon" data-lucide="loader-circle" data-icon-spin="true"></div>
        <h3 class="empty-title">Searching</h3>
        <p class="empty-copy">${escapeHtml(filterSuffix
          ? `Checking the current index and any live fallback matches${filterSuffix}.`
          : 'Checking the current index and any live fallback matches.'
        )}</p>
      </div>
    `;
    return;
  }

  if (!state.results.length) {
    els.resultsNote.textContent = `No matches for "${state.query}"${filterSuffix}`;
    els.results.innerHTML = `
      <div class="empty">
        <h3 class="empty-title">Nothing useful yet</h3>
        <p class="empty-copy">${escapeHtml(filterSuffix
          ? `Try a shorter token, a broader path fragment, or loosen the active filters${filterSuffix}.`
          : 'Try a shorter token, a broader path fragment, or let the index warm up a little longer.'
        )}</p>
      </div>
    `;
    return;
  }

  els.resultsNote.textContent = `${nf.format(state.results.length)} matches for "${state.query}"${filterSuffix}`;
  els.results.innerHTML = state.results.map((item, index) => {
    const meta = [
      item.isDir ? 'Folder' : formatBytes(item.size),
      item.modTime ? df.format(new Date(item.modTime)) : 'Unknown time',
    ].join(' | ');

    return `
      <article class="result ${index === state.selectedIndex ? 'active' : ''}" data-index="${index}">
        <div class="result-icon">
          <span data-lucide="${item.isDir ? 'folder' : 'file'}"></span>
        </div>
        <div class="result-main">
          <button class="result-name" type="button" data-open="${escapeAttr(item.path)}">${escapeHtml(item.name)}</button>
          <p class="result-path">${escapeHtml(item.path)}</p>
          <div class="result-meta">${escapeHtml(meta)}</div>
        </div>
        <div class="result-actions">
          <button class="icon-button" type="button" title="Open item" data-open="${escapeAttr(item.path)}">
            <span data-lucide="external-link"></span>
          </button>
          <button class="icon-button" type="button" title="Open in VS Code" data-vscode="${escapeAttr(item.path)}">
            <span data-lucide="code-xml"></span>
          </button>
          <button class="icon-button" type="button" title="Reveal in Explorer" data-reveal="${escapeAttr(item.path)}">
            <span data-lucide="folder-open"></span>
          </button>
        </div>
      </article>
    `;
  }).join('');

  els.results.querySelectorAll('[data-open]').forEach((node) => {
    node.addEventListener('click', () => openPath(node.dataset.open));
  });

  els.results.querySelectorAll('[data-reveal]').forEach((node) => {
    node.addEventListener('click', () => revealPath(node.dataset.reveal));
  });

  els.results.querySelectorAll('[data-vscode]').forEach((node) => {
    node.addEventListener('click', () => openInVSCode(node.dataset.vscode));
  });
}

function renderIcons() {
  createIcons({
    icons: lucideIcons,
    attrs: {
      width: 18,
      height: 18,
      'stroke-width': 1.9,
    },
  });

  document.querySelectorAll('[data-icon-spin] svg').forEach((node) => {
    node.classList.add('spin');
  });
}

function scrollSelectedIntoView() {
  const node = els.results.querySelector(`.result[data-index="${state.selectedIndex}"]`);
  node?.scrollIntoView({block: 'nearest'});
}

function hasActiveFilters() {
  return state.filters.typeFilter !== 'all' || Boolean(state.filters.folderPrefix) || Boolean(state.filters.extension);
}

function describeActiveFilters() {
  const parts = [];

  if (state.filters.typeFilter === 'files') {
    parts.push('files only');
  } else if (state.filters.typeFilter === 'folders') {
    parts.push('folders only');
  }

  if (state.filters.folderPrefix) {
    parts.push(formatScopeLabel(state.filters.folderPrefix));
  }

  if (state.filters.extension) {
    parts.push(`extension ${formatExtension(state.filters.extension)}`);
  }

  return parts.join(', ');
}

function formatFilterSuffix() {
  const summary = describeActiveFilters();
  return summary ? ` with ${summary}` : '';
}

function formatScopeLabel(path) {
  if (isDriveRoot(path)) {
    return `drive ${path}`;
  }

  return `inside ${path}`;
}

function formatExtension(value) {
  if (!value) return '';
  return value.startsWith('.') ? value : `.${value}`;
}

function deriveDriveRoots(roots) {
  const seen = new Set();
  const drives = [];

  roots.forEach((root) => {
    const drive = extractDriveRoot(root);
    if (!drive) {
      return;
    }

    const key = drive.toLowerCase();
    if (seen.has(key)) {
      return;
    }

    seen.add(key);
    drives.push(drive);
  });

  return drives.sort((left, right) => left.localeCompare(right));
}

function extractDriveRoot(path) {
  const match = String(path).match(/^[A-Za-z]:[\\/]/);
  if (!match) {
    return '';
  }

  return `${match[0][0].toUpperCase()}:\\`;
}

function isDriveActive(drive) {
  if (!drive) {
    return !state.filters.folderPrefix;
  }

  return isPathWithinDrive(state.filters.folderPrefix, drive);
}

function isPathWithinDrive(path, drive) {
  if (!path || !drive) {
    return false;
  }

  return String(path).toLowerCase().startsWith(drive.toLowerCase());
}

function isDriveRoot(path) {
  return /^[A-Za-z]:\\$/.test(String(path));
}

function formatBytes(value) {
  if (!value || value < 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  const digits = size >= 10 || unit === 0 ? 0 : 1;
  return `${size.toFixed(digits)} ${units[unit]}`;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function escapeAttr(value) {
  return escapeHtml(value);
}

let flashTimer;
function flash(message) {
  els.flash.textContent = message;
  els.flash.classList.add('visible');
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => {
    els.flash.classList.remove('visible');
  }, 2200);
}

init();
