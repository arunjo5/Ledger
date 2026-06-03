/* =====================================================================
   httpLedger.example.js  —  TEMPLATE (not wired in by default)
   ---------------------------------------------------------------------
   A starting point for talking to your real backend instead of the mock.
   Every method returns a Promise resolving to the SAME envelope the mock
   uses: { status, ok, body, meta }. Keeping that envelope means the
   response/JSON viewer and all status-code UI keep working unchanged.

   To use:
     1. Implement the endpoints in API_CONTRACT.md on your server.
     2. In client.js, replace the mock import with:
            import { ledger } from "./httpLedger.example.js";
     3. Make the screen handlers async and `await` each ledger call
        (see the call-site list in client.js).
   ===================================================================== */

const BASE = import.meta.env.VITE_API_BASE || "/api/v1";

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
  // The mock surfaces a `cached` flag for idempotent replays. If your
  // server signals replays with a header, map it here.
  const cached = res.headers.get("Idempotency-Replayed") === "true";
  return { status: res.status, ok: res.ok, body: json, meta: { cached } };
}

export const ledger = {
  // POST /transactions   (idempotency key travels as a header)
  createTransaction(req) {
    const { idempotency_key, ...rest } = req;
    return call("POST", "/transactions", { idempotency_key, ...rest },
      idempotency_key ? { "Idempotency-Key": idempotency_key } : {});
  },

  // GET /transactions/:id
  getTransaction(id) {
    return call("GET", "/transactions/" + encodeURIComponent(id));
  },

  // POST /transactions/:id/reverse
  reverseTransaction(id) {
    return call("POST", "/transactions/" + encodeURIComponent(id) + "/reverse");
  },

  // GET /accounts
  listAccounts() {
    return call("GET", "/accounts").then((r) => (r.ok ? r.body.accounts : []));
  },
  // POST /accounts
  createAccount(label) {
    return call("POST", "/accounts", { label });
  },

  // GET /accounts/:id/balance?as_of=ISO   -> { balances: [{currency, amount}] }
  deriveBalances(accountId, asOf) {
    const q = asOf ? "?as_of=" + encodeURIComponent(asOf) : "";
    return call("GET", "/accounts/" + accountId + "/balance" + q)
      .then((r) => (r.ok ? r.body.balances : []));
  },
  // GET /accounts/:id/entries?as_of=ISO
  accountEntries(accountId, asOf) {
    const q = asOf ? "?as_of=" + encodeURIComponent(asOf) : "";
    return call("GET", "/accounts/" + accountId + "/entries" + q)
      .then((r) => (r.ok ? r.body.entries : []));
  },

  // GET /overview
  overview() {
    return call("GET", "/overview");
  },
  // POST /reconcile
  reconcile() {
    return call("POST", "/reconcile");
  },

  // --- demo-only affordance; omit in production ---
  reset() {
    return call("POST", "/demo/reset");
  },

  // Client-only helpers the UI expects. With a real backend, subscribe can
  // be a no-op (or backed by SSE/websocket) and uuid can use crypto.randomUUID.
  subscribe() {
    return () => {};
  },
  uuid() {
    return crypto.randomUUID();
  },
};

export default ledger;
