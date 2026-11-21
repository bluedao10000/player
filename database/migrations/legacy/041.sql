-- Order refund and reversal events separately from ordinary transaction refreshes.
ALTER TABLE reader_store_purchases ADD COLUMN refund_event_at timestamptz;
UPDATE reader_store_purchases SET refund_event_at=signed_at WHERE revoked_at IS NOT NULL;
