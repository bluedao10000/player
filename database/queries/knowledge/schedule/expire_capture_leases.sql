UPDATE episode_capture_attempts SET finished_at=now(),error='采集租约到期' WHERE finished_at IS NULL AND started_at<now()-interval '10 minutes'
