const INITIAL_TABLE_ROWS = 150;
const TABLE_ROWS_INCREMENT = 150;
const INITIAL_CARD_COUNT = 120;
const CARD_INCREMENT = 120;
const EMPTY_LOCAL = Object.freeze({ watchlist: false, notes: "", tags: [], color: "" });
const LOCAL_STORAGE_KEY = "statos.local.v1";
const DEFAULT_SORT = Object.freeze({ key: "marketCap", dir: "desc" });
const VALID_VIEWS = ["tickers", "explorer", "themes", "supply", "sectors", "watchlist", "unclassified", "exports"];
const VALID_LOCAL_FILTERS = ["watchlist", "tagged", "coloured"];
const VALID_SORT_KEYS = ["ticker", "name", "sector", "industry", "marketCap"];

const DEFAULT_EXPORTS = [
  "app_bootstrap.json",
  "tickers_index.json",
  "explorer_index.json",
  "catalogue.json",
  "companies.json",
  "sectors.json",
  "industries.json",
  "themes.json",
  "supply_chains.json",
  "search_index.json",
  "securities.json",
  "listings.json",
  "relationships.json",
  "unclassified.json",
  "review_queues.json",
  "review_summary.json",
  "tickers.csv",
  "securities.csv",
  "listings.csv",
  "unclassified.csv",
  "taxonomy_issues.csv",
  "enrichment_issues.csv",
  "identity_issues.csv",
  "enrichment_failures.csv",
  "stale_reviews.csv",
  "suggested_classification_overrides.csv",
  "suggested_exposures.csv",
  "suggested_ticker_overrides.csv",
  "suggested_identity_overrides.csv",
  "build_manifest.json"
];

const state = {
  bootstrap: null,
  tickers: null,
  explorerIndex: null,
  reviewQueues: null,
  reviewSummary: null,
  visibleReviewRows: [],
  detail: null,
  view: "tickers",
  query: "",
  theme: "",
  supplyTheme: "",
  explorerType: "",
  explorerGroup: "",
  sector: "",
  localFilter: "",
  sort: { ...DEFAULT_SORT },
  reviewFilters: { queue: "", reason: "", severity: "", gap: "", search: "" },
  listWindows: {},
  savedViewSelection: "",
  comparisonOpen: false,
  relationships: null,
  relationshipsError: "",
  modalTicker: "",
  statusTimer: null,
  promises: {},
  maps: {
    themeById: new Map(),
    tickerById: new Map(),
    firstTickerByCompany: new Map(),
    firstTickerByIsin: new Map(),
    tickersByCompany: new Map(),
    tickersByIsin: new Map(),
    companyById: new Map(),
    securityById: new Map(),
    listingById: new Map(),
    explorerGroupById: new Map(),
    layerByThemeAndId: new Map(),
    relationshipsByTicker: new Map()
  },
  local: loadLocal()
};

const $ = (selector) => document.querySelector(selector);
const content = $("#content");
const applyReviewSearch = debounce((value) => {
  state.reviewFilters.search = value.trim().toLowerCase();
  resetListWindows();
  render();
}, 180);

init();

async function init() {
  bindEvents();
  renderSavedViewControls();
  renderComparisonControls();
  renderAppLoading();
  try {
    await loadBootstrap();
    hydrateFilters();
    render();
  } catch (error) {
    renderFatalLoadError(error);
  }
}

function bindEvents() {
  const applySearch = debounce((value) => {
    state.query = value.trim().toLowerCase();
    resetListWindows();
    render();
  }, 180);

  $("#globalSearch").addEventListener("input", (event) => {
    applySearch(event.target.value);
  });
  $("#themeFilter").addEventListener("change", (event) => {
    state.theme = event.target.value;
    if (state.theme) {
      state.supplyTheme = state.theme;
      if (state.view === "explorer") selectExplorerThemeGroup(state.theme);
    }
    resetListWindows();
    render();
  });
  $("#sectorFilter").addEventListener("change", (event) => {
    state.sector = event.target.value;
    resetListWindows();
    render();
  });
  $("#localFilter").addEventListener("change", (event) => {
    state.localFilter = event.target.value;
    resetListWindows();
    render();
  });
  document.querySelector(".tabs").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-view]");
    if (!button) return;
    state.view = button.dataset.view;
    if (state.view === "explorer" && state.theme) selectExplorerThemeGroup(state.theme);
    resetListWindows();
    render();
  });
  content.addEventListener("click", handleContentClick);
  content.addEventListener("change", handleContentChange);
  content.addEventListener("input", handleContentInput);
  $("#modalBody").addEventListener("click", handleContentClick);
  $("#importFile").addEventListener("change", importLocalFile);
  $("#saveCurrentView").addEventListener("click", saveCurrentView);
  $("#savedViewSelect").addEventListener("change", (event) => {
    state.savedViewSelection = event.target.value;
    renderSavedViewControls();
  });
  $("#applySavedView").addEventListener("click", applySelectedSavedView);
  $("#deleteSavedView").addEventListener("click", deleteSelectedSavedView);
  $("#comparisonToggle").addEventListener("click", toggleComparisonPanel);
  $("#closeComparison").addEventListener("click", closeComparisonPanel);
  $("#clearComparison").addEventListener("click", clearComparison);
  $("#comparisonBackdrop").addEventListener("click", closeComparisonPanel);
  $("#comparisonBody").addEventListener("click", handleComparisonClick);
  $("#detailModal").addEventListener("close", () => {
    state.modalTicker = "";
  });
  $("#detailModal").addEventListener("click", closeModalOnBackdropClick);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && state.comparisonOpen) closeComparisonPanel();
  });
}

async function loadBootstrap() {
  state.bootstrap = await fetchJSON("data/app_bootstrap.json");
  indexBootstrap();
}

function indexBootstrap() {
  state.maps.themeById = new Map((state.bootstrap.themes || []).map((theme) => [theme.id, theme]));
  state.maps.layerByThemeAndId = new Map();
  for (const chain of state.bootstrap.supplyChains || []) {
    for (const layer of chain.layers || []) {
      state.maps.layerByThemeAndId.set(`${chain.themeId}:${layer.id}`, layer);
    }
  }
}

function hydrateFilters() {
  const themes = state.bootstrap.themes || [];
  const sectors = state.bootstrap.sectors || [];
  const themeCounts = state.bootstrap.themeCounts || {};
  const themeSelect = $("#themeFilter");
  themeSelect.innerHTML = `<option value="">All themes</option>` + themes.map((theme) => `<option value="${esc(theme.id)}">${esc(theme.name)} (${num(themeCounts[theme.id] || 0)})</option>`).join("");

  const sectorSelect = $("#sectorFilter");
  sectorSelect.innerHTML = `<option value="">All sectors</option>` + sectors.map((sector) => `<option value="${esc(sector.name)}">${esc(sector.name)} (${num(sector.count)})</option>`).join("");
  renderLocalFilterOptions();
}

function renderLocalFilterOptions() {
  const current = state.localFilter;
  const counts = localStateCounts();
  const localSelect = $("#localFilter");
  localSelect.innerHTML = `
    <option value="">All local states</option>
    <option value="watchlist">Watchlist (${num(counts.watchlist)})</option>
    <option value="tagged">Tagged (${num(counts.tagged)})</option>
    <option value="coloured">Colour labelled (${num(counts.coloured)})</option>
  `;
  localSelect.value = current;
}

function localStateCounts() {
  const counts = { watchlist: 0, tagged: 0, coloured: 0 };
  for (const local of Object.values(state.local.tickers || {})) {
    if (local.watchlist) counts.watchlist += 1;
    if ((local.tags || []).length) counts.tagged += 1;
    if (local.color) counts.coloured += 1;
  }
  return counts;
}

function renderSavedViewControls() {
  const select = $("#savedViewSelect");
  if (!select) return;
  const views = savedViews();
  const current = state.savedViewSelection || select.value;
  select.innerHTML = `<option value="">Saved views (${num(views.length)})</option>` + views.map((view) => {
    const label = `${view.name} - ${viewLabel(view.snapshot?.view || view.view)}`;
    return `<option value="${esc(view.id)}">${esc(label)}</option>`;
  }).join("");
  state.savedViewSelection = views.some((view) => view.id === current) ? current : "";
  select.value = state.savedViewSelection;
  $("#applySavedView").disabled = !state.savedViewSelection;
  $("#deleteSavedView").disabled = !state.savedViewSelection;
}

function renderComparisonControls() {
  const count = comparisonTickers().length;
  $("#comparisonCount").textContent = num(count);
  $("#comparisonToggle").classList.toggle("primary", count > 0 || state.comparisonOpen);
  $("#comparisonToggle").setAttribute("aria-expanded", state.comparisonOpen ? "true" : "false");
  $("#clearComparison").disabled = count === 0;
}

function render() {
  if (!state.bootstrap) return;
  renderSavedViewControls();
  renderComparisonControls();
  renderMeta();
  renderMetrics();
  syncTabs();
  if (state.view === "tickers") renderTickers();
  if (state.view === "explorer") renderExplorer();
  if (state.view === "themes") renderThemes();
  if (state.view === "supply") renderSupply();
  if (state.view === "sectors") renderSectors();
  if (state.view === "watchlist") renderWatchlist();
  if (state.view === "unclassified") renderUnclassified();
  if (state.view === "exports") renderExports();
  if (state.comparisonOpen) renderComparisonPanel();
}

function syncTabs() {
  document.querySelectorAll(".tabs button").forEach((tab) => {
    const active = tab.dataset.view === state.view;
    tab.classList.toggle("active", active);
    tab.setAttribute("aria-current", active ? "page" : "false");
  });
}

function renderAppLoading() {
  $("#buildMeta").textContent = "Loading app bootstrap...";
  $("#metrics").innerHTML = "";
  content.innerHTML = loadingBlock("Loading generated data", "Fetching the small startup slice.");
}

function renderFatalLoadError(error) {
  content.innerHTML = `<div class="empty">Unable to load generated data. Run <code>go run ./cmd/statos-build sample</code> from the repo root and serve the <code>site</code> directory.</div>`;
  $("#buildMeta").textContent = error.message;
}

function renderMeta() {
  const manifest = state.bootstrap.manifest || {};
  $("#buildMeta").textContent = `Built ${formatDate(manifest.builtAt)} - ${num(manifest.instrumentCount)} instruments - ${num(manifest.unclassifiedCount)} unclassified - ${manifest.trading212Environment || "unknown"} source`;
}

function renderMetrics() {
  const manifest = state.bootstrap.manifest || {};
  $("#metrics").innerHTML = [
    metric(manifest.instrumentCount, "Trading 212 tickers"),
    metric(manifest.companyCount, "Companies"),
    metric(manifest.securityCount, "Securities"),
    metric(manifest.themeCount, "Themes"),
    metric(manifest.exposureCount, "Manual exposures"),
    metric(manifest.enrichmentFailed, "Enrichment failures")
  ].join("");
}

