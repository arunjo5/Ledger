/* =====================================================================
   Shared UI primitives for LedgerCore.
   Exposed on window so each Babel script can use them.
   ===================================================================== */

import React, { useState, useRef, useEffect, useMemo } from "react";
import Icon from "./Icon.jsx";

/* ---------- amount formatting --------------------------------------- */
// Amounts are signed integers in minor units. We surface the integer as
// primary (monospace) and a faint major-unit hint.
function formatMinor(amount) {
  const sign = amount > 0 ? "+" : amount < 0 ? "-" : "";
  const abs = Math.abs(amount);
  return sign + abs.toString();
}
function formatAmount(amount) {
  const sign = amount > 0 ? "+" : amount < 0 ? "-" : "";
  const abs = Math.abs(amount) / 100;
  return sign + abs.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function amtClass(amount) {
  return "amt " + (amount > 0 ? "amt-pos" : amount < 0 ? "amt-neg" : "amt-zero");
}

function Amount({ value }) {
  return <span className={amtClass(value)}>{formatAmount(value)}</span>;
}

/* ---------- copyable mono id ---------------------------------------- */
function CopyId({ value, truncate }) {
  const [copied, setCopied] = useState(false);
  const display = truncate ? value.slice(0, 8) + "…" + value.slice(-4) : value;
  return (
    <button
      className="copy-id"
      title={value}
      onClick={() => {
        navigator.clipboard && navigator.clipboard.writeText(value);
        setCopied(true);
        setTimeout(() => setCopied(false), 1100);
      }}
    >
      <span className="mono">{display}</span>
      <span className="copy-icon">{copied ? "✓ copied" : "copy"}</span>
    </button>
  );
}

/* ---------- buttons -------------------------------------------------- */
function Button({ variant = "primary", loading, children, disabled, className, ...rest }) {
  return (
    <button className={`btn btn-${variant}${className ? " " + className : ""}`} disabled={disabled || loading} {...rest}>
      {loading && <span className="spinner" />}
      {children}
    </button>
  );
}

/* ---------- status badge -------------------------------------------- */
function StatusBadge({ status }) {
  const map = {
    committed: "spill-committed",
    pending: "spill-pending",
    failed: "spill-failed",
  };
  return <span className={`spill ${map[status] || "spill-pending"}`}>{status}</span>;
}

function Pill({ tone, children }) {
  return <span className={`tag tag-${tone}`}>{children}</span>;
}

/* ---------- result banner ------------------------------------------- */
function Banner({ tone, title, children }) {
  return (
    <div className={`banner banner-${tone}`}>
      <div className="banner-dot" />
      <div className="banner-body">
        <div className="banner-title">{title}</div>
        {children && <div className="banner-text">{children}</div>}
      </div>
    </div>
  );
}

/* ---------- form field ---------------------------------------------- */
function Field({ label, hint, children }) {
  return (
    <label className="field">
      <span className="field-label">
        {label}
        {hint && <span className="field-hint">{hint}</span>}
      </span>
      {children}
    </label>
  );
}

/* ---------- account dropdown (custom menu, by label, with +New) ----- */
function AccountSelect({ accounts, value, onChange, onCreate, prefixIcon, size }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setOpen(false);
    };
    const onKey = (e) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const selected = accounts.find((a) => a.id === value);

  return (
    <div className="dd" ref={ref}>
      <button
        type="button"
        className={`dd-trigger ${open ? "open" : ""} ${size === "lg" ? "dd-lg" : ""}`}
        onClick={() => setOpen((o) => !o)}
      >
        {prefixIcon && <Icon name={prefixIcon} size={size === "lg" ? 18 : 15} style={{ color: "var(--text-3)", flex: "none" }} />}
        {selected ? (
          <span className="dd-value mono">{selected.label}</span>
        ) : (
          <span className="dd-placeholder">select account…</span>
        )}
        {selected && selected.overdraft_protected && <span className="dd-tag">protected</span>}
        <span className="dd-chev">▾</span>
      </button>

      {open && (
        <div className="dd-menu" role="listbox">
          {accounts.map((a) => (
            <button
              type="button"
              key={a.id}
              role="option"
              aria-selected={a.id === value}
              className={`dd-item ${a.id === value ? "selected" : ""}`}
              onClick={() => {
                onChange(a.id);
                setOpen(false);
              }}
            >
              <span className="dd-check">{a.id === value ? "✓" : ""}</span>
              <span className="dd-label mono">{a.label}</span>
              {a.overdraft_protected && <span className="dd-tag">protected</span>}
            </button>
          ))}
          {onCreate && (
            <React.Fragment>
              <div className="dd-sep" />
              <button
                type="button"
                className="dd-item dd-new"
                onClick={() => {
                  setOpen(false);
                  const label = window.prompt("New account label (e.g. customer_c):");
                  if (label && onCreate) onCreate(label.trim());
                }}
              >
                <span className="dd-check">＋</span>
                <span className="dd-label">New account…</span>
              </button>
            </React.Fragment>
          )}
        </div>
      )}
    </div>
  );
}

