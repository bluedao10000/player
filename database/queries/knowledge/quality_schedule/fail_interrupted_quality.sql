UPDATE knowledge_quality_runs SET status='failed',finished_at=now(),error_message='worker interrupted' WHERE status='running'
