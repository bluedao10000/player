UPDATE media_steps SET finished_at=now(),result=$4,error=nullif($5,'') WHERE slot=$1 AND round=$2 AND step=$3 AND attempt=$6