/* ---------- custom date-time picker --------------------------------- */
function dtpPad(n) { return String(n).padStart(2, "0"); }
function dtpParse(s) {
  if (!s) return null;
  const [d, t] = s.split("T");
  const [y, mo, da] = d.split("-").map(Number);
  const [hh, mm] = (t || "00:00").split(":").map(Number);
  return { y, mo, da, hh, mm };
}
function dtpFmt(y, mo, da, hh, mm) {
  return `${y}-${dtpPad(mo)}-${dtpPad(da)}T${dtpPad(hh)}:${dtpPad(mm)}`;
}
function dtpNow() {
  const d = new Date();
  return dtpFmt(d.getFullYear(), d.getMonth() + 1, d.getDate(), d.getHours(), d.getMinutes());
}
const DTP_WEEK = ["S", "M", "T", "W", "T", "F", "S"];

function DateTimePicker({ value, max, onChange, onClear }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  const cur = dtpParse(value) || dtpParse(dtpNow());
  const [view, setView] = useState({ y: cur.y, m: cur.mo });

  useEffect(() => {
    if (!open) return;
    const c = dtpParse(value);
    if (c) setView({ y: c.y, m: c.mo });
    const onDoc = (e) => { if (ref.current && !ref.current.contains(e.target)) setOpen(false); };
    const onKey = (e) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => { document.removeEventListener("mousedown", onDoc); document.removeEventListener("keydown", onKey); };
  }, [open]);

  const sel = dtpParse(value) || cur;
  const maxDT = dtpParse(max);
  const today = dtpParse(dtpNow());

  const monthName = new Date(view.y, view.m - 1, 1).toLocaleString("en-US", { month: "long" });
  const startWeekday = new Date(view.y, view.m - 1, 1).getDay();
  const daysInMonth = new Date(view.y, view.m, 0).getDate();
  const prevDays = new Date(view.y, view.m - 1, 0).getDate();

  const canNext = !maxDT || view.y < maxDT.y || (view.y === maxDT.y && view.m < maxDT.mo);

  const shift = (delta) => {
    let m = view.m + delta, y = view.y;
    if (m < 1) { m = 12; y--; }
    if (m > 12) { m = 1; y++; }
    setView({ y, m });
  };

  const dayDisabled = (da) => {
    if (!maxDT) return false;
    const d = new Date(view.y, view.m - 1, da).setHours(0, 0, 0, 0);
    const md = new Date(maxDT.y, maxDT.mo - 1, maxDT.da).setHours(0, 0, 0, 0);
    return d > md;
  };

  const pickDay = (da) => {
    let v = dtpFmt(view.y, view.m, da, sel.hh, sel.mm);
    if (maxDT) {
      const vt = new Date(view.y, view.m - 1, da, sel.hh, sel.mm).getTime();
      const mt = new Date(maxDT.y, maxDT.mo - 1, maxDT.da, maxDT.hh, maxDT.mm).getTime();
      if (vt > mt) v = dtpFmt(maxDT.y, maxDT.mo, maxDT.da, maxDT.hh, maxDT.mm);
    }
    onChange(v);
  };

  const setTime = (hh, mm) => {
    let v = dtpFmt(sel.y, sel.mo, sel.da, hh, mm);
    if (maxDT) {
      const vt = new Date(sel.y, sel.mo - 1, sel.da, hh, mm).getTime();
      const mt = new Date(maxDT.y, maxDT.mo - 1, maxDT.da, maxDT.hh, maxDT.mm).getTime();
      if (vt > mt) v = dtpFmt(maxDT.y, maxDT.mo, maxDT.da, maxDT.hh, maxDT.mm);
    }
    onChange(v);
  };

  const cells = [];
  for (let i = 0; i < 42; i++) {
    const dayNum = i - startWeekday + 1;
    cells.push(dayNum);
  }

  const triggerLabel = sel
    ? `${new Date(sel.y, sel.mo - 1, sel.da).toLocaleString("en-US", { month: "short", day: "numeric", year: "numeric" })}  ${dtpPad(sel.hh)}:${dtpPad(sel.mm)}`
    : "—";

  return (
    <div className="dtp" ref={ref}>
      <button type="button" className={`dd-trigger ${open ? "open" : ""}`} onClick={() => setOpen((o) => !o)} style={{ minWidth: 220 }}>
        <Icon name="calendar" size={15} style={{ color: "var(--text-3)", flex: "none" }} />
        <span className="dd-value mono">{triggerLabel}</span>
        <span className="dd-chev">▾</span>
      </button>

      {open && (
        <div className="dtp-pop">
          <div className="dtp-head">
            <div className="dtp-month">{monthName} {view.y}</div>
            <div className="dtp-nav">
              <button type="button" className="dtp-navbtn" onClick={() => shift(-1)} aria-label="previous month">‹</button>
              <button type="button" className="dtp-navbtn" onClick={() => canNext && shift(1)} disabled={!canNext} aria-label="next month">›</button>
            </div>
          </div>

          <div className="dtp-grid dtp-week">
            {DTP_WEEK.map((d, i) => <span key={i} className="dtp-wd">{d}</span>)}
          </div>
          <div className="dtp-grid">
            {cells.map((dayNum, i) => {
              if (dayNum < 1) return <span key={i} className="dtp-day out">{prevDays + dayNum}</span>;
              if (dayNum > daysInMonth) return <span key={i} className="dtp-day out">{dayNum - daysInMonth}</span>;
              const isSel = sel && sel.y === view.y && sel.mo === view.m && sel.da === dayNum;
              const isToday = today.y === view.y && today.mo === view.m && today.da === dayNum;
              const disabled = dayDisabled(dayNum);
              return (
                <button
                  key={i}
                  type="button"
                  disabled={disabled}
                  className={`dtp-day${isSel ? " sel" : ""}${isToday && !isSel ? " today" : ""}`}
                  onClick={() => pickDay(dayNum)}
                >
                  {dayNum}
                </button>
              );
            })}
          </div>

          <div className="dtp-time">
            <span className="dtp-time-label">Time</span>
            <div className="dtp-time-fields">
              <input
                className="input mono dtp-tinput"
                value={dtpPad(sel.hh)}
                inputMode="numeric"
                onChange={(e) => { const h = Math.max(0, Math.min(23, parseInt(e.target.value.replace(/\D/g, ""), 10) || 0)); setTime(h, sel.mm); }}
              />
              <span className="dtp-colon">:</span>
              <input
                className="input mono dtp-tinput"
                value={dtpPad(sel.mm)}
                inputMode="numeric"
                onChange={(e) => { const m = Math.max(0, Math.min(59, parseInt(e.target.value.replace(/\D/g, ""), 10) || 0)); setTime(sel.hh, m); }}
              />
            </div>
          </div>

          <div className="dtp-foot">
            <button type="button" className="dtp-link" onClick={() => { if (onClear) onClear(); setOpen(false); }}>Clear</button>
            <button type="button" className="dtp-link" onClick={() => { const t = today; setView({ y: t.y, m: t.mo }); onChange(dtpFmt(t.y, t.mo, t.da, sel.hh, sel.mm)); }}>Today</button>
          </div>
        </div>
      )}
    </div>
  );
}

