-- Entries are append only.
create or replace function entries_block_mutation() returns trigger as $$
begin
    raise exception 'entries are immutable' using errcode = 'LG002';
end;
$$ language plpgsql;

create trigger entries_no_update before update on entries
    for each row execute function entries_block_mutation();

create trigger entries_no_delete before delete on entries
    for each row execute function entries_block_mutation();

-- Entries may only be attached while the transaction is still pending.
-- This keeps a committed transaction's entry set frozen.
create or replace function entries_require_pending() returns trigger as $$
declare
    tx_status text;
begin
    select status into tx_status from transactions where id = new.transaction_id;
    if tx_status is null then
        raise exception 'transaction % not found', new.transaction_id using errcode = 'LG003';
    end if;
    if tx_status <> 'pending' then
        raise exception 'cannot add entries to a % transaction', tx_status using errcode = 'LG003';
    end if;
    return new;
end;
$$ language plpgsql;

create trigger entries_require_pending before insert on entries
    for each row execute function entries_require_pending();

-- committed and failed are terminal states.
create or replace function transactions_guard_transition() returns trigger as $$
begin
    if old.status <> 'pending' and new.status <> old.status then
        raise exception 'cannot change status of a % transaction', old.status using errcode = 'LG004';
    end if;
    return new;
end;
$$ language plpgsql;

create trigger transactions_guard_transition before update on transactions
    for each row execute function transactions_guard_transition();

-- Zero-sum invariant. Deferred so it runs at commit, after all entries
-- for the transaction have been inserted in the same db transaction.
create or replace function transactions_assert_balanced() returns trigger as $$
begin
    if new.status <> 'committed' then
        return null;
    end if;

    if not exists (select 1 from entries where transaction_id = new.id) then
        raise exception 'committed transaction % has no entries', new.id
            using errcode = 'LG001';
    end if;

    if exists (
        select 1 from entries
        where transaction_id = new.id
        group by currency
        having sum(amount) <> 0
    ) then
        raise exception 'committed transaction % is not balanced per currency', new.id
            using errcode = 'LG001';
    end if;

    return null;
end;
$$ language plpgsql;

create constraint trigger trg_transactions_balanced
    after insert or update on transactions
    deferrable initially deferred
    for each row execute function transactions_assert_balanced();
