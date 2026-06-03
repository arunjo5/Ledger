/* =====================================================================
   Screen 5 — Idempotency Demo (showpiece)
   ===================================================================== */
import React, { useState } from "react";
import { Card, Button, Field, Pill, Banner, ResponseViewer } from "../components/ui.jsx";
import { ledger } from "../api/client.js";

function ScreenIdempotency({ accounts }) {
  const a = ledger.accountByLabel("liabilities:customer_a");
  const b = ledger.accountByLabel("liabilities:customer_b");

  const [description, setDescription] = useState("Idempotent transfer demo");
  const [idemKey, setIdemKey] = useState(() => ledger.uuid());
  const [amount, setAmount] = useState(1500);

  const [original, setOriginal] = useState(null);   // { response, body }
  const [latest, setLatest] = useState(null);        // { kind, response }
  const [created, setCreated] = useState(0);
  const [flashCounter, setFlashCounter] = useState(false);
  const [loading, setLoading] = useState("");

  const buildBody = (amt) => ({
    description,
    idempotency_key: idemKey,
    entries: [
      { account_id: a.id, currency: "USD", amount: -amt },
      { account_id: b.id, currency: "USD", amount: amt },
    ],
  });

  const bumpCounter = () => {
    setCreated((c) => c + 1);
    setFlashCounter(true);
    setTimeout(() => setFlashCounter(false), 600);
  };

  const sendOriginal = async () => {
    setLoading("original");
    const body = buildBody(amount);
    const res = await ledger.createTransaction(body);
    setOriginal({ response: res, body });
    setLatest(null);
    if (res.ok && !res.meta.cached) bumpCounter();
    setLoading("");
  };

  const replaySame = async () => {
    setLoading("same");
    const res = await ledger.createTransaction(buildBody(amount));
    setLatest({ kind: "same", response: res });
    if (res.ok && !res.meta.cached) bumpCounter();
    setLoading("");
  };

  const replayModified = async () => {
    setLoading("modified");
    const res = await ledger.createTransaction(buildBody(amount + 500));
    setLatest({ kind: "modified", response: res });
    if (res.ok && !res.meta.cached) bumpCounter();
    setLoading("");
  };

  const resetDemo = () => {
    setIdemKey(ledger.uuid());
    setOriginal(null);
    setLatest(null);
    setCreated(0);
  };

  const origTx = original && original.response.ok ? original.response.body : null;

  // build the "latest" panel summary
  const latestSummary = () => {
    if (!latest) return { label: "—", lines: [["awaiting replay", ""]] };
    const { kind, response } = latest;
    if (kind === "same") {
      return {
        tone: "info",
        label: "CACHED",
        sameId: response.body.id === (origTx && origTx.id),
        lines: [
          ["status", `${response.status} ${response.meta.cached ? "(cached)" : ""}`],
          ["tx id", response.body.id],
        ],
      };
    }
    return {
      tone: "error",
      label: "409",
      lines: [
        ["status", `${response.status} Conflict`],
        ["error", response.body.error],
      ],
    };
  };
  const ls = latestSummary();

  return (
    <div className="stack">
      <div className="screen-head">
        <h2 className="screen-title">Idempotency Demo</h2>
        <p className="screen-desc">
          The same idempotency key makes a write safe to retry. Replay the exact request and you
          get the cached response back. Replay with a changed body under the
          same key and the server rejects it with a 409.
        </p>
      </div>

      <Card title="Sample request" actions={<Button variant="ghost" className="btn-sm" onClick={resetDemo}>Reset demo</Button>}>
        <div className="stack">
          <div className="row row-wrap" style={{ alignItems: "flex-end", gap: 14 }}>
            <div className="grow" style={{ minWidth: 220 }}>
              <Field label="Description">
                <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
              </Field>
            </div>
            <div style={{ width: 150 }}>
              <Field label="Amount" hint="minor units">
                <input
                  className="input mono amt-input"
                  value={amount}
                  inputMode="numeric"
                  onChange={(e) => setAmount(parseInt(e.target.value.replace(/[^\d]/g, ""), 10) || 0)}
                />
              </Field>
            </div>
          </div>
          <Field label="Idempotency key">
            <input className="input mono" style={{ fontSize: 12 }} value={idemKey} onChange={(e) => setIdemKey(e.target.value)} />
          </Field>
          <div className="note">
            entries: <span className="mono">{a ? a.label : "customer_a"} −{amount} USD</span>, <span className="mono">{b ? b.label : "customer_b"} +{amount} USD</span> (balanced)
          </div>
          <div className="row" style={{ gap: 10 }}>
            <Button onClick={sendOriginal} loading={loading === "original"}>
              Send original request
            </Button>
            {origTx && <Pill tone="success">committed · 201</Pill>}
          </div>
        </div>
      </Card>

      {original && original.response.ok && (
        <React.Fragment>
          <div className="row row-wrap" style={{ gap: 14, alignItems: "center" }}>
            <Button variant="secondary" disabled={!origTx} loading={loading === "same"} onClick={replaySame}>
              ↻ Replay same request <span className="muted" style={{ fontWeight: 400 }}>(same key, same body)</span>
            </Button>
            <Button variant="secondary" disabled={!origTx} loading={loading === "modified"} onClick={replayModified}>
              ✎ Replay modified body <span className="muted" style={{ fontWeight: 400 }}>(same key)</span>
            </Button>
            <div className={`counter ${flashCounter ? "flash" : ""}`} style={{ marginLeft: "auto" }}>
              <span className="counter-num">{created}</span>
              <span className="counter-label">{created === 1 ? "transaction" : "transactions"} created</span>
            </div>
          </div>

          {latest && latest.kind === "same" && (
            <Banner tone="info" title="Returned the cached response">
              No new transaction created — the returned tx id is the <strong>same</strong> as the original.
            </Banner>
          )}
          {latest && latest.kind === "modified" && (
            <Banner tone="error" title="409 Conflict">
              Same idempotency key, different body.
            </Banner>
          )}

          {/* side-by-side */}
          <div className="split">
            <div className="split-col">
              <div className="split-head">Original <Pill tone="success">201</Pill></div>
              <div className="split-body">
                <KvRow k="status" v="201 Created" />
                <KvRow k="tx id" v={origTx.id} highlight={ls.sameId} />
                <KvRow k="description" v={origTx.description} sans />
                <KvRow k="idempotency_key" v={idemKey} />
              </div>
            </div>
            <div className="split-col">
              <div className="split-head">
                Latest replay {latest && <Pill tone={ls.tone}>{ls.label}</Pill>}
              </div>
              <div className="split-body">
                {latest ? (
                  ls.lines.map(([k, v], i) => (
                    <KvRow key={i} k={k} v={v} highlight={k === "tx id" && ls.sameId} sans={k === "error"} />
                  ))
                ) : (
                  <div className="empty-state" style={{ padding: 14 }}>Run a replay to compare.</div>
                )}
              </div>
            </div>
          </div>

          <ResponseViewer response={latest ? latest.response : original.response} defaultOpen={true} />
        </React.Fragment>
      )}

      {original && !original.response.ok && (
        <React.Fragment>
          <Banner tone="error" title={`${original.response.status} — request rejected`}>
            {original.response.body.error}
          </Banner>
          <ResponseViewer response={original.response} defaultOpen={true} />
        </React.Fragment>
      )}
    </div>
  );
}

function KvRow({ k, v, highlight, sans }) {
  return (
    <div className="kv">
      <span className="kv-key">{k}</span>
      <span className={"kv-val" + (sans ? "" : " mono")} style={sans ? { fontFamily: "var(--sans)" } : {}}>
        {highlight ? <span className="same-id">{v}</span> : v}
      </span>
    </div>
  );
}

export default ScreenIdempotency;
