#!/usr/bin/env python3
"""Scale test with ~100k cards / files on an isolated temp DB."""

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
PASSWORD = "stress-100k-password"
SESSION = "stress-100k-session-secret"
PORT = "18801"
BASE = f"http://127.0.0.1:{PORT}"

FILE_COUNT = 100_000
CARD_COUNT = 100_000


def http(method: str, path: str, body: bytes | None = None, headers: dict | None = None, timeout: float = 900.0):
    req = urllib.request.Request(BASE + path, data=body, method=method, headers=headers or {})
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = resp.read()
            return resp.status, data, (time.perf_counter() - t0) * 1000, dict(resp.headers)
    except urllib.error.HTTPError as e:
        data = e.read()
        return e.code, data, (time.perf_counter() - t0) * 1000, dict(e.headers)


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


def bench(name: str, fn, n: int = 10, concurrency: int = 1):
    samples: list[tuple[int, float, int]] = []
    t0 = time.perf_counter()
    if concurrency <= 1:
        for _ in range(n):
            st, ms, nbytes = fn()
            samples.append((st, ms, nbytes))
    else:
        with ThreadPoolExecutor(max_workers=concurrency) as ex:
            futs = [ex.submit(fn) for _ in range(n)]
            for f in as_completed(futs):
                try:
                    samples.append(f.result())
                except Exception as e:
                    samples.append((599, 0.0, 0))
                    print(f"  err: {e}")
    wall = (time.perf_counter() - t0) * 1000
    ok = [(ms, nb) for st, ms, nb in samples if 200 <= st < 300]
    bad = [st for st, _, _ in samples if not (200 <= st < 300)]
    print(f"\n== {name} ==")
    if not ok:
        print(f"  ALL FAILED bad={bad[:5]}")
        return
    times = [ms for ms, _ in ok]
    sizes = [nb for _, nb in ok]
    rps = len(ok) / (wall / 1000.0) if wall > 0 else 0
    print(
        f"  ok={len(ok)} err={len(bad)} "
        f"avg={statistics.mean(times):.1f}ms p50={pct(times,50):.1f}ms "
        f"p95={pct(times,95):.1f}ms p99={pct(times,99):.1f}ms "
        f"min={min(times):.1f} max={max(times):.1f} "
        f"avg_bytes={int(statistics.mean(sizes))} wall={wall:.0f}ms ~{rps:.1f}rps"
    )


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
    # speed pragmas for seed only
    cur.execute("PRAGMA journal_mode=WAL")
    cur.execute("PRAGMA synchronous=OFF")
    cur.execute("PRAGMA temp_store=MEMORY")
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

    print(f"seeding {count} files...")
    t0 = time.perf_counter()
    batch = []
    now = time.strftime("%Y-%m-%d %H:%M:%S")
    # tiny shared payload body to reduce disk for stress; path still unique
    payload = json.dumps({"email": "x@y.z", "access_token": "a.b.c", "tokens": {"access_token": "a.b.c"}})
    for i in range(count):
        name = f"f{i:06d}@ex.com"
        path = upload / f"{i:06d}_{name}"
        if i % 2000 == 0 or not path.exists():
            path.write_text(payload, encoding="utf-8")
        else:
            # reuse content file to save disk: hardlink-like copy via same content write every 1
            path.write_text(payload, encoding="utf-8")
        batch.append((name, str(path), now, now, space_id, "available"))
        if len(batch) >= 2000:
            cur.executemany(
                "insert into files(original_name, stored_path, generated_at, uploaded_at, space_id, status) values (?,?,?,?,?,?)",
                batch,
            )
            conn.commit()
            batch.clear()
            if (i + 1) % 20000 == 0:
                print(f"  files {i+1}/{count} ({(time.perf_counter()-t0):.1f}s)")
    if batch:
        cur.executemany(
            "insert into files(original_name, stored_path, generated_at, uploaded_at, space_id, status) values (?,?,?,?,?,?)",
            batch,
        )
        conn.commit()
    n = cur.execute("select count(1) from files where status='available'").fetchone()[0]
    conn.close()
    print(f"  files done available={n} in {(time.perf_counter()-t0):.1f}s")
    return n


def ensure_bin():
    if SERVER_BIN.exists():
        return
    subprocess.check_call(
        ["bash", "-lc", f"export PATH=/usr/local/go/bin:/usr/bin:/bin; cd {ROOT}/server && go build -o bin/faka-server ./cmd/server"]
    )