function metric(value, label) {
  return `<div class="metric"><strong>${num(value)}</strong><span>${esc(label)}</span></div>`;
}

function renderTickers() {
  if (!state.tickers) {
    renderTickerIndexLoading("Loading ticker index", "Fetching compact table rows for filtering and sorting.");
    return;
  }
  const rows = filteredTickers();
  const visible = Math.min(visibleCount("tickers", INITIAL_TABLE_ROWS), rows.length);
  content.innerHTML = `
    <div class="panel-head">
      <h2>Tickers</h2>
      <div class="panel-actions">
        <p class="muted">${tickerCountLabel(visible, rows.length, state.tickers.length)}</p>
      </div>
    </div>
    ${renderGlobalFilterStatus()}
    ${renderTickerTable("tickers", rows)}
  `;
}

function renderTickerIndexLoading(title, detail) {
  ensureTickerIndex().then(render).catch(showContentError);
  content.innerHTML = loadingBlock(title, detail);
}

function tickerCountLabel(visible, matched, total) {
  if (matched === total) return `Showing ${num(visible)} of ${num(total)} loaded rows`;
  return `Showing ${num(visible)} of ${num(matched)} matching rows from ${num(total)} loaded rows`;
}

function hasActiveTickerFilters() {
  return Boolean(state.query || state.theme || state.sector || state.localFilter);
}

function renderGlobalFilterStatus() {
  const filters = [];
  if (state.query) filters.push(`search: ${state.query}`);
  if (state.theme) filters.push(`theme: ${themeName(state.theme)}`);
  if (state.sector) filters.push(`sector: ${state.sector}`);
  if (state.localFilter) filters.push(`local: ${localFilterLabel(state.localFilter)}`);
  if (!filters.length) return "";
  return `
    <div class="filter-status">
      <div class="active-filters">${chips(filters)}</div>
      <button class="small-button" data-action="clear-filters">Clear filters</button>
    </div>
  `;
}

function renderTickerTable(listID, rows) {
  if (!rows.length) return renderTickerEmptyState();
  const visible = visibleCount(listID, INITIAL_TABLE_ROWS);
  const page = rows.slice(0, visible);
  return `
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            ${sortableHead("ticker", "Ticker")}
            ${sortableHead("name", "Company / security")}
            ${sortableHead("sector", "Sector")}
            ${sortableHead("industry", "Industry")}
            <th>Themes</th>
            ${sortableHead("marketCap", "Market cap")}
            <th>Local</th>
          </tr>
        </thead>
        <tbody>
          ${page.map(renderTickerRow).join("")}
        </tbody>
      </table>
    </div>
    ${renderListFooter(listID, rows.length, visible, "rows", TABLE_ROWS_INCREMENT)}
  `;
}

function sortableHead(key, label) {
  const sorted = state.sort.key === key;
  const marker = sorted ? (state.sort.dir === "asc" ? " (asc)" : " (desc)") : "";
  const ariaSort = sorted ? (state.sort.dir === "asc" ? "ascending" : "descending") : "none";
  return `<th aria-sort="${ariaSort}"><button class="ticker-link sort-button" data-action="sort" data-key="${esc(key)}" aria-label="Sort by ${esc(label)}">${esc(label)}<span class="sort-marker">${esc(marker)}</span></button></th>`;
}

function renderTickerEmptyState() {
  return `
    <div class="empty">
      <p>No tickers match the current filters.</p>
    </div>
  `;
}

function renderTickerRow(ticker) {
  const local = getLocal(ticker.ticker);
  return `
    <tr>
      <td><button class="cell-open-button ticker-cell-button" data-action="open" data-ticker="${esc(ticker.ticker)}">${esc(ticker.ticker)}</button></td>
      <td>
        <button class="cell-open-button name-cell-button" data-action="open" data-ticker="${esc(ticker.ticker)}">
          <strong>${esc(ticker.name)}</strong>
          <span class="muted">${esc(ticker.isin || "No ISIN")} - ${esc(ticker.currencyCode || "")} ${esc(ticker.exchangeName || ticker.exchangeCode || "")}</span>
        </button>
      </td>
      <td>${esc(ticker.sector || "Unclassified")}</td>
      <td>${esc(ticker.industry || "Unclassified")}</td>
      <td>${chips(themeNames(ticker.themeIds))}</td>
      <td>${formatMarketCap(ticker.marketCap)}</td>
      <td>
        ${localBadges(local)}
        <div class="row-actions">
          <button class="small-button" data-action="watch" data-ticker="${esc(ticker.ticker)}">${local.watchlist ? "Remove" : "Watch"}</button>
          ${compareButton(ticker.ticker)}
        </div>
      </td>
    </tr>
  `;
}

function renderExplorer() {
  if (!state.tickers || !state.explorerIndex) {
    Promise.all([ensureTickerIndex(), ensureExplorerIndex()]).then(render).catch(showContentError);
    content.innerHTML = loadingBlock("Loading explorer", "Fetching grouped ticker links for sectors, pipelines, categories, and relationships.");
    return;
  }
  const groups = state.explorerIndex.groups || [];
  if (!groups.length) {
    content.innerHTML = `<div class="empty">No explorer groups are available.</div>`;
    return;
  }
  const types = uniqueStrings(groups.map((group) => group.type));
  const typeCounts = countBy(groups, "type");
  if (!state.explorerType || !types.includes(state.explorerType)) state.explorerType = types[0];
  const typeGroups = groups.filter((group) => group.type === state.explorerType);
  if (!state.explorerGroup || !typeGroups.some((group) => group.id === state.explorerGroup)) {
    state.explorerGroup = typeGroups[0]?.id || "";
  }
  const group = state.maps.explorerGroupById.get(state.explorerGroup) || typeGroups[0];
  const rows = explorerRows(group);
  const visible = Math.min(visibleCount(explorerListID(group), INITIAL_TABLE_ROWS), rows.length);
  content.innerHTML = `
    <div class="panel-head">
      <div>
        <h2>Explorer</h2>
        <p class="muted">${group ? esc(explorerGroupSubtitle(group, rows.length)) : "Select a group"}</p>
      </div>
      <div class="panel-actions">
        <p class="muted">${tickerCountLabel(visible, rows.length, group?.count || 0)}</p>
      </div>
    </div>
    ${renderGlobalFilterStatus()}
    ${renderExplorerToolbar(types, typeGroups, group, typeCounts)}
    ${renderExplorerBreadcrumbs(group, rows.length)}
    ${renderExplorerSummary(group)}
    ${group ? renderTickerTable(explorerListID(group), rows) : `<div class="empty">Select a group to inspect related tickers.</div>`}
  `;
}

function renderExplorerToolbar(types, groups, selectedGroup, typeCounts) {
  return `
    <div class="explorer-toolbar">
      <select data-explorer-filter="type" aria-label="Explorer group type">
        ${types.map((type) => `<option value="${esc(type)}" ${type === state.explorerType ? "selected" : ""}>${esc(explorerTypeLabel(type))} (${num(typeCounts[type] || 0)})</option>`).join("")}
      </select>
      <select data-explorer-filter="group" aria-label="Explorer group">
        ${groups.map((group) => `<option value="${esc(group.id)}" ${selectedGroup && group.id === selectedGroup.id ? "selected" : ""}>${esc(explorerGroupOptionLabel(group))}</option>`).join("")}
      </select>
    </div>
  `;
}

function renderExplorerSummary(group) {
  if (!group) return "";
  const details = [
    explorerTypeLabel(group.type),
    group.parentLabel,
    group.edgeCount ? `${num(group.edgeCount)} reviewed edges` : "",
    `${num(group.count)} tickers`
  ].filter(Boolean);
  return `
    <div class="group-summary">
      <div>
        <strong>${esc(group.label)}</strong>
        ${group.description ? `<span>${esc(group.description)}</span>` : ""}
      </div>
      <div class="chips">${chips(details)}</div>
    </div>
  `;
}

function renderExplorerBreadcrumbs(group, matchedCount) {
  if (!group) return "";
  const crumbs = explorerBreadcrumbItems(group);
  if (!crumbs.length) return "";
  crumbs.push({ label: explorerBreadcrumbTickerLabel(group, matchedCount) });
  return `
    <nav class="breadcrumbs" aria-label="Pipeline breadcrumbs">
      ${crumbs.map((crumb, index) => `
        ${index > 0 ? `<span class="breadcrumb-separator" aria-hidden="true">&gt;</span>` : ""}
        ${crumb.groupId
          ? `<button class="breadcrumb-link" data-action="explore-group" data-group-id="${esc(crumb.groupId)}">${esc(crumb.label)}</button>`
          : `<span class="breadcrumb-current">${esc(crumb.label)}</span>`}
      `).join("")}
    </nav>
  `;
}

function explorerBreadcrumbItems(group) {
  if (group.type === "layer") {
    const themeID = group.parentId || group.id.split(":")[1] || "";
    return [
      { label: group.parentLabel || themeName(themeID), groupId: themeID ? `theme:${themeID}` : "" },
      { label: group.label }
    ];
  }
  if (group.type === "theme") {
    return [{ label: group.label || themeName(group.value), groupId: group.id }];
  }
  if (group.parentLabel) {
    return [
      { label: group.parentLabel },
      { label: group.label }
    ];
  }
  return [{ label: group.label }];
}

function explorerBreadcrumbTickerLabel(group, matchedCount) {
  const total = Number(group.count || 0);
  const matched = Number(matchedCount || 0);
  if (!total || matched === total) return `${num(total || matched)} tickers`;
  return `${num(matched)} of ${num(total)} tickers`;
}

function explorerRows(group) {
  if (!group) return [];
  const rows = (group.tickers || []).map((id) => tickerByID(id)).filter(Boolean).filter(explorerTickerMatches);
  rows.sort((a, b) => compareValues(a[state.sort.key], b[state.sort.key]) * (state.sort.dir === "asc" ? 1 : -1));
  return rows;
}

function explorerTickerMatches(ticker) {
  if (state.theme && !(ticker.themeIds || []).includes(state.theme)) return false;
  if (state.sector && ticker.sector !== state.sector) return false;
  const local = getLocal(ticker.ticker);
  if (state.localFilter === "watchlist" && !local.watchlist) return false;
  if (state.localFilter === "tagged" && !(local.tags || []).length) return false;
  if (state.localFilter === "coloured" && !local.color) return false;
  if (!state.query) return true;
  const localText = `${(local.tags || []).join(" ")} ${local.notes || ""}`.toLowerCase();
  return (ticker._searchText || "").includes(state.query) || localText.includes(state.query);
}

function explorerListID(group) {
  return group ? `explorer:${group.id}` : "explorer";
}

function explorerGroupSubtitle(group, matchedCount) {
  const total = group.count || 0;
  if (matchedCount === total) return `${explorerTypeLabel(group.type)} group with ${num(total)} linked tickers`;
  return `${explorerTypeLabel(group.type)} group with ${num(matchedCount)} matching tickers from ${num(total)} linked tickers`;
}

