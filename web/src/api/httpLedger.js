/* HTTP client to the Go backend. Returns the same { status, ok, body, meta }
   envelope the mock used, and keeps a synchronous account cache so the few
   render-time lookups (accountByLabel) still work. */

const BASE = import.meta.env.VITE_API_BASE || "/api/v1";

let accountsCache = [];

async function call(method, path, body, headers = {}) {
  const res = await fetch(BASE + path, {
    method,
    headers: { "Content-Type": "application/json", ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  let json = null;
  try {
    json = await res.json();
  } catch (_) {
    json = null;
  }
  const cached = res.headers.get("Idempotent-Replayed") === "true";
  return { status: res.status, ok: res.ok, body: json, meta: { cached } };
}

function isoOf(asOf) {
  return asOf ? "?as_of=" + encodeURIComponent(new Date(asOf).toISOString()) : "";
}

function shortFailure(name) {
  const n = (name || "").toLowerCase();
  if (n.includes("balance")) return "unbalanced";
  if (n.includes("have entries")) return "empty";
  if (n.includes("belong") || n.includes("orphan")) return "orphaned";
  if (n.includes("pending")) return "pending";
  return "violation";
}

export const ledger = {
  createTransaction(req) {
    const key = req.idempotency_key;
    return call("POST", "/transactions", req, key ? { "Idempotency-Key": key } : {});
  },

  getTransaction(id) {
    return call("GET", "/transactions/" + encodeURIComponent(id));
  },

  // Deterministic key so a repeated reverse replays the same reversal.
  reverseTransaction(id) {
    return call("POST", "/transactions/" + encodeURIComponent(id) + "/reverse", null, {
      "Idempotency-Key": "reverse-" + id,
    });
  },

  async listTransactions() {
    const r = await call("GET", "/transactions");
    return r.ok ? r.body.transactions : [];
  },

  async listAccounts() {
    const r = await call("GET", "/accounts");
    accountsCache = r.ok && r.body ? r.body.accounts : [];
    return accountsCache;
  },
  createAccount(label) {
    return call("POST", "/accounts", { label });
  },
  accountByLabel(label) {
    return accountsCache.find((a) => a.label === label) || null;
  },
  accountById(id) {
    return accountsCache.find((a) => a.id === id) || null;
  },

  async deriveBalances(accountId, asOf) {
    const r = await call("GET", "/accounts/" + accountId + "/balance" + isoOf(asOf));
    return r.ok ? r.body.balances : [];
  },
  async accountEntries(accountId, asOf) {
    const r = await call("GET", "/accounts/" + accountId + "/entries" + isoOf(asOf));
    return r.ok ? r.body.entries : [];
  },

  overview() {
    return call("GET", "/overview");
  },

  async health() {
    try {
      const r = await call("GET", "/health");
      return r.ok;
    } catch (_) {
      return false;
    }
  },

  async reconcile() {
    const t0 = performance.now();
    const r = await call("POST", "/admin/reconcile");
    const duration = Math.max(1, Math.round((performance.now() - t0) * 100) / 100);
    const checks = (r.body.checks || []).map((c) => ({ name: c.name, ok: c.ok, detail: c.detail }));
    const issues = [];
    for (const c of r.body.checks || []) {
      if (!c.ok && Array.isArray(c.offenders)) {
        for (const id of c.offenders) issues.push({ tx_id: id, failure: shortFailure(c.name), detail: c.detail });
      }
    }
    return {
      status: r.status,
      ok: r.ok,
      body: { ok: r.body.ok, ran_at: new Date().toISOString(), duration_ms: duration, checks, issues },
      meta: {},
    };
  },

  reset() {
    return call("POST", "/demo/reset");
  },

  subscribe() {
    return () => {};
  },
  uuid() {
    return crypto.randomUUID();
  },
};

export default ledger;
