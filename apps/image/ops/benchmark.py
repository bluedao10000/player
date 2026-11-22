#!/usr/bin/env python3
"""Benchmark a dedicated, disposable Docker container and its private image volume."""
import argparse
import concurrent.futures
import http.client
import json
import math
import os
import pathlib
import secrets
import subprocess
import threading
import time
import urllib.parse

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--container", required=True)
parser.add_argument("--url", default="http://127.0.0.1:14680")
parser.add_argument("--small", type=pathlib.Path, required=True)
parser.add_argument("--medium", type=pathlib.Path, required=True)
parser.add_argument("--large", type=pathlib.Path, required=True)
args = parser.parse_args()
token = os.environ["IMAGE_API_TOKEN"]
base = urllib.parse.urlsplit(args.url)
assert base.scheme == "http" and base.hostname in {"localhost", "127.0.0.1"}, "use a local test instance"
workers = threading.local()
run_id = secrets.token_hex(4)


def request(path, method="GET", data=None, mime=None, expected=200):
    if not hasattr(workers, "connection"):
        workers.connection = http.client.HTTPConnection(base.hostname, base.port, timeout=120)
    headers = {"Authorization": "Bearer " + token}
    if mime is not None:
        headers["Content-Type"] = mime
    workers.connection.request(method, path, body=data, headers=headers)
    response = workers.connection.getresponse()
    payload = response.read()
    if response.status != expected:
        raise RuntimeError(f"{method} {path}: HTTP {response.status}; {payload[:240]!r}")
    return payload


def upload(path, mime):
    image = json.loads(request("/v1/assets/upload?" + urllib.parse.urlencode({"name": path.name}),
                               "POST", path.read_bytes(), mime, 201))
    uri = f"/v1/assets/{image['id']}"
    request(uri + "/publish", "POST", b'{"reference":"benchmark"}', "application/json")
    return image


def cgroup():
    command = ("cat /sys/fs/cgroup/cpu.stat; "
               "sed 's/^/memory_peak /' /sys/fs/cgroup/memory.peak; "
               "cat /sys/fs/cgroup/io.stat")
    output = subprocess.check_output(["docker", "exec", args.container, "sh", "-c", command], text=True)
    metrics = {"usage_usec": 0, "memory_peak": 0, "rbytes": 0, "wbytes": 0}
    for line in output.splitlines():
        words = line.split()
        if not words:
            continue
        if words[0] in metrics:
            metrics[words[0]] = int(words[1])
        elif ":" in words[0]:
            for pair in words[1:]:
                key, value = pair.split("=")
                if key in ("rbytes", "wbytes"):
                    metrics[key] += int(value)
    return metrics


def measure(label, paths, parallel):
    def timed(path):
        start = time.perf_counter()
        request(path)
        return (time.perf_counter() - start) * 1000

    before_stats = json.loads(request("/v1/stats"))
    before = cgroup()
    start = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=parallel) as pool:
        samples = sorted(pool.map(timed, paths))
    elapsed = time.perf_counter() - start
    after = cgroup()
    after_stats = json.loads(request("/v1/stats"))
    percentiles = {f"p{p}_ms": round(samples[math.ceil(len(samples) * p / 100) - 1], 3)
                   for p in (50, 95, 99)}
    record = dict(case=label, requests=len(samples), concurrency=parallel, qps=round(len(samples) / elapsed, 2),
                  **percentiles, cpu_percent_one_core=round((after["usage_usec"] - before["usage_usec"]) / elapsed / 10000, 2),
                  process_cgroup_peak_mib=round(after["memory_peak"] / 1048576, 2),
                  disk_read_bytes=after["rbytes"] - before["rbytes"], disk_write_bytes=after["wbytes"] - before["wbytes"],
                  hits=after_stats["cache_hits"] - before_stats["cache_hits"],
                  misses=after_stats["cache_misses"] - before_stats["cache_misses"])
    print(json.dumps(record), flush=True)


small, medium, large = [upload(path, mime) for path, mime in
                        ((args.small, "image/webp"), (args.medium, "image/webp"), (args.large, "image/png" if args.large.suffix.lower() == ".png" else "image/jpeg"))]
for image in (small, medium, large):
    print(json.dumps({key: image[key] for key in ("name", "size", "width", "height", "sha256")}), flush=True)
assert small["size"] < 102400 and 102400 <= medium["size"] <= 2097152
assert 5_000_000 <= large["size"] <= 20_000_000 and large["width"] >= 3840
uri = lambda image: f"/i/{image['id']}"
for image, count in ((small, 500), (medium, 500), (large, 80)):
    request(uri(image))
    measure("original_" + image["name"], [uri(image)] * count, 8)
warm = uri(medium) + "?preset=thumb"
request(warm)
measure("warm_thumbnail", [warm] * 600, 8)
measure("cold_large_webp", [uri(large) + f"?w={i}&h=700&format=webp" for i in range(801, 833)], 2)
measure("cold_large_avif", [uri(large) + f"?w={i}&format=avif" for i in range(501, 513)], 2)

# Seed the same SQLite schema while sharing real, validated original bytes.
# This isolates catalog performance from codec throughput and disk capacity.
directory = json.loads(request("/v1/directories", "POST",
                               json.dumps({"name": "benchmark-" + run_id}).encode(), "application/json", 201))["id"]
for previous, count in ((0, 10_000), (10_000, 100_000)):
    sql = f""".timeout 5000
    WITH RECURSIVE n(i) AS (SELECT {previous + 1} UNION ALL SELECT i+1 FROM n WHERE i<{count})
    INSERT INTO assets(id,storage_key,name,directory_id,mime,size,width,height,sha256,orientation,created_at,source,metadata)
    SELECT printf('90000000-0000-4000-0000-%012d', n.i), a.storage_key,
           printf('scene-%d.webp', n.i), '{directory}', a.mime,a.size,a.width,a.height,a.sha256,a.orientation,
           a.created_at,'benchmark', CASE WHEN n.i={count} THEN '{{"tag":"last-{count}"}}' ELSE '{{}}' END
    FROM n CROSS JOIN assets a WHERE a.id='{small['id']}';
    """
    subprocess.run(["docker", "exec", "-i", args.container, "sqlite3", "-bail", "/data/library.sqlite"],
                   input=sql, text=True, check=True)
    path = f"/v1/assets?directory_id={directory}&limit=50"
    assert len(json.loads(request(path))["items"]) == 50
    search = path + f"&q=last-{count}"
    assert len(json.loads(request(search))["items"]) == 1
    measure(f"list_{count}", [path] * 300, 8)
    measure(f"substring_metadata_{count}", [search] * 120, 4)
