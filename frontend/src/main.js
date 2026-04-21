import './style.css';

import {
  CodeXml,
  ExternalLink,
  File,
  Folder,
  FolderOpen,
  LoaderCircle,
  Search,
  createIcons,
} from 'lucide';
import {
  ImagePreview,
  OpenInVSCode,
  OpenPath,
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
  Search,
};

document.querySelector('#app').innerHTML = `
  <main class="shell">
    <section class="workspace">
      <section class="surface" id="surface">
        <section class="search-stage" id="search-stage">
          <div class="search-stage__topline">
            <label class="query-input" for="search-input">
              <span class="query-icon" data-lucide="search"></span>
              <span class="query-label">Search</span>
              <input id="search-input" class="search-input" type="text" autocomplete="off" spellcheck="false" placeholder="Type a file, folder, extension, or path fragment">
            </label>
          </div>

          <div class="search-stage__filters" id="filterbar">
            <div class="filter-popover">
              <button class="filter-trigger" id="scope-filter-trigger" type="button" data-filter-trigger="scope" aria-expanded="false">
                <span>Scope</span>
                <strong id="scope-filter-value">All drives</strong>
              </button>
              <div class="filter-panel" id="filter-panel-scope" data-filter-panel="scope">
                <div class="filter-options filter-options-drives" id="drive-filters"></div>
              </div>
            </div>

            <div class="filter-popover">
              <button class="filter-trigger" id="type-filter-trigger" type="button" data-filter-trigger="type" aria-expanded="false">
                <span>Type</span>
                <strong id="type-filter-value">Both</strong>
              </button>
              <div class="filter-panel" id="filter-panel-type" data-filter-panel="type">
                <div class="filter-toggle filter-toggle-segmented">
                  <button class="filter-chip active" type="button" data-type-filter="all">Both</button>
                  <button class="filter-chip" type="button" data-type-filter="files">Files</button>
                  <button class="filter-chip" type="button" data-type-filter="folders">Folders</button>
                </div>
              </div>
            </div>

            <div class="filter-popover filter-popover-field">
              <button class="filter-trigger" id="path-filter-trigger" type="button" data-filter-trigger="path" aria-expanded="false">
                <span>Path</span>
                <strong id="path-filter-value">Any</strong>
              </button>
              <div class="filter-panel" id="filter-panel-path" data-filter-panel="path">
                <label class="filter-field" for="folder-filter">
                  <span class="filter-label">Path Focus</span>
                  <input id="folder-filter" class="filter-text" type="text" autocomplete="off" spellcheck="false" placeholder="Optional folder path">
                </label>
              </div>
            </div>

            <div class="filter-popover filter-popover-field">
              <button class="filter-trigger" id="extension-filter-trigger" type="button" data-filter-trigger="extension" aria-expanded="false">
                <span>Ext</span>
                <strong id="extension-filter-value">Any</strong>
              </button>
              <div class="filter-panel" id="filter-panel-extension" data-filter-panel="extension">
                <label class="filter-field" for="extension-filter">
                  <span class="filter-label">Extension</span>
                  <input id="extension-filter" class="filter-text" type="text" autocomplete="off" spellcheck="false" placeholder=".pdf or js">
                </label>
              </div>
            </div>

            <button class="clear-filters-button" id="clear-filters-btn" type="button" hidden>Clear filters</button>
          </div>

          <section class="shortcut-strip" aria-label="Keyboard shortcuts">
            <p class="section-label">Keys</p>
            <div class="shortcut-list">
              <p><kbd>/</kbd><span>Focus search</span></p>
              <p><kbd>Enter</kbd><span>Open selected</span></p>
              <p><kbd>Up/Down</kbd><span>Move through matches</span></p>
            </div>
          </section>
        </section>

        <div class="post-search-empty" id="post-search-empty" hidden>
          <p>Type to see matching files.</p>
        </div>

        <div class="results-shell" id="results-shell">
          <div class="results-toolbar" id="results-toolbar">
            <div class="results-toolbar__copy">
              <p class="section-label">Matches</p>
              <p class="results-note" id="results-note">Type to search the active index.</p>
              <p class="results-filter-summary" id="results-filter-summary"></p>
            </div>
            <div class="view-toggle" aria-label="Result view">
              <button class="view-toggle-button active" type="button" data-view-mode="list">List</button>
              <button class="view-toggle-button" type="button" data-view-mode="mosaic">Mosaic</button>
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
    scopeDrive: '',
    folderPrefix: '',
    extension: '',
  },
  status: null,
  results: [],
  selectedIndex: -1,
  viewMode: 'list',
  openFilter: null,
  hasActivatedSearch: false,
  searchTimer: null,
  isSearching: false,
};

const els = {
  surface: document.getElementById('surface'),
  resultsNote: document.getElementById('results-note'),
  resultsFilterSummary: document.getElementById('results-filter-summary'),
  results: document.getElementById('results'),
  resultsShell: document.getElementById('results-shell'),
  postSearchEmpty: document.getElementById('post-search-empty'),
  search: document.getElementById('search-input'),
  filterbar: document.getElementById('filterbar'),
  filterTriggers: Array.from(document.querySelectorAll('[data-filter-trigger]')),
  filterPanels: Array.from(document.querySelectorAll('[data-filter-panel]')),
  folderFilter: document.getElementById('folder-filter'),
  extensionFilter: document.getElementById('extension-filter'),
  scopeFilterValue: document.getElementById('scope-filter-value'),
  typeFilterValue: document.getElementById('type-filter-value'),
  pathFilterValue: document.getElementById('path-filter-value'),
  extensionFilterValue: document.getElementById('extension-filter-value'),
  driveFilters: document.getElementById('drive-filters'),
  clearFilters: document.getElementById('clear-filters-btn'),
  flash: document.getElementById('flash'),
  typeFilters: Array.from(document.querySelectorAll('[data-type-filter]')),
  viewModeButtons: Array.from(document.querySelectorAll('[data-view-mode]')),
};

const nf = new Intl.NumberFormat();
const imagePreviewCache = new Map();
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
  els.filterTriggers.forEach((node) => {
    node.addEventListener('click', onFilterTriggerClick);
  });
  els.typeFilters.forEach((node) => {
    node.addEventListener('click', onTypeFilterClick);
  });
  els.viewModeButtons.forEach((node) => {
    node.addEventListener('click', onViewModeClick);
  });
  document.addEventListener('keydown', onGlobalKeyDown);
  document.addEventListener('click', onDocumentClick);

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
  if (state.query) {
    state.hasActivatedSearch = true;
  }
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

function onFilterTriggerClick(event) {
  const nextValue = event.currentTarget.dataset.filterTrigger || null;
  state.openFilter = state.openFilter === nextValue ? null : nextValue;
  render();

  if (state.openFilter === 'path') {
    els.folderFilter.focus();
  } else if (state.openFilter === 'extension') {
    els.extensionFilter.focus();
  }
}

function onDriveFilterClick(event) {
  const nextValue = event.currentTarget.dataset.driveFilter || '';
  if (state.filters.scopeDrive === nextValue) {
    return;
  }

  state.filters.scopeDrive = nextValue;
  state.selectedIndex = -1;
  state.openFilter = null;
  scheduleSearch();
}

function onClearFilters() {
  state.filters = {
    typeFilter: 'all',
    scopeDrive: '',
    folderPrefix: '',
    extension: '',
  };
  els.folderFilter.value = '';
  els.extensionFilter.value = '';
  state.selectedIndex = -1;
  state.openFilter = null;
  scheduleSearch();
}

function onViewModeClick(event) {
  const nextValue = event.currentTarget.dataset.viewMode || 'list';
  if (state.viewMode === nextValue) {
    return;
  }

  state.viewMode = nextValue;
  render();
  scrollSelectedIntoView();
}

function onDocumentClick(event) {
  if (!state.openFilter || els.filterbar.contains(event.target)) {
    return;
  }

  state.openFilter = null;
  render();
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
      folderPrefix: getEffectiveFolderPrefix(),
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
  if (event.key === 'Escape' && state.openFilter) {
    state.openFilter = null;
    render();
    return;
  }

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
  renderSearchVisibility();
  renderFilterControls();
  renderFilterPanels();
  renderResultsToolbar();
  renderResults();
  renderIcons();
  hydrateImagePreviews();
}

function renderSearchVisibility() {
  const hasQuery = Boolean(state.query);
  const isInitialIdle = !hasQuery && !state.hasActivatedSearch;
  els.surface.classList.toggle('is-idle', isInitialIdle);
  els.surface.classList.toggle('is-active', !isInitialIdle);
  els.postSearchEmpty.hidden = hasQuery || isInitialIdle;
  els.resultsShell.hidden = !hasQuery;
}

function renderFilterControls() {
  els.typeFilters.forEach((node) => {
    node.classList.toggle('active', node.dataset.typeFilter === state.filters.typeFilter);
  });

  renderDriveFilters();
  els.clearFilters.hidden = !hasActiveFilters();
  els.scopeFilterValue.textContent = formatScopeValue();
  els.typeFilterValue.textContent = formatTypeValue();
  els.pathFilterValue.textContent = state.filters.folderPrefix ? compactPath(state.filters.folderPrefix) : 'Any';
  els.extensionFilterValue.textContent = state.filters.extension ? formatExtension(state.filters.extension) : 'Any';

  els.filterTriggers.forEach((node) => {
    const filterName = node.dataset.filterTrigger;
    node.classList.toggle('is-open', state.openFilter === filterName);
    node.classList.toggle('is-active', isFilterActive(filterName));
    node.setAttribute('aria-expanded', String(state.openFilter === filterName));
  });
}

function renderFilterPanels() {
  els.filterPanels.forEach((node) => {
    node.hidden = node.dataset.filterPanel !== state.openFilter;
  });
}

function renderResultsToolbar() {
  const filterSummary = describeActiveFilters();

  els.resultsFilterSummary.textContent = filterSummary ? `Filtered by ${filterSummary}.` : '';
  els.viewModeButtons.forEach((node) => {
    node.classList.toggle('active', node.dataset.viewMode === state.viewMode);
  });
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
  els.results.classList.toggle('is-list', state.viewMode === 'list');
  els.results.classList.toggle('is-mosaic', state.viewMode === 'mosaic');

  if (!state.query) {
    els.resultsNote.textContent = 'Type to search everywhere.';
    els.results.innerHTML = '';
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

  if (state.viewMode === 'mosaic') {
    renderMosaicResults();
  } else {
    renderListResults();
  }
}

function renderListResults() {
  els.results.innerHTML = state.results.map((item, index) => {
    const meta = [
      item.isDir ? 'Folder' : formatBytes(item.size),
      item.modTime ? df.format(new Date(item.modTime)) : 'Unknown time',
    ].join(' | ');

    return `
      <article class="result ${index === state.selectedIndex ? 'active' : ''}" data-index="${index}">
        ${renderResultMedia(item, 'result-icon')}
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

  bindResultActions();
}

