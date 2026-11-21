select (editor_profile_preview(base,draft)#>>'{profile_pages,world/setting,world_setting}') from editor_tasks where id=$1
