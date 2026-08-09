<p align="center">
  <img src="docs/logo.svg" alt="Reasonix" width="640"/>
</p>

> 鈿狅笍 **闈炲畼鏂瑰畾鍒剁増** 鈥斺€?鍩轰簬 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)锛坄main-v2` 鍒嗘敮锛孧IT 鍗忚锛夌殑浜屾寮€鍙戯紝涓庡畼鏂归」鐩棤鍏炽€傝嫢浣犲彧鎯崇敤绋冲畾瀹樻柟鐗堬紝璇峰墠寰€瀹樻柟浠撳簱銆?
<p align="center">
  <strong>绠€浣撲腑鏂?/strong>
</p>

---

# Reasonix 瀹氬埗鐗堬紙DeepSeek-Reasonix-Custom锛?
涓€涓负瀵硅瘽绠＄悊鍋氫簡澶ч噺澧炲己鐨?Reasonix 妗岄潰瀹㈡埛绔€傛牳蹇冩€濊矾锛?*瀵硅瘽搴旇鍙互琚暣鐞?* 鈥斺€?鎺掑簭銆佸悎骞躲€佺Щ鍔ㄣ€佷竴閿矇娴革紝鑰屼笉鏄爢鍦ㄤ竴涓垪琛ㄩ噷鍚冪伆銆?
## 鍔熻兘浜偣

| 鍔熻兘 | 璇存槑 |
| --- | --- |
| 馃梻锔?**瀵硅瘽鎷栨嫿鎺掑簭** | 渚ц竟鏍忓璇濆彲涓婁笅鎷栧姩璋冩暣椤哄簭锛岄『搴忔湰鍦颁繚瀛?|
| 馃敆 **鍚堝苟瀵硅瘽** | 涓や釜瀵硅瘽鍚堟垚涓€涓€傛敮鎸?*鍙抽敭鑿滃崟**閫夋嫨鐩爣锛屾垨**鎷栨嫿鎮仠**鑷姩褰掍綅銆侀珮浜袱涓璇濄€?*鍙屽嚮**纭涓诲璇濓紙5 绉掑彲閫夋湡锛屽叾浠栨嫋鍔ㄥ彲鎵撴柇锛夈€傝法宸ヤ綔鍖虹姝㈠悎骞躲€傚悎骞跺悗鍙?`Ctrl+Z` 鎾ら攢 |
| 馃摝 **璺ㄩ」鐩Щ鍔?* | 鎶婂璇濈Щ鍒板彟涓€涓伐浣滃尯锛岃嚜鍔ㄥ叧闂師鏍囩銆佹敞鍏ョ郴缁熼€氱煡鎻愮ず锛屽け璐ヨ嚜鍔ㄥ洖婊?|
| 馃 **AI 鏍囬** | 鏈€鍚庝竴杞秷鎭梺鐐广€孉I 鏍囬銆嶏紝AI 鎬荤粨鏈€杩戝嚑杞璇濈敓鎴愭柊鏍囬锛岄瑙堢‘璁ゅ悗搴旂敤锛屽彲鎾ゅ洖 |
| 鈴?**缁熶竴鎾ら攢** | 鍚堝苟 / AI 鏍囬鍏辩敤涓€鏉℃挙閿€鍘嗗彶锛宍Ctrl+Z` 杩炵画鎾ら攢锛堣緭鍏ユ鍐呬笉鍙楀奖鍝嶏級 |
| 馃 **绾噣妯″紡** | 鏍囬鏍忋€屸浂銆嶄竴閿繘鍏ユ矇娴歌亰澶╋細鎵€鏈夋爮锛堥《鏍?渚ц竟鏍?鍙充晶闈㈡澘/鐘舵€佹爮/宸ュ叿鏍忥級鍏ㄩ儴娑堝け銆佽亰澶╁尯閾烘弧锛屽彧鐣欐秷鎭祦鍜岃緭鍏ユ锛宍Esc` 閫€鍑哄苟鎭㈠鍘熷竷灞€ |
| 馃悑 **瀹氬埗鍥炬爣** | DeepSeek 椴搁奔鍥炬爣锛屾柟渚夸笌瀹樻柟鐗堝尯鍒?|

## 瀹夎锛圵indows锛?
### 鏂瑰紡涓€锛氱洿鎺ヤ笅杞斤紙鎺ㄨ崘锛?
1. 鍒?[Releases](../../releases) 涓嬭浇鏈€鏂扮殑 `reasonix-desktop-custom.exe`锛?2. **棣栨杩愯鍓?*璁剧疆鏁版嵁鐩綍闅旂锛堥伩鍏嶅拰瀹樻柟鐗堟暟鎹贩鍦ㄤ竴璧凤級锛屾柊寤?`ReasonixCustom.cmd` 鍐呭濡備笅锛?
   ```cmd
   @echo off
   set REASONIX_HOME=%APPDATA%\reasonix-custom
   start "" "%~dp0reasonix-desktop-custom.exe"
   ```

3. 鍙屽嚮杩愯銆?*API key 鍗曠嫭閰嶇疆**锛堣涓嬶級銆?
### 鏂瑰紡浜岋細婧愮爜缂栬瘧

闇€瑕?Go 1.21+銆丯ode 20+銆乸npm銆亀ails v2銆?
```bash
git clone https://github.com/Sighing-wind/DeepSeek-Reasonix-Custom.git
cd DeepSeek-Reasonix-Custom/desktop
wails build
# 浜х墿: build/bin/reasonix-desktop.exe
```

## 璇曠敤 key锛堝彲閫夛級

浣滆€呭彲鑳介€氳繃鏈嬪弸鍦?绉佽亰鎻愪緵**鍏变韩璇曠敤 key**銆備娇鐢ㄦ柟寮忥細

1. 鍦?[Releases](../../releases) 涓嬭浇 **銆屾ā鏉?zip銆?*锛堜笉鍚?key锛屽畨鍏級骞惰В鍘嬶紱
2. 鍙屽嚮 **銆屽惎鍔ㄥ畾鍒剁増-鍙屽嚮鎴?cmd銆?*锛屾寜鎻愮ず**绮樿创 key 鍚庡洖杞?*锛堟棤闇€缂栬緫鏂囦欢锛夛紱
3. 鍚姩鍣ㄨ嚜鍔ㄥ啓鍏ラ厤缃苟鍚姩锛屼箣鍚庢甯镐娇鐢ㄣ€?
璇曠敤 key 涓哄叡浜搴︺€佹暟閲忔湁闄愩€佺敤灏藉嵆鍋滐紱**姝ｅ紡浣跨敤璇疯嚜琛屾敞鍐?*锛堣涓嬶級銆?
## 棣栨浣跨敤锛氶厤缃?API key

Reasonix 鏄鎴风锛孉I 鑳藉姏鏉ヨ嚜**浣犺嚜宸辨敞鍐屾ā鍨嬪钩鍙?*鎷垮埌鐨?API key锛堜笉闇€瑕?Reasonix 璐﹀彿锛夛細

| 骞冲彴 | 鐢ㄩ€?| 鑾峰彇鏂瑰紡 |
| --- | --- | --- |
| DeepSeek 寮€鏀惧钩鍙?| 涓诲姏瀵硅瘽妯″瀷 | platform.deepseek.com 娉ㄥ唽 鈫?鍏呭€?鈫?鍒涘缓 API key |
| Moonshot锛圞imi锛?| 瑙嗚/澶囬€夋ā鍨?| platform.moonshot.cn 娉ㄥ唽 鈫?鍒涘缓 API key |
| 鏅鸿氨 AI | 澶囬€夋ā鍨?| open.bigmodel.cn 娉ㄥ唽 鈫?鍒涘缓 API key |

鎶?key 濉繘 `%APPDATA%\reasonix\\.env`锛坄DEEPSEEK_API_KEY=sk-...` 鏍煎紡锛夛紝鎴栧湪搴旂敤鍐呰缃〉閰嶇疆銆?
## 涓庡畼鏂圭増鍏卞瓨 / 鏁版嵁闅旂

