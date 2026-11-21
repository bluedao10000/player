SELECT id::text,stage,documents FROM research_runs WHERE topic=$1 AND v=$2 AND policy=$3 AND stage IN ('collect','write')
