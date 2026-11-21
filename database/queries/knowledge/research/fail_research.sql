UPDATE research_runs SET stage='failed',finished_at=now(),error=$2 WHERE id=$1
