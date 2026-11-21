"""Add one Folio service path to the existing Mesh device profile and L4 policy."""
import argparse
import ipaddress
import json
import os
import urllib.request

def api(path, method="GET", data=None):
    base = "https://api.cloudflare.com/client/v4/accounts/" + os.environ["CLOUDFLARE_ACCOUNT_ID"]
    req = urllib.request.Request(base + path, method=method,
        headers={"Authorization": "Bearer " + os.environ["CLOUDFLARE_API_TOKEN"], "Content-Type": "application/json"},
        data=json.dumps(data).encode() if data is not None else None)
    with urllib.request.urlopen(req, timeout=30) as response:
        result = json.load(response)
    if not result["success"]:
        raise RuntimeError("Cloudflare could not apply " + path)
    return result["result"]

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", choices=["release", "backup", "image"])
    parser.add_argument("source", type=ipaddress.IPv4Address)
    parser.add_argument("destination", type=ipaddress.IPv4Address)
    args = parser.parse_args()
    if not all(ip in ipaddress.ip_network("100.96.0.0/12") for ip in (args.source, args.destination)):
        parser.error("expected enrolled Mesh /32 addresses")
    name, port, priority = {
        "release": ("Folio Silicon Valley to Aliyun reader", 2222, 9010),
        "image": ("Folio Aliyun to Guangzhou images", 14580, 9030),
        "backup": ("Folio backup to Guangzhou PostgreSQL", 5433, 9020),
    }[args.path]
    traffic = f'net.src.internal_src_ip == {args.source} and net.dst.ip == {args.destination} and net.dst.port == {port} and net.protocol == "tcp"'
    rules = api("/gateway/rules")
    rule = next((r for r in rules if r["name"] == name), None)
    if rule:
        if (rule["traffic"], rule["action"], rule["enabled"]) != (traffic, "allow", True):
            raise RuntimeError("existing named gateway rule differs")
    else:
        api("/gateway/rules", "POST", {"name": name, "enabled": True, "action": "allow",
            "filters": ["l4"], "precedence": priority, "traffic": traffic})
    profile = next(p for p in api("/devices/policies") if p["name"] == "Folio Mesh database peers")
    path = "/devices/policy/" + profile["policy_id"] + "/include"
    routes = api(path)
    new = [{"address": str(ip) + "/32"} for ip in (args.source, args.destination)
           if not any(r.get("address") == str(ip) + "/32" for r in routes)]
    if new:
        api(path, "PUT", routes + new)
    print(name, args.source, "->", args.destination, port)

if __name__ == "__main__":
    main()
