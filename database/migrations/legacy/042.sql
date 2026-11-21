CREATE TABLE billing_store_topups (
    transaction_id text PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES billing_accounts,
    environment text NOT NULL, bundle_id text NOT NULL,
    credited boolean NOT NULL DEFAULT false,
    refund_event_at timestamptz,
    purchased_at timestamptz NOT NULL,
    currency text, price_milli bigint
);
ALTER TABLE billing_entries DROP CONSTRAINT billing_entries_kind_check, DROP CONSTRAINT billing_entries_check;
ALTER TABLE billing_entries ADD CONSTRAINT billing_entries_check CHECK (
    (kind IN ('credit','store_credit') AND order_id IS NULL AND amount_micros>0)
 OR (kind='store_refund' AND order_id IS NULL AND amount_micros<0)
 OR (kind='charge' AND order_id IS NOT NULL AND amount_micros<=0)
 OR (kind='refund' AND order_id IS NOT NULL AND amount_micros>=0)
);
