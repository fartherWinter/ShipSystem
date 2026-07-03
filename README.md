# 鑸硅埗绠＄悊涓庣洃鎺ц皟搴︾郴缁?
涓夋寮?MVP锛欸o 涓诲悗绔€丳ython 鍒嗘瀽/妯℃嫙鏈嶅姟銆丷eact 绠＄悊鍓嶇銆?
## 鎶€鏈爤

- 鍚庣锛欸o銆丟in銆丟ORM銆丳ostgreSQL/PostGIS銆丣WT銆乄ebSocket
- 鍒嗘瀽鏈嶅姟锛歅ython銆丗astAPI銆乭ttpx
- 鍓嶇锛歊eact銆乂ite銆乀ypeScript銆丄nt Design銆丱penLayers
- 鏈湴缂栨帓锛欴ocker Compose

## 蹇€熷惎鍔?
```bash
cp .env.example .env
python scripts/runtime_precheck.py
docker compose up --build
```

璁块棶锛?
- 鍓嶇锛歨ttp://localhost:3000
- Go API锛歨ttp://localhost:8080/api/v1
- Python 鍒嗘瀽鏈嶅姟锛歨ttp://localhost:8090

榛樿璐﹀彿锛?
- 鐢ㄦ埛鍚嶏細`admin`
- 瀵嗙爜锛歚Admin123!`

## 鏈湴寮€鍙?
鍚庣锛?
```bash
cd backend
go mod tidy
go run ./cmd/api
```

Python锛?
```bash
cd analytics
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8090
```

鍓嶇锛?
```bash
cd frontend
npm install
npm run dev
```

## 閰嶇疆涓庢暟鎹簱杩佺Щ

- `APP_ENV=development` 榛樿鍏佽 `DATABASE_AUTO_MIGRATE=true` 鍜?`SEED_DEMO_DATA=true`锛屾柟渚挎湰鍦板揩閫熷惎鍔ㄣ€?- `APP_ENV=production` 蹇呴』璁剧疆 `DATABASE_AUTO_MIGRATE=false`銆乣SEED_DEMO_DATA=false`锛屽苟鏇挎崲寮?`JWT_SECRET` / `ADMIN_PASSWORD`銆?- Python 浠跨湡鎺у埗鎺ュ彛鐢?Go API 浠ｇ悊璋冪敤锛岀敓浜х幆澧冨繀椤婚厤缃悗绔寔鏈夌殑 `ANALYTICS_ADMIN_TOKEN`锛沘nalytics 鍥炶皟 Go API 蹇呴』閰嶇疆涓ょ涓€鑷寸殑 `GO_API_TOKEN`锛屽墠绔笉鐩存帴鎸佹湁杩欎簺 token銆?- `GO_API_TOKEN` 鍙綔涓?analytics 鍒?Go API 鐨勬湇鍔＄ Bearer token 浣跨敤锛岀洰鍓嶄粎鎺堜簣浣嶇疆涓婃姤鍜岄浄杈炬垬鏂椾笂鎶ュ叆鍙ｆ潈闄愶紝涓嶆巿浜?RBAC銆佽皟搴︾姸鎬佹垨 analytics 鎺у埗鎺ュ彛鏉冮檺銆?- 娴忚鍣ㄧ櫥褰曞悗鐢?Go API 璁剧疆 `shipsystem_token` HttpOnly Cookie锛汻EST 涓?WebSocket 榛樿浣跨敤 Cookie 閴存潈锛屽悗绔粛鍏煎 `Authorization: Bearer <JWT>` 渚夸簬鏈嶅姟绔垨璋冭瘯宸ュ叿璋冪敤銆?- `APP_ENV=production` 鏃惰璇?Cookie 浼氬己鍒惰缃?`Secure`锛涜嫢 TLS 鍦?Nginx/Ingress 灞傜粓姝紝鍙嶅悜浠ｇ悊闇€瑕佷紶閫?`X-Forwarded-Proto: https`銆?- Go API 鍜屽墠绔?Nginx 鍧囪缃熀纭€瀹夊叏鍝嶅簲澶达紱鍓嶇鍚敤 CSP锛岄粯璁ゅ彧鍏佽鑷韩鑴氭湰銆佹牱寮忓拰蹇呰鍦板浘鐡︾墖/鍚屾簮杩炴帴銆?- Go API 浣跨敤鏄惧紡 HTTP Server 瓒呮椂閰嶇疆锛屽苟鍦?SIGINT/SIGTERM 涓嬫墽琛屼紭闆呭叧闂€?- Go API 涓烘瘡涓姹傜敓鎴愭垨閫忎紶 `X-Request-ID`锛岃闂棩蹇椼€佷笟鍔￠敊璇?JSON 鍜?panic recovery 500 鍝嶅簲閮芥惡甯﹀悓涓€ requestId锛汫o 璋冪敤 Python analytics 鏃朵細缁х画杞彂璇ヨ姹?ID锛宎nalytics 鑷韩閿欒 JSON 涓庡洖璋?Go API 鏃朵篃浼氬甫鍚屼竴涓姹?ID锛涘墠绔敊璇彁绀轰細闄勫甫璇ヨ姹?ID锛屼究浜庢帓鏌ャ€?- 琛ㄧ粨鏋勮縼绉荤敱 Go 搴旂敤鍐呭祵鐨?`backend/internal/database/migrations/*.sql` 鎵ц锛屾墽琛岃褰曞啓鍏?`schema_migrations`銆?- 褰?`DATABASE_AUTO_MIGRATE=false` 鏃讹紝Go API 鍚姩鍙細鏍￠獙 `schema_migrations` 鏄惁宸茶鐩栧叏閮ㄥ唴宓?migration锛涜嫢浠嶆湁 pending migration锛屼細鎷掔粷鍚姩骞舵彁绀哄厛鎵ц `go run ./cmd/migrate -action=up`銆?- 瑙掕壊銆佽彍鍗曠瓑浼氬奖鍝嶅瓨閲忕幆澧冭涓虹殑鍩虹鏁版嵁鍙樻洿锛屼篃蹇呴』閫氳繃鏂扮殑鍚戝墠杩佺Щ琛ラ綈锛沗seed` 鍙繚璇佹柊搴撳垵濮嬪寲锛屼笉鏇夸唬鍙戝竷杩佺Щ銆?- `backend/migrations/001_init.sql` 浠呯敤浜?Docker Compose 棣栨鍒濆鍖?PostgreSQL 鏃跺惎鐢?PostGIS銆?
杩佺Щ鍛戒护锛?
```bash
cd backend
go run ./cmd/migrate -action=status
go run ./cmd/migrate -action=check
go run ./cmd/migrate -action=up
```

褰撳墠杩佺Щ绛栫暐鍙彁渚涘悜鍓嶈縼绉伙紝涓嶆彁渚涜嚜鍔?down 鍥炴粴锛涚敓浜у洖婊氬簲閫氳繃鏄庣‘鐨勪慨澶嶈縼绉绘垨鐗堟湰鍖栧彂甯冨洖閫€瀹屾垚銆?
## 鐢熶骇绾у垎闃舵鏁存敼璺嚎

### 闃舵 1锛氬畨鍏ㄤ笌杩愯鍩虹嚎

鐩爣锛氳椤圭洰鍏堝叿澶囩敓浜х幆澧冨彲鍚姩銆佸彲鎷掔粷鍗遍櫓閰嶇疆銆佸彲瀹¤鍩虹杈圭晫銆?
- 鐢熶骇鐜绂佹榛樿 `JWT_SECRET`銆侀粯璁ょ鐞嗗憳瀵嗙爜銆侀€氶厤 CORS銆佽嚜鍔ㄥ缓琛ㄥ拰婕旂ず鏁版嵁銆?- Go API 涓?Python analytics 浣跨敤鏈嶅姟绔?token 閫氫俊锛屽墠绔笉淇濆瓨 analytics 绠＄悊 token锛岀敓浜х幆澧冨繀椤诲悓鏃堕厤缃?`ANALYTICS_ADMIN_TOKEN` 鍜?`GO_API_TOKEN`銆?- 娴忚鍣ㄤ紭鍏堜娇鐢?HttpOnly Cookie 淇濆瓨鐧诲綍鎬侊紝鍏煎 `Authorization: Bearer` 渚涙湇鍔＄鍜岃皟璇曞伐鍏蜂娇鐢ㄣ€?- 琛ラ綈鍩虹瀹夊叏鍝嶅簲澶淬€丯ginx CSP銆乄ebSocket 閴存潈鍜屽績璺炽€?
### 闃舵 2锛氭潈闄愩€佹牎楠屼笌涓氬姟鐘舵€?
鐩爣锛氭妸 MVP 鐨勨€滆兘璋冪敤鈥濇彁鍗囦负鈥滄寜瑙掕壊銆佹寜鐘舵€併€佹寜杈撳叆杈圭晫姝ｇ‘璋冪敤鈥濄€?
- 鎸?`super_admin`銆乣admin`銆乣dispatcher`銆乣viewer` 寤虹珛鍚庣璺敱鏉冮檺杈圭晫銆?- URL ID銆佸潗鏍囥€佽埅鍚戙€侀€熷害銆佹椂闂寸瓑杈撳叆蹇呴』鏄惧紡鏍￠獙锛岄潪娉曡姹傝繑鍥?400銆?- 鍛婅纭淇濇寔骞傜瓑锛岃皟搴︿簨浠剁姸鎬佹祦杞繀椤荤鍚堜笟鍔￠棴鐜€?- 鍓嶇鑿滃崟鍜岄〉闈㈣闂寜瑙掕壊鏀舵暃锛岄伩鍏嶅彧鍋?UI 闅愯棌銆佷笉鍋氬悗绔嫤鎴€?
### 闃舵 3锛氭暟鎹簱娌荤悊涓庡彂甯冩祦绋?
鐩爣锛氭妸寮€鍙戞湡 `AutoMigrate` 鏀归€犳垚鍙拷韪€佸彲楠屾敹銆佸彲鍙戝竷鐨勮縼绉绘祦绋嬨€?
- 琛ㄧ粨鏋勪互 `backend/internal/database/migrations/*.sql` 涓哄熀绾匡紝鎵ц鐘舵€佸啓鍏?`schema_migrations`銆?- 鐢熶骇鐜鍏抽棴 `DATABASE_AUTO_MIGRATE`锛屽彂甯冩椂鏄惧紡鎵ц `cmd/migrate`銆?- 鏂板瓧娈点€佹柊绱㈠紩銆佹柊琛ㄥ繀椤婚€氳繃鏂板杩佺Щ鏂囦欢钀藉湴锛屽苟琛ュ厖鏈€灏忔暟鎹吋瀹硅鏄庛€?- 鑿滃崟銆佽鑹茬瓑鍩虹鏁版嵁鏂板鎴栦慨姝ｅ繀椤诲悓鏃惰鐩?fresh seed 鍜屽瓨閲忓簱 forward migration銆?- 鍥炴粴涓嶈嚜鍔ㄧ敓鎴?down SQL锛屼紭鍏堜娇鐢ㄥ彲瀹¤鐨勫悜鍓嶄慨澶嶈縼绉汇€?
### 闃舵 4锛氬彲闈犳€с€佸彲瑙傛祴鎬т笌閾捐矾楠岃瘉