function explorerGroupOptionLabel(group) {
  const prefix = group.parentLabel ? `${group.parentLabel} / ` : "";
  const suffix = group.edgeCount ? `, ${num(group.edgeCount)} edges` : "";
  return `${prefix}${group.label} (${num(group.count)}${suffix})`;
}

function explorerTypeLabel(type) {
  const labels = {
    theme: "Pipeline",
    layer: "Pipeline layer",
    sector: "Sector",
    industry: "Industry",
    category: "Category",
    flag: "Structure flag",
    relationship: "Relationship"
  };
  return labels[type] || type;
}

function renderThemes() {
  const themes = state.bootstrap.themes || [];
  const counts = state.bootstrap.themeCounts || {};
  content.innerHTML = `
    <div class="panel-head"><h2>Themes</h2><p class="muted">${num(themes.length)} taxonomy pillars</p></div>
    <div class="grid">
      ${themes.map((theme) => `
        <article class="card">
          <h3>${esc(theme.name)}</h3>
          <p>${esc(theme.description || "")}</p>
          <div class="chips" style="margin-top:10px">${chips([`${num(counts[theme.id] || 0)} mapped tickers`])}</div>
          <div class="card-actions"><button class="small-button" data-action="explore-group" data-group-id="theme:${esc(theme.id)}">Open tickers</button></div>
        </article>
      `).join("")}
    </div>
  `;
}

function renderSupply() {
  if (!state.tickers) {
    renderTickerIndexLoading("Loading supply-chain tickers", "Fetching compact ticker rows to resolve exposure cards.");
    return;
  }
  const chains = state.bootstrap.supplyChains || [];
  if (!chains.length) {
    content.innerHTML = `<div class="empty">No supply chains are defined yet.</div>`;
    return;
  }
  const selectedTheme = state.theme || state.supplyTheme || chains[0].themeId;
  const chain = chains.find((item) => item.themeId === selectedTheme) || chains[0];
  content.innerHTML = `
    <div class="supply-toolbar">
      <div>
        <h2>${esc(chain.name)}</h2>
        <p class="muted">${esc(chain.description || "")}</p>
      </div>
      <select id="supplyThemeSelect" aria-label="Supply chain theme">
        ${chains.map((item) => `<option value="${esc(item.themeId)}" ${item.themeId === chain.themeId ? "selected" : ""}>${esc(supplyChainOptionLabel(item))}</option>`).join("")}
      </select>
    </div>
    ${renderGlobalFilterStatus()}
    <div class="supply-map">
      ${(chain.layers || []).map((layer) => renderLayer(chain.themeId, layer)).join("")}
    </div>
  `;
  $("#supplyThemeSelect").addEventListener("change", (event) => {
    state.supplyTheme = event.target.value;
    state.theme = event.target.value;
    $("#themeFilter").value = state.theme;
    resetListWindows();
    renderSupply();
  });
}

function renderLayer(themeID, layer) {
  const allExposures = (state.bootstrap.exposures || []).filter((exposure) => exposure.themeId === themeID && exposure.layerId === layer.id);
  const exposures = allExposures.filter(supplyExposureMatches);
  const listID = `supply:${themeID}:${layer.id}`;
  const visible = visibleCount(listID, INITIAL_CARD_COUNT);
  const cards = exposures.slice(0, visible).map((exposure) => {
    const ticker = tickerForExposure(exposure);
    const title = ticker ? ticker.name : (exposure.companyId || exposure.ticker || exposure.isin || "Unresolved exposure");
    const width = Math.min(280, 140 + Number(exposure.exposureScore || 0) * 26);
    const confidenceClass = exposure.confidence && exposure.confidence.includes("high") ? "high" : exposure.confidence && exposure.confidence.includes("medium") ? "medium" : "low";
    return `
      <article class="supply-card ${confidenceClass}" style="--card-width:${width}px">
        ${ticker ? `<button data-action="open" data-ticker="${esc(ticker.ticker)}">${esc(title)}</button>` : `<strong>${esc(title)}</strong>`}
        <div class="meta">${ticker ? esc(ticker.ticker) : esc(exposure.companyId || "")} - score ${esc(String(exposure.exposureScore || 0))}</div>
        <div class="chips" style="margin-top:8px">${chips([exposure.confidence || "unrated", ticker ? ticker.industry : "manual"])}</div>
        ${ticker ? `<div class="card-actions">${compareButton(ticker.ticker)}</div>` : ""}
      </article>
    `;
  }).join("");
  return `
    <section class="layer-row">
      <div class="layer-label">
        <strong>${esc(layer.name)}</strong>
        <span>${esc(layer.description || "")}</span>
        <span>${num(exposures.length)} of ${num(allExposures.length)} mapped tickers shown</span>
        <button class="small-button" data-action="explore-group" data-group-id="layer:${esc(themeID)}:${esc(layer.id)}">Open layer</button>
      </div>
      <div>
        <div class="layer-cards">${cards || renderLayerEmptyState(allExposures.length)}</div>
        ${renderListFooter(listID, exposures.length, visible, "cards", CARD_INCREMENT)}
      </div>
    </section>
  `;
}

function renderLayerEmptyState(total) {
  const message = total > 0
    ? "No mapped tickers match the current filters in this layer."
    : "No mapped tickers in this layer yet.";
  return `<div class="empty">${esc(message)}</div>`;
}

function supplyExposureMatches(exposure) {
  const ticker = tickerForExposure(exposure);
  if (state.sector && (!ticker || ticker.sector !== state.sector)) return false;
  if (state.localFilter) {
    if (!ticker) return false;
    const local = getLocal(ticker.ticker);
    if (state.localFilter === "watchlist" && !local.watchlist) return false;
    if (state.localFilter === "tagged" && !(local.tags || []).length) return false;
    if (state.localFilter === "coloured" && !local.color) return false;
  }
  if (!state.query) return true;
  const local = ticker ? getLocal(ticker.ticker) : EMPTY_LOCAL;
  const localText = `${(local.tags || []).join(" ")} ${local.notes || ""}`.toLowerCase();
  const exposureText = [
    exposure.ticker,
    exposure.companyId,
    exposure.isin,
    exposure.layerId,
    exposure.confidence,
    exposure.rationale
  ].filter(Boolean).join(" ").toLowerCase();
  return (ticker?._searchText || "").includes(state.query) || localText.includes(state.query) || exposureText.includes(state.query);
}

function supplyChainOptionLabel(chain) {
  const count = state.bootstrap.themeCounts?.[chain.themeId] || 0;
  return `${themeName(chain.themeId)} (${num(count)} tickers, ${num((chain.layers || []).length)} layers)`;
}

function renderSectors() {
  const cards = [
    ...(state.bootstrap.sectors || []).map((group) => ({ ...group, kind: "Sector" })),
    ...(state.bootstrap.industries || []).map((group) => ({ ...group, kind: "Industry" }))
  ];
  content.innerHTML = `
    <div class="panel-head"><h2>Sector and industry explorer</h2><p class="muted">Counts use current generated catalogue data.</p></div>
    ${renderBatchedCards("sector-industry", cards, renderGroupCard)}
  `;
}

function renderGroupCard(group) {
  const extra = Math.max(0, Number(group.count || 0) - (group.tickers || []).length);
  const chipValues = [...(group.tickers || [])];
  if (extra > 0) chipValues.push(`+${num(extra)} more`);
  const groupType = group.kind.toLowerCase();
  return `
    <article class="card">
      <h3>${esc(group.kind)}: ${esc(group.name)} (${num(group.count)})</h3>
      <div class="chips">${chips(chipValues)}</div>
      <div class="card-actions"><button class="small-button" data-action="explore-group" data-group-id="${esc(groupType)}:${esc(group.id)}">Open tickers</button></div>
    </article>
  `;
}

function renderWatchlist() {
  if (!state.tickers) {
    renderTickerIndexLoading("Loading watchlist rows", "Fetching compact ticker rows before applying local watchlist state.");
    return;
  }
  const rows = filteredTickers().filter((ticker) => getLocal(ticker.ticker).watchlist);
  content.innerHTML = `
    <div class="panel-head">
      <h2>Watchlist</h2>
      <p class="muted">${num(rows.length)} watched tickers</p>
    </div>
    ${renderGlobalFilterStatus()}
    ${renderTickerTable("watchlist", rows)}
  `;
}

function renderUnclassified() {
  if (!state.reviewQueues) {
    ensureReviewQueues().then(render).catch(showContentError);
    content.innerHTML = loadingBlock("Loading review queues", "Fetching structured taxonomy, enrichment, identity, and stale-review rows.");
    return;
  }
  const rows = filteredReviewRows();
  state.visibleReviewRows = rows;
  content.innerHTML = `
    <div class="panel-head">
      <h2>Review queues</h2>
      <p class="muted">${num(rows.length)} shown from ${num(state.reviewQueues.length)} structured rows</p>
    </div>
    ${renderGlobalFilterStatus()}
    ${renderReviewToolbar()}
    ${renderReviewFilterStatus()}
    ${renderReasonCounts()}
    ${renderReviewTable(rows)}
  `;
}

function renderReviewToolbar() {
  const filters = state.reviewFilters;
  const reasonCounts = state.reviewSummary?.byReasonCode || countBy(state.reviewQueues, "reasonCode");
  const queueCounts = state.reviewSummary?.byQueue || countBy(state.reviewQueues, "queue");
  const severityCounts = state.reviewSummary?.bySeverity || countBy(state.reviewQueues, "severity");
  const reasons = sortedKeys(reasonCounts);
  const queues = sortedKeys(queueCounts);
  const severities = ["high", "medium", "low"].filter((severity) => severityCounts[severity] || state.reviewQueues.some((row) => row.severity === severity));
  return `
    <div class="review-toolbar">
      <input id="reviewSearch" type="search" aria-label="Review queue search" value="${esc(filters.search)}" placeholder="Search review rows, identifiers, source files, or actions">
      <select data-review-filter="queue" aria-label="Review queue filter">
        <option value="">All queues</option>
        ${queues.map((queue) => `<option value="${esc(queue)}" ${filters.queue === queue ? "selected" : ""}>${esc(labelForQueue(queue))} (${num(queueCounts[queue])})</option>`).join("")}
      </select>
      <select data-review-filter="reason" aria-label="Reason code filter">
        <option value="">All reasons</option>
        ${reasons.map((reason) => `<option value="${esc(reason)}" ${filters.reason === reason ? "selected" : ""}>${esc(reason)} (${num(reasonCounts[reason])})</option>`).join("")}
      </select>
      <select data-review-filter="severity" aria-label="Severity filter">
        <option value="">All severities</option>
        ${severities.map((severity) => `<option value="${esc(severity)}" ${filters.severity === severity ? "selected" : ""}>${esc(severity)} (${num(severityCounts[severity])})</option>`).join("")}
      </select>
      <select data-review-filter="gap" aria-label="Taxonomy gap filter">
        <option value="">All gaps</option>
        <option value="sector" ${filters.gap === "sector" ? "selected" : ""}>Sector gap</option>
        <option value="industry" ${filters.gap === "industry" ? "selected" : ""}>Industry gap</option>
        <option value="theme" ${filters.gap === "theme" ? "selected" : ""}>Theme gap</option>
      </select>
    </div>
  `;
}

