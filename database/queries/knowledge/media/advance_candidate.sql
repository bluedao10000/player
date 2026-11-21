UPDATE media_candidates SET stage=$2,round=$3,focus=$4,reason=$5,attempt=$6,reference_sha=$7,updated_at=now() WHERE slot=$1
