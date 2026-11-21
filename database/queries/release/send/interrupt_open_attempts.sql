UPDATE reader_release_attempts SET finished_at=now(),error='发布进程中断，等待重试' WHERE finished_at IS NULL
