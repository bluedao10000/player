INSERT INTO reader_bookmarks(reader_id,href,title,work,recorded_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(reader_id,href) DO NOTHING
