INSERT INTO reader_positions(reader_id,href,title,work,percent,anchor,excerpt,recorded_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(reader_id,href) DO UPDATE SET title=excluded.title,work=excluded.work,percent=excluded.percent,anchor=excluded.anchor,excerpt=excluded.excerpt,recorded_at=excluded.recorded_at
 WHERE reader_positions.recorded_at<excluded.recorded_at
