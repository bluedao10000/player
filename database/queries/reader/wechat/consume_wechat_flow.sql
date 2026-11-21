DELETE FROM reader_wechat_flows WHERE state=$1 AND browser_hash=$2 AND expires_at>now() RETURNING return_to,bind_session