鐩爣锛氳 Go銆丳ython銆丷eact 涓夋閾捐矾鍑虹幇澶辫触鏃跺彲鎭㈠銆佸彲瀹氫綅銆佸彲楠岃瘉銆?
- analytics 鍥炶皟 Go API 闇€瑕佽秴鏃躲€侀噸璇曞拰鏃ュ織锛屼豢鐪熸湇鍔℃毚闇茬姸鎬佹鏌ユ帴鍙ｃ€?- WebSocket 骞挎挱閬垮厤鎱㈠鎴风闃诲锛岃繛鎺ラ渶瑕?deadline銆乸ing/pong 鍜屽紓甯告竻鐞嗐€?- 琛ュ厖绔埌绔?smoke check锛氱櫥褰曘€佽埞鑸跺垪琛ㄣ€侀浄杈句笂鎶ャ€佹垬鏂椾豢鐪熴€佸洖鏀?鎴樻姤銆乄ebSocket 瀹炴椂鎺ㄩ€併€?- Docker 闀滃儚閫愭琛ラ綈 healthcheck銆侀潪 root 杩愯銆佹棩蹇楀瓧娈靛拰璧勬簮闄愬埗銆?
### 闃舵 5锛氬墠绔伐绋嬪寲涓庢€ц兘

