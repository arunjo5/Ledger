/* =====================================================================
   App shell — sidebar console + breadcrumb topbar + screen routing
   ===================================================================== */
import React, { useState as useStateApp, useEffect as useEffectApp } from "react";
import Icon from "./components/Icon.jsx";
import { Button } from "./components/ui.jsx";
import { ledger } from "./api/client.js";
import ScreenOverview from "./screens/ScreenOverview.jsx";
import ScreenCreate from "./screens/ScreenCreate.jsx";
import ScreenTransactions from "./screens/ScreenTransactions.jsx";
import ScreenBalance from "./screens/ScreenBalance.jsx";
import ScreenReconciliation from "./screens/ScreenReconciliation.jsx";
import ScreenIdempotency from "./screens/ScreenIdempotency.jsx";

const NAV = [
  { id: "overview", label: "Overview", icon: "grid" },
  { id: "create", label: "Create transaction", icon: "plusSquare" },
  { id: "transactions", label: "Transactions", icon: "list" },
  { id: "balance", label: "Account balance", icon: "wallet" },
  { id: "reconciliation", label: "Reconciliation", icon: "shield" },
  { id: "idempotency", label: "Idempotency", icon: "swap" },
];

function App() {
  const [tab, setTab] = useStateApp("overview");
  const [accounts, setAccounts] = useStateApp([]);
  const [ready, setReady] = useStateApp(false);
  const [version, setVersion] = useStateApp(0);
  const [pendingTxId, setPendingTxId] = useStateApp(null);
  const [connected, setConnected] = useStateApp(true);

  const refreshAccounts = async () => {
    setAccounts(await ledger.listAccounts());
    setVersion((v) => v + 1);
  };

  useEffectApp(() => {
    (async () => {
      await refreshAccounts();
      setReady(true);
    })();
    // eslint-disable-next-line
  }, []);

  useEffectApp(() => {
    let alive = true;
    const check = async () => {
      const ok = await ledger.health();
      if (alive) setConnected(ok);
    };
    check();
    const id = setInterval(check, 20000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  const openTx = (id) => {
    setPendingTxId(id);
    setTab("transactions");
  };
  const goTo = (id) => setTab(id);

  const resetDemo = async () => {
    await ledger.reset();
    await refreshAccounts();
    setPendingTxId(null);
  };

  if (!ready) {
    return (
      <div className="app">
        <main className="content" style={{ padding: 40 }}>
          <div className="empty-state">Connecting to ledger…</div>
        </main>
      </div>
    );
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-glyph" aria-label="Ledger">
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect width="40" height="40" rx="11" fill="#18181B" />
              <rect x="12" y="13" width="16" height="3.1" rx="1.55" fill="#fff" />
              <rect x="12" y="18.45" width="16" height="3.1" rx="1.55" fill="#fff" fillOpacity="0.78" />
              <rect x="12" y="23.9" width="16" height="3.1" rx="1.55" fill="#fff" fillOpacity="0.56" />
            </svg>
          </span>
          <div>
            <div className="brand-name">LedgerCore</div>
          </div>
        </div>
        <nav className="nav">
          <div className="nav-section">Console</div>
          {NAV.map((n) => (
            <button
              key={n.id}
              className={`nav-item ${tab === n.id ? "active" : ""}`}
              onClick={() => setTab(n.id)}
            >
              <Icon name={n.icon} size={18} className="ic" />
              {n.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <Button variant="secondary" onClick={resetDemo} style={{ width: "100%", justifyContent: "center" }}>
            <Icon name="refresh" size={14} /> Reset demo data
          </Button>
        </div>
      </aside>

      <div className="main">
        <div className="topbar">
          <div className="crumb">
            <span className="sep">Console</span> <span className="sep">/</span> <b>{(NAV.find((n) => n.id === tab) || {}).label}</b>
          </div>
          <div className={`health-pill ${connected ? "ok" : "down"}`}>
            {connected ? "connected" : "offline"}
          </div>
        </div>

        <main className="content">
          {tab === "overview" && <ScreenOverview version={version} onOpenTx={openTx} goTo={goTo} />}
          {tab === "create" && <ScreenCreate accounts={accounts} refreshAccounts={refreshAccounts} />}
          {tab === "transactions" && (
            <ScreenTransactions
              accounts={accounts}
              initialId={pendingTxId}
              clearInitialId={() => setPendingTxId(null)}
            />
          )}
          {tab === "balance" && <ScreenBalance accounts={accounts} onOpenTx={openTx} version={version} />}
          {tab === "reconciliation" && <ScreenReconciliation onReset={resetDemo} />}
          {tab === "idempotency" && <ScreenIdempotency accounts={accounts} />}
        </main>
      </div>
    </div>
  );
}

export default App;
