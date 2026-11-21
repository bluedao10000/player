SELECT editor_profile_text(editor_public_profile(editor_raw_person($1,'mio'),$1,'person/mio'),'person/mio'),
 editor_profile_text(editor_public_profile(editor_raw_overview($1),$1,'world/setting'),'world/setting')
