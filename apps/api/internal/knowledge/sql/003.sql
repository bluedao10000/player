ALTER TABLE knowledge_source_versions ADD COLUMN snapshot_key text;
ALTER TABLE knowledge_source_versions ADD COLUMN parser_type text;
ALTER TABLE knowledge_source_versions ADD COLUMN asset_name text;
ALTER TABLE knowledge_sources ADD COLUMN priority int NOT NULL DEFAULT 50;
UPDATE knowledge_sources SET priority=100 WHERE type='github-jsonl';
CREATE TABLE knowledge_field_priorities (
 field text NOT NULL,source_id int REFERENCES knowledge_sources,priority int NOT NULL,PRIMARY KEY(field,source_id)
);
INSERT INTO knowledge_field_priorities SELECT f,id,200 FROM knowledge_sources CROSS JOIN unnest(ARRAY['original','start','end']) f WHERE type='npm-dist';
CREATE TABLE knowledge_curation_actions(
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,work_id uuid NOT NULL REFERENCES works,
 operator text NOT NULL,action text NOT NULL,field text NOT NULL,before_value jsonb,after_value jsonb,
 reason text NOT NULL,source text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE work_field_overrides(work_id uuid REFERENCES works,field text,value jsonb NOT NULL,PRIMARY KEY(work_id,field));
CREATE TABLE knowledge_identity_pins(record_id bigint PRIMARY KEY REFERENCES knowledge_source_records,work_id uuid NOT NULL REFERENCES works);
CREATE TABLE knowledge_match_rejections(record_id bigint REFERENCES knowledge_source_records,work_id uuid REFERENCES works,PRIMARY KEY(record_id,work_id));
ALTER TABLE knowledge_match_candidates ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
CREATE TABLE knowledge_quality_rules(rule_id text PRIMARY KEY,field text NOT NULL,severity text NOT NULL CHECK(severity IN ('error','warning','info')),reason text NOT NULL,enabled boolean NOT NULL DEFAULT true);
INSERT INTO knowledge_quality_rules VALUES
 ('required','type','error','缺少作品类型'),('empty_title','canonical_title','error','标题为空'),
 ('url','url','warning','来源链接缺少有效 HTTP(S) 地址'),('title_norm','canonical_title','info','标题空白不规范'),
 ('language','lang','info','语言标签需要规范化'),('alias','titles','warning','同一语言有重复别名'),
 ('duplicate_id','refs','warning','多个作品共用来源 ID'),('conflict','refs','error','同一作品同一 provider 有不同 ID'),
 ('date','end_date','error','结束日期早于开始日期'),('enum','type/status','warning','作品类型或状态不在支持的范围'),
 ('cover','picture','info','缺少封面来源线索'),('relation','related','warning','关联 URL 无本地作品'),
 ('suspicious','episodes','warning','集数异常，需要核对');
CREATE TABLE knowledge_quality_issues(
 entity_id uuid REFERENCES works,rule_id text REFERENCES knowledge_quality_rules,field text NOT NULL,severity text NOT NULL,
 current_value jsonb,suggested_value jsonb,source text NOT NULL,reason text NOT NULL,status text NOT NULL CHECK(status IN ('open','resolved')),
 checked_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(entity_id,rule_id)
);
CREATE TABLE knowledge_quality_scores(work_id uuid PRIMARY KEY REFERENCES works,score int NOT NULL,components jsonb NOT NULL,checked_at timestamptz NOT NULL);
