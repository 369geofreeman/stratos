const INITIAL_TABLE_ROWS = 150;
const TABLE_ROWS_INCREMENT = 150;
const INITIAL_CARD_COUNT = 120;
const CARD_INCREMENT = 120;
const EMPTY_LOCAL = Object.freeze({ watchlist: false, notes: "", tags: [], color: "" });

const DEFAULT_EXPORTS = [
  "app_bootstrap.json",
  "tickers_index.json",
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
  reviewQueues: null,
  reviewSummary: null,
  visibleReviewRows: [],
  detail: null,
  view: "tickers",
  query: "",
  theme: "",
  supplyTheme: "",
  sector: "",
  localFilter: "",
  sort: { key: "ticker", dir: "asc" },
  reviewFilters: { queue: "", reason: "", severity: "", gap: "", search: "" },
  listWindows: {},
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
    resetListWindows();
    render();
  });
  content.addEventListener("click", handleContentClick);
  content.addEventListener("change", handleContentChange);
  content.addEventListener("input", handleContentInput);
  $("#modalBody").addEventListener("click", handleContentClick);
  $("#importFile").addEventListener("change", importLocalFile);
}

async function loadBootstrap() {
  state.bootstrap = await fetchJSON("data/app_bootstrap.json");
  indexBootstrap();
}

function indexBootstrap() {
  state.maps.themeById = new Map((state.bootstrap.themes || []).map((theme) => [theme.id, theme]));
}

function hydrateFilters() {
  const themes = state.bootstrap.themes || [];
  const sectors = state.bootstrap.sectors || [];
  const themeSelect = $("#themeFilter");
  themeSelect.innerHTML = `<option value="">All themes</option>` + themes.map((theme) => `<option value="${esc(theme.id)}">${esc(theme.name)}</option>`).join("");

  const sectorSelect = $("#sectorFilter");
  sectorSelect.innerHTML = `<option value="">All sectors</option>` + sectors.map((sector) => `<option value="${esc(sector.name)}">${esc(sector.name)} (${num(sector.count)})</option>`).join("");
}

function render() {
  if (!state.bootstrap) return;
  renderMeta();
  renderMetrics();
  syncTabs();
  if (state.view === "tickers") renderTickers();
  if (state.view === "themes") renderThemes();
  if (state.view === "supply") renderSupply();
  if (state.view === "sectors") renderSectors();
  if (state.view === "watchlist") renderWatchlist();
  if (state.view === "unclassified") renderUnclassified();
  if (state.view === "exports") renderExports();
}