鐩爣锛氳鍓嶇浠庢紨绀洪〉闈㈡彁鍗囦负鍙淮鎶ょ殑涓氬姟鎺у埗鍙般€?
- 浣跨敤鐪熷疄璺敱銆佺櫥褰曞畧鍗拰瑙掕壊鑿滃崟锛岄伩鍏嶆墍鏈夐〉闈㈠爢鍦ㄥ崟缁勪欢閲屻€?- 瀵瑰湴鍥俱€丄nt Design銆丱penLayers 鍋氭寜闇€鍔犺浇鍜?chunk 鎷嗗垎锛屾寔缁帇浣庨灞忓寘浣撱€?- 寤虹珛椤甸潰绾?smoke 娴嬭瘯锛岃鐩栫櫥褰曟€併€佷富瀵艰埅銆佹牳蹇冭〃鏍煎拰鍦板浘瀹炴椂鍒锋柊銆?- UI 鐘舵€侀渶瑕佽鐩栧姞杞姐€佺┖鏁版嵁銆侀敊璇€佹棤鏉冮檺鍜岄噸璇曘€?
鎺ㄨ崘姣忎釜闃舵鐨勬渶灏忛獙鏀跺懡浠わ細

```bash
python scripts/preflight_check.py
```

璇ュ懡浠や細椤哄簭鎵ц Go 鍚庣娴嬭瘯銆丳ython analytics 娴嬭瘯銆佽剼鏈崟鍏冩祴璇曘€佸墠绔敓浜ф瀯寤哄拰 Compose 闈欐€佸熀绾挎鏌ャ€傞渶瑕佸崟鐙帓閿欐椂锛屼篃鍙互鍒嗘鎵ц锛?
```bash
cd backend
go test ./...
```

