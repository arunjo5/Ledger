/* =====================================================================
   LedgerCore — in-memory store + mock API layer
   ---------------------------------------------------------------------
   Entirely client-side. No network. No persistence beyond the tab.
   Every public method returns a synthetic HTTP response of the shape:
       { status: number, ok: boolean, body: any, meta?: {...} }
   so the UI can render real status codes + raw JSON.

   Invariants enforced here (the UI merely reflects them):
     - Zero-sum: a transaction's entries must net to 0 per currency.
     - Idempotency: same key + same body -> cached response, no new write.
                    same key + different body -> 409 conflict.
     - Immutability: entries are never edited/deleted. "Undo" = reversal.
     - Overdraft: protected accounts may never go negative in any currency.
     - Balances are DERIVED from SUM(entries), never stored.
   ===================================================================== */
function createLedger() {
  "use strict";

  const DAY = 24 * 60 * 60 * 1000;
  const uuid = () =>
    (crypto.randomUUID && crypto.randomUUID()) ||
    "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
      const r = (Math.random() * 16) | 0;
      return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
    });

  // ---- internal state ------------------------------------------------
  let accounts = [];          // { id, label, created_at, overdraft_protected }
  let transactions = [];      // committed transactions (see shape in spec)
  let idemCache = new Map();  // key -> { bodyHash, txId, response }
  let requestLog = [];        // every write attempt (drives the Overview)
  let subscribers = new Set();

  function logRequest(entry) {
    requestLog.push({
      id: entry.id || uuid(),
      status: entry.status,            // committed | pending | failed
      description: entry.description || "",
      lines: entry.lines != null ? entry.lines : 0,
      created_at: entry.created_at || new Date().toISOString(),
      tx_id: entry.tx_id || null,
    });
  }

  function notify() { subscribers.forEach((fn) => fn()); }

  // Deterministic serialization so the same logical body hashes the same.
  function bodyHash(body) {
    const norm = {
      description: body.description || "",
      entries: (body.entries || []).map((e) => ({
        account_id: e.account_id,
        currency: e.currency,
        amount: e.amount,
      })),
    };
    return JSON.stringify(norm);
  }

  function accountByLabel(label) {
    return accounts.find((a) => a.label === label) || null;
  }
  function accountById(id) {
    return accounts.find((a) => a.id === id) || null;
  }

  // ---- seed ----------------------------------------------------------
  function reset() {
    accounts = [];
    transactions = [];
    idemCache = new Map();
    requestLog = [];

    const now = Date.now();
    const mk = (label, prot) => {
      const a = {
        id: uuid(),
        label,
        created_at: new Date(now - 30 * DAY).toISOString(),
        overdraft_protected: prot,
      };
      accounts.push(a);
      return a;
    };
    const cashOp = mk("cash:operating", false);
    const cashFx = mk("cash:fx_buffer", false);
    const revenue = mk("revenue:sales", false);
    const fees = mk("fees:processing", false);
    const customer_a = mk("liabilities:customer_a", true);
    const customer_b = mk("liabilities:customer_b", true);

    // Seed transactions: [daysAgo, description, [[account, currency, amount]...]]
    const seed = [
      [21, "Customer funding \u2014 wire", [[cashOp, "USD", -10000], [customer_a, "USD", 10000]]],
      [14, "Customer funding \u2014 ACH", [[cashOp, "USD", -5000], [customer_b, "USD", 5000]]],
      [10, "Internal transfer customer_a \u2192 customer_b", [[customer_a, "USD", -3000], [customer_b, "USD", 3000]]],
      [5, "Processing fee", [[customer_b, "USD", -100], [fees, "USD", 100]]],
      [2, "FX funding (EUR)", [[cashFx, "EUR", -4000], [customer_a, "EUR", 4000]]],
      [1, "Customer payment \u2014 invoice 8821", [[customer_a, "USD", -2000], [revenue, "USD", 2000]]],
    ];

    for (const [daysAgo, description, rows] of seed) {
      const ts = new Date(now - daysAgo * DAY).toISOString();
      const tx = {
        id: uuid(),
        status: "committed",
        description,
        entries: rows.map(([acct, currency, amount]) => ({
          id: uuid(),
          account_id: acct.id,
          currency,
          amount,
        })),
        created_at: ts,
        committed_at: ts,
        reverses_transaction_id: null,
      };
      transactions.push(tx);
      logRequest({ id: tx.id, status: "committed", description, lines: tx.entries.length, created_at: ts, tx_id: tx.id });
    }

    // A couple of non-committed requests so the Overview has pending/failed.
    logRequest({
      status: "pending",
      description: "Payout to merchant acme_co",
      lines: 2,
      created_at: new Date(now - 6 * 60 * 60 * 1000).toISOString(),
    });
    logRequest({
      status: "failed",
      description: "Overdraft attempt \u2014 customer_b",
      lines: 2,
      created_at: new Date(now - 3 * 60 * 60 * 1000).toISOString(),
    });

    notify();
  }

  // ---- derivations ---------------------------------------------------
  // Balance = SUM(entries) over committed transactions with committed_at <= asOf.
  function deriveBalances(accountId, asOf) {
    const cutoff = asOf ? new Date(asOf).getTime() : Infinity;
    const byCur = new Map();
    for (const tx of transactions) {
      if (tx.status !== "committed") continue;
      if (new Date(tx.committed_at).getTime() > cutoff) continue;
      for (const e of tx.entries) {
        if (e.account_id !== accountId) continue;
        byCur.set(e.currency, (byCur.get(e.currency) || 0) + e.amount);
      }
    }
    return [...byCur.entries()]
      .map(([currency, amount]) => ({ currency, amount }))
      .sort((a, b) => a.currency.localeCompare(b.currency));
  }

  // Recent entries (newest first) touching an account, with tx context.
  function accountEntries(accountId, asOf) {
    const cutoff = asOf ? new Date(asOf).getTime() : Infinity;
    const out = [];
    for (const tx of transactions) {
      if (tx.status !== "committed") continue;
      if (new Date(tx.committed_at).getTime() > cutoff) continue;
      for (const e of tx.entries) {
        if (e.account_id !== accountId) continue;
        out.push({
          id: e.id,
          transaction_id: tx.id,
          created_at: tx.committed_at,
          currency: e.currency,
          amount: e.amount,
        });
      }
    }
    out.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
    return out;
  }

  // Net a set of entries by currency -> { USD: 0, EUR: -200 }
  function netByCurrency(entries) {
    const m = {};
    for (const e of entries) {
      const amt = Number(e.amount) || 0;
      m[e.currency] = (m[e.currency] || 0) + amt;
    }
    return m;
  }

  // ---- validation helpers --------------------------------------------
  function validateEntries(entries) {
    if (!Array.isArray(entries) || entries.length === 0) {
      return { error: "transaction must have at least one entry", code: "empty_transaction" };
    }
    for (const e of entries) {
      if (!e.account_id || !accountById(e.account_id)) {
        return { error: "entry references unknown account", code: "unknown_account" };
      }
      if (!/^[A-Z]{3}$/.test(e.currency || "")) {
        return { error: `invalid currency "${e.currency}" (expected 3 uppercase letters)`, code: "bad_currency" };
      }
      if (!Number.isInteger(Number(e.amount))) {
        return { error: "amount must be a signed integer (minor units)", code: "bad_amount" };
      }
    }
    return null;
  }

  // Would committing `entries` drive any overdraft-protected account negative?
  function overdraftCheck(entries) {
    // group delta per (account, currency)
    const delta = new Map(); // key acctId|cur -> amount
    for (const e of entries) {
      const k = e.account_id + "|" + e.currency;
      delta.set(k, (delta.get(k) || 0) + Number(e.amount));
    }
    for (const [k, d] of delta.entries()) {
      const [acctId, cur] = k.split("|");
      const acct = accountById(acctId);
      if (!acct || !acct.overdraft_protected) continue;
      const current = deriveBalances(acctId).find((b) => b.currency === cur);
      const resulting = (current ? current.amount : 0) + d;
      if (resulting < 0) {
        return { error: `would overdraw account ${acct.label}`, code: "overdraft", account: acct.label, currency: cur };
      }
    }
    return null;
  }

  function resp(status, body, meta) {
    return { status, ok: status >= 200 && status < 300, body, meta: meta || {} };
  }

  // ---- the mock API --------------------------------------------------
  // POST /transactions
  function createTransaction(req) {
    const { description = "", idempotency_key = "", entries = [] } = req || {};

    // 1) idempotency replay check (before validation, mirrors real systems)
    if (idempotency_key && idemCache.has(idempotency_key)) {
      const cached = idemCache.get(idempotency_key);
      if (cached.bodyHash === bodyHash(req)) {
        return resp(cached.response.status, cached.response.body, { cached: true });
      }
      const conflict = resp(409, {
        error: "Same idempotency key, different body",
        code: "idempotency_conflict",
        idempotency_key,
      });
      logRequest({ status: "failed", description, lines: (entries || []).length, tx_id: null });
      return conflict;
    }

    // 2) structural validation
    const structural = validateEntries(entries);
    if (structural) {
      logRequest({ status: "failed", description, lines: (entries || []).length, tx_id: null });
      return resp(400, structural);
    }

    // 3) zero-sum per currency
    const net = netByCurrency(entries);
    const unbalanced = Object.entries(net).filter(([, v]) => v !== 0);
    if (unbalanced.length) {
      const detail = {};
      unbalanced.forEach(([c, v]) => (detail[c] = v));
      const r = resp(400, {
        error: "transaction does not net to zero per currency",
        code: "unbalanced",
        net: detail,
      });
      logRequest({ status: "failed", description, lines: entries.length, tx_id: null });
      return r;
    }

    // 4) overdraft protection
    const od = overdraftCheck(entries);
    if (od) {
      const r = resp(400, od);
      logRequest({ status: "failed", description, lines: entries.length, tx_id: null });
      return r;
    }

    // 5) commit
    const ts = new Date().toISOString();
    const tx = {
      id: uuid(),
      status: "committed",
      description,
      entries: entries.map((e) => ({
        id: uuid(),
        account_id: e.account_id,
        currency: e.currency,
        amount: Number(e.amount),
      })),
      created_at: ts,
      committed_at: ts,
      reverses_transaction_id: req.reverses_transaction_id || null,
    };
    transactions.push(tx);
    logRequest({ id: tx.id, status: "committed", description, lines: tx.entries.length, created_at: ts, tx_id: tx.id });

    const r = resp(201, tx);
    if (idempotency_key) {
      idemCache.set(idempotency_key, { bodyHash: bodyHash(req), txId: tx.id, response: r });
    }
    notify();
    return r;
  }

  // GET /transactions/:id
  function getTransaction(id) {
    const tx = transactions.find((t) => t.id === id);
    if (!tx) return resp(404, { error: "no transaction with that id", code: "not_found" });
    return resp(200, tx);
  }

  // POST /transactions/:id/reverse  -> mirror-image transaction
  function reverseTransaction(id) {
    const tx = transactions.find((t) => t.id === id);
    if (!tx) return resp(404, { error: "no transaction with that id", code: "not_found" });

    const mirror = tx.entries.map((e) => ({
      account_id: e.account_id,
      currency: e.currency,
      amount: -e.amount,
    }));

    // overdraft still applies to reversals
    const od = overdraftCheck(mirror);
    if (od) return resp(400, od);

    const ts = new Date().toISOString();
    const rev = {
      id: uuid(),
      status: "committed",
      description: "Reversal of " + tx.id,
      entries: mirror.map((e) => ({ id: uuid(), ...e })),
      created_at: ts,
      committed_at: ts,
      reverses_transaction_id: tx.id,
    };
    transactions.push(rev);
    logRequest({ id: rev.id, status: "committed", description: rev.description, lines: rev.entries.length, created_at: ts, tx_id: rev.id });
    notify();
    return resp(201, rev);
  }

  // ---- accounts ------------------------------------------------------
  function listAccounts() {
    return accounts.map((a) => ({ ...a }));
  }
  function createAccount(label) {
    label = (label || "").trim();
    if (!label) return resp(400, { error: "label required", code: "bad_label" });
    if (accountByLabel(label)) return resp(409, { error: "account label already exists", code: "duplicate_label" });
    const a = { id: uuid(), label, created_at: new Date().toISOString(), overdraft_protected: false };
    accounts.push(a);
    notify();
    return resp(201, a);
  }

  // ---- reconciliation ------------------------------------------------
  function reconcile() {
    const t0 = performance.now();
    const checks = [];
    const issues = [];

    // 1) every committed tx nets to zero per currency
    let unbalancedCount = 0;
    for (const tx of transactions) {
      if (tx.status !== "committed") continue;
      const net = netByCurrency(tx.entries);
      const bad = Object.entries(net).filter(([, v]) => v !== 0);
      if (bad.length) {
        unbalancedCount++;
        issues.push({ tx_id: tx.id, failure: "unbalanced", detail: bad.map(([c, v]) => `${c} ${v > 0 ? "+" : ""}${v}`).join(", ") });
      }
    }
    checks.push({
      name: "Every committed transaction balances to zero per currency",
      ok: unbalancedCount === 0,
      detail: unbalancedCount === 0 ? "all transactions net to 0" : `${unbalancedCount} unbalanced`,
    });

    // 2) no committed tx with zero entries
    let emptyCount = 0;
    for (const tx of transactions) {
      if (tx.status !== "committed") continue;
      if (!tx.entries || tx.entries.length === 0) {
        emptyCount++;
        issues.push({ tx_id: tx.id, failure: "empty", detail: "committed transaction has no entries" });
      }
    }
    checks.push({
      name: "No committed transaction has zero entries",
      ok: emptyCount === 0,
      detail: emptyCount === 0 ? "none empty" : `${emptyCount} empty`,
    });

    // 3) no entries without a committed transaction
    let orphanCount = 0;
    for (const tx of transactions) {
      if (tx.status === "committed") continue;
      orphanCount += (tx.entries || []).length;
    }
    checks.push({
      name: "No entries without a committed transaction",
      ok: orphanCount === 0,
      detail: orphanCount === 0 ? "no orphaned entries" : `${orphanCount} orphaned`,
    });

    // 4) no lingering pending transactions
    const pending = transactions.filter((t) => t.status === "pending");
    if (pending.length) pending.forEach((t) => issues.push({ tx_id: t.id, failure: "pending", detail: "transaction left pending" }));
    checks.push({
      name: "No lingering pending transactions",
      ok: pending.length === 0,
      detail: pending.length === 0 ? "none pending" : `${pending.length} pending`,
    });

    const ok = checks.every((c) => c.ok);
    const duration = Math.max(1, Math.round((performance.now() - t0) * 100) / 100);
    return resp(200, {
      ok,
      ran_at: new Date().toISOString(),
      duration_ms: duration,
      checks,
      issues,
    });
  }

  // ---- overview / dashboard ------------------------------------------
  function overview() {
    const committed = transactions.filter((t) => t.status === "committed").length;
    const pending = requestLog.filter((r) => r.status === "pending").length;
    const failed = requestLog.filter((r) => r.status === "failed").length;
    const total = requestLog.length;
    const recent = [...requestLog]
      .sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
      .slice(0, 8);
    return resp(200, { total, committed, pending, failed, recent });
  }

  // ---- subscriptions / counters --------------------------------------
  function subscribe(fn) {
    subscribers.add(fn);
    return () => subscribers.delete(fn);
  }
  function transactionCount() {
    return transactions.filter((t) => t.status === "committed").length;
  }

  // boot
  reset();

  return {
    reset,
    listAccounts,
    createAccount,
    accountByLabel,
    accountById,
    deriveBalances,
    accountEntries,
    netByCurrency,
    createTransaction,
    getTransaction,
    reverseTransaction,
    reconcile,
    overview,
    subscribe,
    transactionCount,
    uuid,
  };
}

export const ledger = createLedger();
export default ledger;

