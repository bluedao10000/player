SELECT EXISTS(SELECT FROM backup_failed_attempts WHERE pod_uid=$1)
