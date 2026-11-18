import type { ScopeAcceptance, WorkKnowledgeStatus } from "@anime/shared/knowledge";

const releaseLabels: Record<string, string> = {
  FINISHED: "已完结",
  ONGOING: "连载中",
  UPCOMING: "未开播",
  UNKNOWN: "发行状态未知",
};

export function WorldInventory({
  worlds, scopes,
}: {
  worlds: WorkKnowledgeStatus[] | null;
  scopes: ScopeAcceptance[] | null;
}) {
  return (
    <section className="wb-inventory" aria-labelledby="world-inventory">
      <h2 id="world-inventory">作品知识状态</h2>
      <p className="wb-caption">当前作品版本的资料库存；分集验收结果见「范围验收」。</p>
      {worlds === null && <p role="status">作品状态暂不可用。</p>}
      {worlds?.length === 0 && <p>暂无公开作品世界。</p>}
      <div className="wb-worlds">
        {worlds?.map((w) => (
          <article key={w.work_id}>
            <h3>
              <a href={w.slug ? "/work/" + w.slug : "/knowledge/" + w.work_id}>
                {w.title}
              </a>
            </h3>
            <p>
              {w.edition} ·{" "}
              {releaseLabels[w.release_status ?? "UNKNOWN"] ?? w.release_status}
              {w.total_episodes != null &&
                w.total_episodes > 0 &&
                " · 全 " + w.total_episodes + " 集"}
            </p>
            <strong>
              {w.maturity === "linked" ? "已有关联知识" : "基础档案"}
            </strong>
            <span>
              {" "}
              · {scopes?.some(s => s.world === w.slug && s.published) ? <a href="#scope-acceptance">部分集数已发布</a> : w.visibility === "sample" ? "样板预览" : "仅资料页"}
              {w.revision != null && " v" + w.revision}
            </span>
            <dl>
              <div>
                <dt>人物</dt>
                <dd>
                  {w.characters} 位 · {w.characters_in_scenes} 位关联场景
                </dd>
              </div>
              <div>
                <dt>剧情</dt>
                <dd>
                  {w.storylines} 条故事线 · {w.chapters} 篇章节 · {w.scenes} 幕
                </dd>
              </div>
              <div>
                <dt>世界观</dt>
                <dd>{w.settings} 个已登记地点</dd>
              </div>
              <div>
                <dt>时间线</dt>
                <dd>按选题范围核对故事内事件</dd>
              </div>
              <div>
                <dt>图片</dt>
                <dd>
                  主视觉{w.has_hero ? "已引用" : "缺图"} · {w.portraits} 张立绘
                  · {w.chapter_images} 张章节配图引用
                </dd>
              </div>
            </dl>
            {w.readable_episodes.length > 0 && (
              <p>可读集号 · {w.readable_episodes.join("、")}</p>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}