```bash
cd analytics
uv run --with-requirements requirements.txt python -m unittest discover -s tests
```

```bash
cd frontend
npm run build
npm run test:e2e
```

```bash
python scripts/preflight_check.py --only frontend-e2e
python scripts/collect_release_evidence.py --skip-migration-status --include-frontend-e2e --output-dir .release-evidence/latest-frontend-e2e
```

```bash
python -m unittest discover -s scripts_tests
python scripts/check_compose_config.py
python scripts/check_event_contract.py
python scripts/check_callback_contract.py
python scripts/check_openapi_contract.py
python scripts/check_frontend_api_contract.py
python scripts/check_rbac_matrix.py
```

闇€瑕侀獙璇佷粨鍌ㄥ眰浜嬪姟骞跺彂鍜屽箓绛夋椂锛屽彲鐩存帴鎷夎捣涓存椂 PostGIS 娴嬭瘯搴撳苟鎵ц浠撳偍灞傞泦鎴愭祴璇曪細
```bash
python scripts/run_repository_db_integration.py
```

璇ヨ剼鏈細鑷姩鐢熸垚涓存椂 Compose 鏂囦欢銆侀€夋嫨绌洪棽鏈湴绔彛銆佸惎鍔ㄧ嫭绔?PostGIS 瀹瑰櫒銆佽缃?`SHIPSYSTEM_REPOSITORY_TEST_DSN`銆佹墽琛?`go test ./internal/repositories`锛屽畬鎴愬悗榛樿鑷姩娓呯悊瀹瑰櫒鍜屽嵎銆傝嫢鍙兂澶嶇敤宸叉湁鍙涪寮冩暟鎹簱锛屼篃鍙互鎵嬪伐鎸囧畾 DSN锛?```bash
set SHIPSYSTEM_REPOSITORY_TEST_DSN=host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test port=5432 sslmode=disable TimeZone=Asia/Shanghai
python scripts/preflight_check.py --only db-integration
```

