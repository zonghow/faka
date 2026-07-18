# Cards & Files API

规格：`admin.openapi.yaml`

**鉴权：每个请求带后台密码**（无需 login / session / spaces）

```http
X-Admin-Password: YOUR_PASSWORD
```

或：

```http
Authorization: Bearer YOUR_PASSWORD
```

## 覆盖接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/cards` | 列卡密 |
| POST | `/api/admin/cards` | 生成卡密 |
| POST | `/api/admin/cards/status` | 改卡密状态 |
| POST | `/api/admin/cards/download` | 下载卡密 txt |
| GET | `/api/admin/cards/{id}/redemptions` | 兑换记录 |
| GET | `/api/admin/files` | 列文件 |
| POST | `/api/admin/files/upload` | 上传 json/zip |
| POST | `/api/admin/files/status` | 改文件状态 |
| POST | `/api/admin/files/download` | 下载文件 |

## 脚本示例

```bash
BASE=https://key.whistlelads.com
PASS='YOUR_PASSWORD'
H=(-H "X-Admin-Password: $PASS")

# 1) 上传
curl -sS "${H[@]}" -F 'file=@./a.json' -F 'file=@./b.json' \
  "$BASE/api/admin/files/upload"

# 2) 生成 50 张卡（每张 1 文件）— 响应含 ids + codes
CREATE=$(curl -sS "${H[@]}" -H 'Content-Type: application/json' \
  -d '{"file_count":1,"quantity":50}' \
  "$BASE/api/admin/cards")
echo "$CREATE" | jq '{count, ids, codes}'

# 3) 用返回的 ids 下载并标为 pending
IDS=$(echo "$CREATE" | jq -c '.ids')
curl -sS "${H[@]}" -H 'Content-Type: application/json' \
  -d "{\"ids\":$IDS,\"mark_pending\":true}" \
  -o cards.txt \
  "$BASE/api/admin/cards/download"

# 4) 改状态示例
curl -sS "${H[@]}" -H 'Content-Type: application/json' \
  -d "{\"ids\":$IDS,\"target_status\":\"voided\"}" \
  "$BASE/api/admin/cards/status"
```

## 上传 zip → 按新增数生成 → 下载并 pending

```bash
UP=$(curl -sS "${H[@]}" -F 'file=@batch.zip' "$BASE/api/admin/files/upload")
N=$(echo "$UP" | jq .created)

CREATE=$(curl -sS "${H[@]}" -H 'Content-Type: application/json' \
  -d "{\"file_count\":1,\"quantity\":$N}" \
  "$BASE/api/admin/cards")
IDS=$(echo "$CREATE" | jq -c '.ids')

curl -sS "${H[@]}" -H 'Content-Type: application/json' \
  -d "{\"ids\":$IDS,\"mark_pending\":true}" \
  -o cards.txt \
  "$BASE/api/admin/cards/download"
```
