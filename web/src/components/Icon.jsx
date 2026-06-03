import React from "react";
/* =====================================================================
   Icon set — simple inline line icons, stroke = currentColor.
   ===================================================================== */
const ICON_PATHS = {
  grid: <React.Fragment><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><rect x="14" y="14" width="7" height="7" rx="1.5" /></React.Fragment>,
  plusSquare: <React.Fragment><rect x="3" y="3" width="18" height="18" rx="3" /><path d="M12 8v8M8 12h8" /></React.Fragment>,
  list: <React.Fragment><path d="M8 6h13M8 12h13M8 18h13" /><path d="M3.5 6h.01M3.5 12h.01M3.5 18h.01" /></React.Fragment>,
  wallet: <React.Fragment><rect x="3" y="6" width="18" height="13" rx="2.5" /><path d="M3 10h18" /><path d="M16 14h2" /></React.Fragment>,
  shield: <React.Fragment><path d="M12 3l7 3v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z" /><path d="M9 12l2 2 4-4" /></React.Fragment>,
  swap: <React.Fragment><path d="M17 3l3 3-3 3" /><path d="M20 6H8a4 4 0 00-4 4" /><path d="M7 21l-3-3 3-3" /><path d="M4 18h12a4 4 0 004-4" /></React.Fragment>,
  doubleCheck: <React.Fragment><path d="M2 13l4 4 9-11" /><path d="M11 16.5L13 19l9-11" /></React.Fragment>,
  clock: <React.Fragment><circle cx="12" cy="12" r="9" /><path d="M12 7.5V12l3 2" /></React.Fragment>,
  calendar: <React.Fragment><rect x="3" y="4.5" width="18" height="16" rx="2.5" /><path d="M3 9.5h18M8 2.5v4M16 2.5v4" /></React.Fragment>,
  xCircle: <React.Fragment><circle cx="12" cy="12" r="9" /><path d="M15 9l-6 6M9 9l6 6" /></React.Fragment>,
  hash: <React.Fragment><path d="M5 9h14M5 15h14M10 4L8 20M16 4l-2 16" /></React.Fragment>,
  info: <React.Fragment><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></React.Fragment>,
  arrowRight: <React.Fragment><path d="M5 12h14M13 6l6 6-6 6" /></React.Fragment>,
  refresh: <React.Fragment><path d="M21 12a9 9 0 11-3-6.7L21 8" /><path d="M21 4v4h-4" /></React.Fragment>,
};

function Icon({ name, size = 18, strokeWidth = 1.7, className, style }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      style={style}
      aria-hidden="true"
    >
      {ICON_PATHS[name] || null}
    </svg>
  );
}

export default Icon;
export { Icon };
