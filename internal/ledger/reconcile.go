package ledger

import (
	"context"
	"fmt"
)

type ReconcileCheck struct {
	Name      string   `json:"name"`
	OK        bool     `json:"ok"`
	Detail    string   `json:"detail"`
	Offenders []string `json:"offenders,omitempty"`
}

type ReconcileResult struct {
	OK     bool             `json:"ok"`
	Checks []ReconcileCheck `json:"checks"`
}

func (s *Store) RecoverPending(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`update transactions set status = 'failed' where status = 'pending'`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *Store) Reconcile(ctx context.Context) (ReconcileResult, error) {
	unbalanced, err := s.reconcileIDs(ctx,
		`select distinct t.id::text
		 from transactions t
		 join entries e on e.transaction_id = t.id
		 where t.status = 'committed'
		 group by t.id, e.currency
		 having sum(e.amount) <> 0`)
	if err != nil {
		return ReconcileResult{}, err
	}

	empty, err := s.reconcileIDs(ctx,
		`select t.id::text from transactions t
		 where t.status = 'committed'
		   and not exists (select 1 from entries e where e.transaction_id = t.id)`)
	if err != nil {
		return ReconcileResult{}, err
	}

	orphaned, err := s.reconcileIDs(ctx,
		`select distinct e.transaction_id::text
		 from entries e
		 join transactions t on t.id = e.transaction_id
		 where t.status <> 'committed'`)
	if err != nil {
		return ReconcileResult{}, err
	}

	stale, err := s.reconcileIDs(ctx,
		`select id::text from transactions
		 where status = 'pending' and created_at < now() - interval '1 minute'`)
	if err != nil {
		return ReconcileResult{}, err
	}

	checks := []ReconcileCheck{
		reconcileCheck("committed transactions balance to zero per currency", "all committed transactions balance", unbalanced),
		reconcileCheck("committed transactions have entries", "no empty committed transactions", empty),
		reconcileCheck("entries belong only to committed transactions", "no orphaned entries", orphaned),
		reconcileCheck("no pending transactions older than one minute", "no stale pending transactions", stale),
	}

	result := ReconcileResult{OK: true, Checks: checks}
	for _, c := range checks {
		if !c.OK {
			result.OK = false
		}
	}
	return result, nil
}

func reconcileCheck(name, okDetail string, offenders []string) ReconcileCheck {
	if len(offenders) == 0 {
		return ReconcileCheck{Name: name, OK: true, Detail: okDetail}
	}
	return ReconcileCheck{
		Name:      name,
		OK:        false,
		Detail:    fmt.Sprintf("%d offending transactions", len(offenders)),
		Offenders: offenders,
	}
}

func (s *Store) reconcileIDs(ctx context.Context, query string) ([]string, error) {
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