涔熷彲浠ュ湪瀹屾暣 preflight 涓拷鍔犺寮烘牎楠岋細
```bash
python scripts/preflight_check.py --include-db-integration
```

闇€瑕侀瑙堝巻鍙茶建杩广€佸凡纭鍛婅銆佸凡鍏抽棴璋冨害浜嬩欢鍜屽凡鍋滄瀵规垬鍥炴斁鏁版嵁鐨勭暀瀛樻竻鐞嗗奖鍝嶆椂锛屽彲鎵ц鍙棰勮锛?```bash
python scripts/retention_maintenance.py --days 30
```

鐪熸娓呯悊蹇呴』鏄惧紡杩藉姞 `--apply`锛屽彂甯冩垨鐢熶骇鎵ц鍓嶅簲鍏堝畬鎴愭暟鎹簱澶囦唤锛屽苟鎶婇瑙堣緭鍑洪殢鍙戝竷璇佹嵁褰掓。锛?```bash
python scripts/retention_maintenance.py --days 30 --max-track-points-per-ship 5000 --max-battle-snapshots-per-session 1000
python scripts/retention_maintenance.py --days 30 --max-track-points-per-ship 5000 --max-battle-snapshots-per-session 1000 --apply
```

涔熷彲浠ユ妸棰勮绾冲叆鏄惧紡鍙戝竷鍓嶆鏌ワ細
```bash
python scripts/preflight_check.py --only retention-preview
python scripts/preflight_check.py --include-retention-preview
```