/* ---------- syntax-highlighted JSON --------------------------------- */
function highlightJson(value) {
  const json = JSON.stringify(value, null, 2);
  const esc = json
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  return esc.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?)/g,
    (match) => {
      let cls = "j-num";
      if (/^"/.test(match)) {
        cls = /:$/.test(match) ? "j-key" : "j-str";
      } else if (/true|false/.test(match)) {
        cls = "j-bool";
      } else if (/null/.test(match)) {
        cls = "j-null";
      }
      return `<span class="${cls}">${match}</span>`;
    }
  );
}

// Collapsible HTTP status + raw JSON viewer attached to every API call.
function ResponseViewer({ response, defaultOpen = false }) {
  const [open, setOpen] = useState(defaultOpen);
  if (!response) return null;
  const { status, ok, body, meta } = response;
  const statusText = STATUS_TEXT[status] || "";
  const tone = ok ? "ok" : status >= 500 ? "err" : status === 409 ? "warn" : "err";
  return (
    <div className="resp">
      <button className="resp-head" onClick={() => setOpen((o) => !o)}>
        <span className={`chev ${open ? "open" : ""}`}>▸</span>
        <span className="resp-label">Response</span>
        <span className={`http http-${tone}`}>
          {status} {statusText}
        </span>
        {meta && meta.cached && <Pill tone="info">CACHED</Pill>}
        <span className="resp-spacer" />
        <span className="resp-toggle">{open ? "hide" : "show"} JSON</span>
      </button>
      {open && (
        <pre className="json" dangerouslySetInnerHTML={{ __html: highlightJson(body) }} />
      )}
    </div>
  );
}

