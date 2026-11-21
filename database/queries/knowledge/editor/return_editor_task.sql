UPDATE editor_tasks SET round=round+1,attempt=1,status='evidence',feedback=$2,issue='',retries=0,due=now(),lease=NULL WHERE id=$1
