SELECT EXISTS(SELECT FROM reader_paid_periods a JOIN reader_paid_periods b
 ON a.reader_id=b.reader_id AND a.starts_at<b.ends_at AND b.starts_at<a.ends_at AND a.starts_at<>b.starts_at)
 OR count(DISTINCT starts_at)<>3 OR max(ends_at)-min(starts_at)<>interval '2160 hours' FROM reader_paid_periods