function renderReviewFilterStatus() {
  const filters = state.reviewFilters;
  const active = [];
  if (filters.search) active.push(`review search: ${filters.search}`);
  if (filters.queue) active.push(`queue: ${labelForQueue(filters.queue)}`);
  if (filters.reason) active.push(`reason: ${filters.reason}`);
  if (filters.severity) active.push(`severity: ${filters.severity}`);
  if (filters.gap) active.push(`gap: ${filters.gap}`);
  if (!active.length) return "";
  return `
    <div class="filter-status review-filter-status">
      <div class="active-filters">${chips(active)}</div>
      <button class="small-button" data-action="clear-review-filters">Clear review filters</button>
    </div>
  `;
}

function renderReasonCounts() {
  const counts = state.reviewSummary?.byReasonCode || countBy(state.reviewQueues, "reasonCode");
  const items = sortedEntries(counts).slice(0, 18);
  if (!items.length) return "";
  return `
    <div class="reason-counts">
      ${items.map(([reason, count]) => `<button class="chip" data-action="review-reason" data-reason="${esc(reason)}">${esc(reason)} ${num(count)}</button>`).join("")}
    </div>
  `;
}

function renderReviewTable(rows) {
  if (!rows.length) {
    return `
      <div class="empty">
        <p>No review rows match the current filters.</p>
      </div>
    `;
  }
  const listID = "review-queues";
  const visible = visibleCount(listID, INITIAL_TABLE_ROWS);
  const page = rows.slice(0, visible);
  return `
    <div class="table-wrap">
      <table>
        <thead><tr><th>Queue</th><th>Security</th><th>Classification</th><th>Action</th><th>Manual row</th></tr></thead>
        <tbody>
          ${page.map((row, index) => `
            <tr>
              <td>
                <div class="chips">
                  <span class="chip">${esc(labelForQueue(row.queue))}</span>
                  <span class="chip ${esc(row.severity || "")}">${esc(row.severity || "unknown")}</span>
                </div>
                <div class="muted reason-code">${esc(row.reasonCode || "")}</div>
              </td>
              <td>
                ${row.ticker ? `<button class="ticker-link" data-action="open" data-ticker="${esc(row.ticker)}">${esc(row.ticker)}</button>` : `<strong>${esc(row.companyId || row.securityId || row.isin || "Unresolved")}</strong>`}
                <div><strong>${esc(row.name || "")}</strong></div>
                <div class="muted">${esc([row.isin, row.companyId, row.securityId].filter(Boolean).join(" - "))}</div>
              </td>
              <td>
                <div>${esc(row.sector || "No sector")} / ${esc(row.industry || "No industry")}</div>
                <div class="chips review-chips">${chips([...(row.themeIds || []), ...(row.layerIds || [])])}</div>
              </td>
              <td>
                <div>${esc(row.suggestedAction || "")}</div>
                <div class="muted">${esc(sourceLabel(row))}</div>
              </td>
              <td>
                ${row.suggestedCsvRow ? `
                  <button class="small-button" data-action="copy-review-row" data-review-index="${esc(String(index))}">Copy row</button>
                  <div class="muted">${esc(row.suggestedManualFile || "")}</div>
                  <code class="csv-snippet">${esc(row.suggestedCsvRow)}</code>
                ` : `<span class="muted">${esc(row.suggestedManualFile || "No template")}</span>`}
              </td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    </div>
    ${renderListFooter(listID, rows.length, visible, "rows", TABLE_ROWS_INCREMENT)}
  `;
}

function renderExports() {
  const files = (state.bootstrap.generatedFiles || DEFAULT_EXPORTS.map((name) => ({ path: `site/data/${name}` })));
  content.innerHTML = `
    <div class="panel-head"><h2>Data exports</h2><p class="muted">Generated files are static and committed under site/data.</p></div>
    <div class="exports">
      ${files.map(renderExportLink).join("")}
    </div>
    <div class="panel-head local-export-head">
      <div>
        <h2>Local browser data</h2>
        <p class="muted">${localExportSummary()}</p>
      </div>
      <div class="chips">
        <button class="small-button primary" data-action="export-local">Export local JSON</button>
        <button class="small-button" data-action="import-local">Import local JSON</button>
      </div>
    </div>
  `;
}

function renderExportLink(file) {
  const name = fileNameFromPath(file.path);
  const meta = file.bytes ? `${file.format || "file"} - ${formatBytes(file.bytes)}` : "static export";
  return `<a class="export-link" href="data/${esc(name)}"><strong>${esc(name)}</strong><span>${esc(meta)}</span></a>`;
}

function renderBatchedCards(listID, rows, renderer) {
  if (!rows.length) return `<div class="empty">No rows are available.</div>`;
  const visible = visibleCount(listID, INITIAL_CARD_COUNT);
  return `
    <div class="grid">
      ${rows.slice(0, visible).map(renderer).join("")}
    </div>
    ${renderListFooter(listID, rows.length, visible, "cards", CARD_INCREMENT)}
  `;
}

function renderListFooter(listID, total, visible, label, increment) {
  if (total <= visible) return "";
  const shown = Math.min(visible, total);
  return `
    <div class="list-footer">
      <span class="muted">Showing ${num(shown)} of ${num(total)} ${esc(label)}</span>
      <button class="small-button" data-action="load-more" data-list="${esc(listID)}" data-increment="${esc(String(increment))}">Load more</button>
    </div>
  `;
}

function visibleCount(listID, initial) {
  return state.listWindows[listID] || initial;
}

function handleContentClick(event) {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const action = button.dataset.action;
  if (action === "load-more") {
    const listID = button.dataset.list;
    const increment = Number(button.dataset.increment || TABLE_ROWS_INCREMENT);
    state.listWindows[listID] = visibleCount(listID, increment) + increment;
    render();
  }
  if (action === "open" && button.dataset.ticker) openTicker(button.dataset.ticker);
  if (action === "explore-group" && button.dataset.groupId) openExplorerGroup(button.dataset.groupId);
  if (action === "watch") toggleWatch(button.dataset.ticker);
  if (action === "compare-add" && button.dataset.ticker) addComparisonTicker(button.dataset.ticker);
  if (action === "compare-remove" && button.dataset.ticker) removeComparisonTicker(button.dataset.ticker);
  if (action === "compare-open") openComparisonPanel();
  if (action === "sort") sortBy(button.dataset.key);
  if (action === "clear-filters") clearTickerFilters();
  if (action === "clear-review-filters") clearReviewFilters();
  if (action === "export-local") exportLocal();
  if (action === "import-local") $("#importFile").click();
  if (action === "review-reason") {
    state.reviewFilters.reason = button.dataset.reason || "";
    resetListWindows();
    render();
  }
  if (action === "copy-review-row") {
    const row = state.visibleReviewRows[Number(button.dataset.reviewIndex || 0)];
    if (row && row.suggestedCsvRow) {
      copyText(row.suggestedCsvRow).then(() => {
        button.textContent = "Copied";
        window.setTimeout(() => {
          button.textContent = "Copy row";
        }, 900);
      }).catch((error) => {
        button.textContent = "Copy failed";
        button.title = error.message;
        window.setTimeout(() => {
          button.textContent = "Copy row";
          button.removeAttribute("title");
        }, 1400);
      });
    }
  }
}

function handleContentChange(event) {
  const explorerField = event.target.closest("[data-explorer-filter]");
  if (explorerField) {
    if (explorerField.dataset.explorerFilter === "type") {
      state.explorerType = explorerField.value;
      state.explorerGroup = "";
    }
    if (explorerField.dataset.explorerFilter === "group") {
      state.explorerGroup = explorerField.value;
    }
    resetListWindows();
    render();
    return;
  }
  const field = event.target.closest("[data-review-filter]");
  if (!field) return;
  state.reviewFilters[field.dataset.reviewFilter] = field.value;
  resetListWindows();
  render();
}

function handleContentInput(event) {
  if (event.target.id !== "reviewSearch") return;
  applyReviewSearch(event.target.value);
}

function openExplorerGroup(groupID) {
  const group = state.maps.explorerGroupById.get(groupID);
  const parts = groupID.split(":");
  const groupType = group?.type || parts[0] || state.explorerType;
  state.view = "explorer";
  state.explorerGroup = groupID;
  state.explorerType = groupType;
  if (groupType === "theme") {
    state.theme = group?.value || parts[1] || "";
    state.supplyTheme = state.theme;
    state.sector = "";
  }
  if (groupType === "layer") {
    state.theme = group?.parentId || parts[1] || "";
    state.supplyTheme = state.theme;
    state.sector = "";
  }
  if (groupType === "sector") {
    const sector = group || (state.bootstrap.sectors || []).find((item) => item.id === parts[1]);
    state.sector = sector?.label || sector?.name || state.sector;
    state.theme = "";
  }
  syncTopFilterControls();
  resetListWindows();
  render();
}

function selectExplorerThemeGroup(themeID) {
  state.explorerType = "theme";
  state.explorerGroup = `theme:${themeID}`;
}

function syncTopFilterControls() {
  $("#globalSearch").value = state.query;
  $("#themeFilter").value = state.theme;
  $("#sectorFilter").value = state.sector;
  $("#localFilter").value = state.localFilter;
}

async function openTicker(tickerID) {
  if (!state.tickers) {
    try {
      await ensureTickerIndex();
    } catch (error) {
      showContentError(error);
      return;
    }
  }
  const ticker = tickerByID(tickerID);
  if (!ticker) return;
  showModalLoading(ticker);
  try {
    await ensureDetailData();
    renderTickerModal(tickerID);
  } catch (error) {
    $("#modalBody").innerHTML = `<div class="empty">Unable to load detail data: ${esc(error.message)}</div>`;
  }
}

function showModalLoading(ticker) {
  state.modalTicker = ticker.ticker;
  $("#modalTitle").textContent = ticker.ticker;
  $("#modalSubtitle").textContent = ticker.name;
  $("#modalBody").innerHTML = loadingBlock("Loading ticker detail", "Fetching company, security, listing, and relationship slices.");
  showModal();
}

function renderTickerModal(tickerID) {
  const ticker = tickerByID(tickerID);
  if (!ticker) return;
  state.modalTicker = tickerID;
  const company = companyByID(ticker.companyId) || {};
  const security = securityByID(ticker.securityId) || {};
  const listing = listingByID(ticker.listingId) || {};
  const local = getLocal(ticker.ticker);
  const related = uniqueStrings([...(ticker.relatedTickers || []), ...relationshipTickerIDs(ticker.ticker)]).filter((id) => id !== ticker.ticker);
  $("#modalTitle").textContent = ticker.ticker;
  $("#modalSubtitle").textContent = ticker.name;
  $("#modalBody").innerHTML = `
    <div class="detail-grid">
      <section>
        <div class="facts">
          ${fact("Company", company.name || ticker.name)}
          ${fact("ISIN", ticker.isin || "None")}
          ${fact("Instrument type", ticker.type || "Unknown")}
          ${fact("Category", ticker.instrumentCategory || "Unknown")}
          ${fact("Currency", ticker.currencyCode || listing.currencyCode || "Unknown")}
          ${fact("Exchange", ticker.exchangeName || listing.exchangeName || ticker.exchangeCode || "Unknown")}
          ${fact("Listing", listing.id || ticker.listingId || "Unknown")}
          ${fact("Yahoo symbol", ticker.yahooSymbol || company.yahooSymbol || "Unresolved")}
          ${fact("Sector", ticker.sector || company.sector || "Unclassified")}
          ${fact("Industry", ticker.industry || company.industry || "Unclassified")}
          ${fact("Market cap", formatMarketCap(ticker.marketCap || company.marketCap))}
          ${fact("Directionality", ticker.directionality)}
          ${fact("Identity confidence", ticker.identityConfidence || company.identityConfidence || "Unknown")}
          ${fact("Security", security.id || ticker.securityId)}
          ${fact("Last refreshed", formatDate(ticker.lastRefreshed || company.lastRefreshed))}
        </div>
        <div class="card detail-card">
          <h3>Theme and layer memberships</h3>
          ${renderMemberships(ticker)}
        </div>
        <div class="card detail-card">
          <h3>Related tickers</h3>
          ${renderRelatedTickerChips(related)}
        </div>
        <div class="card detail-card">
          <h3>Reviewed relationships</h3>
          ${renderRelationships(ticker.ticker)}
        </div>
        <div class="card detail-card">
          <h3>Sources</h3>
          ${renderSources(combinedSources([...(company.sources || []), ...(security.sources || []), ...(listing.sources || [])]))}
        </div>
      </section>
      <section class="local-tools">
        ${compareButton(ticker.ticker)}
        <button class="small-button ${local.watchlist ? "primary" : ""}" id="modalWatch">${local.watchlist ? "Remove from watchlist" : "Add to watchlist"}</button>
        <div class="control-row">
          <label for="modalColor">Colour</label>
          <select id="modalColor">
            ${["", "green", "amber", "red", "blue", "violet"].map((color) => `<option value="${color}" ${local.color === color ? "selected" : ""}>${color || "none"}</option>`).join("")}
          </select>
        </div>
        <div class="control-row">
          <label for="modalTags">Tags</label>
          <input id="modalTags" value="${esc((local.tags || []).join(", "))}" placeholder="quality, watch, review">
        </div>
        <div>
          <label class="muted" for="modalNotes">Notes</label>
          <textarea id="modalNotes" placeholder="Local research notes">${esc(local.notes || "")}</textarea>
        </div>
      </section>
    </div>
  `;
  $("#modalWatch").addEventListener("click", () => {
    toggleWatch(ticker.ticker);
    renderTickerModal(ticker.ticker);
  });
  $("#modalColor").addEventListener("change", (event) => {
    localFor(ticker.ticker).color = event.target.value;
    saveLocal();
    renderLocalFilterOptions();
    render();
  });
  $("#modalTags").addEventListener("input", (event) => {
    localFor(ticker.ticker).tags = event.target.value.split(",").map((item) => item.trim()).filter(Boolean);
    saveLocal();
    renderLocalFilterOptions();
    render();
  });
  $("#modalNotes").addEventListener("input", (event) => {
    localFor(ticker.ticker).notes = event.target.value;
    saveLocal();
  });
  showModal();
}

function showModal() {
  const modal = $("#detailModal");
  if (modal.open) return;
  if (modal.showModal) modal.showModal();
  else modal.setAttribute("open", "open");
}

function closeModalOnBackdropClick(event) {
  const modal = event.currentTarget;
  const rect = modal.getBoundingClientRect();
  const insideModal = (
    event.clientX >= rect.left &&
    event.clientX <= rect.right &&
    event.clientY >= rect.top &&
    event.clientY <= rect.bottom
  );
  if (insideModal) return;
  if (modal.close) modal.close();
  else modal.removeAttribute("open");
}

function fact(label, value) {
  return `<div class="fact"><span>${esc(label)}</span><strong>${esc(String(value || ""))}</strong></div>`;
}

function renderMemberships(ticker) {
  const exposures = exposuresForTicker(ticker);
  if (exposures.length) {
    return `
      <div class="membership-list">
        ${exposures.map((exposure) => {
          const details = [
            layerName(exposure.themeId, exposure.layerId),
            exposure.exposureScore ? `score ${exposure.exposureScore}` : "",
            exposure.confidence || "",
            exposure.lastReviewed ? `reviewed ${exposure.lastReviewed}` : ""
          ].filter(Boolean);
          return `
            <div>
              <strong>${esc(themeName(exposure.themeId))}</strong>
              <span>${esc(details.join(" / "))}</span>
            </div>
          `;
        }).join("")}
      </div>
    `;
  }
  const fallback = [...themeNames(ticker.themeIds), ...(ticker.layerIds || []).map((id) => layerName("", id) || id)];
  return `<div class="chips">${chips(fallback) || `<span class="muted">None mapped yet</span>`}</div>`;
}

function exposuresForTicker(ticker) {
  return (state.bootstrap.exposures || []).filter((exposure) => (
    (exposure.ticker && exposure.ticker === ticker.ticker) ||
    (exposure.companyId && exposure.companyId === ticker.companyId) ||
    (exposure.isin && exposure.isin === ticker.isin)
  ));
}

function renderRelationships(tickerID) {
  const rows = relationshipsForTicker(tickerID);
  if (!rows.length) return `<p class="muted">No reviewed relationships.</p>`;
  return `
    <div class="relationship-list">
      ${rows.map((row) => {
        const isSource = endpointContainsTicker(row, "source", tickerID);
        const other = isSource ? relationshipEndpointLabel(row, "target") : relationshipEndpointLabel(row, "source");
        const context = [themeName(row.themeId), layerName(row.themeId, row.layerId), row.confidence].filter(Boolean).join(" / ");
        return `
          <div>
            <strong>${esc(relationshipTypeLabel(row.relationshipType))}</strong>
            <span>${esc(other || "Unresolved")}</span>
            ${context ? `<small>${esc(context)}</small>` : ""}
            ${row.sourceUrl ? `<a href="${esc(row.sourceUrl)}">source</a>` : ""}
          </div>
        `;
      }).join("")}
    </div>
  `;
}

function renderSources(sources) {
  if (!sources.length) return `<p class="muted">No sources attached.</p>`;
  return `<div class="chips">${sources.map((source) => source.url ? `<a class="chip" href="${esc(source.url)}">${esc(source.label || source.kind)}</a>` : `<span class="chip">${esc(source.label || source.kind)}</span>`).join("")}</div>`;
}

function filteredTickers() {
  const rows = state.tickers.filter(tickerMatches);
  rows.sort((a, b) => compareValues(a[state.sort.key], b[state.sort.key]) * (state.sort.dir === "asc" ? 1 : -1));
  return rows;
}

function filteredReviewRows() {
  const filters = state.reviewFilters;
  return state.reviewQueues.filter((row) => {
    if (filters.queue && row.queue !== filters.queue) return false;
    if (filters.reason && row.reasonCode !== filters.reason) return false;
    if (filters.severity && row.severity !== filters.severity) return false;
    if (filters.gap && !reviewMatchesGap(row, filters.gap)) return false;
    if (state.theme && !(row.themeIds || []).includes(state.theme)) return false;
    if (state.sector && row.sector !== state.sector) return false;
    if (filters.search && !(row._searchText || "").includes(filters.search)) return false;
    if (state.query && !(row._searchText || "").includes(state.query)) return false;
    return true;
  });
}

function reviewMatchesGap(row, gap) {
  if (gap === "sector") return row.reasonCode === "missing_sector";
  if (gap === "industry") return row.reasonCode === "missing_industry";
  if (gap === "theme") return row.reasonCode === "missing_theme_exposure";
  return true;
}

function tickerMatches(ticker) {
  if (state.theme && !(ticker.themeIds || []).includes(state.theme)) return false;
  if (state.sector && ticker.sector !== state.sector) return false;
  const local = getLocal(ticker.ticker);
  if (state.localFilter === "watchlist" && !local.watchlist) return false;
  if (state.localFilter === "tagged" && !(local.tags || []).length) return false;
  if (state.localFilter === "coloured" && !local.color) return false;
  if (!state.query) return true;
  const localText = `${(local.tags || []).join(" ")} ${local.notes || ""}`.toLowerCase();
  return (ticker._searchText || "").includes(state.query) || localText.includes(state.query);
}

function buildReviewSearchText(row) {
  return [
    row.queue,
    row.reasonCode,
    row.severity,
    row.ticker,
    row.name,
    row.isin,
    row.companyId,
    row.securityId,
    row.sector,
    row.industry,
    row.sourceFile,
    row.suggestedAction,
    row.suggestedManualFile,
    row.issueType,
    row.staleBucket,
    ...(row.themeIds || []),
    ...(row.layerIds || [])
  ].filter(Boolean).join(" ").toLowerCase();
}

function sortBy(key) {
  if (state.sort.key === key) state.sort.dir = state.sort.dir === "asc" ? "desc" : "asc";
  else state.sort = { key, dir: key === "marketCap" ? "desc" : "asc" };
  resetListWindows();
  render();
}

function clearTickerFilters() {
  state.query = "";
  state.theme = "";
  state.sector = "";
  state.localFilter = "";
  syncTopFilterControls();
  resetListWindows();
  render();
}

function clearReviewFilters() {
  state.reviewFilters = { queue: "", reason: "", severity: "", gap: "", search: "" };
  resetListWindows();
  render();
}

function resetListWindows() {
  state.listWindows = {};
}

function getLocal(ticker) {
  return state.local.tickers[ticker] || EMPTY_LOCAL;
}

function localFor(ticker) {
  if (!state.local.tickers[ticker]) state.local.tickers[ticker] = { watchlist: false, notes: "", tags: [], color: "" };
  return state.local.tickers[ticker];
}

function toggleWatch(ticker) {
  const local = localFor(ticker);
  local.watchlist = !local.watchlist;
  saveLocal();
  renderLocalFilterOptions();
  render();
}

function localBadges(local) {
  const badges = [];
  if (local.watchlist) badges.push(`<span class="chip green">watchlist</span>`);
  if (local.color) badges.push(`<span class="chip ${esc(local.color)}">${esc(local.color)}</span>`);
  for (const tag of local.tags || []) badges.push(`<span class="chip">${esc(tag)}</span>`);
  return badges.join(" ");
}

function loadLocal() {
  try {
    return normalizeLocal(JSON.parse(localStorage.getItem(LOCAL_STORAGE_KEY)));
  } catch {
    return normalizeLocal(null);
  }
}

function saveLocal() {
  state.local = normalizeLocal(state.local);
  localStorage.setItem(LOCAL_STORAGE_KEY, JSON.stringify(state.local));
}

function exportLocal() {
  const blob = new Blob([JSON.stringify(state.local, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `statos-local-${new Date().toISOString().slice(0, 10)}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

async function importLocalFile(event) {
  const file = event.target.files[0];
  if (!file) return;
  try {
    const text = await file.text();
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== "object" || !parsed.tickers) throw new Error("Invalid local Statos export");
    state.local = normalizeLocal(parsed);
    saveLocal();
    renderLocalFilterOptions();
    state.savedViewSelection = "";
    resetListWindows();
    render();
  } catch (error) {
    window.alert(error.message);
  } finally {
    event.target.value = "";
  }
}

