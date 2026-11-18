CREATE TABLE media_candidates (
 slot integer PRIMARY KEY CHECK(slot BETWEEN 1 AND 9), scene text NOT NULL, variant integer NOT NULL,
 stage text NOT NULL DEFAULT 'queued' CHECK(stage IN ('queued','generate','review','recrop','recheck','passed','held','failed')),
 round integer NOT NULL DEFAULT 1 CHECK(round IN (1,2)), focus integer NOT NULL DEFAULT 40 CHECK(focus BETWEEN 0 AND 100),
 reason text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(scene,variant)
);
INSERT INTO media_candidates(slot,scene,variant)
 SELECT i, (ARRAY['港口日景半身','雨夜侧脸','渡口持信'])[(i-1)/3+1],(i-1)%3+1 FROM generate_series(1,9)i;
CREATE TABLE media_steps (
 slot integer REFERENCES media_candidates, round integer NOT NULL, step text NOT NULL,
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz, result jsonb, error text,
 PRIMARY KEY(slot,round,step)
);
