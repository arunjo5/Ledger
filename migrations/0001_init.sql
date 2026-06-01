create table accounts (
    id uuid primary key default gen_random_uuid(),
    label text unique,
    created_at timestamptz not null default now()
);

create table transactions (
    id uuid primary key default gen_random_uuid(),
    status text not null default 'pending'
        check (status in ('pending', 'committed', 'failed')),
    description text not null default '',
    idempotency_key text not null unique
        check (char_length(idempotency_key) between 1 and 255),
    request_hash bytea not null,
    response_body text,
    response_status smallint,
    created_at timestamptz not null default now(),
    committed_at timestamptz
);

create table entries (
    id bigint generated always as identity primary key,
    transaction_id uuid not null references transactions(id),
    account_id uuid not null references accounts(id),
    currency text not null check (currency ~ '^[A-Z]{3}$'),
    amount bigint not null check (amount <> 0),
    created_at timestamptz not null default now()
);

create index entries_transaction_id_idx on entries (transaction_id);
create index entries_account_recent_idx on entries (account_id, id desc);
create index entries_balance_idx on entries (account_id, currency) include (amount);
create index transactions_pending_idx on transactions (created_at) where status = 'pending';
