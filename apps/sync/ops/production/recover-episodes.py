"""Resume only the source and image requests claimed by the half-hour dispatcher."""
from concurrent.futures import ThreadPoolExecutor
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
from urllib.parse import urlparse
from uuid import uuid4

from inventory import schedule_page

ROOT = Path(os.environ.get("EPISODE_RECOVERY_ROOT", "/srv/folio-data"))
AGENT = os.environ.get("EPISODE_RESEARCH_CONTAINER", "folio-pi-research")


def sql(query, **parameters):
    command = ["docker", "exec", "-i", "folio-postgres", "psql", "-XqAt", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "folio"]
    for key, value in parameters.items():
        command.extend(["-v", f"{key}={value}"])
    return subprocess.check_output(command, input=query, text=True, timeout=30).strip()


def repair_images(job):
    paths = sql("""SELECT DISTINCT a.image FROM episode_plans p,
        jsonb_array_elements(p.spec->'people') person, edition_reader_art a
        WHERE p.topic=:'topic' AND a.world_slug=p.world_slug AND a.kind='character'
        AND a.item_slug=person->>'slug' AND a.image ~ '^/art/portraits/[a-f0-9]{64}\\.webp$';""", topic=job["topic"])
    restored = 0
    for image in paths.splitlines():
        name = Path(image).name
        digest = name.removesuffix(".webp")
        if not re.fullmatch("[a-f0-9]{64}", digest):
            raise ValueError("肖像摘要无效")
        dest = ROOT / "media" / name
        if dest.exists() and hashlib.sha256(dest.read_bytes()).hexdigest() == digest:
            continue
        with tempfile.NamedTemporaryFile(dir=dest.parent) as temp:
            subprocess.run(["curl", "--disable", "-fsS", "--max-time", "30", "--max-filesize", "10000000",
                            "-o", temp.name, "https://img.codeiy.com/i/" + digest], check=True)
            data = Path(temp.name).read_bytes()
            if data[:4] != b"RIFF" or data[8:12] != b"WEBP" or hashlib.sha256(data).hexdigest() != digest:
                raise ValueError("CDN 肖像与登记指纹不符")
            os.chmod(temp.name, 0o644)
            # Link the fully verified file atomically. Preserve any pre-existing conflicting file for inspection.
            try:
                os.link(temp.name, dest)
                restored += 1
            except FileExistsError:
                if hashlib.sha256(dest.read_bytes()).hexdigest() != digest:
                    raise ValueError("已有肖像文件指纹不符")
    return f"恢复 {restored} 张肖像，保留原候选及验图回执"


def run_source_agent(payload):
    output = subprocess.check_output(["docker", "exec", "-i", AGENT, "node", "apps/agent/src/episode-source-recovery.ts"],
                                     input=json.dumps(payload), text=True, timeout=500)
    return json.loads(output)


