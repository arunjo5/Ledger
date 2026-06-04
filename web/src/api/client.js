/* =====================================================================
   API client seam. The whole UI imports `ledger` from this file, so the
   backend sits behind one import. httpLedger.js talks to the LedgerCore
   API and returns a { status, ok, body, meta } envelope to each screen.
   ===================================================================== */

import { ledger as httpLedger } from "./httpLedger.js";

export const ledger = httpLedger;
export default ledger;
