SELECT count(*),extract(epoch FROM max(ends_at-starts_at))/3600 FROM reader_store_purchases
