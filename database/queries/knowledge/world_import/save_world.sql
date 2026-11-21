INSERT INTO work_worlds(work_id,slug,version_id,name,edition,intro,cast_title,image,origin)
    VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,''),$9) ON CONFLICT(work_id) DO UPDATE SET version_id=excluded.version_id,
    name=excluded.name,edition=excluded.edition,intro=excluded.intro,cast_title=excluded.cast_title,image=excluded.image,published_at=now(),revision=work_worlds.revision+1