function normalizeLocal(value) {
  const local = value && typeof value === "object" ? { ...value } : {};
  if (!local.tickers || typeof local.tickers !== "object" || Array.isArray(local.tickers)) local.tickers = {};
  const tickers = {};
  for (const [ticker, raw] of Object.entries(local.tickers)) {
    const item = raw && typeof raw === "object" ? raw : {};
    tickers[ticker] = {
      watchlist: Boolean(item.watchlist),
      notes: String(item.notes || ""),
      tags: uniqueStrings(asArray(item.tags).map((tag) => String(tag).trim()).filter(Boolean)),
      color: ["green", "amber", "red", "blue", "violet"].includes(item.color) ? item.color : ""
    };
  }
  local.tickers = tickers;
  local.savedViews = normalizeSavedViews(local.savedViews);
  local.comparison = uniqueStrings(asArray(local.comparison).map((ticker) => String(ticker).trim()).filter(Boolean));
  return local;
}

function normalizeSavedViews(views) {
  return asArray(views).map((view, index) => {
    const raw = view && typeof view === "object" ? view : {};
    const name = String(raw.name || `View ${index + 1}`).trim().replace(/\s+/g, " ").slice(0, 40);
    const id = String(raw.id || `view-${Date.now().toString(36)}-${index}`).trim();
    return {
      id,
      name,
      createdAt: String(raw.createdAt || ""),
      updatedAt: String(raw.updatedAt || raw.createdAt || ""),
      snapshot: sanitizeViewSnapshot(raw.snapshot || raw.state || raw)
    };
  }).filter((view) => view.id && view.name);
}

