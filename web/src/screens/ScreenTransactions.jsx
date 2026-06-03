/* =====================================================================
   Screen 2 — Transactions (browsable list, filter, detail, reverse)
   ===================================================================== */
import React, { useState, useEffect } from "react";
import { Card, Button, Banner, CopyId, StatusBadge, Pill, EntriesTable, ResponseViewer } from "../components/ui.jsx";
import { ledger } from "../api/client.js";

function shortId(id) {
  return "txn_" + id.slice(0, 8) + "…" + id.slice(-4);
}
function fmtDate(iso) {
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
}

function ScreenTransactions({ accounts, initialId, clearInitialId }) {
  const [list, setList] = useState([]);
  const [filter, setFilter] = useState("");
  const [tx, setTx] = useState(null);
  const [response, setResponse] = useState(null);
  const [reversal, setReversal] = useState(null);
  const [revResponse, setRevResponse] = useState(null);
  const [reversing, setReversing] = useState(false);

  const labelOf = (id) => {
    const a = accounts.find((x) => x.id === id);
    return a ? a.label : "—";
  };

  const refreshList = async () => {
    setList(await ledger.listTransactions());
  };

  useEffect(() => {
    refreshList();
  }, []);

  const open = async (id) => {
    setReversal(null);
    setRevResponse(null);
    const res = await ledger.getTransaction(id);
    setResponse(res);
    setTx(res.ok ? res.body : null);
  };

  // open a transaction linked from another screen
  useEffect(() => {
    if (initialId) {
      open(initialId);
      clearInitialId && clearInitialId();
    }
    // eslint-disable-next-line
  }, [initialId]);

  const reverse = async () => {
    if (!tx) return;
    setReversing(true);
    const res = await ledger.reverseTransaction(tx.id);
    setRevResponse(res);
    if (res.ok) {
      setReversal(res.body);
      await refreshList();
      const fresh = await ledger.getTransaction(tx.id);
      if (fresh.ok) setTx(fresh.body);
    }
    setReversing(false);
  };

  const q = filter.trim().toLowerCase();
  const filtered = q
    ? list.filter((t) => t.id.toLowerCase().includes(q) || (t.description || "").toLowerCase().includes(q))
    : list;

  return (
    <div className="stack">
      <div className="screen-head">
        <h2 className="screen-title">Transactions</h2>
        <p className="screen-desc">
          Browse every transaction, newest first. Filter by id or description, then open one to
          see its entries or post a reversing transaction.
        </p>
      </div>

      <Card title="All transactions">
        <div className="stack">
          <input
            className="input mono"
            placeholder="filter by id or description…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          {filtered.length === 0 ? (
            <div className="empty-state">{list.length === 0 ? "No transactions yet." : "No matches."}</div>
          ) : (
            <div className="tbl-scroll">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Transaction</th>
                    <th>Description</th>
                    <th>Status</th>
                    <th className="num">Lines</th>
                    <th className="num">Created</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((t) => (
                    <tr
                      key={t.id}
                      className={`clickable ${tx && tx.id === t.id ? "row-active" : ""}`}
                      onClick={() => open(t.id)}
                    >
                      <td><span className="txid">{shortId(t.id)}</span></td>
                      <td>{t.description}</td>
                      <td><StatusBadge status={t.status} /></td>
                      <td className="num">{t.lines}</td>
                      <td className="num mono" style={{ color: "var(--text-2)", fontSize: 13 }}>{fmtDate(t.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Card>

      {response && !response.ok && (
        <Banner tone="warn" title="No transaction with that id">
          Check the id and try again.
        </Banner>
      )}

      {tx && (
        <Card
          title="Transaction"
          actions={
            <Button variant="secondary" className="btn-sm" loading={reversing} onClick={reverse}>
              ↺ Reverse this transaction
            </Button>
          }
        >
          <div className="stack">
            <div className="row row-wrap" style={{ gap: 18 }}>
              <div className="row" style={{ gap: 8 }}>
                <span className="muted">id</span>
                <CopyId value={tx.id} />
              </div>
              <StatusBadge status={tx.status} />
              {tx.reverses_transaction_id && <Pill tone="info">reversal</Pill>}
            </div>

            <div className="kv-grid">
              <KV k="description" v={tx.description} />
              <KV k="created_at" v={new Date(tx.created_at).toLocaleString()} mono />
              <KV k="committed_at" v={tx.committed_at ? new Date(tx.committed_at).toLocaleString() : "—"} mono />
              {tx.reverses_transaction_id && <KVLink k="reverses" id={tx.reverses_transaction_id} onNav={() => open(tx.reverses_transaction_id)} />}
            </div>

            <EntriesTable entries={tx.entries} accountLabel={labelOf} />
          </div>
        </Card>
      )}

      {revResponse && !revResponse.ok && (
        <Banner tone="error" title={`${revResponse.status} — reversal rejected`}>
          {revResponse.body.error}
        </Banner>
      )}

      {reversal && (
        <Card title="Reversing transaction created">
          <div className="stack">
            <Banner tone="success" title="Reversal committed">
              The mirror-image transaction undoes the original without mutating it.
            </Banner>
            <div className="row" style={{ gap: 8 }}>
              <span className="muted">new tx id</span>
              <CopyId value={reversal.id} />
              <span className="link" onClick={() => open(reversal.id)}>
                open →
              </span>
            </div>
            <EntriesTable entries={reversal.entries} accountLabel={labelOf} />
          </div>
        </Card>
      )}

      {response && <ResponseViewer response={revResponse || response} defaultOpen={false} />}
    </div>
  );
}

function KV({ k, v, mono }) {
  return (
    <div className="kv">
      <span className="kv-key">{k}</span>
      <span className={"kv-val" + (mono ? " mono" : "")} style={mono ? {} : { fontFamily: "var(--sans)" }}>
        {v}
      </span>
    </div>
  );
}
function KVLink({ k, id, onNav }) {
  return (
    <div className="kv">
      <span className="kv-key">{k}</span>
      <span className="link" onClick={onNav}>
        {id.slice(0, 8)}… →
      </span>
    </div>
  );
}

export default ScreenTransactions;
