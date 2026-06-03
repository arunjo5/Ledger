/* =====================================================================
   Account Balance — # prefix, quick chips, derived balance, time-travel
   ===================================================================== */
import React, { useState as useStateBal, useEffect as useEffectBal } from "react";
import { Button, Pill, Amount, formatAmount, AccountSelect, DateTimePicker } from "../components/ui.jsx";
import Icon from "../components/Icon.jsx";
import { ledger } from "../api/client.js";

function toLocalInput(date) {
  const d = new Date(date);
  const pad = (n) => String(n).padStart(2, "0");
  return (
    d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) +
    "T" + pad(d.getHours()) + ":" + pad(d.getMinutes())
  );
}

function ScreenBalance({ accounts, onOpenTx, version }) {
  const defaultAcct = () => {
    const a = ledger.accountByLabel("cash:operating");
    return a ? a.id : accounts[0] && accounts[0].id;
  };
  const [acctId, setAcctId] = useStateBal(defaultAcct);
  const [asOf, setAsOf] = useStateBal("");
  const [now, setNow] = useStateBal(toLocalInput(Date.now()));
  const [fetching, setFetching] = useStateBal(false);

  const effectiveAsOf = asOf || null;
  const [balances, setBalances] = useStateBal([]);
  const [entries, setEntries] = useStateBal([]);
  const [tick, setTick] = useStateBal(0);
  useEffectBal(() => {
    if (!acctId) {
      setBalances([]);
      setEntries([]);
      return;
    }
    let alive = true;
    Promise.all([
      ledger.deriveBalances(acctId, effectiveAsOf),
      ledger.accountEntries(acctId, effectiveAsOf),
    ]).then(([b, e]) => {
      if (alive) {
        setBalances(b);
        setEntries(e);
      }
    });
    return () => {
      alive = false;
    };
  }, [acctId, effectiveAsOf, version, tick]);
  const acct = accounts.find((a) => a.id === acctId);

  const fmtDate = (iso) => {
    const d = new Date(iso);
    const pad = (n) => String(n).padStart(2, "0");
    return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
  };

  const doFetch = () => setTick((t) => t + 1);

  // quick chips: first handful of accounts
  const quick = accounts.slice(0, 5);

  return (
    <div className="stack" style={{ gap: 18 }}>
      <div className="page-head">
        <div>
          <h1 className="page-title">Account balance</h1>
          <p className="page-sub">Balances are computed live by summing committed entries for the account.</p>
        </div>
      </div>

      <div className="panel">
        <div className="card-body">
          <div className="row" style={{ gap: 12 }}>
            <div className="grow">
              <AccountSelect accounts={accounts} value={acctId} onChange={setAcctId} prefixIcon="hash" size="lg" />
            </div>
            <Button className="btn-lg" loading={fetching} onClick={doFetch}>Fetch</Button>
          </div>

          <div className="chips">
            <span className="chips-label">Quick:</span>
            {quick.map((a) => (
              <button
                key={a.id}
                className={`chip ${a.id === acctId ? "active" : ""}`}
                onClick={() => setAcctId(a.id)}
              >
                {a.label}
              </button>
            ))}
          </div>

          <div className="row row-wrap" style={{ gap: 12, marginTop: 16, alignItems: "center" }}>
            <span className="section-label">Balance as of</span>
            <DateTimePicker
              value={asOf || now}
              max={now}
              onChange={(v) => setAsOf(v)}
              onClear={() => { setNow(toLocalInput(Date.now())); setAsOf(""); }}
            />
            <Button variant="secondary" className="btn-sm" onClick={() => { setNow(toLocalInput(Date.now())); setAsOf(""); }}>
              <Icon name="refresh" size={14} /> now
            </Button>
            {asOf && <Pill tone="warn">historical</Pill>}
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head" style={{ paddingBottom: 0, borderBottom: "none" }}>
          <div>
            <div className="panel-title">Balance</div>
            <div className="panel-sub mono" style={{ marginTop: 4 }}>{acct ? acct.label : "—"}</div>
          </div>
        </div>
        <div style={{ borderTop: "1px solid var(--border)", marginTop: 16 }}>
          {balances.length === 0 ? (
            <div className="empty-state">No balance in any currency at this point in time.</div>
          ) : (
            balances.map((b) => (
              <div className="bal-row" key={b.currency}>
                <span className="bal-ccy">{b.currency}</span>
                <span className={"amt-big " + (b.amount < 0 ? "amt-neg" : "")}>{formatAmount(b.amount)}</span>
              </div>
            ))
          )}
        </div>
        {balances.length > 0 && (
          <div className="infobox">
            <Icon name="info" size={18} className="infobox-ic" />
            <div>
              <div className="infobox-title">Derived value</div>
              <div className="infobox-text">
                Computed from {entries.length} {entries.length === 1 ? "entry" : "entries"}. Not stored.
                {acct && acct.overdraft_protected ? " Overdraft-protected." : ""}
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="panel">
        <div className="panel-head">
          <div>
            <div className="panel-title">Recent entries</div>
            <div className="panel-sub">Most recent first</div>
          </div>
        </div>
        {entries.length === 0 ? (
          <div className="empty-state">No entries yet for this account.</div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Transaction</th>
                <th>CCY</th>
                <th className="num">Amount</th>
                <th className="num">When</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr key={e.id} className="clickable" onClick={() => onOpenTx(e.transaction_id)}>
                  <td><span className="txid">txn_{e.transaction_id.slice(0, 8)}…{e.transaction_id.slice(-4)}</span></td>
                  <td className="mono">{e.currency}</td>
                  <td className="num"><Amount value={e.amount} /></td>
                  <td className="num mono" style={{ color: "var(--text-2)", fontSize: 13 }}>{fmtDate(e.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

export default ScreenBalance;
