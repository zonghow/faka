#!/usr/bin/env python3
"""Backend scale / stress test against an isolated temp DB."""

from __future__ import annotations

import json
import os
import signal
import statistics
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SERVER_BIN = ROOT / "server" / "bin" / "faka-server"
PASSWORD = "stress-test-password"
SESSION = "stress-test-session-secret"
PORT = "18799"
BASE = f"http://127.0.0.1:{PORT}"


def http(method: str, path: str, body: bytes | None = None, headers: dict | None = None, timeout: float = 300.0):
    req = urllib.request.Request(BASE + path, data=body, method=method, headers=headers or {})
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = resp.read()
            ms = (time.perf_counter() - t0) * 1000
            return resp.status, data, ms, dict(resp.headers)
    except urllib.error.HTTPError as e:
        data = e.read()
        ms = (time.perf_counter() - t0) * 1000
        return e.code, data, ms, dict(e.headers)


def auth(json_body: bool = True) -> dict:
    h = {"X-Admin-Password": PASSWORD, "Accept": "application/json"}
    if json_body:
        h["Content-Type"] = "application/json"
    return h


def pct(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    s = sorted(values)
    i = min(len(s) - 1, max(0, int(round((p / 100.0) * (len(s) - 1)))))
    return s[i]


def summarize(name: str, samples: list[tuple[int, float]], wall_ms: float):
    ok = [(st, ms) for st, ms in samples if 200 <= st < 300]
    bad = [(st, ms) for st, ms in samples if not (200 <= st < 300)]
    print(f"\n== {name} ==")
    if not samples:
        print("  no samples")
        return
    times = [ms for _, ms in ok] or [ms for _, ms in samples]
    rps = (len(ok) / (wall_ms / 1000.0)) if wall_ms > 0 and ok else 0
    print(
        f"  ok={len(ok)} err={len(bad)} "
        f"avg={statistics.mean(times):.1f}ms p50={pct(times,50):.1f}ms "
        f"p95={pct(times,95):.1f}ms p99={pct(times,99):.1f}ms "
        f"min={min(times):.1f} max={max(times):.1f} "
        f"wall={wall_ms:.0f}ms ~{rps:.1f}rps"
    )
    if bad[:3]:
        print(f"  sample errors: {bad[:3]}")


def run_bench(name: str, fn, n: int = 20, concurrency: int = 1):
    samples: list[tuple[int, float]] = []
    t0 = time.perf_counter()
    if concurrency <= 1:
        for _ in range(n):
            st, ms = fn()
            samples.append((st, ms))
    else:
        with ThreadPoolExecutor(max_workers=concurrency) as ex:
            futs = [ex.submit(fn) for _ in range(n)]
            for f in as_completed(futs):
                try:
                    samples.append(f.result())
                except Exception:
                    samples.append((599, 0.0))
    wall = (time.perf_counter() - t0) * 1000
    summarize(name, samples, wall)


def seed_files(tmpdir: Path, count: int) -> int:
    import sqlite3

    db = tmpdir / "data" / "tikawang.db"
    upload = tmpdir / "storage" / "uploads"
    upload.mkdir(parents=True, exist_ok=True)
    for _ in range(100):
        if db.exists():
            break
        time.sleep(0.05)

    conn = sqlite3.connect(str(db))
    cur = conn.cursor()
    row = cur.execute("select id from spaces order by id asc limit 1").fetchone()
    if not row:
        cur.execute(
            "insert into spaces(name, card_prefix, created_at, updated_at) values(?,?,datetime('now'),datetime('now'))",
            ("default", "STRESS"),
        )
        conn.commit()
        space_id = cur.execute("select id from spaces order by id asc limit 1").fetchone()[0]
    else:
        space_id = row[0]

    print(f"seeding {count} available files (space={space_id})...")
    t0 = time.perf_counter()
    batch = []
    now = time.strftime("%Y-%m-%d %H:%M:%S")
    for i in range(count):
        name = f"stress_{i:06d}@example.com"
        path = upload / f"{i:06d}_{name}"
        path.write_text(
            json.dumps(
                {
                    "email": name,
                    "access_token": "aaa.bbb.ccc",
                    "tokens": {"access_token": "aaa.bbb.ccc"},
                }
            ),
            encoding="utf-8",
        )
        batch.append((name, str(path), now, now, space_id, "available"))
        if len(batch) >= 800:
            cur.executemany(
                "insert into files(original_name, stored_path, generated_at, uploaded_at, space_id, status) values (?,?,?,?,?,?)",
                batch,
            )
            conn.commit()
            batch.clear()
    if batch:
        cur.executemany(
            "insert into files(original_name, stored_path, generated_at, uploaded_at, space_id, status) values (?,?,?,?,?,?)",
            batch,
        )
        conn.commit()
    n = cur.execute("select count(1) from files where status='available'").fetchone()[0]
    conn.close()
    print(f"  done available={n} in {(time.perf_counter()-t0)*1000:.0f}ms")
    return n


def ensure_server_built():
    if SERVER_BIN.exists():
        return
    print("building server binary...")
    subprocess.check_call(
        [
            "bash",
            "-lc",
            f"export PATH=/usr/local/go/bin:/usr/bin:/bin; cd {ROOT}/server && go build -o bin/faka-server ./cmd/server",
        ]
    )


def main() -> int:
    ensure_server_built()
    tmp = Path(tempfile.mkdtemp(prefix="faka-stress-"))
    (tmp / "data").mkdir()
    (tmp / "storage" / "uploads").mkdir(parents=True)
    (tmp / "storage" / "downloads").mkdir(parents=True)
    (tmp / "web" / "dist").mkdir(parents=True)
    (tmp / "web" / "dist" / "index.html").write_text("<html>stress</html>", encoding="utf-8")
    log_path = tmp / "server.log"

    env = os.environ.copy()
    env.update(
        {
            "PATH": "/usr/local/go/bin:/usr/bin:/bin",
            "TIKAWANG_BASE_DIR": str(tmp),
            "TIKAWANG_ADDR": f"0.0.0.0:{PORT}",
            "TIKAWANG_AUTH_PASSWORD": PASSWORD,
            "TIKAWANG_SESSION_SECRET": SESSION,
            "TIKAWANG_STATIC_DIR": str(tmp / "web" / "dist"),
            "TIKAWANG_DATABASE_URL": f"sqlite:///{(tmp / 'data' / 'tikawang.db').as_posix()}",
            "TIKAWANG_STORAGE_DIR": str(tmp / "storage"),
        }
    )
    print(f"temp dir: {tmp}")
    proc = subprocess.Popen(
        [str(SERVER_BIN)],
        cwd=str(tmp),
        env=env,
        stdout=open(log_path, "w"),
        stderr=subprocess.STDOUT,
    )

    try:
        for i in range(80):
            try:
                st, _, ms, _ = http("GET", "/api/inventory")
                if st == 200:
                    print(f"server ready (attempt {i+1}, inventory {ms:.1f}ms)")
                    break
            except Exception:
                time.sleep(0.1)
        else:
            print("server failed to start")
            print(log_path.read_text()[-3000:])
            return 1

        # seed large inventory
        seed_files(tmp, 8000)

        # ---- create cards scale ----
        print("\n== create cards scale ==")
        for qty in [100, 1000, 5000, 10000]:
            body = json.dumps({"file_count": 1, "quantity": qty}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards", body, auth(), timeout=600)
            j = json.loads(data.decode() or "{}")
            print(f"  qty={qty}: http={st} {ms:.0f}ms count={j.get('count')} ids={len(j.get('ids') or [])}")

        # higher file_count cards for redeem
        print("\n== create multi-file cards ==")
        for fc, qty in [(10, 40), (100, 15), (500, 4), (1000, 2)]:
            body = json.dumps({"file_count": fc, "quantity": qty}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards", body, auth(), timeout=300)
            j = json.loads(data.decode() or "{}")
            print(f"  file_count={fc} qty={qty}: http={st} {ms:.0f}ms count={j.get('count')}")

        # pending all available cards
        st, data, ms, _ = http("GET", "/api/admin/cards?status=available&page_size=10000", headers=auth(False))
        cards = json.loads(data.decode()).get("cards", [])
        ids = [c["id"] for c in cards]
        if ids:
            body = json.dumps({"ids": ids, "target_status": "pending"}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards/status", body, auth(), timeout=300)
            print(f"\nmark pending {len(ids)}: http={st} {ms:.0f}ms")

        st, data, _, _ = http("GET", "/api/admin/cards?status=pending&page_size=10000", headers=auth(False))
        pending = json.loads(data.decode()).get("cards", [])
        by_fc: dict[int, list[dict]] = {}
        for c in pending:
            by_fc.setdefault(int(c["file_count"]), []).append(c)
        print("pending by file_count:", {k: len(v) for k, v in sorted(by_fc.items())})

        # ---- read benchmarks ----
        run_bench(
            "GET /api/inventory seq x50",
            lambda: (lambda s, d, m, h: (s, m))(*http("GET", "/api/inventory")),
            n=50,
        )
        run_bench(
            "GET /api/inventory concurrent x100 c=20",
            lambda: (lambda s, d, m, h: (s, m))(*http("GET", "/api/inventory")),
            n=100,
            concurrency=20,
        )

        for ps in [200, 2000, 10000]:
            p = f"/api/admin/cards?page=1&page_size={ps}"
            run_bench(
                f"GET /api/admin/cards page_size={ps} x8",
                lambda path=p: (lambda s, d, m, h: (s, m))(*http("GET", path, headers=auth(False))),
                n=8,
            )
        for ps in [200, 2000, 10000]:
            p = f"/api/admin/files?page=1&page_size={ps}"
            run_bench(
                f"GET /api/admin/files page_size={ps} x6",
                lambda path=p: (lambda s, d, m, h: (s, m))(*http("GET", path, headers=auth(False))),
                n=6,
            )

        def mixed_list():
            import random

            path = "/api/admin/cards?page=1&page_size=200" if random.random() < 0.5 else "/api/admin/files?page=1&page_size=200"
            s, d, m, _ = http("GET", path, headers=auth(False))
            return s, m

        run_bench("mixed admin list concurrent x60 c=15", mixed_list, n=60, concurrency=15)

        # bulk status / download
        st, data, _, _ = http("GET", "/api/admin/cards?status=pending&page_size=500", headers=auth(False))
        sample_ids = [c["id"] for c in json.loads(data.decode()).get("cards", [])][:500]
        if sample_ids:
            body = json.dumps({"ids": sample_ids, "target_status": "pending"}).encode()
            run_bench(
                f"POST cards/status bulk={len(sample_ids)} x5",
                lambda b=body: (lambda s, d, m, h: (s, m))(*http("POST", "/api/admin/cards/status", b, auth())),
                n=5,
            )
            # download requires available
            body_av = json.dumps({"ids": sample_ids[:200], "target_status": "available"}).encode()
            http("POST", "/api/admin/cards/status", body_av, auth())
            body_dl = json.dumps({"ids": sample_ids[:200], "mark_pending": True}).encode()
            st, data, ms, h = http("POST", "/api/admin/cards/download", body_dl, auth(), timeout=180)
            print(f"\n== cards/download 200: http={st} {ms:.0f}ms bytes={len(data)}")

        # ---- redeem ----
        print("\n== redeem scale ==")
        import sqlite3

        for fc in [1, 10, 100, 500, 1000]:
            pool = by_fc.get(fc, [])
            if not pool:
                print(f"  fc={fc}: no cards")
                continue
            conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
            avail_n = conn.execute("select count(1) from files where status='available'").fetchone()[0]
            conn.close()
            if avail_n < fc:
                print(f"  fc={fc}: skip, available={avail_n}")
                continue
            code = pool.pop()["code"]
            form = f"card_code={code}&output_format=cpa".encode()
            st, data, ms, _ = http(
                "POST",
                "/api/redeem",
                form,
                {"Content-Type": "application/x-www-form-urlencoded"},
                timeout=600,
            )
            print(f"  redeem cpa file_count={fc}: http={st} {ms:.0f}ms bytes={len(data)}")

        ones = by_fc.get(1, [])
        if len(ones) >= 18:
            # sequential
            times = []
            for c in ones[:10]:
                form = f"card_code={c['code']}&output_format=cpa".encode()
                st, data, ms, _ = http(
                    "POST",
                    "/api/redeem",
                    form,
                    {"Content-Type": "application/x-www-form-urlencoded"},
                    timeout=180,
                )
                times.append((st, ms))
            ok = [ms for st, ms in times if st == 200]
            print(
                f"  redeem fc=1 x10 seq: ok={len(ok)} avg={statistics.mean(ok):.0f}ms p95={pct(ok,95):.0f}ms max={max(ok):.0f}ms"
                if ok
                else f"  redeem fc=1 seq failed: {times[:3]}"
            )

            # concurrent distinct codes
            codes = [c["code"] for c in ones[10:18]]

            def redeem_code(code: str):
                form = f"card_code={code}&output_format=cpa".encode()
                st, data, ms, _ = http(
                    "POST",
                    "/api/redeem",
                    form,
                    {"Content-Type": "application/x-www-form-urlencoded"},
                    timeout=180,
                )
                return st, ms

            t0 = time.perf_counter()
            with ThreadPoolExecutor(max_workers=8) as ex:
                results = list(ex.map(redeem_code, codes))
            wall = (time.perf_counter() - t0) * 1000
            ok = [ms for st, ms in results if st == 200]
            bad = [st for st, _ in results if st != 200]
            print(
                f"  redeem fc=1 x8 concurrent: ok={len(ok)} bad={bad} wall={wall:.0f}ms "
                f"avg_ok={statistics.mean(ok):.0f}ms" if ok else f"  redeem concurrent failed {results}"
            )

        # final snapshot
        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        print("\n== final DB ==")
        for t in ["files", "cards", "redemptions"]:
            print(f"  {t}:", conn.execute(f"select count(1) from {t}").fetchone()[0])
        print("  files:", conn.execute("select status,count(1) from files group by status").fetchall())
        print("  cards:", conn.execute("select status,count(1) from cards group by status").fetchall())
        dbp = tmp / "data" / "tikawang.db"
        print(f"  db size: {dbp.stat().st_size/1024/1024:.1f} MB")
        conn.close()
        st, data, ms, _ = http("GET", "/api/inventory")
        print(f"  inventory API {ms:.1f}ms: {data.decode()[:200]}")
        print("\nDONE")
        return 0
    finally:
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=5)
        except Exception:
            proc.kill()
        print(f"temp kept: {tmp}")
        print(f"log: {log_path}")


if __name__ == "__main__":
    raise SystemExit(main())