function syncTabs() {
  document.querySelectorAll(".tabs button").forEach((tab) => {
    tab.classList.toggle("active", tab.dataset.view === state.view);
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
        ${hasActiveTickerFilters() ? `<button class="small-button" data-action="clear-filters">Clear filters</button>` : ""}
      </div>
    </div>
    ${renderActiveTickerFilters()}
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

function renderActiveTickerFilters() {
  const filters = [];
  if (state.query) filters.push(`search: ${state.query}`);
  if (state.theme) filters.push(`theme: ${themeName(state.theme)}`);
  if (state.sector) filters.push(`sector: ${state.sector}`);
  if (state.localFilter) filters.push(`local: ${state.localFilter}`);
  if (!filters.length) return "";
  return `<div class="active-filters">${chips(filters)}</div>`;
}

function renderTickerTable(listID, rows) {
  if (!rows.length) return `<div class="empty">No tickers match the current filters.</div>`;
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
  const marker = state.sort.key === key ? (state.sort.dir === "asc" ? " up" : " down") : "";
  return `<th><button class="ticker-link" data-action="sort" data-key="${esc(key)}">${esc(label)}${marker}</button></th>`;
}

function renderTickerRow(ticker) {
  const local = getLocal(ticker.ticker);
  return `
    <tr>
      <td><button class="ticker-link" data-action="open" data-ticker="${esc(ticker.ticker)}">${esc(ticker.ticker)}</button></td>
      <td><strong>${esc(ticker.name)}</strong><div class="muted">${esc(ticker.isin || "No ISIN")} - ${esc(ticker.currencyCode || "")} ${esc(ticker.exchangeName || ticker.exchangeCode || "")}</div></td>
      <td>${esc(ticker.sector || "Unclassified")}</td>
      <td>${esc(ticker.industry || "Unclassified")}</td>
      <td>${chips(themeNames(ticker.themeIds))}</td>
      <td>${formatMarketCap(ticker.marketCap)}</td>
      <td>${localBadges(local)} <button class="small-button" data-action="watch" data-ticker="${esc(ticker.ticker)}">${local.watchlist ? "Remove" : "Watch"}</button></td>
    </tr>
  `;
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
  const selectedTheme = state.supplyTheme || state.theme || chains[0].themeId;
  const chain = chains.find((item) => item.themeId === selectedTheme) || chains[0];
  content.innerHTML = `
    <div class="supply-toolbar">
      <div>
        <h2>${esc(chain.name)}</h2>
        <p class="muted">${esc(chain.description || "")}</p>
      </div>
      <select id="supplyThemeSelect" aria-label="Supply chain theme">
        ${chains.map((item) => `<option value="${esc(item.themeId)}" ${item.themeId === chain.themeId ? "selected" : ""}>${esc(themeName(item.themeId))}</option>`).join("")}
      </select>
    </div>
    <div class="supply-map">
      ${(chain.layers || []).map((layer) => renderLayer(chain.themeId, layer)).join("")}
    </div>
  `;
  $("#supplyThemeSelect").addEventListener("change", (event) => {
    state.supplyTheme = event.target.value;
    resetListWindows();
    renderSupply();
  });
}

function renderLayer(themeID, layer) {
  const exposures = (state.bootstrap.exposures || []).filter((exposure) => exposure.themeId === themeID && exposure.layerId === layer.id);
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
      </article>
    `;
  }).join("");
  return `
    <section class="layer-row">
      <div class="layer-label"><strong>${esc(layer.name)}</strong><span>${esc(layer.description || "")}</span></div>
      <div>
        <div class="layer-cards">${cards || `<div class="empty">No mapped tickers in this layer yet.</div>`}</div>
        ${renderListFooter(listID, exposures.length, visible, "cards", CARD_INCREMENT)}
      </div>
    </section>
  `;
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
  return `
    <article class="card">
      <h3>${esc(group.kind)}: ${esc(group.name)} (${num(group.count)})</h3>
      <div class="chips">${chips(chipValues)}</div>
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
    ${renderReviewToolbar()}
    ${renderReasonCounts()}
    ${renderReviewTable(rows)}
  `;
}

function renderReviewToolbar() {
  const filters = state.reviewFilters;
  const reasons = sortedKeys(state.reviewSummary?.byReasonCode || countBy(state.reviewQueues, "reasonCode"));
  const queues = sortedKeys(state.reviewSummary?.byQueue || countBy(state.reviewQueues, "queue"));
  const severities = ["high", "medium", "low"].filter((severity) => (state.reviewSummary?.bySeverity || {})[severity] || state.reviewQueues.some((row) => row.severity === severity));
  return `
    <div class="review-toolbar">
      <input id="reviewSearch" type="search" value="${esc(filters.search)}" placeholder="Search review rows, identifiers, source files, or actions">
      <select data-review-filter="queue" aria-label="Review queue filter">
        <option value="">All queues</option>
        ${queues.map((queue) => `<option value="${esc(queue)}" ${filters.queue === queue ? "selected" : ""}>${esc(labelForQueue(queue))}</option>`).join("")}
      </select>
      <select data-review-filter="reason" aria-label="Reason code filter">
        <option value="">All reasons</option>
        ${reasons.map((reason) => `<option value="${esc(reason)}" ${filters.reason === reason ? "selected" : ""}>${esc(reason)}</option>`).join("")}
      </select>
      <select data-review-filter="severity" aria-label="Severity filter">
        <option value="">All severities</option>
        ${severities.map((severity) => `<option value="${esc(severity)}" ${filters.severity === severity ? "selected" : ""}>${esc(severity)}</option>`).join("")}
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
  if (!rows.length) return `<div class="empty">No review rows match the current filters.</div>`;
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
      <h2>Local browser data</h2>
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
  if (action === "watch") toggleWatch(button.dataset.ticker);
  if (action === "sort") sortBy(button.dataset.key);
  if (action === "clear-filters") clearTickerFilters();
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
      }).catch((error) => window.alert(error.message));
    }
  }
}

