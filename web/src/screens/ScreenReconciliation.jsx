/* =====================================================================
   Screen 4 — Reconciliation
   ===================================================================== */
import React, { useState } from "react";
import { Card, Button, Banner, Pill, ResponseViewer } from "../components/ui.jsx";
import { ledger } from "../api/client.js";

function ScreenReconciliation({ onReset }) {
  const [report, setReport] = useState(null);
  const [response, setResponse] = useState(null);
  const [loading, setLoading] = useState(false);

  const run = async () => {
    setLoading(true);
    const res = await ledger.reconcile();
    setResponse(res);
    setReport(res.body);
    setLoading(false);
  };

  const reset = () => {
    onReset && onReset();
    setReport(null);
    setResponse(null);
  };

  return (
    <div className="stack">
      <div className="screen-head">
        <h2 className="screen-title">Reconciliation</h2>
        <p className="screen-desc">
          Re-derive every invariant from the ledger and confirm it is internally consistent.
          Each committed transaction is checked for balance, non-emptiness, and clean status.
        </p>
      </div>

      <Card title="Run">
        <div className="toolbar">
          <Button onClick={run} loading={loading}>
            Run reconciliation
          </Button>
          <Button variant="ghost" onClick={reset}>
            Reset demo data
          </Button>
        </div>
      </Card>

      {report && (
        <React.Fragment>
          <div className={`big-banner ${report.ok ? "big-banner-pass" : "big-banner-fail"}`}>
            <div className="bb-icon">{report.ok ? "✓" : "!"}</div>
            <div>
              <div className="bb-title">
                {report.ok ? "Ledger consistent" : `${report.issues.length} issue${report.issues.length === 1 ? "" : "s"} found`}
              </div>
              <div className="bb-sub mono">
                ran at {new Date(report.ran_at).toLocaleTimeString()} · {report.duration_ms} ms ·{" "}
                {report.checks.length} invariants checked
              </div>
            </div>
          </div>

          <Card title="Invariants" className="flush">
            {report.checks.map((c, i) => (
              <div className="check-item" key={i}>
                <div className={`check-mark ${c.ok ? "pass" : "fail"}`}>{c.ok ? "✓" : "×"}</div>
                <div>
                  <div className="check-name">{c.name}</div>
                  <div className="check-detail">{c.detail}</div>
                </div>
              </div>
            ))}
          </Card>

          {report.issues.length > 0 && (
            <Card title="Offending transactions" className="flush">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>transaction id</th>
                    <th>failure</th>
                    <th>detail</th>
                  </tr>
                </thead>
                <tbody>
                  {report.issues.map((iss, i) => (
                    <tr key={i}>
                      <td className="mono" style={{ fontSize: 12 }}>{iss.tx_id}</td>
                      <td>
                        <Pill tone="error">{iss.failure}</Pill>
                      </td>
                      <td className="mono" style={{ fontSize: 12 }}>{iss.detail}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          )}

          <ResponseViewer response={response} />
        </React.Fragment>
      )}
    </div>
  );
}

export default ScreenReconciliation;
