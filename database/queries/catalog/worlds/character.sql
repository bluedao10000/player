SELECT editor_public_profile(p,$1,'person/'||$2) FROM editor_raw_person($1,$2) p WHERE p IS NOT NULL