function handleContentChange(event) {
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
  $("#modalTitle").textContent = ticker.ticker;
  $("#modalSubtitle").textContent = ticker.name;
  $("#modalBody").innerHTML = loadingBlock("Loading ticker detail", "Fetching company, security, listing, and relationship slices.");
  showModal();
}

function renderTickerModal(tickerID) {
  const ticker = tickerByID(tickerID);
  if (!ticker) return;
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
          <div class="chips">${chips([...themeNames(ticker.themeIds), ...(ticker.layerIds || [])]) || `<span class="muted">None mapped yet</span>`}</div>
        </div>
        <div class="card detail-card">
          <h3>Related tickers</h3>
          <div class="chips">${related.map((id) => `<button class="chip" data-action="open" data-ticker="${esc(id)}">${esc(id)}</button>`).join("") || `<span class="muted">None</span>`}</div>
        </div>
        <div class="card detail-card">
          <h3>Reviewed relationships</h3>
          ${renderRelationships(ticker.ticker)}
        </div>
        <div class="card detail-card">
          <h3>Sources</h3>
          ${renderSources(combinedSources(company.sources || []))}
        </div>
      </section>
      <section class="local-tools">
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
    render();
  });
  $("#modalTags").addEventListener("input", (event) => {
    localFor(ticker.ticker).tags = event.target.value.split(",").map((item) => item.trim()).filter(Boolean);
    saveLocal();
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

function fact(label, value) {
  return `<div class="fact"><span>${esc(label)}</span><strong>${esc(String(value || ""))}</strong></div>`;
}

function renderRelationships(tickerID) {
  const rows = relationshipsForTicker(tickerID);
  if (!rows.length) return `<p class="muted">No reviewed relationships.</p>`;
  return `
    <div class="relationship-list">
      ${rows.map((row) => {
        const isSource = endpointContainsTicker(row, "source", tickerID);
        const other = isSource ? relationshipEndpointLabel(row, "target") : relationshipEndpointLabel(row, "source");
        const context = [row.themeId, row.layerId, row.confidence].filter(Boolean).join(" / ");
        return `
          <div>
            <strong>${esc(row.relationshipType || "related")}</strong>
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
  else state.sort = { key, dir: "asc" };
  resetListWindows();
  render();
}

function clearTickerFilters() {
  state.query = "";
  state.theme = "";
  state.sector = "";
  state.localFilter = "";
  $("#globalSearch").value = "";
  $("#themeFilter").value = "";
  $("#sectorFilter").value = "";
  $("#localFilter").value = "";
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
    const parsed = JSON.parse(localStorage.getItem("statos.local.v1"));
    if (!parsed || typeof parsed !== "object") return { tickers: {} };
    if (!parsed.tickers || typeof parsed.tickers !== "object") parsed.tickers = {};
    return parsed;
  } catch {
    return { tickers: {} };
  }
}

function saveLocal() {
  localStorage.setItem("statos.local.v1", JSON.stringify(state.local));
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
    state.local = parsed;
    saveLocal();
    resetListWindows();
    render();
  } catch (error) {
    window.alert(error.message);
  } finally {
    event.target.value = "";
  }
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

async function ensureDetailData() {
  if (state.detail) return state.detail;
  if (!state.promises.detail) {
    state.promises.detail = Promise.all([
      fetchJSON("data/companies.json"),
      fetchJSON("data/securities.json"),
      fetchJSON("data/listings.json"),
      fetchJSON("data/relationships.json")
    ]).then(([companies, securities, listings, relationships]) => {
      state.detail = {
        companies: asArray(companies),
        securities: asArray(securities),
        listings: asArray(listings),
        relationships: asArray(relationships)
      };
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
  const relationshipsByTicker = new Map();
  for (const row of state.detail.relationships) {
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
    await navigator.clipboard.writeText(text);
    return;
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
