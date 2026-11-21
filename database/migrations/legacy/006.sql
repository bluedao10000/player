-- Content belongs to immutable source releases. Switching a world's version publishes
-- its graph at once. A public child identity is the stable (work UUID, slug) pair.
INSERT INTO knowledge_sources(name,type,repository_url,data_url,check_url,asset_name) VALUES
 ('tidepost-original','world-original','local:worlddata','local:worlddata/tidepost.json','local:worlddata/tidepost.json','tidepost.json'),
 ('bebop-tv-guide','world-enrichment','https://www.cowboy-bebop.net/','https://api.bgm.tv/v0','https://www.cowboy-bebop.net/story/','cowboy-bebop-tv.json');
CREATE TABLE world_releases (
 version_id bigint PRIMARY KEY REFERENCES knowledge_source_versions,
 work_id uuid NOT NULL REFERENCES works, UNIQUE(work_id,version_id)
);
CREATE TABLE work_worlds (
 work_id uuid PRIMARY KEY, slug text UNIQUE NOT NULL, version_id bigint NOT NULL,
 revision bigint NOT NULL DEFAULT 1, published_at timestamptz NOT NULL DEFAULT now(),
 name text NOT NULL, edition text NOT NULL, intro text NOT NULL, cast_title text NOT NULL,
 image text, origin text NOT NULL CHECK(origin IN ('original','external')),
 FOREIGN KEY(work_id,version_id) REFERENCES world_releases(work_id,version_id)
);
CREATE TABLE home_world_slots (
 position int PRIMARY KEY CHECK(position>0), work_id uuid UNIQUE NOT NULL REFERENCES work_worlds
);
CREATE TABLE world_evidence(
 v bigint REFERENCES world_releases, key text NOT NULL, uri text NOT NULL, locator text NOT NULL, digest text NOT NULL, PRIMARY KEY(v,key)
);
CREATE TABLE work_characters(
 v bigint, slug text, name text NOT NULL, role text NOT NULL, portrait text, href text,
 position int NOT NULL, external_id text, original_name text, evidence text NOT NULL,
 PRIMARY KEY(v,slug), UNIQUE(v,external_id), FOREIGN KEY(v,evidence) REFERENCES world_evidence
);
CREATE TABLE work_chapters(
 v bigint, slug text, name text NOT NULL, label text NOT NULL, intro text NOT NULL, href text, image text,
 position int NOT NULL, episode int, external_id text, original_name text, evidence text NOT NULL,
 PRIMARY KEY(v,slug), UNIQUE(v,episode), UNIQUE(v,external_id), FOREIGN KEY(v,evidence) REFERENCES world_evidence
);
CREATE TABLE work_storylines(v bigint REFERENCES world_releases,slug text,title text NOT NULL,intro text NOT NULL, position int NOT NULL, PRIMARY KEY(v,slug));
CREATE TABLE work_storyline_chapters(v bigint,line text,chapter text,position int NOT NULL, PRIMARY KEY(v,line,chapter), UNIQUE(v,line,position),
 FOREIGN KEY(v,line) REFERENCES work_storylines, FOREIGN KEY(v,chapter) REFERENCES work_chapters);
CREATE TABLE chapter_blocks(v bigint,chapter text,key text,text text NOT NULL,position int NOT NULL, PRIMARY KEY(v,chapter,key),
 UNIQUE(v,chapter,position), FOREIGN KEY(v,chapter) REFERENCES work_chapters);
CREATE TABLE chapter_appearances(v bigint,chapter text,person text,evidence text NOT NULL, PRIMARY KEY(v,chapter,person),
 FOREIGN KEY(v,person) REFERENCES work_characters, FOREIGN KEY(v,chapter) REFERENCES work_chapters, FOREIGN KEY(v,evidence) REFERENCES world_evidence);
CREATE INDEX chapter_by_character ON chapter_appearances(v,person,chapter);
CREATE TABLE work_settings(v bigint REFERENCES world_releases,name text,PRIMARY KEY(v,name));
CREATE TABLE work_scenes(v bigint,slug text,chapter text NOT NULL,name text NOT NULL,place text NOT NULL,text text NOT NULL,
 block text NOT NULL,position int NOT NULL,evidence text NOT NULL, PRIMARY KEY(v,slug), UNIQUE(v,chapter,slug), UNIQUE(v,chapter,position),
 FOREIGN KEY(v,chapter,block) REFERENCES chapter_blocks, FOREIGN KEY(v,place) REFERENCES work_settings, FOREIGN KEY(v,evidence) REFERENCES world_evidence);
CREATE TABLE scene_appearances(v bigint,chapter text,scene text,person text,evidence text NOT NULL, PRIMARY KEY(v,scene,person),
 FOREIGN KEY(v,chapter,scene) REFERENCES work_scenes(v,chapter,slug), FOREIGN KEY(v,chapter,person) REFERENCES chapter_appearances, FOREIGN KEY(v,evidence) REFERENCES world_evidence);
