SELECT a.checks FROM scope_acceptances a JOIN scope_heads h ON h.latest=a.id WHERE h.published IS NULL AND h.topic=$1
