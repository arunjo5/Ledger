package ledger

import (
	"context"
	"time"
)

type RecentTx struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Lines       int       `json:"lines"`
	CreatedAt   time.Time `json:"created_at"`
	TxID        *string   `json:"tx_id"`
}

type Overview struct {
	Total     int        `json:"total"`
	Committed int        `json:"committed"`
	Pending   int        `json:"pending"`
	Failed    int        `json:"failed"`
	Recent    []RecentTx `json:"recent"`
}

func (s *Store) Overview(ctx context.Context) (Overview, error) {
	ov := Overview{Recent: []RecentTx{}}

	counts, err := s.pool.Query(ctx, `select status, count(*) from transactions group by status`)
	if err != nil {
		return Overview{}, err
	}
	for counts.Next() {
		var status string
		var count int
		if err := counts.Scan(&status, &count); err != nil {
			counts.Close()
			return Overview{}, err
		}
		switch status {
		case "committed":
			ov.Committed = count
		case "pending":
			ov.Pending = count
		case "failed":
			ov.Failed = count
		}
		ov.Total += count
	}
	counts.Close()
	if err := counts.Err(); err != nil {
		return Overview{}, err
	}

	recent, err := s.pool.Query(ctx,
		`select t.id::text, t.status, t.description, count(e.id), t.created_at
		 from transactions t
		 left join entries e on e.transaction_id = t.id
		 group by t.id, t.status, t.description, t.created_at
		 order by t.created_at desc
		 limit 8`)
	if err != nil {
		return Overview{}, err
	}
	defer recent.Close()
	for recent.Next() {
		var rt RecentTx
		if err := recent.Scan(&rt.ID, &rt.Status, &rt.Description, &rt.Lines, &rt.CreatedAt); err != nil {
			return Overview{}, err
		}
		if rt.Status == "committed" {
			id := rt.ID
			rt.TxID = &id
		}
		ov.Recent = append(ov.Recent, rt)
	}
	return ov, recent.Err()
}