鍏朵腑 `scripts/check_compose_config.py` 浼氬熀浜庢覆鏌撳悗鐨?Compose 閰嶇疆妫€鏌ユ湇鍔?healthcheck銆佸仴搴蜂緷璧栥€佹湇鍔＄ token 浼犻€掋€侀暅鍍忔爣绛惧浐瀹氥€丏ockerfile 鏈€缁堥樁娈甸潪 root 鐢ㄦ埛銆佹牴 `.dockerignore` 鐨勬晱鎰熸枃浠?鏋勫缓浜х墿鎺掗櫎锛屼互鍙婂墠绔?Nginx 瀹夊叏鍝嶅簲澶村拰 WebSocket 浠ｇ悊澶淬€?
`scripts/check_event_contract.py` 浼氭牎楠屽悗绔?WebSocket 骞挎挱浜嬩欢銆佸墠绔?`WsMessage` 绫诲瀷銆佸墠绔簨浠舵秷璐广€丷EADME 娑堟伅鍒楄〃鍜?smoke 鍏抽敭浜嬩欢鐩戝惉锛岄伩鍏嶆帴鍙ｄ簨浠跺绾︽紓绉汇€?
`scripts/check_callback_contract.py` 浼氭牎楠?Python analytics 闆疯揪瀵规垬鍥炶皟鏍锋湰涓?Go `RadarReportPayload` / 瀛?payload 鐨?JSON 瀛楁涓€鑷达紝閬垮厤浠跨湡鍣ㄥ拰 Go 鎺ユ敹妯″瀷瀛楁婕傜Щ銆?
`scripts/check_openapi_contract.py` 浼氭牎楠?`docs/openapi.yaml` 涓?Go handler 瀹為檯濂戠害涓€鑷达細涓嶄粎瑕嗙洊鍏ㄩ儴 REST 璺敱锛岃繕鏍￠獙鍙椾繚鎶ゆ帴鍙ｇ殑 `x-roles`銆乸ublic/protected 閴存潈澹版槑銆佸叧閿?requestBody銆佹垚鍔熺姸鎬佺爜浠ュ強甯歌 `400/401/403/404/500/502` 鍝嶅簲锛岄伩鍏?OpenAPI 鏂囨。涓庡悗绔涓洪潤榛樻紓绉汇€?
`scripts/check_frontend_api_contract.py` 浼氭牎楠屽墠绔?`frontend/src/api/client.ts` 涓殑 REST 璋冪敤閮借 `docs/openapi.yaml` 瑕嗙洊锛岄伩鍏嶅墠绔皟鐢ㄤ笉瀛樺湪鎴栨湭鏂囨。鍖栫殑鎺ュ彛銆?
`scripts/check_rbac_matrix.py` 浼氭牎楠?`docs/rbac_matrix.yaml` 涓殑 REST 鏉冮檺鐭╅樀涓?Go handler 瀹為檯娉ㄥ唽鐨?RBAC middleware 涓€鑷达紝鍚屾椂鏍￠獙鍓嶇椤甸潰瑙掕壊鐭╅樀涓?`frontend/src/App.tsx` 涓€鑷达紝閬垮厤鏉冮檺鍙樻洿婕忓璁°€?
`python -m unittest discover -s scripts_tests` 浼氱粰鍏抽敭鍙戝竷鍓?gate 鑴氭湰琛ヨ嚜娴嬶紝褰撳墠瑕嗙洊 OpenAPI 濂戠害瑙ｆ瀽銆乺untime precheck 璇婃柇鍒嗘敮銆乧ompose baseline 瑙ｆ瀽鏍￠獙銆乻moke gate 鐨勭函鍑芥暟鍜?cookie/requestId 鏈湴绾︽潫锛屼互鍙?frontend API / RBAC / WebSocket event / analytics callback 绛夐潤鎬佸绾﹁剼鏈殑鏍稿績瑙ｆ瀽閫昏緫锛岄伩鍏嶉棬绂佽剼鏈湰韬洖褰掑悗闈欓粯澶辨晥銆?
鍓嶇 `npm run build` 鐜板湪浼氬厛鎵ц `npm run test:static`锛屽 `frontend/scripts/check_frontend_routes.mjs` 鍜?`frontend/scripts/check_frontend_bundle.mjs` 鐨勬牳蹇冩鏌ラ€昏緫鍋?`node:test` 鍥炲綊锛屽啀缁х画 `tsc -b`銆佽矾鐢?gate銆乂ite build 鍜?bundle budget gate锛岄檷浣庡墠绔棬绂佽剼鏈湰韬紓绉诲悗闈欓粯澶辨晥鐨勯闄┿€?
鏇村畬鏁寸殑鍙戝竷鍓嶆鏌ャ€佽縼绉汇€佽繍琛屾€?smoke銆佸け璐ュ鐞嗗拰璇佹嵁褰掓。娴佺▼瑙?`docs/release_runbook.md`銆?
鍚姩鏈湴瀹屾暣鏍堝墠锛屽缓璁厛鎵ц杩愯鎬佸墠缃鏌ワ細

```bash
python scripts/runtime_precheck.py
```

