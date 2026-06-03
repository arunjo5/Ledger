/* =====================================================================
   Screen 1 — Create Transaction
   ===================================================================== */
import React, { useState, useMemo } from "react";
import { Banner, Card, Field, Button, AccountSelect, CopyId, StatusBadge, EntriesTable, ResponseViewer } from "../components/ui.jsx";
import { ledger } from "../api/client.js";

function ScreenCreate({ accounts, refreshAccounts }) {
  const blankRow = () => ({ key: ledger.uuid(), account_id: "", currency: "USD", amount: "" });

  const prefill = () => {
    const a = ledger.accountByLabel("liabilities:customer_a");
    const b = ledger.accountByLabel("liabilities:customer_b");
    return [
      { key: ledger.uuid(), account_id: a ? a.id : "", currency: "USD", amount: "-2500" },
      { key: ledger.uuid(), account_id: b ? b.id : "", currency: "USD", amount: "2500" },
    ];
  };

  const [description, setDescription] = useState("Move funds between customers");
  const [idemKey, setIdemKey] = useState(() => ledger.uuid());
  const [rows, setRows] = useState(prefill);
  const [response, setResponse] = useState(null);
  const [loading, setLoading] = useState(false);

  const labelOf = (id) => {
    const a = accounts.find((x) => x.id === id);
    return a ? a.label : "—";
  };

  const setRow = (key, patch) =>
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  const removeRow = (key) => setRows((rs) => rs.filter((r) => r.key !== key));

  // live balance check
  const net = useMemo(() => {
    const m = {};
    rows.forEach((r) => {
      const amt = parseInt(r.amount, 10);
      if (!r.currency) return;
      if (Number.isNaN(amt)) return;
      m[r.currency] = (m[r.currency] || 0) + amt;
    });
    return m;
  }, [rows]);
  const offenders = Object.entries(net).filter(([, v]) => v !== 0);
  const balanced = offenders.length === 0 && rows.some((r) => r.amount !== "");

  const submit = async () => {
    setLoading(true);
    setResponse(null);
    const entries = rows
      .filter((r) => r.account_id && r.amount !== "")
      .map((r) => ({
        account_id: r.account_id,
        currency: (r.currency || "").toUpperCase(),
        amount: parseInt(r.amount, 10),
      }));
    const res = await ledger.createTransaction({ description, idempotency_key: idemKey, entries });
    setResponse(res);
    setLoading(false);
  };

  const banner = () => {
    if (!response) return null;
    const { status, ok, body, meta } = response;
    if (ok && meta.cached) {
      return (
        <Banner tone="info" title="Cached idempotent response">
          Same key + same body — returned the original response.{" "}
          <span className="mono">no new transaction created.</span>
        </Banner>
      );
    }
    if (ok) {
      return (
        <Banner tone="success" title="Transaction committed">
          Entries net to zero in every currency.
        </Banner>
      );
    }
    if (status === 409) {
      return (
        <Banner tone="warn" title="409 Conflict — idempotency key reused">
          {body.error}
        </Banner>
      );
    }
    return (
      <Banner tone="error" title={`${status} ${body.code === "overdraft" ? "Overdraft protection" : "Validation error"}`}>
        {body.error}
        {body.net && (
          <span className="mono">
            {" "}
            ({Object.entries(body.net).map(([c, v]) => `${c} ${v > 0 ? "+" : ""}${v}`).join(", ")})
          </span>
        )}
      </Banner>
    );
  };

  const committed = response && response.ok && response.status === 201;

  return (
    <div className="stack">
      <div className="screen-head">
        <h2 className="screen-title">Create Transaction</h2>
        <p className="screen-desc">
          Compose a balanced double-entry transaction. Every currency must net to zero, protected
          accounts can't go negative, and the idempotency key makes the write safe to retry.
        </p>
      </div>

      <Card title="Request">
        <div className="stack">
          <div className="row row-wrap" style={{ alignItems: "flex-end" }}>
            <div className="grow">
              <Field label="Description">
                <input
                  className="input"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="What is this transaction for?"
                />
              </Field>
            </div>
            <div style={{ width: 320 }}>
              <Field label="Idempotency key" hint="safe-retry token">
                <div className="row" style={{ gap: 8 }}>
                  <input
                    className="input mono"
                    value={idemKey}
                    onChange={(e) => setIdemKey(e.target.value)}
                    style={{ fontSize: 12 }}
                  />
                  <Button variant="secondary" className="btn btn-secondary" onClick={() => setIdemKey(ledger.uuid())}>
                    Generate
                  </Button>
                </div>
              </Field>
            </div>
          </div>

          <div>
            <div className="field-label" style={{ marginBottom: 6 }}>
              Entries
              <span className="field-hint">signed integers · minor units</span>
            </div>
            <table className="editor-table">
              <thead>
                <tr>
                  <th style={{ width: "46%" }}>Account</th>
                  <th style={{ width: 110 }}>Currency</th>
                  <th style={{ width: 150 }} className="num">Amount</th>
                  <th style={{ width: 36 }}></th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.key}>
                    <td>
                      <AccountSelect
                        accounts={accounts}
                        value={r.account_id}
                        onChange={(id) => setRow(r.key, { account_id: id })}
                        onCreate={async (label) => {
                          const res = await ledger.createAccount(label);
                          if (res.ok) {
                            await refreshAccounts();
                            setRow(r.key, { account_id: res.body.id });
                          }
                        }}
                      />
                    </td>
                    <td>
                      <input
                        className="input mono cur-input"
                        value={r.currency}
                        maxLength={3}
                        onChange={(e) => setRow(r.key, { currency: e.target.value.toUpperCase() })}
                      />
                    </td>
                    <td>
                      <input
                        className="input mono amt-input"
                        value={r.amount}
                        inputMode="numeric"
                        placeholder="0"
                        onChange={(e) => setRow(r.key, { amount: e.target.value.replace(/[^\d-]/g, "") })}
                      />
                    </td>
                    <td>
                      <button className="icon-btn" title="remove entry" onClick={() => removeRow(r.key)}>
                        ×
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="row" style={{ marginTop: 10, gap: 8 }}>
              <Button variant="secondary" className="btn-sm" onClick={() => setRows((rs) => [...rs, blankRow()])}>
                + Add entry
              </Button>
              <Button variant="ghost" className="btn-sm" onClick={() => setRows([blankRow()])}>
                Clear
              </Button>
            </div>
          </div>

          {/* live checker */}
          <div className={`checker ${balanced ? "checker-ok" : "checker-bad"}`}>
            <span className="checker-dot" style={{ background: balanced ? "var(--green)" : "var(--red)" }} />
            {balanced ? (
              <strong>Balanced — every currency nets to 0</strong>
            ) : (
              <strong>Not balanced{offenders.length ? " per currency" : " yet"}</strong>
            )}
            {offenders.length > 0 && (
              <span className="checker-detail">
                {offenders.map(([c, v]) => `${c} ${v > 0 ? "+" : ""}${v}`).join("  ·  ")}
              </span>
            )}
          </div>

          <div className="row">
            <Button onClick={submit} loading={loading}>
              Submit transaction
            </Button>
            <span className="note">Submit is always enabled — server-side rejection is demoable too.</span>
          </div>
        </div>
      </Card>

      {banner()}

      {committed && (
        <Card title="Result">
          <div className="stack">
            <div className="row row-wrap" style={{ gap: 16 }}>
              <div className="row" style={{ gap: 8 }}>
                <span className="muted">tx id</span>
                <CopyId value={response.body.id} />
              </div>
              <StatusBadge status={response.body.status} />
            </div>
            <EntriesTable entries={response.body.entries} accountLabel={labelOf} />
          </div>
        </Card>
      )}

      {response && <ResponseViewer response={response} defaultOpen={!response.ok} />}
    </div>
  );
}

export default ScreenCreate;
