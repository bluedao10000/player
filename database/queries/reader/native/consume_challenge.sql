DELETE FROM reader_native_challenges WHERE nonce=$1 AND expires_at>now()