function renderMosaicResults() {
  els.results.innerHTML = state.results.map((item, index) => {
    const meta = [
      item.isDir ? 'Folder' : formatBytes(item.size),
      item.modTime ? df.format(new Date(item.modTime)) : 'Unknown time',
    ].join(' | ');

    return `
      <article class="result-tile ${index === state.selectedIndex ? 'active' : ''}" data-index="${index}">
        <button class="result-tile-main ${isImageResult(item) ? 'has-image-media' : ''}" type="button" data-open="${escapeAttr(item.path)}">
          ${isImageResult(item) ? renderResultMedia(item, 'result-tile-icon', 'span') : ''}
          <span class="result-tile-title">
            ${isImageResult(item) ? '' : renderResultMedia(item, 'result-tile-icon', 'span')}
            <span class="result-tile-name">${escapeHtml(item.name)}</span>
          </span>
          <span class="result-tile-path">${escapeHtml(shortenPath(item.path))}</span>
          <span class="result-tile-meta">${escapeHtml(meta)}</span>
        </button>
        <div class="result-tile-actions">
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

  bindResultActions();
}

function bindResultActions() {
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

function renderResultMedia(item, className, tagName = 'div') {
  const iconName = item.isDir ? 'folder' : 'file';

  if (!isImageResult(item)) {
    return `
      <${tagName} class="${className}">
        <span data-lucide="${iconName}"></span>
      </${tagName}>
    `;
  }

  return `
    <${tagName} class="${className} image-preview" data-preview-path="${escapeAttr(item.path)}">
      <span class="image-preview-fallback" data-lucide="${iconName}"></span>
    </${tagName}>
  `;
}

function hydrateImagePreviews() {
  els.results.querySelectorAll('[data-preview-path]').forEach((node) => {
    const path = node.dataset.previewPath;
    if (!path) {
      return;
    }

    loadImagePreview(path).then((dataUrl) => {
      if (!dataUrl || node.dataset.previewPath !== path) {
        return;
      }

      node.innerHTML = `<img src="${escapeAttr(dataUrl)}" alt="">`;
      node.classList.add('has-preview');
    });
  });
}

function loadImagePreview(path) {
  if (imagePreviewCache.has(path)) {
    const cached = imagePreviewCache.get(path);
    return cached instanceof Promise ? cached : Promise.resolve(cached);
  }

  const request = ImagePreview(path)
    .then((dataUrl) => {
      imagePreviewCache.set(path, dataUrl || '');
      return dataUrl || '';
    })
    .catch(() => {
      imagePreviewCache.set(path, '');
      return '';
    });

  imagePreviewCache.set(path, request);
  return request;
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
  const node = els.results.querySelector(`.result[data-index="${state.selectedIndex}"], .result-tile[data-index="${state.selectedIndex}"]`);
  node?.scrollIntoView({block: 'nearest'});
}

function hasActiveFilters() {
  return state.filters.typeFilter !== 'all'
    || Boolean(state.filters.scopeDrive)
    || Boolean(state.filters.folderPrefix)
    || Boolean(state.filters.extension);
}

function isFilterActive(filterName) {
  if (filterName === 'scope') {
    return Boolean(state.filters.scopeDrive);
  }

  if (filterName === 'path') {
    return Boolean(state.filters.folderPrefix);
  }

  if (filterName === 'type') {
    return state.filters.typeFilter !== 'all';
  }

  if (filterName === 'extension') {
    return Boolean(state.filters.extension);
  }

  return false;
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
  } else if (state.filters.scopeDrive) {
    parts.push(formatScopeLabel(state.filters.scopeDrive));
  }

  if (state.filters.extension) {
    parts.push(`extension ${formatExtension(state.filters.extension)}`);
  }

  return parts.join(', ');
}

function formatScopeValue() {
  if (!state.filters.scopeDrive) {
    return 'All drives';
  }

  return state.filters.scopeDrive;
}

function formatTypeValue() {
  if (state.filters.typeFilter === 'files') {
    return 'Files';
  }

  if (state.filters.typeFilter === 'folders') {
    return 'Folders';
  }

  return 'Both';
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
    return !state.filters.scopeDrive;
  }

  return isPathWithinDrive(state.filters.scopeDrive, drive);
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

function isImageResult(item) {
  if (item.isDir) {
    return false;
  }

  return isImageExtension(item.ext || item.path);
}

function isImageExtension(value) {
  return ['.apng', '.avif', '.bmp', '.gif', '.ico', '.jpg', '.jpeg', '.png', '.svg', '.webp']
    .includes(String(value).toLowerCase().replace(/^.*(\.[^.\\/]+)$/, '$1'));
}

function getEffectiveFolderPrefix() {
  return state.filters.folderPrefix || state.filters.scopeDrive;
}

function compactPath(path) {
  const value = String(path);
  if (value.length <= 22) {
    return value;
  }

  return `...${value.slice(-19)}`;
}

function shortenPath(path) {
  const parts = String(path).split(/[\\/]+/).filter(Boolean);
  if (parts.length <= 3) {
    return String(path);
  }

  return `${parts[0]}\\...\\${parts.slice(-2).join('\\')}`;
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
