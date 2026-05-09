const state = {
  catalogue: null,
  view: "tickers",
  query: "",
  theme: "",
  sector: "",
  localFilter: "",
  sort: { key: "ticker", dir: "asc" },
  local: loadLocal()
};

const $ = (selector) => document.querySelector(selector);
const content = $("#content");

init();

async function init() {
  bindEvents();
  try {
    const response = await fetch("data/catalogue.json", { cache: "no-store" });
    if (!response.ok) throw new Error(`catalogue fetch failed: ${response.status}`);
    state.catalogue = await response.json();
    hydrateFilters();
    render();
  } catch (error) {
    content.innerHTML = `<div class="empty">Unable to load generated data. Run <code>go run ./cmd/statos-build sample</code> from the repo root and serve the <code>site</code> directory.</div>`;
    $("#buildMeta").textContent = error.message;
  }
}

function bindEvents() {
  $("#globalSearch").addEventListener("input", (event) => {
    state.query = event.target.value.trim().toLowerCase();
    render();
  });
  $("#themeFilter").addEventListener("change", (event) => {
    state.theme = event.target.value;
    render();
  });
  $("#sectorFilter").addEventListener("change", (event) => {
    state.sector = event.target.value;
    render();
  });
  $("#localFilter").addEventListener("change", (event) => {
    state.localFilter = event.target.value;
    render();
  });
  document.querySelector(".tabs").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-view]");
    if (!button) return;
    state.view = button.dataset.view;
    document.querySelectorAll(".tabs button").forEach((tab) => tab.classList.toggle("active", tab === button));
    render();
  });
  content.addEventListener("click", handleContentClick);
  $("#modalBody").addEventListener("click", handleContentClick);
  $("#importFile").addEventListener("change", importLocalFile);
}

function hydrateFilters() {
  const themeSelect = $("#themeFilter");
  themeSelect.innerHTML = `<option value="">All themes</option>` + state.catalogue.themes.map((theme) => `<option value="${esc(theme.id)}">${esc(theme.name)}</option>`).join("");

  const sectorSelect = $("#sectorFilter");
  sectorSelect.innerHTML = `<option value="">All sectors</option>` + state.catalogue.sectors.map((sector) => `<option value="${esc(sector.name)}">${esc(sector.name)} (${sector.count})</option>`).join("");
}

function render() {
  if (!state.catalogue) return;
  renderMeta();
  renderMetrics();
  if (state.view === "tickers") renderTickers();
  if (state.view === "themes") renderThemes();
  if (state.view === "supply") renderSupply();
  if (state.view === "sectors") renderSectors();
  if (state.view === "watchlist") renderWatchlist();
  if (state.view === "unclassified") renderUnclassified();
  if (state.view === "exports") renderExports();
}

function renderMeta() {
  const manifest = state.catalogue.manifest;
  $("#buildMeta").textContent = `Built ${formatDate(manifest.builtAt)} - ${manifest.instrumentCount} instruments - ${manifest.unclassifiedCount} unclassified - ${manifest.trading212Environment || "unknown"} source`;
}

function renderMetrics() {
  const manifest = state.catalogue.manifest;
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
  const rows = filteredTickers();
  content.innerHTML = `
    <div class="panel-head">
      <h2>Tickers</h2>
      <p class="muted">${rows.length} shown</p>
    </div>
    ${renderTickerTable(rows)}
  `;
}

