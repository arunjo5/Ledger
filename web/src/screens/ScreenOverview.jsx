/* =====================================================================
   Overview — read-only dashboard (stat cards + recent activity)
   ===================================================================== */

import React, { useState as useStateOv, useEffect as useEffectOv } from "react";
import { StatusBadge } from "../components/ui.jsx";
import Icon from "../components/Icon.jsx";
import { ledger } from "../api/client.js";

function StatCard({ label, value, foot, icon, iconClass }) {
  return (
    <div className="stat-card">
      <div className="stat-head">
        <span className="stat-label">{label}</span>
        <Icon name={icon} size={20} className={iconClass} />
      </div>
      <div className="stat-num">{value}</div>
      <div className="stat-foot">{foot}</div>
    </div>
  );
}

function ScreenOverview({ version, onOpenTx, goTo }) {
  const [data, setData] = useStateOv(null);
  useEffectOv(() => {
    let alive = true;
    ledger.overview().then((r) => alive && setData(r.body));
    return () => {
      alive = false;
    };
  }, [version]);

  if (!data) {
    return (
      <div className="stack" style={{ gap: 18 }}>
        <div className="empty-state">Loading dashboard…</div>
      </div>
    );
  }

  const fmtDate = (iso) => {
    const d = new Date(iso);
    const pad = (n) => String(n).padStart(2, "0");
    return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
  };

  return (
    <div className="stack" style={{ gap: 18 }}>
      <div className="page-head">
        <div>
          <h1 className="page-title">Overview</h1>
          <p className="page-sub">
            Read-only snapshot of the ledger. Balances are derived from entries; nothing is
            cached server-side.
          </p>
        </div>
      </div>

      <div className="stat-grid">
        <StatCard label="Total transactions" value={data.total} foot="all time" icon="list" iconClass="stat-ic-gray" />
        <StatCard label="Committed" value={data.committed} foot="durable, balanced" icon="doubleCheck" iconClass="stat-ic-green" />
        <StatCard label="Pending" value={data.pending} foot="awaiting confirmation" icon="clock" iconClass="stat-ic-amber" />
        <StatCard label="Failed" value={data.failed} foot="rejected before commit" icon="xCircle" iconClass="stat-ic-red" />
      </div>

      <div className="panel">
        <div className="panel-head">
          <div>
            <div className="panel-title">Recent activity</div>
            <div className="panel-sub">Latest writes across all accounts</div>
          </div>
          <button className="panel-link" onClick={() => goTo("transactions")}>
            View all <Icon name="arrowRight" size={16} />
          </button>
        </div>
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
            {data.recent.map((r) => (
              <tr
                key={r.id}
                className={r.tx_id ? "clickable" : ""}
                onClick={() => r.tx_id && onOpenTx(r.tx_id)}
              >
                <td>
                  <span className="txid">
                    {r.tx_id ? "txn_" + r.id.slice(0, 8) + "…" + r.id.slice(-4) : "—"}
                  </span>
                </td>
                <td>{r.description}</td>
                <td><StatusBadge status={r.status} /></td>
                <td className="num">{r.lines}</td>
                <td className="num mono" style={{ color: "var(--text-2)", fontSize: 13 }}>{fmtDate(r.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default ScreenOverview;
