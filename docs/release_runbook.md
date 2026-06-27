# ShipSystem 鍙戝竷 Runbook

閫傜敤鑼冨洿锛歋hipSystem 棰勭敓浜у拰鐢熶骇鍙戝竷鍓嶅悗鐨勪汉宸ユ墽琛屾祦绋嬨€傚綋鍓嶄粨搴撳皻鏈帴鍏?CI/CD 鏃讹紝鏈?Runbook 鏄彂甯冭瘉鎹棴鐜殑鏈€灏忔爣鍑嗐€?
## 1. 鍙戝竷鍓嶅噯澶?
- 纭鏈鍙戝竷瀵瑰簲鐨勪唬鐮佺増鏈€侀暅鍍?tag銆佹暟鎹簱杩佺Щ鏂囦欢鍜岄厤缃彉鏇存竻鍗曘€?- 纭鐢熶骇鐜涓嶄細浣跨敤榛樿 `JWT_SECRET`銆侀粯璁?`ADMIN_PASSWORD`銆佺┖ `ANALYTICS_ADMIN_TOKEN`銆佺┖ `GO_API_TOKEN`銆侀€氶厤 CORS銆乣DATABASE_AUTO_MIGRATE=true` 鎴?`SEED_DEMO_DATA=true`銆?- 纭 `.env`銆佸瘑閽ユ枃浠躲€佺紦瀛樼洰褰曞拰鏋勫缓浜х墿涓嶄細杩涘叆闀滃儚涓婁笅鏂囷紝鎵ц `scripts/check_compose_config.py` 楠岃瘉銆?- 鑻ュ寘鍚暟鎹簱缁撴瀯鎴栧熀纭€鏁版嵁鍙樻洿锛屽厛纭澶囦唤绐楀彛銆佽縼绉绘墽琛屼汉銆佸洖婊氬彂甯冨寘鍜屾仮澶嶈礋璐ｄ汉銆?
## 2. 闈欐€佸彂甯冨墠 Gate

浼樺厛鎵ц缁熶竴鍏ュ彛锛?
```bash
python scripts/preflight_check.py
```

璇ュ懡浠ら『搴忔墽琛岋細

- `go test ./...`
- `uv run --with-requirements requirements.txt python -m unittest discover -s tests`
- `python -m unittest discover -s scripts_tests`
- `npm run build`
- `python scripts/check_compose_config.py`
- `python scripts/check_event_contract.py`
- `python scripts/check_callback_contract.py`
- `python scripts/check_openapi_contract.py`
- `python scripts/check_frontend_api_contract.py`
- `python scripts/check_rbac_matrix.py`

鍏朵腑 OpenAPI gate 涓嶅啀鍙槸璺敱瀛樺湪鎬ф鏌ワ細瀹冧細鏍￠獙 `docs/openapi.yaml` 涓殑鍙椾繚鎶ゆ帴鍙?`x-roles`銆乸ublic/protected 閴存潈澹版槑銆佸叧閿?requestBody銆佹垚鍔熺姸鎬佺爜浠ュ強甯歌 `400/401/403/404/500/502` 鍝嶅簲鏄惁浠嶄笌 Go handler 濂戠害涓€鑷淬€傚彂甯冨墠鑻ユ湁鎺ュ彛琛屼负鍙樻洿锛岃繖涓?gate 蹇呴』涓€骞堕€氳繃銆?
瀵逛簬鎵挎媴鍙戝竷闂ㄧ鑱岃矗鐨?Python 鑴氭湰锛岄澶栨墽琛?`python -m unittest discover -s scripts_tests`銆傚綋鍓嶅畠浼氳鐩?OpenAPI 濂戠害鑴氭湰鐨勮嚜瀹氫箟 YAML 瑙ｆ瀽銆乺untime precheck 鐨?Docker 鏈嶅姟鐘舵€佸拰绔彛鍗犵敤璇婃柇鍒嗘敮銆乧ompose baseline 瑙ｆ瀽鏍￠獙銆乻moke gate 鐨勭函鍑芥暟鍜?cookie/requestId 鏈湴绾︽潫锛屼互鍙?frontend API / RBAC / WebSocket event / analytics callback 绛夐潤鎬佸绾﹁剼鏈殑鏍稿績瑙ｆ瀽閫昏緫锛岄伩鍏嶉棬绂佽剼鏈嚜韬洖褰掑悗璇斁琛屻€?
鍓嶇 `npm run build` 鐜板湪浼氬厛鎵ц `npm run test:static`锛屽 `frontend/scripts/check_frontend_routes.mjs` 鍜?`frontend/scripts/check_frontend_bundle.mjs` 鐨勬牳蹇冮€昏緫鍋?`node:test` 鍥炲綊锛屽啀缁х画 `tsc -b`銆佽矾鐢?gate銆乂ite build 鍜?bundle budget gate銆傝嫢鍓嶇 gate 鑷韩鍥炲綊锛岃繖涓€姝ヤ細鍏堜簬姝ｅ紡鏋勫缓澶辫触銆?
Windows/Codex 娌欑涓紝Python 瀛愯繘绋嬪惎鍔?`npm` 鍙兘瑙﹀彂 Node 瀵?`C:\Users\chenn` 鐨?realpath 鏉冮檺閿欒銆傝嫢鍙湪 preflight 鐨?frontend 姝ラ澶辫触锛屼絾鐩存帴鎵ц浠ヤ笅鍛戒护閫氳繃锛屽彲灏嗙洿鎺ュ懡浠よ緭鍑轰綔涓哄墠绔?gate 璇佹嵁锛?
```bash
cd frontend
npm run build
```

娑夊強鍛婅纭銆佽皟搴︾姸鎬佹祦杞垨浠撳偍浜嬪姟杈圭晫鐨勫彉鏇达紝浼樺厛鎵ц涓€閿寲浠撳偍骞跺彂闆嗘垚 gate锛?```bash
python scripts/run_repository_db_integration.py
```

璇ヨ剼鏈細鑷姩鎷夎捣涓存椂 PostGIS 娴嬭瘯搴擄紝鎵ц `go test ./internal/repositories`锛屽苟鍦ㄦ垚鍔熸垨澶辫触鍚庨粯璁ゆ竻鐞嗘祴璇曞鍣ㄥ拰鍗枫€傝嫢闇€瑕佸鐢ㄧ幇鎴愮殑鍙涪寮冩暟鎹簱锛屽啀閫€鍥炴墜宸?DSN 鏂瑰紡锛?```bash
set SHIPSYSTEM_REPOSITORY_TEST_DSN=host=localhost user=shipsystem password=shipsystem dbname=shipsystem_test port=5432 sslmode=disable TimeZone=Asia/Shanghai
python scripts/preflight_check.py --only db-integration
```

鑻ラ渶瑕佹妸瀹冪撼鍏ュ畬鏁村彂甯冨墠妫€鏌ワ細
```bash
python scripts/preflight_check.py --include-db-integration
```

闇€瑕佽瘎浼板巻鍙叉暟鎹暀瀛樺拰瀹归噺褰卞搷鏃讹紝鎵ц鍙棰勮锛涜姝ラ涓嶄細鍒犻櫎鏁版嵁锛?```bash
python scripts/preflight_check.py --only retention-preview
```

涔熷彲浠ュ湪瀹屾暣鍙戝竷鍓嶆鏌ヤ腑杩藉姞棰勮锛?```bash
python scripts/preflight_check.py --include-retention-preview
```


若需要在调整留存阈值前先量化 battle/radar 数据增长速度，先执行容量估算：
```bash
python scripts/run_capacity_smoke.py --estimate-only
```

若预生产栈已启动，也可以执行一次轻量真实采样，读取 battle timeline、snapshots 和 report，作为留存参数或容量预算证据：
```bash
python scripts/run_capacity_smoke.py --track-counts 5,20 --ticks 6 --duration-seconds 30
```

## 3. 鏁版嵁搴撹縼绉?
鍙戝竷鍓嶆煡鐪嬭縼绉荤姸鎬侊細

```bash
cd backend
go run ./cmd/migrate -action=status
go run ./cmd/migrate -action=check
```

鎵ц杩佺Щ锛?
```bash
cd backend
go run ./cmd/migrate -action=up
```

杩佺Щ鍘熷垯锛?
- 鍙仛鍚戝墠杩佺Щ锛屼笉渚濊禆鑷姩 down銆?- 鐢熶骇鐜淇濇寔 `DATABASE_AUTO_MIGRATE=false`銆?- Go API 鍦?`DATABASE_AUTO_MIGRATE=false` 鏃朵細鎵ц鍚姩杩佺Щ闂ㄧ锛涜嫢 `schema_migrations` 浠嶆湁 pending migration锛屾湇鍔′細鎷掔粷鍚姩銆?- 鏂板瓧娈点€佹柊绱㈠紩銆佹柊琛ㄣ€佽鑹茶彍鍗曞熀纭€鏁版嵁淇閮藉繀椤婚€氳繃鏂板 migration 鏂囦欢钀藉湴銆?- 杩佺Щ澶辫触鏃跺仠姝㈠彂甯冿紝淇濈暀鏃ュ織銆佹暟鎹簱鐘舵€佸拰澶辫触 migration 鍚嶇О锛屾寜鎭㈠鏂规澶勭悊銆?
## 4. 鏁版嵁鐣欏瓨涓庡閲忔不鐞?
鐢熶骇鎴栭鐢熶骇娓呯悊鍘嗗彶鏁版嵁鍓嶏紝鍏堝畬鎴愭暟鎹簱澶囦唤鎴栧钩鍙板揩鐓э紝鍐嶆墽琛岄瑙堬細

```bash
python scripts/retention_maintenance.py --days 30
```

鎸夊閲忎笂闄愰瑙堟瘡鑹樿埞鍜屾瘡涓凡鍋滄瀵规垬浼氳瘽淇濈暀鐨勬渶鏂拌褰曪細

```bash
python scripts/retention_maintenance.py --days 30 --max-track-points-per-ship 5000 --max-battle-snapshots-per-session 1000 --max-battle-events-per-session 2000 --max-radar-targets-per-session 1000
```

纭棰勮杈撳嚭銆佸浠界姸鎬佸拰娓呯悊绐楀彛鍚庯紝鎵嶅厑璁歌拷鍔?`--apply` 鎵ц鍒犻櫎锛?
```bash
python scripts/retention_maintenance.py --days 30 --max-track-points-per-ship 5000 --max-battle-snapshots-per-session 1000 --max-battle-events-per-session 2000 --max-radar-targets-per-session 1000 --apply
```

娓呯悊鍘熷垯锛?
- `--apply` 涓嶆槸榛樿琛屼负锛屾湭鏄惧紡浼犲叆鏃跺彧缁熻鍖归厤琛屾暟銆?- `OPEN` 鍛婅涓嶄細琚竻鐞嗭紝鍙竻鐞嗚秴杩囦繚鐣欐湡鐨?`ACKED` 鍛婅銆?- `NEW`銆乣DISPATCHED`銆乣PROCESSING` 璋冨害浜嬩欢涓嶄細琚竻鐞嗭紝鍙竻鐞嗚秴杩囦繚鐣欐湡鐨?`COMPLETED` / `CANCELLED` 浜嬩欢銆?- `running` 瀵规垬浼氳瘽鍙婂叾鍥炴斁銆佷簨浠躲€侀浄杈剧洰鏍囦笉浼氳娓呯悊锛涘凡鍋滄涓旇秴杩囦繚鐣欐湡鐨勪細璇濅細鍏堟竻鐞嗗瓙琛紝鍐嶆竻鐞嗕細璇濊銆?- 杈撳嚭涓殑 DSN 浼氶殣钘?`password`锛屼絾鍙戝竷璇佹嵁浠嶄笉搴斿寘鍚湡瀹炲瘑閽ユ垨瀹屾暣 `.env` 鍐呭銆?
## 5. 鍚姩涓庡仴搴锋鏌?
鍚姩鎴栨洿鏂版湇鍔″悗锛屽厛妫€鏌ユ帰閽堬細

- Go API `/health`锛氳繘绋嬪瓨娲汇€?- Go API `/ready`锛氭暟鎹簱杩炴帴鍙敤銆?- analytics `/health`锛氬垎鏋愭湇鍔″瓨娲汇€?- frontend `/`锛氬墠绔潤鎬佽祫婧愬彲璁块棶銆?
Docker Compose 鏈湴楠岃瘉鍙娇鐢細

```bash
python scripts/runtime_precheck.py
docker compose up --build
```

`scripts/runtime_precheck.py` 浼氬湪鍚姩瀹屾暣鏍堝墠妫€鏌?Docker CLI銆丏ocker Compose銆丏ocker Desktop Service 鐘舵€併€丏ocker daemon 璁块棶鏉冮檺銆丆ompose 閰嶇疆娓叉煋銆乻moke 鐩爣鍦板潃绔彛浠ュ強 Compose 瀹為檯鍙戝竷绔彛銆傚畠浼氫娇鐢ㄥ伐浣滃尯 `.docker-codex` 浣滀负 Docker 閰嶇疆鐩綍锛岄伩鍏嶆湰鏈虹敤鎴风洰褰?Docker 閰嶇疆鏉冮檺褰卞搷鍙戝竷鍓嶆鏌ワ紱鍦?Windows 涓婁細鎶婄鍙ｅ崰鐢ㄧ洿鎺ュ睍寮€鍒?PID 鍜岃繘绋嬪悕锛屼究浜庤瘑鍒槸璋佸崰鐢ㄤ簡 `3000/4173/5432/8080/8090` 杩欑被瀹為檯杩愯绔彛銆?
鑻ラ粯璁ょ鍙ｅ凡琚崰鐢紝鍏堢敓鎴愭湰鍦?override锛?
```bash
python scripts/generate_compose_local_override.py
python scripts/preflight_check.py --only compose-override
```

鍐嶄娇鐢?override 鍚姩瀹屾暣鏍堬細

```bash
docker compose -f docker-compose.yml -f .docker-codex/compose.smoke.override.yml up --build
```

鐢熶骇鐜搴斾娇鐢ㄥ搴旈儴缃茬郴缁熺殑婊氬姩鍙戝竷銆佸仴搴锋鏌ュ拰鍥炴粴鑳藉姏銆?
## 6. 杩愯鎬?Smoke

瀹屾暣鏍堝惎鍔ㄥ悗鎵ц锛?
```bash
python scripts/runtime_precheck.py
python scripts/smoke_check.py
```

榛樿妫€鏌ワ細

- backend銆乤nalytics銆乫rontend 鍋ュ悍銆?- 鐧诲綍 Cookie 鍜岄€€鍑?Cookie 娓呯悊銆?- 鏍稿績鍒楄〃鎺ュ彛銆?- analytics 鐘舵€佷唬鐞嗐€?- 鍒涘缓 battle session銆?- WebSocket Cookie 閴存潈杩炴帴銆?- 闆疯揪涓婃姤銆乄ebSocket 瀹炴椂鎺ㄩ€侊紙鑷冲皯楠岃瘉 `radar_scan_updated`銆乣projectile_updated`銆乣battle_event_created`銆乣battle_state_updated`锛夈€佹垬鏂楃姸鎬併€佹椂闂磋酱銆佸揩鐓у拰鎴樻姤銆?
闇€瑕佽繛鎺ラ潪榛樿鍦板潃鏃朵娇鐢ㄧ幆澧冨彉閲忥細