function renderTickerTable(rows) {
  if (!rows.length) return `<div class="empty">No tickers match the current filters.</div>`;
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
          ${rows.map(renderTickerRow).join("")}
        </tbody>
      </table>
    </div>
  `;
}

function sortableHead(key, label) {
  const marker = state.sort.key === key ? (state.sort.dir === "asc" ? " up" : " down") : "";
  return `<th><button class="ticker-link" data-action="sort" data-key="${esc(key)}">${esc(label)}${marker}</button></th>`;
}

function renderTickerRow(ticker) {
  const local = localFor(ticker.ticker);
  return `
    <tr>
      <td><button class="ticker-link" data-action="open" data-ticker="${esc(ticker.ticker)}">${esc(ticker.ticker)}</button></td>
      <td><strong>${esc(ticker.name)}</strong><div class="muted">${esc(ticker.isin || "No ISIN")} - ${esc(ticker.currencyCode || "")} ${esc(ticker.exchangeName || "")}</div></td>
      <td>${esc(ticker.sector || "Unclassified")}</td>
      <td>${esc(ticker.industry || "Unclassified")}</td>
      <td>${chips(themeNames(ticker.themeIds))}</td>
      <td>${formatMarketCap(ticker.marketCap)}</td>
      <td>${localBadges(local)} <button class="small-button" data-action="watch" data-ticker="${esc(ticker.ticker)}">${local.watchlist ? "Remove" : "Watch"}</button></td>
    </tr>
  `;
}

function renderThemes() {
  const counts = new Map();
  for (const ticker of state.catalogue.tickers) {
    for (const themeID of ticker.themeIds || []) {
      counts.set(themeID, (counts.get(themeID) || 0) + 1);
    }
  }
  content.innerHTML = `
    <div class="panel-head"><h2>Themes</h2><p class="muted">${state.catalogue.themes.length} taxonomy pillars</p></div>
    <div class="grid">
      ${state.catalogue.themes.map((theme) => `
        <article class="card">
          <h3>${esc(theme.name)}</h3>
          <p>${esc(theme.description || "")}</p>
          <div class="chips" style="margin-top:10px">${chips([`${counts.get(theme.id) || 0} mapped tickers`])}</div>
        </article>
      `).join("")}
    </div>
  `;
}

function renderSupply() {
  const chains = state.catalogue.supplyChains || [];
  if (!chains.length) {
    content.innerHTML = `<div class="empty">No supply chains are defined yet.</div>`;
    return;
  }
  const selectedTheme = state.theme || chains[0].themeId;
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
      ${chain.layers.map((layer) => renderLayer(chain.themeId, layer)).join("")}
    </div>
  `;
  $("#supplyThemeSelect").addEventListener("change", (event) => {
    state.theme = event.target.value;
    $("#themeFilter").value = state.theme;
    renderSupply();
  });
}

function renderLayer(themeID, layer) {
  const exposures = state.catalogue.exposures.filter((exposure) => exposure.themeId === themeID && exposure.layerId === layer.id);
  const cards = exposures.map((exposure) => {
    const ticker = tickerForExposure(exposure);
    const company = ticker ? companyByID(ticker.companyId) : companyByID(exposure.companyId);
    const title = ticker ? ticker.name : (company ? company.name : exposure.companyId || exposure.ticker || exposure.isin);
    const width = Math.min(280, 140 + Number(exposure.exposureScore || 0) * 26);
    const confidenceClass = exposure.confidence && exposure.confidence.includes("high") ? "high" : exposure.confidence && exposure.confidence.includes("medium") ? "medium" : "low";
    return `
      <article class="supply-card ${confidenceClass}" style="--card-width:${width}px">
        <button data-action="open" data-ticker="${esc(ticker ? ticker.ticker : "")}">${esc(title)}</button>
        <div class="meta">${ticker ? esc(ticker.ticker) : esc(exposure.companyId || "")} - score ${esc(String(exposure.exposureScore || 0))}</div>
        <div class="chips" style="margin-top:8px">${chips([exposure.confidence || "unrated", ticker ? ticker.industry : "manual"])}</div>
      </article>
    `;
  }).join("");
  return `
    <section class="layer-row">
      <div class="layer-label"><strong>${esc(layer.name)}</strong><span>${esc(layer.description || "")}</span></div>
      <div class="layer-cards">${cards || `<div class="empty">No mapped tickers in this layer yet.</div>`}</div>
    </section>
  `;
}

function renderSectors() {
  content.innerHTML = `
    <div class="panel-head"><h2>Sector and industry explorer</h2><p class="muted">Counts use current generated catalogue data.</p></div>
    <div class="grid">
      ${state.catalogue.sectors.map((sector) => `
        <article class="card">
          <h3>${esc(sector.name)} (${sector.count})</h3>
          <div class="chips">${chips((sector.tickers || []).slice(0, 12))}</div>
        </article>
      `).join("")}
      ${state.catalogue.industries.map((industry) => `
        <article class="card">
          <h3>${esc(industry.name)} (${industry.count})</h3>
          <div class="chips">${chips((industry.tickers || []).slice(0, 12))}</div>
        </article>
      `).join("")}
    </div>
  `;
}