璇ヨ剼鏈細妫€鏌?Docker CLI銆丏ocker Compose銆丏ocker Desktop Service 鐘舵€併€丏ocker daemon 璁块棶鏉冮檺銆丆ompose 閰嶇疆娓叉煋銆乻moke 鐩爣鍦板潃绔彛浠ュ強 `docker compose config` 瀹為檯鍙戝竷绔彛鍗犵敤銆傝剼鏈細浼樺厛浣跨敤宸ヤ綔鍖哄唴鐨?`.docker-codex` 浣滀负 `DOCKER_CONFIG`锛岄伩鍏嶆湰鏈虹敤鎴风洰褰?Docker 閰嶇疆鏉冮檺闂褰卞搷妫€鏌ョ粨鏋滐紱鍦?Windows 涓婅繕浼氱洿鎺ヨ緭鍑鸿鍗犵敤绔彛瀵瑰簲鐨?PID 鍜岃繘绋嬪悕锛屼究浜庡揩閫熷畾浣嶆槸璋佸崰鐢ㄤ簡 `3000/4173/5432/8080/8090` 杩欑被瀹為檯杩愯绔彛銆?
鑻ラ粯璁ょ鍙ｈ鍗犵敤锛屽彲鍏堢敓鎴愭湰鍦?override锛?
```bash
python scripts/generate_compose_local_override.py
python scripts/preflight_check.py --only compose-override
```

鐢熸垚鍚庝娇鐢ㄥ弻 Compose 鏂囦欢鍚姩锛?
```bash
docker compose -f docker-compose.yml -f .docker-codex/compose.smoke.override.yml up --build
python scripts/smoke_check.py
```

濡傞渶璁?smoke 鐩存帴鍛戒腑鏂扮鍙ｏ紝鎸夎剼鏈緭鍑鸿缃幆澧冨彉閲忥細

```bash
set SHIPSYSTEM_BACKEND_URL=http://127.0.0.1:18080
set SHIPSYSTEM_ANALYTICS_URL=http://127.0.0.1:18090
set SHIPSYSTEM_FRONTEND_URL=http://127.0.0.1:13000
set SHIPSYSTEM_WS_URL=ws://127.0.0.1:18080/ws/monitor
```


如需在调整留存阈值前先做 battle/radar 容量估算，可先执行纯估算模式：
```bash
python scripts/run_capacity_smoke.py --estimate-only
```

若完整栈已经启动，也可以对当前 battle/radar 链路做一次轻量真实采样，读取 timeline、snapshots 和 report 结果作为容量证据：
```bash
python scripts/run_capacity_smoke.py --track-counts 5,20 --ticks 6 --duration-seconds 30
```

鑴氭湰榛樿妫€鏌?`http://localhost:8080`銆乣http://localhost:8090`銆乣http://localhost:3000`锛屽苟浣跨敤 `.env.example` 涓殑鏈湴榛樿璐﹀彿銆傛鏌ヨ寖鍥村寘鎷湇鍔″仴搴枫€佺櫥褰?Cookie銆佹牳蹇冨垪琛ㄣ€乤nalytics 鐘舵€佷唬鐞嗐€佸垱寤哄鎴樹細璇濄€乄ebSocket 杩炴帴銆侀浄杈句笂鎶ャ€乄ebSocket 瀹炴椂鎺ㄩ€侊紙鑷冲皯楠岃瘉 `radar_scan_updated`銆乣projectile_updated`銆乣battle_event_created`銆乣battle_state_updated`锛夈€佹垬鏂楃姸鎬併€佹椂闂磋酱銆佸揩鐓с€佹垬鎶ュ拰閫€鍑?Cookie 娓呯悊銆傚彲閫氳繃 `SHIPSYSTEM_BACKEND_URL`銆乣SHIPSYSTEM_ANALYTICS_URL`銆乣SHIPSYSTEM_FRONTEND_URL`銆乣SHIPSYSTEM_WS_URL`銆乣SHIPSYSTEM_USERNAME`銆乣SHIPSYSTEM_PASSWORD` 瑕嗙洊銆傝剼鏈細涓?HTTP 璇锋眰鍐欏叆 `X-Request-ID`锛孒TTP 澶辫触鏃惰緭鍑虹姸鎬佺爜銆侀敊璇秷鎭€佸搷搴斾綋鎽樿鍜?requestId锛屼究浜庣洿鎺ュ洖鏌?Go/Python 鏃ュ織銆?
Go API 鎻愪緵涓や釜杩愯鎺㈤拡锛歚/health` 鍙〃绀鸿繘绋嬪瓨娲伙紝`/ready` 浼氶澶栨鏌ユ暟鎹簱杩炴帴锛汥ocker Compose 鐨勫悗绔?healthcheck 浣跨敤 `/ready`锛岄伩鍏嶄緷璧栨湇鍔″湪鏁版嵁搴撲笉鍙敤鏃惰繃鏃╁惎鍔ㄣ€?
## API 杈圭晫

REST 鍓嶇紑锛歚/api/v1`