const STATUS_TEXT = {
  200: "OK",
  201: "Created",
  400: "Bad Request",
  404: "Not Found",
  409: "Conflict",
  500: "Internal Server Error",
};

/* ---------- entries table with per-currency totals ------------------ */
function EntriesTable({ entries, accountLabel }) {
  const totals = useMemo(() => {
    const m = {};
    entries.forEach((e) => (m[e.currency] = (m[e.currency] || 0) + Number(e.amount)));
    return m;
  }, [entries]);
  const balanced = Object.values(totals).every((v) => v === 0);

  return (
    <table className="tbl">
      <thead>
        <tr>
          <th>Account</th>
          <th>Currency</th>
          <th className="num">Amount</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((e, i) => (
          <tr key={e.id || i}>
            <td>{accountLabel(e.account_id)}</td>
            <td className="mono">{e.currency}</td>
            <td className="num">
              <Amount value={Number(e.amount)} />
            </td>
          </tr>
        ))}
      </tbody>
      <tfoot>
        {Object.entries(totals).map(([cur, total]) => (
          <tr key={cur} className="totals-row">
            <td className="totals-label">net {cur}</td>
            <td className="mono">{cur}</td>
            <td className="num">
              <span className={total === 0 ? "amt amt-pos" : "amt amt-neg"} style={{ fontWeight: 600 }}>
                {formatAmount(total)}
                {total === 0 && <span className="check"> ✓</span>}
              </span>
            </td>
          </tr>
        ))}
      </tfoot>
    </table>
  );
}

/* ---------- card / section shells ----------------------------------- */
function Card({ title, actions, children, className }) {
  return (
    <div className={`card ${className || ""}`}>
      {(title || actions) && (
        <div className="card-head">
          {title && <h3 className="card-title">{title}</h3>}
          {actions && <div className="card-actions">{actions}</div>}
        </div>
      )}
      <div className="card-body">{children}</div>
    </div>
  );
}

export {
  formatMinor,
  formatAmount,
  amtClass,
  Amount,
  CopyId,
  Button,
  StatusBadge,
  Pill,
  Banner,
  Field,
  AccountSelect,
  DateTimePicker,
  ResponseViewer,
  EntriesTable,
  Card,
};

