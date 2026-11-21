UPDATE research_runs SET stage='ready',model=$2,claims=$3,finished_at=now() WHERE id=$1