鏈哄櫒鍙濂戠害瑙?`docs/openapi.yaml`锛屽苟鐢?`scripts/check_openapi_contract.py` 鏍￠獙璺敱銆丷BAC 瑙掕壊澹版槑銆侀壌鏉冭竟鐣屻€佸叧閿姹備綋鍜屽父瑙侀敊璇搷搴旇鐩栥€?瑙掕壊鏉冮檺鐭╅樀瑙?`docs/rbac_matrix.yaml`锛屽苟鐢?`scripts/check_rbac_matrix.py` 鏍￠獙鍚庣璺敱鍜屽墠绔〉闈㈣鑹茶鐩栥€?
- `POST /auth/login`
- `GET /ships`銆乣POST /ships`銆乣GET /ships/{id}`銆乣PUT /ships/{id}`銆乣DELETE /ships/{id}`
- `POST /ships/{id}/locations`
- `GET /ships/{id}/tracks`
- `GET /alarms`銆乣PUT /alarms/{id}/ack`
- `GET /dispatch-events`銆乣POST /dispatch-events`銆乣PUT /dispatch-events/{id}/status`
- `GET /battle/scenarios`銆乣GET /battle/sessions`銆乣POST /battle/sessions`
- `GET /battle/sessions/{sessionId}/timeline`銆乣GET /battle/sessions/{sessionId}/snapshots`銆乣GET /battle/sessions/{sessionId}/report`銆乣GET /battle/sessions/{sessionId}/state`銆乣POST /battle/sessions/{sessionId}/stop`
- `POST /radar/reports`
- `GET /analytics/simulate/status`銆乣POST /analytics/simulate/start`銆乣POST /analytics/simulate/stop`銆乣POST /analytics/simulate/battle/start`銆乣POST /analytics/simulate/battle/stop`
- `GET /rbac/users`銆乣GET /rbac/roles`銆乣GET /rbac/menus`

WebSocket锛?
- `/ws/monitor` 榛樿浣跨敤 `shipsystem_token` Cookie锛涙湇鍔＄鎴栬皟璇曞伐鍏蜂篃鍙户缁娇鐢?`/ws/monitor?token={JWT}`
- 娑堟伅绫诲瀷锛歚ship_location_updated`銆乣alarm_created`銆乣dispatch_event_updated`銆乣radar_scan_updated`銆乣projectile_updated`銆乣battle_event_created`銆乣battle_state_updated`銆乣heartbeat`

## 绗竴鐗堣兘鍔?
- 鑸硅埗鍙拌处 CRUD锛岄粯璁よ蒋鍒犻櫎
- JWT 鐧诲綍鍜岄粯璁?RBAC 鏁版嵁
- 浣嶇疆涓婃姤銆佽建杩规煡璇€丳ostGIS 绌洪棿瀛楁涓庣储寮?- 瓒呴€熷憡璀︾敓鎴愬拰纭
- 璋冨害浜嬩欢鍒涘缓涓庣姸鎬侀棴鐜?- WebSocket 瀹炴椂鎺ㄩ€?- Python 妯℃嫙鍣ㄦ帹鍔ㄥ湴鍥惧疄鏃舵洿鏂?