```bash
set SHIPSYSTEM_BACKEND_URL=http://host:8080
set SHIPSYSTEM_ANALYTICS_URL=http://host:8090
set SHIPSYSTEM_FRONTEND_URL=http://host:3000
set SHIPSYSTEM_WS_URL=ws://host:8080/ws/monitor
python scripts/smoke_check.py
```

## 7. 澶辫触澶勭悊涓庡洖婊?
- preflight 澶辫触锛氫笉杩涘叆鍙戝竷锛涙寜澶辫触妯″潡鍗曠嫭閲嶈窇鏈€灏忓懡浠ゅ畾浣嶃€?- 杩佺Щ澶辫触锛氬仠姝㈠簲鐢ㄥ彂甯冿紱淇濈暀 `schema_migrations` 鐘舵€佸拰鏁版嵁搴撻敊璇紝鎸夊浠?鎭㈠鏂规澶勭悊銆?- 鐣欏瓨娓呯悊棰勮寮傚父锛氫笉鎵ц `--apply`锛屽厛纭 DSN銆佽〃缁撴瀯鐗堟湰銆佸浠界姸鎬佸拰娓呯悊绐楀彛銆?- 鍋ュ悍妫€鏌ュけ璐ワ細鍋滄鎵╁鎴栧垏娴侊紝鍥炴粴鍒颁笂涓€鐗堟湰闀滃儚/閮ㄧ讲鍖呫€?- smoke 澶辫触锛氫繚鐣欏け璐ユ楠ゃ€丠TTP 鐘舵€併€佸搷搴斾綋鎽樿鍜?requestId锛涙牴鎹?requestId 鍥炴煡 Go/Python 鏃ュ織銆?- 瀹夊叏閰嶇疆澶辫触锛氫笉寰椾复鏃舵斁瀹界敓浜ф牎楠岋紱蹇呴』淇 secret銆丆ORS銆佽縼绉诲拰 seed 閰嶇疆鍚庨噸鏂板彂甯冦€?
## 8. 鍙戝竷璇佹嵁褰掓。

姣忔鍙戝竷鑷冲皯褰掓。锛?
- 浠ｇ爜鐗堟湰鎴栭暅鍍?tag銆?- `python scripts/preflight_check.py` 杈撳嚭锛屾垨绛変环鍒嗘杈撳嚭銆?- `go run ./cmd/migrate -action=status` 鍙戝竷鍓嶅悗杈撳嚭銆?- 鑻ユ墽琛岀暀瀛樼淮鎶わ紝褰掓。 `scripts/retention_maintenance.py` 棰勮杈撳嚭銆佸疄闄?`--apply` 杈撳嚭鍜屽浠?蹇収缂栧彿銆?- `scripts/smoke_check.py` 杈撳嚭銆?- 鍏抽敭閰嶇疆纭锛歚APP_ENV`銆乣DATABASE_AUTO_MIGRATE`銆乣SEED_DEMO_DATA`銆丆ORS origins銆乼oken 娉ㄥ叆鏂瑰紡銆?- 宸茬煡椋庨櫓銆佸洖婊氱獥鍙ｅ拰璐熻矗浜恒€?