- 瀹氬埗鐗堜娇鐢ㄧ嫭绔嬬殑 `REASONIX_HOME`锛堝 `%APPDATA%\reasonix-custom`锛夛紝**涓嶈鍐欏畼鏂圭増鏁版嵁**锛屼袱鑰呭彲鍚屾椂瀹夎浜掍笉骞叉壈锛?- 鍗歌浇锛氬垹闄?exe 涓?`%APPDATA%\reasonix-custom` 鐩綍鍗冲彲锛屾棤娈嬬暀鏈嶅姟銆?
## 宸茬煡闂涓庨闄?
1. **钃濆睆椋庨櫓锛堥噸瑕侊級**锛氫釜鍒満鍣ㄨ嫢瑁呮湁**铏氭嫙鏄剧ず椹卞姩**锛堝鍚戞棩钁?OrayIddDriver銆丄skLink 绛夎繙绋?涓叉祦杞欢锛夛紝鍚姩鏈▼搴忓彲鑳借Е鍙戣摑灞?`0xBE`锛圓TTEMPTED_WRITE_TO_READONLY_MEMORY锛夈€傞亣鍒拌鍏堢鐢?鍗歌浇鐩稿叧铏氭嫙鏄剧ず椹卞姩鍐嶄娇鐢紙璁惧绠＄悊鍣?鈫?鏄剧ず閫傞厤鍣?鈫?绂佺敤锛夈€備笌 Reasonix 鏈綋鏃犵洿鎺ュ叧绯伙紝浣嗚鐭ユ倝锛?2. **鍚堝苟/绉诲姩鏄暟鎹搷浣?*锛氳櫧鐒舵敮鎸?`Ctrl+Z` 鎾ら攢锛屼粛寤鸿鎿嶄綔鍓嶅湪璁剧疆閲屽浠芥暟鎹洰褰曪紱
3. Windows 瀵?*鏈鍚?exe** 鍙兘寮瑰嚭 SmartScreen 鎻愮ず锛岄€夋嫨銆屼粛瑕佽繍琛屻€嶅嵆鍙紱
4. 鍩轰簬 `main-v2` 鍒嗘敮寮€鍙戯紝瀹樻柟鍚庣画鏇存柊闇€瑕佹墜鍔ㄥ悓姝ワ紝鍔熻兘鍙兘钀藉悗浜庡畼鏂规渶鏂扮増锛?5. 浠呴獙璇佽繃 Windows锛沵acOS/Linux 鏈祴璇曘€?
## 鍏嶈矗澹版槑

鏈」鐩负绗笁鏂逛釜浜轰慨鏀圭増锛?*涓?Reasonix 瀹樻柟銆丏eepSeek 瀹樻柟鍧囨棤鍏宠仈**銆傛寜 MIT 鍗忚鎻愪緵锛屼笉鎻愪緵浠讳綍鎷呬繚锛涗娇鐢ㄨ繃绋嬩腑浜х敓鐨勬暟鎹涪澶便€佺郴缁熷紓甯哥瓑椋庨櫓鐢变娇鐢ㄨ€呰嚜琛屾壙鎷呫€傚畼鏂规枃妗ｄ笌鏀寔璇峰墠寰€ [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)銆?
## License

MIT 鈥斺€?缁ф壙鑷笂娓?[esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)銆傚畾鍒堕儴鍒嗗悓鏍蜂互 MIT 鍙戝竷銆?
