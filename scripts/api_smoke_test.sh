#!/usr/bin/env bash
set -euo pipefail

BASE="${1:-http://127.0.0.1:18743}"
PASS="${TIKAWANG_AUTH_PASSWORD:-Wgs0405java}"
COOKIE_JAR="$(mktemp)"
TMPDIR="$(mktemp -d)"
cleanup() { rm -f "$COOKIE_JAR" /tmp/faka_card_code /tmp/faka_card_ids; rm -rf "$TMPDIR"; }
trap cleanup EXIT

json_get() {
  python3 -c 'import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1]))' "$1"
}

echo "== inventory =="
curl -fsS "$BASE/api/inventory" | tee "$TMPDIR/inventory.json" >/dev/null

echo "== login =="
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"password\":\"$PASS\"}" \
  "$BASE/api/auth/login" | tee "$TMPDIR/login.json" >/dev/null

echo "== me =="
curl -fsS -b "$COOKIE_JAR" "$BASE/api/auth/me" | tee "$TMPDIR/me.json" >/dev/null

echo "== spaces =="
curl -fsS -b "$COOKIE_JAR" "$BASE/api/admin/spaces" | tee "$TMPDIR/spaces.json" >/dev/null
SPACE_ID=$(python3 -c 'import json; d=json.load(open("'"$TMPDIR/spaces.json"'")); print(d["spaces"][0]["id"])')
echo "space=$SPACE_ID"
printf '127.0.0.1\tFALSE\t/\tFALSE\t0\ttikawang_space_id\t%s\n' "$SPACE_ID" >> "$COOKIE_JAR"

echo "== dashboard =="
curl -fsS -b "$COOKIE_JAR" "$BASE/api/admin/dashboard" | tee "$TMPDIR/dash.json" >/dev/null

echo "== create upload json =="
TS=$(date +%s)
printf '%s\n' "{\"access_token\":\"aaa.bbb.ccc\",\"email\":\"smoke${TS}@example.com\",\"tokens\":{\"access_token\":\"aaa.bbb.ccc\"}}" > "$TMPDIR/a.json"
printf '%s\n' "{\"access_token\":\"ddd.eee.fff\",\"email\":\"smoke2${TS}@example.com\",\"tokens\":{\"access_token\":\"ddd.eee.fff\"}}" > "$TMPDIR/b.json"

echo "== upload =="
curl -fsS -b "$COOKIE_JAR" -F "file=@$TMPDIR/a.json;filename=smoke_${TS}_a.json" -F "file=@$TMPDIR/b.json;filename=smoke_${TS}_b.json" \
  "$BASE/api/admin/files/upload" | tee "$TMPDIR/upload.json" >/dev/null
python3 -c 'import json; d=json.load(open("'"$TMPDIR/upload.json"'")); assert d.get("ok") is not False; print(d.get("message"), "created=", d.get("created"), "duplicated=", d.get("duplicated"))'

echo "== files list =="
curl -fsS -b "$COOKIE_JAR" "$BASE/api/admin/files?page=1&page_size=50" | tee "$TMPDIR/files.json" >/dev/null

echo "== create cards =="
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"file_count":1,"quantity":2}' \
  "$BASE/api/admin/cards" | tee "$TMPDIR/create_cards.json" >/dev/null

echo "== cards list =="
curl -fsS -b "$COOKIE_JAR" "$BASE/api/admin/cards?page=1&page_size=50&status=available" | tee "$TMPDIR/cards.json" >/dev/null
python3 - <<PY
import json
d=json.load(open("$TMPDIR/cards.json"))
cards=d["cards"]
assert len(cards) >= 1, "no available cards"
ids=[c["id"] for c in cards[:2]]
open("/tmp/faka_card_code","w").write(cards[0]["code"])
open("/tmp/faka_card_ids","w").write(",".join(str(i) for i in ids))
print("card", cards[0]["code"], "ids", ids)
PY
CARD_CODE=$(cat /tmp/faka_card_code)
IDS_JSON=$(python3 -c 'ids=open("/tmp/faka_card_ids").read().split(","); print("["+",".join(ids)+"]")')

echo "== mark pending =="
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"ids\":$IDS_JSON,\"target_status\":\"pending\"}" \
  "$BASE/api/admin/cards/status" | tee "$TMPDIR/card_status.json" >/dev/null

echo "== redeem cpa =="
curl -fsS -o "$TMPDIR/redeem.zip" -D "$TMPDIR/redeem.headers" \
  -F "card_code=$CARD_CODE" -F "output_format=cpa" \
  "$BASE/api/redeem"
test -s "$TMPDIR/redeem.zip"
echo "redeem bytes=$(wc -c < "$TMPDIR/redeem.zip")"

echo "== card redemptions =="
CARD_ID=$(python3 -c 'print(open("/tmp/faka_card_ids").read().split(",")[0])')
curl -fsS -b "$COOKIE_JAR" "$BASE/api/admin/cards/$CARD_ID/redemptions" | tee "$TMPDIR/redemptions.json" >/dev/null

echo "== logout =="
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{}' "$BASE/api/auth/logout" | tee "$TMPDIR/logout.json" >/dev/null

echo "ALL SMOKE TESTS PASSED"