function renderWatchlist() {
  const rows = filteredTickers().filter((ticker) => localFor(ticker.ticker).watchlist);
  content.innerHTML = `
    <div class="panel-head">
      <h2>Watchlist</h2>
      <p class="muted">${rows.length} watched tickers</p>
    </div>
    ${renderTickerTable(rows)}
  `;
}

function renderUnclassified() {
  const rows = state.catalogue.unclassified.filter((row) => {
    const ticker = tickerByID(row.ticker);
    return !ticker || tickerMatches(ticker);
  });
  content.innerHTML = `
    <div class="panel-head">
      <h2>Unclassified queue</h2>
      <p class="muted">${rows.length} rows need review</p>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>Ticker</th><th>Name</th><th>ISIN</th><th>Reason</th></tr></thead>
        <tbody>
          ${rows.map((row) => `
            <tr>
              <td><button class="ticker-link" data-action="open" data-ticker="${esc(row.ticker)}">${esc(row.ticker)}</button></td>
              <td>${esc(row.name)}</td>
              <td>${esc(row.isin || "")}</td>
              <td>${esc(row.reason)}</td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    </div>
  `;
}

function renderExports() {
  content.innerHTML = `
    <div class="panel-head"><h2>Data exports</h2><p class="muted">Generated files are static and committed under site/data.</p></div>
    <div class="exports">
      ${["catalogue.json", "securities.json", "listings.json", "relationships.json", "tickers.csv", "securities.csv", "listings.csv", "identity_issues.csv", "enrichment_failures.csv", "companies.json", "sectors.json", "industries.json", "themes.json", "supply_chains.json", "search_index.json", "unclassified.csv", "build_manifest.json"].map((name) => `<a class="export-link" href="data/${name}">${name}</a>`).join("")}
    </div>
    <div class="panel-head" style="margin-top:18px">
      <h2>Local browser data</h2>
      <div class="chips">
        <button class="small-button primary" data-action="export-local">Export local JSON</button>
        <button class="small-button" data-action="import-local">Import local JSON</button>
      </div>
    </div>
  `;
}

function handleContentClick(event) {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const action = button.dataset.action;
  if (action === "open" && button.dataset.ticker) openTicker(button.dataset.ticker);
  if (action === "watch") toggleWatch(button.dataset.ticker);
  if (action === "sort") sortBy(button.dataset.key);
  if (action === "export-local") exportLocal();
  if (action === "import-local") $("#importFile").click();
}

function openTicker(tickerID) {
  const ticker = tickerByID(tickerID);
  if (!ticker) return;
  const company = companyByID(ticker.companyId) || {};
  const security = state.catalogue.securities.find((item) => item.id === ticker.securityId) || {};
  const local = localFor(ticker.ticker);
  $("#modalTitle").textContent = ticker.ticker;
  $("#modalSubtitle").textContent = ticker.name;
  $("#modalBody").innerHTML = `
    <div class="detail-grid">
      <section>
        <div class="facts">
          ${fact("Company", company.name || ticker.name)}
          ${fact("ISIN", ticker.isin || "None")}
          ${fact("Instrument type", ticker.type || "Unknown")}
          ${fact("Currency", ticker.currencyCode || "Unknown")}
          ${fact("Exchange", ticker.exchangeName || ticker.exchangeCode || "Unknown")}
          ${fact("Yahoo symbol", ticker.yahooSymbol || "Unresolved")}
          ${fact("Sector", ticker.sector || "Unclassified")}
          ${fact("Industry", ticker.industry || "Unclassified")}
          ${fact("Market cap", formatMarketCap(ticker.marketCap || company.marketCap))}
          ${fact("Directionality", ticker.directionality)}
          ${fact("Security", security.id || ticker.securityId)}
          ${fact("Last refreshed", formatDate(ticker.lastRefreshed))}
        </div>
        <div class="card" style="margin-top:12px">
          <h3>Theme and layer memberships</h3>
          <div class="chips">${chips([...themeNames(ticker.themeIds), ...(ticker.layerIds || [])]) || `<span class="muted">None mapped yet</span>`}</div>
        </div>
        <div class="card" style="margin-top:12px">
          <h3>Related tickers</h3>
          <div class="chips">${(ticker.relatedTickers || []).map((id) => `<button class="chip" data-action="open" data-ticker="${esc(id)}">${esc(id)}</button>`).join("") || `<span class="muted">None</span>`}</div>
        </div>
        <div class="card" style="margin-top:12px">
          <h3>Sources</h3>
          ${renderSources(ticker.sources || company.sources || [])}
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
    openTicker(ticker.ticker);
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
  const modal = $("#detailModal");
  if (modal.showModal) modal.showModal();
  else modal.setAttribute("open", "open");
}

