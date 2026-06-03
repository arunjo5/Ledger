/* =====================================================================
   API client seam
   ---------------------------------------------------------------------
   The whole UI imports `ledger` from THIS file and nothing else. To point
   the app at your real backend, change the single import below from the
   in-memory mock to your HTTP client (see httpLedger.example.js).

   IMPORTANT — sync vs async:
   The mock is SYNCHRONOUS: every method returns a response object
   ({ status, ok, body, meta }) directly. A real HTTP client must return a
   Promise. When you swap, make the screen handlers `async` and `await`
   each ledger call. The few call sites are:
     - ScreenCreate.jsx        ledger.createTransaction / createAccount
     - ScreenTransactions.jsx  ledger.getTransaction / reverseTransaction
     - ScreenBalance.jsx       ledger.deriveBalances / accountEntries (read)
     - ScreenReconciliation.jsx ledger.reconcile
     - ScreenIdempotency.jsx   ledger.createTransaction
     - App.jsx                 ledger.listAccounts / subscribe / reset

   The response envelope ({ status, ok, body, meta }) is intentional — keep
   it on the real client so the "raw JSON + HTTP status" viewer keeps working.
   ===================================================================== */

import { ledger as httpLedger } from "./httpLedger.js";

export const ledger = httpLedger;
export default ledger;
