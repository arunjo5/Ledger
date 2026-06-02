alter table transactions
    add column reverses_transaction_id uuid references transactions(id);

create index transactions_reverses_idx on transactions (reverses_transaction_id)
    where reverses_transaction_id is not null;