function sanitizeViewSnapshot(snapshot) {
  const raw = snapshot && typeof snapshot === "object" ? snapshot : {};
  return {
    view: VALID_VIEWS.includes(raw.view) ? raw.view : "tickers",
    query: String(raw.query || "").trim().toLowerCase(),
    theme: String(raw.theme || ""),
    sector: String(raw.sector || ""),
    localFilter: VALID_LOCAL_FILTERS.includes(raw.localFilter) ? raw.localFilter : "",
    explorerType: String(raw.explorerType || ""),
    explorerGroup: String(raw.explorerGroup || ""),
    supplyTheme: String(raw.supplyTheme || ""),
    sort: sanitizeSort(raw.sort),
    reviewFilters: sanitizeReviewFilters(raw.reviewFilters)
  };
}

function sanitizeSort(sort) {
  const raw = sort && typeof sort === "object" ? sort : {};
  const key = VALID_SORT_KEYS.includes(raw.key) ? raw.key : DEFAULT_SORT.key;
  const dir = raw.dir === "asc" || raw.dir === "desc" ? raw.dir : (key === DEFAULT_SORT.key ? DEFAULT_SORT.dir : "asc");
  return { key, dir };
}

function sanitizeReviewFilters(filters) {
  const raw = filters && typeof filters === "object" ? filters : {};
  return {
    queue: String(raw.queue || ""),
    reason: String(raw.reason || ""),
    severity: String(raw.severity || ""),
    gap: ["sector", "industry", "theme"].includes(raw.gap) ? raw.gap : "",
    search: String(raw.search || "").trim().toLowerCase()
  };
}

function savedViews() {
  if (!Array.isArray(state.local.savedViews)) state.local.savedViews = [];
  return state.local.savedViews;
}

function comparisonTickers() {
  if (!Array.isArray(state.local.comparison)) state.local.comparison = [];
  return state.local.comparison;
}

function saveCurrentView() {
  const input = $("#savedViewName");
  const name = input.value.trim().replace(/\s+/g, " ").slice(0, 40);
  if (!name) {
    setSavedViewStatus("Enter a view name first.");
    input.focus();
    return;
  }
  const now = new Date().toISOString();
  const view = {
    id: `view-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`,
    name,
    createdAt: now,
    updatedAt: now,
    snapshot: captureCurrentView()
  };
  savedViews().push(view);
  state.savedViewSelection = view.id;
  input.value = "";
  saveLocal();
  renderSavedViewControls();
  setSavedViewStatus(`Saved ${name}.`);
}

function captureCurrentView() {
  const reviewSearch = $("#reviewSearch");
  const explorerType = document.querySelector("[data-explorer-filter='type']");
  const explorerGroup = document.querySelector("[data-explorer-filter='group']");
  return sanitizeViewSnapshot({
    view: state.view,
    query: $("#globalSearch").value,
    theme: $("#themeFilter").value,
    sector: $("#sectorFilter").value,
    localFilter: $("#localFilter").value,
    explorerType: explorerType?.value || state.explorerType,
    explorerGroup: explorerGroup?.value || state.explorerGroup,
    supplyTheme: currentSupplyTheme(),
    sort: { ...state.sort },
    reviewFilters: { ...state.reviewFilters, search: reviewSearch?.value || state.reviewFilters.search }
  });
}

function currentSupplyTheme() {
  const select = $("#supplyThemeSelect");
  return select?.value || state.supplyTheme || state.theme || "";
}

async function applySelectedSavedView() {
  const view = selectedSavedView();
  if (!view || !state.bootstrap) return;
  await applyViewSnapshot(view.snapshot);
  state.savedViewSelection = view.id;
  renderSavedViewControls();
  setSavedViewStatus(`Applied ${view.name}.`);
}

async function applyViewSnapshot(snapshot) {
  const saved = sanitizeViewSnapshot(snapshot);
  state.view = saved.view;
  state.query = saved.query;
  state.theme = validThemeID(saved.theme);
  state.sector = validSectorName(saved.sector);
  state.localFilter = saved.localFilter;
  state.supplyTheme = validSupplyTheme(saved.supplyTheme) || (validSupplyTheme(state.theme) ? state.theme : "");
  if (state.view === "supply" && state.supplyTheme) state.theme = state.supplyTheme;
  state.sort = sanitizeSort(saved.sort);
  state.reviewFilters = sanitizeReviewFilters(saved.reviewFilters);
  state.explorerType = saved.explorerType;
  state.explorerGroup = saved.explorerGroup;
  if (state.view === "explorer") await restoreExplorerSelection({ ...saved, theme: state.theme, supplyTheme: state.supplyTheme });
  syncTopFilterControls();
  resetListWindows();
  render();
}

async function restoreExplorerSelection(snapshot) {
  try {
    await ensureExplorerIndex();
  } catch {
    state.explorerType = snapshot.explorerType;
    state.explorerGroup = snapshot.explorerGroup;
    return;
  }
  const groups = state.explorerIndex?.groups || [];
  if (!groups.length) return;
  const types = uniqueStrings(groups.map((group) => group.type));
  const savedGroup = groups.find((group) => group.id === snapshot.explorerGroup);
  let type = savedGroup?.type || (types.includes(snapshot.explorerType) ? snapshot.explorerType : "");
  if (!type && snapshot.theme && groups.some((group) => group.id === `theme:${snapshot.theme}`)) type = "theme";
  if (!type || !types.includes(type)) type = types[0];
  const typeGroups = groups.filter((group) => group.type === type);
  let group = savedGroup && savedGroup.type === type ? savedGroup : null;
  if (!group && snapshot.theme) group = typeGroups.find((item) => item.id === `theme:${snapshot.theme}` || item.parentId === snapshot.theme);
  if (!group && snapshot.supplyTheme) group = typeGroups.find((item) => item.id === `theme:${snapshot.supplyTheme}` || item.parentId === snapshot.supplyTheme);
  if (!group) group = typeGroups[0] || null;
  state.explorerType = type;
  state.explorerGroup = group?.id || "";
}

function deleteSelectedSavedView() {
  const view = selectedSavedView();
  if (!view) return;
  state.local.savedViews = savedViews().filter((item) => item.id !== view.id);
  state.savedViewSelection = "";
  saveLocal();
  renderSavedViewControls();
  setSavedViewStatus(`Deleted ${view.name}.`);
}

function selectedSavedView() {
  const id = state.savedViewSelection || $("#savedViewSelect").value;
  return savedViews().find((view) => view.id === id) || null;
}

function setSavedViewStatus(message) {
  $("#savedViewStatus").textContent = message;
  window.clearTimeout(state.statusTimer);
  state.statusTimer = window.setTimeout(() => {
    $("#savedViewStatus").textContent = "";
  }, 2400);
}

function validThemeID(id) {
  if (!id) return "";
  return state.maps.themeById.has(id) ? id : "";
}

function validSectorName(name) {
  if (!name) return "";
  return (state.bootstrap?.sectors || []).some((sector) => sector.name === name) ? name : "";
}

function validSupplyTheme(id) {
  if (!id) return "";
  return (state.bootstrap?.supplyChains || []).some((chain) => chain.themeId === id) ? id : "";
}

function localExportSummary() {
  const localRows = Object.values(state.local.tickers || {});
  const annotated = localRows.filter((local) => local.watchlist || local.color || (local.tags || []).length || local.notes).length;
  return `${num(annotated)} annotated tickers - ${num(savedViews().length)} saved views - ${num(comparisonTickers().length)} comparison tickers`;
}

function viewLabel(view) {
  const labels = {
    tickers: "Tickers",
    explorer: "Explorer",
    themes: "Themes",
    supply: "Supply chain",
    sectors: "Sectors",
    watchlist: "Watchlist",
    unclassified: "Review queues",
    exports: "Exports"
  };
  return labels[view] || view || "View";
}

function compareButton(tickerID) {
  const added = comparisonHas(tickerID);
  return `<button class="small-button compare-button ${added ? "primary" : ""}" data-action="${added ? "compare-remove" : "compare-add"}" data-ticker="${esc(tickerID)}" aria-pressed="${added ? "true" : "false"}" title="${added ? "Remove from comparison" : "Add to comparison"}">${added ? "Compared" : "Compare"}</button>`;
}

function comparisonHas(tickerID) {
  return comparisonTickers().includes(tickerID);
}

