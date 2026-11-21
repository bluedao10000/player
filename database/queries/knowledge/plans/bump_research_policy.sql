UPDATE research_topics t SET policy=t.policy+1 FROM research_runs r WHERE r.id=$1 AND (r.topic,r.policy)=(t.id,t.policy) AND t.policy<3