def source(job):
    number = job["number"]
    catalog = json.loads((ROOT / "catalog" / (job["world"] + ".json")).read_text())
    sample = job["sample"]
    if not sample:
        raise ValueError("缺少同版参照集")
    sql("""UPDATE episode_production_requests SET production_policy=jsonb_set(production_policy,'{recovery_sample}',:'sample'::jsonb)
      WHERE id=:'id' AND production_policy->>'dispatch_slot'=:'slot' AND NOT production_policy?'recovery_sample';""",
      id=job["id"], slot=job["slot"], sample=json.dumps(sample))
    episodes = schedule_page(int(catalog["externalID"]), 0)["data"]
    by_number = {ep["ep"]: ep for ep in episodes}
    def target(n):
        ep = by_number[n]
        if not ep["name"] or ep["airdate"] >= datetime.date.today().isoformat() or ep["airdate"] == "0000-00-00":
            raise ValueError("等待已播目录确认集名")
        return {"number": n, "title": ep["name"], "sourceNumber": int(ep["sort"])}
    offset = sample.pop("offset", None)
    sample.update(target(sample["number"]))
    current = target(number)
    if offset is not None:
        sample["sourceNumber"] = sample["number"] + offset
        current["sourceNumber"] = number + offset
    request = str(uuid4())
    payload = dict(target=current, sample=sample, work=catalog["title"], call=request, previousError=job.get("previous"),
                   domains=[urlparse(sample[k]).hostname for k in ("official", "guide") if sample.get(k)])
    payload["alternateGuide"] = urlparse(sample.get("guide", "")).hostname == "en.wikipedia.org" or bool(
        re.search(r'Invalid URL|HTTP (403|404|410)|来源与登记域名不一致|同站分集目录未唯一|未返回已核实的分集链接|指南正文不足', job.get("previous") or ""))
    reusable = sql("""SELECT production_policy->'source_bundle' FROM episode_production_requests
      WHERE world=:'world' AND production_policy?'source_bundle' ORDER BY abs(first_episode-:'number'::int) LIMIT 1;""",
      world=job["world"], number=number)
    if reusable:
        payload["reusable"] = json.loads(reusable)
    previous_calls = sql("""SELECT DISTINCT ON (m.operation) m.operation||'|'||m.response_id
      FROM episode_call_reservations c JOIN model_usage_receipts m ON m.subject='source:'||c.call_id
      WHERE c.request_id=:'id' AND m.operation IN ('source_rule','source_discovery') AND m.outcome='ok'
      AND m.response_id IS NOT NULL ORDER BY m.operation,m.finished_at DESC;""", id=job["id"])
    receipts = dict(row.split("|", 1) for row in previous_calls.splitlines())
    prior = receipts.get("source_discovery")
    payload.update(priorDiscovery=prior, priorRule=receipts.get("source_rule"), sourceCandidates=job.get("sourceCandidates"))
    result = run_source_agent({**payload, "reuseOnly": True}) if sample.get("official") else {}
    if not result.get("verified"):
        # Hold the request lock while opening a reservation. Leave the planning lane paused.
        reserved = sql(r"""BEGIN;
          UPDATE episode_production_requests SET operator_paused=false,paused_reason=NULL
          WHERE id=:'id' AND reserved=0 AND operator_paused AND paused_reason='待补可复用的同版本来源模板'
          AND production_policy->>'dispatch_slot'=:'slot' RETURNING id AS locked \gset
          SELECT reserve_episode_call(:'locked',:'call','source_rule',:'allowance'::int);
          UPDATE episode_production_requests SET operator_paused=true,paused_reason=coalesce(paused_reason,'待补可复用的同版本来源模板') WHERE id=:'locked';
          COMMIT;""", id=job["id"], slot=job["slot"], call=request, allowance=12000 if prior and not payload["alternateGuide"] else 32000)
        if reserved != "t":
            raise ValueError("来源生成预算未获预留")
        tokens = None
        try:
            result = run_source_agent(payload)
            tokens = result["tokens"]
        finally:
            sql("SELECT settle_episode_call(:'id',:'call',nullif(:'tokens','')::bigint);",
                id=job["id"], call=request, tokens="" if tokens is None else tokens)
        if result.get("error"):
            raise ValueError(result["error"])
    verified = result["verified"]
    verified["sources"] = verified["sources"][:1]
    verified["proofs"] = {k: v[:1] for k, v in verified["proofs"].items()}
    count = subprocess.check_output([os.environ.get("EPISODE_KNOWLEDGE_BIN", str(ROOT / "bin/knowledge")), "episode-source-rule-attach", job["world"]],
                                   input=json.dumps(verified, ensure_ascii=False, separators=(",", ":")), text=True).strip()
    if count != "1":
        raise ValueError("未挂接已验证规则，请核对范围与预算")
    sql("""UPDATE episode_production_requests SET operator_paused=false,paused_reason=NULL,updated_at=now(),
      production_policy=production_policy-'repair_source' WHERE id=:'id' AND production_policy->>'dispatch_slot'=:'slot' AND stage='inventory'
      AND operator_paused AND paused_reason='等待广州批次' AND reserved=0
      AND production_policy->'source_bundle'->>'ruleSha256'=:'digest';""",
        id=job["id"], slot=job["slot"], digest=verified["ruleSha256"])
    return "已验证目标及参照集，已按本轮名额交给制作队列"


def finish_images_for_batch(slot):
    # A configuration can finish after dispatch. Use the original episode's slot
    # for its portrait repair, so the next batch does not have to claim it again.
    while True:
        rows = sql("""SELECT json_build_object('id',r.id,'world',r.world,'number',r.first_episode,'topic',p.topic)
          FROM episode_production_requests r LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
          LEFT JOIN scope_heads h ON h.topic=p.topic LEFT JOIN scope_acceptances a ON a.id=h.latest
          WHERE r.production_policy->>'queue'='gz-20-aired-20261125' AND r.production_policy->>'dispatch_slot'=:'slot'
          AND r.production_policy->>'recovery_action' IN ('admit','configuration','source')
          AND r.production_policy->>'portrait_slot' IS DISTINCT FROM :'slot' AND NOT r.operator_paused
          AND r.paused_reason IS NULL AND r.stage NOT IN ('blocked','local_ready','published')
          AND a.passed IS DISTINCT FROM true;""", slot=slot)
        if not rows:
            pending = sql("""SELECT EXISTS (SELECT FROM episode_production_requests r
              LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
              LEFT JOIN scope_heads h ON h.topic=p.topic LEFT JOIN scope_acceptances a ON a.id=h.latest
              WHERE r.production_policy->>'queue'='gz-20-aired-20261125' AND r.production_policy->>'dispatch_slot'=:'slot'
              AND NOT r.operator_paused AND r.paused_reason IS NULL AND r.stage NOT IN ('blocked','local_ready','published')
              AND a.passed IS DISTINCT FROM true);""", slot=slot)
            if pending != "t":
                return
        for row in rows.splitlines():
            job = json.loads(row)
            if not job["topic"]:
                continue
            try:
                message = repair_images(job)
            except Exception as exc:
                message = str(exc)[:350]
            sql("""UPDATE episode_production_requests SET production_policy=production_policy||jsonb_build_object(
              'portrait_slot',:'slot'::text,'portrait_result',:'message'::text)
              WHERE id=:'id' AND production_policy->>'dispatch_slot'=:'slot';""",
                id=job["id"], slot=slot, message=message)
            print(job["world"], job["number"], message, flush=True)
        if time.time() >= int(slot) + 24 * 60:
            return
        time.sleep(15)