def main() -> int:
    ensure_bin()
    tmp = Path(tempfile.mkdtemp(prefix="faka-100k-"))
    (tmp / "data").mkdir()
    (tmp / "storage" / "uploads").mkdir(parents=True)
    (tmp / "storage" / "downloads").mkdir(parents=True)
    (tmp / "web" / "dist").mkdir(parents=True)
    (tmp / "web" / "dist" / "index.html").write_text("<html>100k</html>", encoding="utf-8")
    log_path = tmp / "server.log"
    print(f"temp: {tmp}")

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
    proc = subprocess.Popen([str(SERVER_BIN)], cwd=str(tmp), env=env, stdout=open(log_path, "w"), stderr=subprocess.STDOUT)
    try:
        for i in range(100):
            try:
                st, _, ms, _ = http("GET", "/api/inventory")
                if st == 200:
                    print(f"ready attempt={i+1} inventory={ms:.1f}ms")
                    break
            except Exception:
                time.sleep(0.1)
        else:
            print(log_path.read_text()[-3000:])
            return 1

        seed_files(tmp, FILE_COUNT)

        # create 100k cards in chunks of 10k (API max)
        print("\n== create 100k cards (10 x 10000) ==")
        t0 = time.perf_counter()
        total = 0
        for i in range(CARD_COUNT // 10000):
            body = json.dumps({"file_count": 1, "quantity": 10000}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards", body, auth(), timeout=900)
            j = json.loads(data.decode() or "{}")
            total += int(j.get("count") or 0)
            print(f"  batch {i+1}/10: http={st} {ms:.0f}ms count={j.get('count')} ids={len(j.get('ids') or [])}")
        print(f"  total cards created={total} wall={(time.perf_counter()-t0):.1f}s")

        # multi-file cards for redeem
        print("\n== multi-file cards ==")
        for fc, qty in [(10, 20), (100, 10), (500, 4), (1000, 2), (5000, 1)]:
            body = json.dumps({"file_count": fc, "quantity": qty}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards", body, auth(), timeout=300)
            j = json.loads(data.decode() or "{}")
            print(f"  fc={fc} qty={qty}: http={st} {ms:.0f}ms count={j.get('count')}")

        # mark 20k pending for redeem pool
        st, data, ms, _ = http("GET", "/api/admin/cards?status=available&page_size=10000", headers=auth(False))
        # need multiple pages - get ids via sqlite faster
        import sqlite3

        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        ids = [r[0] for r in conn.execute("select id from cards where status='available' order by id asc limit 20000").fetchall()]
        conn.close()
        print(f"\nmarking {len(ids)} cards pending via API chunks...")
        t0 = time.perf_counter()
        for i in range(0, len(ids), 5000):
            chunk = ids[i : i + 5000]
            body = json.dumps({"ids": chunk, "target_status": "pending"}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards/status", body, auth(), timeout=300)
            print(f"  status chunk {i//5000+1}: http={st} {ms:.0f}ms size={len(chunk)}")
        print(f"  mark pending wall={(time.perf_counter()-t0):.1f}s")

        # ---- benches ----
        bench(
            "inventory seq x30",
            lambda: (lambda s, d, m, h: (s, m, len(d)))(*http("GET", "/api/inventory")),
            n=30,
        )
        bench(
            "inventory concurrent x80 c=20",
            lambda: (lambda s, d, m, h: (s, m, len(d)))(*http("GET", "/api/inventory")),
            n=80,
            concurrency=20,
        )

        for ps in [200, 2000, 10000]:
            p = f"/api/admin/cards?page=1&page_size={ps}"
            bench(
                f"cards list page_size={ps} x5",
                lambda path=p: (lambda s, d, m, h: (s, m, len(d)))(*http("GET", path, headers=auth(False))),
                n=5,
            )
        for ps in [200, 2000, 10000]:
            p = f"/api/admin/files?page=1&page_size={ps}"
            bench(
                f"files list page_size={ps} x5",
                lambda path=p: (lambda s, d, m, h: (s, m, len(d)))(*http("GET", path, headers=auth(False))),
                n=5,
            )

        # deep page (near end of 100k)
        for page in [1, 5, 10]:
            p = f"/api/admin/cards?page={page}&page_size=10000"
            st, data, ms, _ = http("GET", p, headers=auth(False), timeout=300)
            print(f"\n cards page={page} size=10000: http={st} {ms:.0f}ms bytes={len(data)}")

        def mixed():
            import random

            path = (
                f"/api/admin/cards?page={random.randint(1,5)}&page_size=200"
                if random.random() < 0.5
                else f"/api/admin/files?page={random.randint(1,5)}&page_size=200"
            )
            s, d, m, _ = http("GET", path, headers=auth(False))
            return s, m, len(d)

        bench("mixed list concurrent x40 c=10", mixed, n=40, concurrency=10)

        # bulk status 5000
        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        bulk_ids = [r[0] for r in conn.execute("select id from cards where status='pending' limit 5000").fetchall()]
        conn.close()
        if bulk_ids:
            body = json.dumps({"ids": bulk_ids, "target_status": "pending"}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards/status", body, auth(), timeout=300)
            print(f"\n bulk status 5000: http={st} {ms:.0f}ms")

        # download 1000 cards (available)
        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        dl_ids = [r[0] for r in conn.execute("select id from cards where status='available' order by id desc limit 1000").fetchall()]
        conn.close()
        if dl_ids:
            body = json.dumps({"ids": dl_ids, "mark_pending": True}).encode()
            st, data, ms, _ = http("POST", "/api/admin/cards/download", body, auth(), timeout=300)
            print(f" cards/download 1000: http={st} {ms:.0f}ms bytes={len(data)}")

        # redeem scale
        print("\n== redeem ==")
        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        for fc in [1, 10, 100, 500, 1000, 5000]:
            row = conn.execute(
                "select code from cards where status='pending' and file_count=? order by id asc limit 1",
                (fc,),
            ).fetchone()
            avail = conn.execute("select count(1) from files where status='available'").fetchone()[0]
            if not row:
                print(f"  fc={fc}: no pending card")
                continue
            if avail < fc:
                print(f"  fc={fc}: skip available={avail}")
                continue
            code = row[0]
            form = f"card_code={code}&output_format=cpa".encode()
            st, data, ms, _ = http(
                "POST",
                "/api/redeem",
                form,
                {"Content-Type": "application/x-www-form-urlencoded"},
                timeout=900,
            )
            print(f"  redeem fc={fc}: http={st} {ms:.0f}ms bytes={len(data)} avail_before={avail}")
        # concurrent redeem fc=1
        codes = [r[0] for r in conn.execute("select code from cards where status='pending' and file_count=1 limit 20").fetchall()]
        conn.close()
        if len(codes) >= 16:
            def one(code: str):
                form = f"card_code={code}&output_format=cpa".encode()
                st, data, ms, _ = http(
                    "POST",
                    "/api/redeem",
                    form,
                    {"Content-Type": "application/x-www-form-urlencoded"},
                    timeout=180,
                )
                return st, ms, len(data)

            t0 = time.perf_counter()
            with ThreadPoolExecutor(max_workers=8) as ex:
                results = list(ex.map(one, codes[:8]))
            wall = (time.perf_counter() - t0) * 1000
            ok = [ms for st, ms, _ in results if st == 200]
            bad = [st for st, _, _ in results if st != 200]
            print(
                f"  redeem fc=1 x8 concurrent: ok={len(ok)} bad={bad} wall={wall:.0f}ms avg={statistics.mean(ok):.0f}ms"
                if ok
                else f"  concurrent redeem failed {results}"
            )

        # final
        conn = sqlite3.connect(str(tmp / "data" / "tikawang.db"))
        print("\n== final ==")
        for t in ["files", "cards", "redemptions"]:
            print(f"  {t}:", conn.execute(f"select count(1) from {t}").fetchone()[0])
        print("  files:", conn.execute("select status,count(1) from files group by status").fetchall())
        print("  cards:", conn.execute("select status,count(1) from cards group by status").fetchall())
        dbp = tmp / "data" / "tikawang.db"
        print(f"  db: {dbp.stat().st_size/1024/1024:.1f} MB")
        # index list
        print("  indexes:", [r[0] for r in conn.execute("select name from sqlite_master where type='index'").fetchall()][:20], "...")
        conn.close()
        st, data, ms, _ = http("GET", "/api/inventory")
        print(f"  inventory {ms:.1f}ms {data.decode()[:180]}")
        print("DONE")
        return 0
    finally:
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=8)
        except Exception:
            proc.kill()
        print(f"temp: {tmp}")
        print(f"log: {log_path}")


if __name__ == "__main__":
    raise SystemExit(main())