function fact(label, value) {
  return `<div class="fact"><span>${esc(label)}</span><strong>${esc(String(value || ""))}</strong></div>`;
}

function renderSources(sources) {
  if (!sources.length) return `<p class="muted">No sources attached.</p>`;
  return `<div class="chips">${sources.map((source) => source.url ? `<a class="chip" href="${esc(source.url)}">${esc(source.label || source.kind)}</a>` : `<span class="chip">${esc(source.label || source.kind)}</span>`).join("")}</div>`;
}

function filteredTickers() {
  const rows = state.catalogue.tickers.filter(tickerMatches);
  rows.sort((a, b) => compareValues(a[state.sort.key], b[state.sort.key]) * (state.sort.dir === "asc" ? 1 : -1));
  return rows;
}

function tickerMatches(ticker) {
  if (state.theme && !(ticker.themeIds || []).includes(state.theme)) return false;
  if (state.sector && ticker.sector !== state.sector) return false;
  const local = localFor(ticker.ticker);
  if (state.localFilter === "watchlist" && !local.watchlist) return false;
  if (state.localFilter === "tagged" && !(local.tags || []).length) return false;
  if (state.localFilter === "coloured" && !local.color) return false;
  if (!state.query) return true;
  const company = companyByID(ticker.companyId) || {};
  const notes = local.notes || "";
  const haystack = [
    ticker.ticker, ticker.name, ticker.isin, ticker.yahooSymbol, ticker.sector, ticker.industry,
    ticker.country, company.name, ...(ticker.themeIds || []).map(themeName), ...(local.tags || []), notes
  ].join(" ").toLowerCase();
  return haystack.includes(state.query);
}

function sortBy(key) {
  if (state.sort.key === key) state.sort.dir = state.sort.dir === "asc" ? "desc" : "asc";
  else state.sort = { key, dir: "asc" };
  render();
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
    return JSON.parse(localStorage.getItem("statos.local.v1")) || { tickers: {} };
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
    render();
  } finally {
    event.target.value = "";
  }
}

function tickerByID(id) {
  return state.catalogue.tickers.find((ticker) => ticker.ticker === id);
}

function companyByID(id) {
  return state.catalogue.companies.find((company) => company.id === id);
}

function tickerForExposure(exposure) {
  if (exposure.ticker) return tickerByID(exposure.ticker);
  if (exposure.companyId) return state.catalogue.tickers.find((ticker) => ticker.companyId === exposure.companyId);
  if (exposure.isin) return state.catalogue.tickers.find((ticker) => ticker.isin === exposure.isin);
  return null;
}

function themeName(id) {
  const theme = state.catalogue.themes.find((item) => item.id === id);
  return theme ? theme.name : id;
}

function themeNames(ids) {
  return (ids || []).map(themeName);
}

function chips(values) {
  return (values || []).filter(Boolean).map((value) => `<span class="chip">${esc(String(value))}</span>`).join("");
}

function compareValues(a, b) {
  if (typeof a === "number" || typeof b === "number") return Number(a || 0) - Number(b || 0);
  return String(a || "").localeCompare(String(b || ""));
}

function formatMarketCap(value) {
  const numValue = Number(value || 0);
  if (!numValue) return "Unknown";
  if (numValue >= 1e12) return `$${(numValue / 1e12).toFixed(2)}T`;
  if (numValue >= 1e9) return `$${(numValue / 1e9).toFixed(1)}B`;
  if (numValue >= 1e6) return `$${(numValue / 1e6).toFixed(0)}M`;
  return `$${numValue.toLocaleString()}`;
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