def run_claimed_job(job):
    try:
        message = repair_images(job) if job["action"] == "image" else source(job)
        success = True
    except Exception as exc:
        message = str(exc)[:350]
        success = False
    sql("""UPDATE episode_production_requests SET production_policy=production_policy||jsonb_build_object(
        'recovered_slot',:'slot'::text,'recovery_result',:'result'::text)
        WHERE id=:'id' AND production_policy->>'dispatch_slot'=:'slot';""",
        slot=job["slot"], result=message, id=job["id"])
    print(job["world"], job["number"], message, flush=True)
    return success


def batch_progress(slot):
    return sql("""SELECT json_build_object(
      'checked_cst',now() AT TIME ZONE 'Asia/Shanghai','worlds',count(DISTINCT r.world),'queued',count(*),
      'five_dimensions_passed',count(*) FILTER(WHERE a.passed),
      'published',count(*) FILTER(WHERE r.stage='published'),
      'claimed_this_slot',count(*) FILTER(WHERE r.production_policy->>'dispatch_slot'=:'slot'),
      'accepted_among_claimed',count(*) FILTER(WHERE r.production_policy->>'dispatch_slot'=:'slot' AND a.passed),
      'awaiting_usage',count(*) FILTER(WHERE r.paused_reason='missing_usage_receipt'))
      FROM episode_production_requests r LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
      LEFT JOIN scope_heads h ON h.topic=p.topic LEFT JOIN scope_acceptances a ON a.id=h.latest
      WHERE r.production_policy->>'queue'='gz-20-aired-20261125';""", slot=slot)


def main():
    slot = sql("SELECT extract(epoch FROM date_bin('30 minutes',now(),'2000-01-01 UTC'::timestamptz))::bigint;")
    jobs = sql("""SELECT json_build_object('world',r.world,'number',r.first_episode,'id',r.id,
      'action',r.production_policy->>'recovery_action','previous',r.production_policy->>'recovery_result','slot',r.production_policy->>'dispatch_slot','topic',p.topic,
      'sourceCandidates',r.production_policy->'source_candidates','sample',coalesce(r.production_policy->'recovery_sample',(SELECT jsonb_strip_nulls(jsonb_build_object('number',v.first_episode,
        'official',v.production_policy->'source_bundle'->>'official','guide',v.production_policy->'source_bundle'->>'guide',
        'offset',nullif(v.production_policy->'source_bundle'->>'sourceNumber','0')::int-v.first_episode))
        FROM episode_production_requests v WHERE v.world=r.world AND v.first_episode<>r.first_episode
        AND v.production_policy->'source_bundle'->'sourceRules'->>'guide'='paired'
        ORDER BY abs(v.first_episode-r.first_episode),v.first_episode LIMIT 1),
        (SELECT jsonb_strip_nulls(jsonb_build_object('number',t.number,'official',t.spec->>'official',
        'guide',nullif(t.spec->>'guide',''),'offset',nullif(t.spec->>'sourceNumber','0')::int-t.number))
        FROM episode_plans t WHERE t.world_slug=r.world AND t.number<>r.first_episode
        ORDER BY abs(t.number-r.first_episode),t.number LIMIT 1)))
      FROM episode_production_requests r LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
      WHERE r.production_policy->>'queue'='gz-20-aired-20261125' AND r.production_policy->>'recovery_action' IN ('image','source')
      AND r.production_policy->>'dispatch_slot'=extract(epoch FROM date_bin('30 minutes',now(),'2000-01-01 UTC'::timestamptz))::bigint::text
      AND r.production_policy->>'recovered_slot' IS DISTINCT FROM r.production_policy->>'dispatch_slot'
      ORDER BY r.production_policy->>'recovery_action',r.first_episode;""")
    with ThreadPoolExecutor(max_workers=3) as workers:
        results = list(workers.map(run_claimed_job, (json.loads(line) for line in jobs.splitlines())))
    finish_images_for_batch(slot)
    print(batch_progress(slot), flush=True)
    return int(not all(results))


if __name__ == "__main__":
    raise SystemExit(main())