function addComparisonTicker(tickerID) {
  const ticker = String(tickerID || "").trim();
  if (!ticker || comparisonHas(ticker)) return;
  comparisonTickers().push(ticker);
  saveLocal();
  setSavedViewStatus(`${ticker} added to comparison.`);
  refreshComparisonState();
}

function removeComparisonTicker(tickerID) {
  const ticker = String(tickerID || "").trim();
  if (!ticker) return;
  state.local.comparison = comparisonTickers().filter((id) => id !== ticker);
  saveLocal();
  setSavedViewStatus(`${ticker} removed from comparison.`);
  refreshComparisonState();
}

function clearComparison() {
  if (!comparisonTickers().length) return;
  state.local.comparison = [];
  saveLocal();
  setSavedViewStatus("Comparison cleared.");
  refreshComparisonState();
}

function refreshComparisonState() {
  renderComparisonControls();
  render();
  if (state.modalTicker && $("#detailModal").open) renderTickerModal(state.modalTicker);
}

function toggleComparisonPanel() {
  if (state.comparisonOpen) closeComparisonPanel();
  else openComparisonPanel();
}

function openComparisonPanel() {
  state.comparisonOpen = true;
  renderComparisonControls();
  renderComparisonPanel();
}

function closeComparisonPanel() {
  state.comparisonOpen = false;
  $("#comparisonPanel").hidden = true;
  $("#comparisonBackdrop").hidden = true;
  renderComparisonControls();
}

function handleComparisonClick(event) {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const action = button.dataset.action;
  if (action === "open" && button.dataset.ticker) openTicker(button.dataset.ticker);
  if (action === "compare-add" && button.dataset.ticker) addComparisonTicker(button.dataset.ticker);
  if (action === "compare-remove" && button.dataset.ticker) removeComparisonTicker(button.dataset.ticker);
}

function renderComparisonPanel() {
  const panel = $("#comparisonPanel");
  const backdrop = $("#comparisonBackdrop");
  panel.hidden = !state.comparisonOpen;
  backdrop.hidden = !state.comparisonOpen;
  renderComparisonControls();
  if (!state.comparisonOpen) return;

  const ids = comparisonTickers();
  $("#comparisonSubtitle").textContent = ids.length ? `${num(ids.length)} tickers in local comparison` : "No tickers selected";
  if (!state.tickers) {
    $("#comparisonBody").innerHTML = loadingBlock("Loading comparison rows", "Fetching compact ticker index before rendering comparison.");
    ensureTickerIndex().then(renderComparisonPanel).catch((error) => {
      $("#comparisonBody").innerHTML = `<div class="empty">Unable to load ticker index: ${esc(error.message)}</div>`;
    });
    return;
  }
  if (!ids.length) {
    $("#comparisonBody").innerHTML = renderComparisonEmpty();
    return;
  }
  if (!state.relationships && !state.relationshipsError && !state.promises.relationships) {
    ensureRelationshipsData().then(renderComparisonPanel).catch((error) => {
      state.relationshipsError = error.message;
      renderComparisonPanel();
    });
  }
  const entries = ids.map((id) => comparisonEntry(id));
  const relationshipNote = state.relationshipsError
    ? `<p class="muted comparison-note">Reviewed relationship counts are unavailable: ${esc(state.relationshipsError)}</p>`
    : (!state.relationships ? `<p class="muted comparison-note">Showing indexed related tickers while reviewed relationship counts load.</p>` : "");
  $("#comparisonBody").innerHTML = `${relationshipNote}${renderComparisonTable(entries)}`;
}

function renderComparisonEmpty() {
  return `
    <div class="empty">
      <p>No tickers in comparison.</p>
      <p>Use Compare from ticker rows, supply cards, related ticker chips, or the ticker detail modal.</p>
    </div>
  `;
}

function comparisonEntry(id) {
  const ticker = tickerByID(id);
  const related = ticker ? comparisonRelatedTickers(ticker) : [];
  return {
    id,
    ticker,
    local: getLocal(id),
    related,
    reviewedRelationshipCount: ticker && state.relationships ? relationshipsForTicker(ticker.ticker).length : null
  };
}

function renderComparisonTable(entries) {
  const fields = comparisonFields();
  return `
    <div class="comparison-scroll">
      <table class="compare-table" style="--compare-count:${Math.max(entries.length, 1)}">
        <thead>
          <tr>
            <th>Field</th>
            ${entries.map(renderComparisonColumnHead).join("")}
          </tr>
        </thead>
        <tbody>
          ${fields.map((field) => `
            <tr>
              <th scope="row">${esc(field.label)}</th>
              ${entries.map((entry) => `<td>${field.value(entry)}</td>`).join("")}
            </tr>
          `).join("")}
        </tbody>
      </table>
    </div>
  `;
}

function renderComparisonColumnHead(entry) {
  const label = entry.ticker ? entry.ticker.ticker : entry.id;
  return `
    <th>
      <div class="compare-item-head">
        <button class="ticker-link" data-action="open" data-ticker="${esc(entry.id)}">${esc(label)}</button>
        <div class="row-actions">
          <button class="small-button" data-action="open" data-ticker="${esc(entry.id)}">Open</button>
          <button class="small-button" data-action="compare-remove" data-ticker="${esc(entry.id)}">Remove</button>
        </div>
      </div>
    </th>
  `;
}

function comparisonFields() {
  return [
    { label: "Ticker", value: (entry) => entry.ticker ? `<button class="ticker-link" data-action="open" data-ticker="${esc(entry.id)}">${esc(entry.id)}</button>` : missingTickerCell(entry.id) },
    { label: "Company / security", value: (entry) => comparisonText(entry, (ticker) => ticker.name) },
    { label: "Sector", value: (entry) => comparisonText(entry, (ticker) => ticker.sector || "Unclassified") },
    { label: "Industry", value: (entry) => comparisonText(entry, (ticker) => ticker.industry || "Unclassified") },
    { label: "Category / type", value: (entry) => comparisonText(entry, (ticker) => [ticker.instrumentCategory, ticker.type].filter(Boolean).join(" / ") || "Unknown") },
    { label: "Market cap", value: (entry) => comparisonText(entry, (ticker) => formatMarketCap(ticker.marketCap)) },
    { label: "Exchange / currency", value: (entry) => comparisonText(entry, (ticker) => [ticker.exchangeName || ticker.exchangeCode, ticker.currencyCode].filter(Boolean).join(" / ") || "Unknown") },
    { label: "Theme memberships", value: (entry) => entry.ticker ? (chips(themeNames(entry.ticker.themeIds)) || mutedText("None")) : mutedText("Missing") },
    { label: "Layer memberships", value: (entry) => entry.ticker ? (chips(layerLabelsForTicker(entry.ticker)) || mutedText("None")) : mutedText("Missing") },
    { label: "Relationships", value: renderComparisonRelationships },
    { label: "Local state", value: (entry) => localBadges(entry.local) || mutedText("None") },
    { label: "Notes preview", value: (entry) => notesPreview(entry.local.notes) ? `<span class="notes-preview">${esc(notesPreview(entry.local.notes))}</span>` : mutedText("None") }
  ];
}

function comparisonText(entry, reader) {
  if (!entry.ticker) return mutedText("Missing from current index");
  return `<span>${esc(reader(entry.ticker))}</span>`;
}

function missingTickerCell(id) {
  return `<span>${esc(id)}</span><div class="muted">Missing from current index</div>`;
}

function renderComparisonRelationships(entry) {
  if (!entry.ticker) return mutedText("Missing");
  const countLabel = entry.reviewedRelationshipCount == null
    ? `${num(entry.related.length)} indexed related`
    : `${num(entry.reviewedRelationshipCount)} reviewed / ${num(entry.related.length)} related`;
  return `
    <div class="comparison-relations">
      <span class="muted">${esc(countLabel)}</span>
      ${renderRelatedTickerChips(entry.related, 8)}
    </div>
  `;
}

function comparisonRelatedTickers(ticker) {
  return uniqueStrings([
    ...(ticker.relatedTickers || []),
    ...(state.relationships ? relationshipTickerIDs(ticker.ticker) : [])
  ]).filter((id) => id !== ticker.ticker);
}

function layerLabelsForTicker(ticker) {
  const exposures = exposuresForTicker(ticker);
  if (exposures.length) {
    return uniqueStrings(exposures.map((exposure) => [themeName(exposure.themeId), layerName(exposure.themeId, exposure.layerId)].filter(Boolean).join(" / ")));
  }
  return uniqueStrings((ticker.layerIds || []).map((id) => layerName("", id) || id));
}

function renderRelatedTickerChips(ids, limit = 40) {
  const related = uniqueStrings(ids);
  if (!related.length) return `<span class="muted">None</span>`;
  const visible = related.slice(0, limit);
  const hidden = related.length - visible.length;
  return `
    <div class="chips related-chips">
      ${visible.map((id) => `
        <span class="ticker-chip-actions">
          <button class="chip chip-button" data-action="open" data-ticker="${esc(id)}">${esc(id)}</button>
          <button class="chip chip-compare" data-action="${comparisonHas(id) ? "compare-remove" : "compare-add"}" data-ticker="${esc(id)}" aria-label="${comparisonHas(id) ? "Remove" : "Add"} ${esc(id)} ${comparisonHas(id) ? "from" : "to"} comparison" title="${comparisonHas(id) ? "Remove from comparison" : "Add to comparison"}">${comparisonHas(id) ? "x" : "+"}</button>
        </span>
      `).join("")}
      ${hidden > 0 ? `<span class="chip">+${num(hidden)} more</span>` : ""}
    </div>
  `;
}

function notesPreview(notes) {
  const text = String(notes || "").replace(/\s+/g, " ").trim();
  if (text.length <= 160) return text;
  return `${text.slice(0, 157)}...`;
}

function mutedText(value) {
  return `<span class="muted">${esc(value)}</span>`;
}

async function ensureTickerIndex() {
  if (state.tickers) return state.tickers;
  if (!state.promises.tickers) {
    state.promises.tickers = fetchJSON("data/tickers_index.json").then((data) => {
      state.tickers = Array.isArray(data) ? data : (data.tickers || []);
      indexTickerRows();
      return state.tickers;
    });
  }
  return state.promises.tickers;
}

async function ensureExplorerIndex() {
  if (state.explorerIndex) return state.explorerIndex;
  if (!state.promises.explorerIndex) {
    state.promises.explorerIndex = fetchJSON("data/explorer_index.json").then((data) => {
      state.explorerIndex = data && typeof data === "object" ? data : { groups: [] };
      indexExplorerGroups();
      return state.explorerIndex;
    });
  }
  return state.promises.explorerIndex;
}

function indexExplorerGroups() {
  state.maps.explorerGroupById = new Map((state.explorerIndex.groups || []).map((group) => [group.id, group]));
}

function indexTickerRows() {
  const tickerById = new Map();
  const firstTickerByCompany = new Map();
  const firstTickerByIsin = new Map();
  const tickersByCompany = new Map();
  const tickersByIsin = new Map();
  for (const ticker of state.tickers) {
    ticker._searchText = buildTickerSearchText(ticker);
    tickerById.set(ticker.ticker, ticker);
    if (ticker.companyId) {
      if (!firstTickerByCompany.has(ticker.companyId)) firstTickerByCompany.set(ticker.companyId, ticker);
      if (!tickersByCompany.has(ticker.companyId)) tickersByCompany.set(ticker.companyId, []);
      tickersByCompany.get(ticker.companyId).push(ticker.ticker);
    }
    if (ticker.isin) {
      if (!firstTickerByIsin.has(ticker.isin)) firstTickerByIsin.set(ticker.isin, ticker);
      if (!tickersByIsin.has(ticker.isin)) tickersByIsin.set(ticker.isin, []);
      tickersByIsin.get(ticker.isin).push(ticker.ticker);
    }
  }
  state.maps.tickerById = tickerById;
  state.maps.firstTickerByCompany = firstTickerByCompany;
  state.maps.firstTickerByIsin = firstTickerByIsin;
  state.maps.tickersByCompany = tickersByCompany;
  state.maps.tickersByIsin = tickersByIsin;
}

function buildTickerSearchText(ticker) {
  return [
    ticker.ticker,
    ticker.name,
    ticker.isin,
    ticker.yahooSymbol,
    ticker.sector,
    ticker.industry,
    ticker.country,
    ticker.type,
    ticker.instrumentCategory,
    ticker.currencyCode,
    ticker.exchangeName,
    ticker.exchangeCode,
    ticker.directionality,
    ticker.identityConfidence,
    ...(ticker.structureFlags || []),
    ...(ticker.themeIds || []).map(themeName),
    ...(ticker.layerIds || [])
  ].filter(Boolean).join(" ").toLowerCase();
}

async function ensureReviewQueues() {
  if (state.reviewQueues) return state.reviewQueues;
  if (!state.promises.reviewQueues) {
    state.promises.reviewQueues = Promise.all([
      fetchJSON("data/review_queues.json"),
      fetchJSON("data/review_summary.json")
    ]).then(([queues, summary]) => {
      state.reviewQueues = asArray(queues);
      state.reviewSummary = summary && typeof summary === "object" ? summary : {};
      for (const row of state.reviewQueues) row._searchText = buildReviewSearchText(row);
      return state.reviewQueues;
    });
  }
  return state.promises.reviewQueues;
}

async function ensureRelationshipsData() {
  if (state.relationships) return state.relationships;
  if (state.detail?.relationships) {
    state.relationships = state.detail.relationships;
    indexRelationships(state.relationships);
    return state.relationships;
  }
  if (!state.promises.relationships) {
    state.promises.relationships = fetchJSON("data/relationships.json").then((relationships) => {
      state.relationships = asArray(relationships);
      indexRelationships(state.relationships);
      return state.relationships;
    });
  }
  return state.promises.relationships;
}

async function ensureDetailData() {
  if (state.detail) return state.detail;
  if (!state.promises.detail) {
    const relationshipsPromise = state.relationships ? Promise.resolve(state.relationships) : fetchJSON("data/relationships.json");
    state.promises.detail = Promise.all([
      fetchJSON("data/companies.json"),
      fetchJSON("data/securities.json"),
      fetchJSON("data/listings.json"),
      relationshipsPromise
    ]).then(([companies, securities, listings, relationships]) => {
      state.detail = {
        companies: asArray(companies),
        securities: asArray(securities),
        listings: asArray(listings),
        relationships: state.relationships || asArray(relationships)
      };
      state.relationships = state.detail.relationships;
      indexDetailData();
      return state.detail;
    });
  }
  return state.promises.detail;
}

function indexDetailData() {
  state.maps.companyById = new Map(state.detail.companies.map((company) => [company.id, company]));
  state.maps.securityById = new Map(state.detail.securities.map((security) => [security.id, security]));
  state.maps.listingById = new Map(state.detail.listings.map((listing) => [listing.id, listing]));
  indexRelationships(state.detail.relationships);
}

function indexRelationships(rows) {
  const relationshipsByTicker = new Map();
  for (const row of rows || []) {
    for (const ticker of uniqueStrings([...relationshipEndpointTickers(row, "source"), ...relationshipEndpointTickers(row, "target")])) {
      if (!ticker) continue;
      if (!relationshipsByTicker.has(ticker)) relationshipsByTicker.set(ticker, []);
      relationshipsByTicker.get(ticker).push(row);
    }
  }
  state.maps.relationshipsByTicker = relationshipsByTicker;
}

async function fetchJSON(path) {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) throw new Error(`${path} fetch failed: ${response.status}`);
  return response.json();
}

function showContentError(error) {
  content.innerHTML = `<div class="empty">Unable to load data slice: ${esc(error.message)}</div>`;
  $("#buildMeta").textContent = error.message;
}

function loadingBlock(title, detail) {
  return `
    <div class="loading">
      <strong>${esc(title)}</strong>
      <span>${esc(detail || "")}</span>
    </div>
  `;
}

function tickerByID(id) {
  return state.maps.tickerById.get(id) || null;
}

function companyByID(id) {
  return state.maps.companyById.get(id) || null;
}

function securityByID(id) {
  return state.maps.securityById.get(id) || null;
}

function listingByID(id) {
  return state.maps.listingById.get(id) || null;
}

function relationshipsForTicker(tickerID) {
  return state.maps.relationshipsByTicker.get(tickerID) || [];
}

function relationshipTickerIDs(tickerID) {
  const out = [];
  for (const row of relationshipsForTicker(tickerID)) {
    for (const id of [...relationshipEndpointTickers(row, "source"), ...relationshipEndpointTickers(row, "target")]) {
      if (id && id !== tickerID) out.push(id);
    }
  }
  return uniqueStrings(out);
}

function relationshipEndpointTickers(row, side) {
  const ticker = row[`${side}Ticker`];
  if (ticker) return [ticker];
  const companyID = row[`${side}CompanyId`];
  if (companyID) return state.maps.tickersByCompany.get(companyID) || [];
  const isin = row[`${side}Isin`];
  if (isin) return state.maps.tickersByIsin.get(isin) || [];
  return [];
}

function endpointContainsTicker(row, side, tickerID) {
  return relationshipEndpointTickers(row, side).includes(tickerID);
}

function relationshipEndpointLabel(row, side) {
  const ticker = row[`${side}Ticker`];
  if (ticker) return ticker;
  const companyID = row[`${side}CompanyId`];
  if (companyID) {
    const tickerRow = state.maps.firstTickerByCompany.get(companyID);
    return tickerRow ? `${tickerRow.ticker} - ${tickerRow.name}` : companyID;
  }
  const isin = row[`${side}Isin`];
  if (isin) {
    const tickerRow = state.maps.firstTickerByIsin.get(isin);
    return tickerRow ? `${tickerRow.ticker} - ${tickerRow.name}` : isin;
  }
  return "";
}

function tickerForExposure(exposure) {
  if (exposure.ticker) return tickerByID(exposure.ticker);
  if (exposure.companyId) return state.maps.firstTickerByCompany.get(exposure.companyId) || null;
  if (exposure.isin) return state.maps.firstTickerByIsin.get(exposure.isin) || null;
  return null;
}

function themeName(id) {
  const theme = state.maps.themeById.get(id);
  return theme ? theme.name : id;
}

function themeNames(ids) {
  return (ids || []).map(themeName);
}

function layerName(themeID, layerID) {
  if (!layerID) return "";
  const scoped = state.maps.layerByThemeAndId.get(`${themeID}:${layerID}`);
  if (scoped) return scoped.name || layerID;
  for (const chain of state.bootstrap.supplyChains || []) {
    const layer = (chain.layers || []).find((item) => item.id === layerID);
    if (layer) return layer.name || layerID;
  }
  return layerID;
}

function localFilterLabel(value) {
  const labels = {
    watchlist: "Watchlist",
    tagged: "Tagged",
    coloured: "Colour labelled"
  };
  return labels[value] || value;
}

function relationshipTypeLabel(value) {
  if (!value) return "Related";
  return String(value).split("_").map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(" ");
}

function labelForQueue(queue) {
  if (queue === "stale_review") return "Stale review";
  if (!queue) return "";
  return queue.charAt(0).toUpperCase() + queue.slice(1);
}

function sourceLabel(row) {
  const source = row.sourceFile ? `${row.sourceFile}${row.sourceRow ? `:${row.sourceRow}` : ""}` : "";
  const reviewed = row.lastReviewed ? `reviewed ${row.lastReviewed}` : "";
  return [source, reviewed].filter(Boolean).join(" - ");
}

function chips(values) {
  return (values || []).filter(Boolean).map((value) => `<span class="chip">${esc(String(value))}</span>`).join("");
}

function countBy(rows, key) {
  const out = {};
  for (const row of rows || []) {
    const value = row[key];
    if (!value) continue;
    out[value] = (out[value] || 0) + 1;
  }
  return out;
}

function sortedKeys(object) {
  return Object.keys(object || {}).sort((a, b) => a.localeCompare(b));
}

function sortedEntries(object) {
  return Object.entries(object || {}).sort((a, b) => {
    if (b[1] === a[1]) return a[0].localeCompare(b[0]);
    return Number(b[1] || 0) - Number(a[1] || 0);
  });
}

function compareValues(a, b) {
  if (typeof a === "number" || typeof b === "number") return Number(a || 0) - Number(b || 0);
  return String(a || "").localeCompare(String(b || ""));
}

function combinedSources(sources) {
  const seen = new Set();
  const out = [];
  for (const source of sources || []) {
    const key = [source.kind, source.url, source.label].join("\x00");
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(source);
  }
  return out;
}

function uniqueStrings(values) {
  return [...new Set((values || []).filter(Boolean))];
}

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

async function copyText(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back for browsers or contexts that expose Clipboard API but deny it.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand("copy");
  textarea.remove();
  if (!ok) throw new Error("Copy failed");
}

function debounce(fn, wait) {
  let timer = null;
  return (...args) => {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => fn(...args), wait);
  };
}

function fileNameFromPath(path) {
  return String(path || "").split("/").pop();
}

function formatMarketCap(value) {
  const numValue = Number(value || 0);
  if (!numValue) return "Unknown";
  if (numValue >= 1e12) return `$${(numValue / 1e12).toFixed(2)}T`;
  if (numValue >= 1e9) return `$${(numValue / 1e9).toFixed(1)}B`;
  if (numValue >= 1e6) return `$${(numValue / 1e6).toFixed(0)}M`;
  return `$${numValue.toLocaleString()}`;
}

function formatBytes(value) {
  const bytes = Number(value || 0);
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function formatDate(value) {
  if (!value) return "Unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toISOString().slice(0, 10);
}

function num(value) {
  return Number(value || 0).toLocaleString();
}

function esc(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
