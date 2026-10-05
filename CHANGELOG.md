# Changelog

## [0.36.0](https://github.com/Zver64/crypto_scaner/compare/v0.35.0...v0.36.0) (2026-10-05)


### Features

* backtest strategy alerts on stored history in the admin settings ([34854c3](https://github.com/Zver64/crypto_scaner/commit/34854c3cd687a849a0187f25b1b1bdcff5784e28))


### Bug Fixes

* keep the HTTP status of error responses without a JSON body ([cf33bd4](https://github.com/Zver64/crypto_scaner/commit/cf33bd42a157d1f6090500010ee562a3be075e1c))
* show the favorite spinner only on symbols being updated ([f037b74](https://github.com/Zver64/crypto_scaner/commit/f037b74c59b6fe52b8dc2afb96f01cd844dcf7cf))

## [0.35.0](https://github.com/Zver64/crypto_scaner/compare/v0.34.0...v0.35.0) (2026-10-05)


### Features

* edit the scale and levels of an existing pane indicator ([0cd131b](https://github.com/Zver64/crypto_scaner/commit/0cd131b18bcd7df52b33d4c89ab5414a626f1ecc))
* share a chart pane between indicators of one type ([022dc0e](https://github.com/Zver64/crypto_scaner/commit/022dc0ee3315f51abc5b3c3c77fbdb12aae9683b))


### Bug Fixes

* draw no scale levels in a pane shared by several indicators ([3610a63](https://github.com/Zver64/crypto_scaner/commit/3610a6367ea156ff0cfc5368aafc6f56fb44ca1f))

## [0.34.0](https://github.com/Zver64/crypto_scaner/compare/v0.33.1...v0.34.0) (2026-10-05)


### Features

* choose whether each scanner indicator is drawn on charts ([71109a7](https://github.com/Zver64/crypto_scaner/commit/71109a7d0fbb3d0972b30a40ac2a823d9c090069))
* offer to add the missing indicators when importing a strategy ([a657dc8](https://github.com/Zver64/crypto_scaner/commit/a657dc8ce12880611a5bff4864aa9621dd52b802))
* remove only unused scanner indicators instead of all of them ([c2fab25](https://github.com/Zver64/crypto_scaner/commit/c2fab257fa6a98de890a99b621a3816938663a90))
* show why an invalid strategy no longer compiles ([ad95140](https://github.com/Zver64/crypto_scaner/commit/ad95140aa8fd3b1406349d915b881b6877bdaa85))


### Bug Fixes

* give scanner indicators with different parameters distinct titles ([e9c3840](https://github.com/Zver64/crypto_scaner/commit/e9c3840b4b09e771192f0e573d1c6455d2ffd5b6))
* keep the candle pane height fixed when indicator panes are added ([c34b96e](https://github.com/Zver64/crypto_scaner/commit/c34b96e52ccd816d6358f506fc1bc5a1b8ac1894))
* log scheduled Binance stream rotations as info instead of warnings ([bfa2b47](https://github.com/Zver64/crypto_scaner/commit/bfa2b47ff29704fe500bd156758fac12e1d70959))
* stop reporting client-canceled requests as errors and describe failed requests ([f62f67e](https://github.com/Zver64/crypto_scaner/commit/f62f67e2627135a5cefd4ca67cb182ba870266d0))

## [0.33.1](https://github.com/Zver64/crypto_scaner/compare/v0.33.0...v0.33.1) (2026-10-03)


### Bug Fixes

* show a loader on the apply settings button while analysis reloads ([777d7e6](https://github.com/Zver64/crypto_scaner/commit/777d7e6dafbc1780917a411d6e67a422d51492b1))

## [0.33.0](https://github.com/Zver64/crypto_scaner/compare/v0.32.0...v0.33.0) (2026-10-03)


### Features

* send backend warnings and errors to the administrator in Telegram ([dadbf89](https://github.com/Zver64/crypto_scaner/commit/dadbf89f58fde4487652066ccd641904f14b38f0))


### Bug Fixes

* split CoinGecko market requests to stay under the URL limit ([5127413](https://github.com/Zver64/crypto_scaner/commit/51274138643d40f87a21598bc4d9d86c5b5f6f07))

## [0.32.0](https://github.com/Zver64/crypto_scaner/compare/v0.31.0...v0.32.0) (2026-10-03)


### Features

* show four significant digits in numbers ([8e12f2a](https://github.com/Zver64/crypto_scaner/commit/8e12f2ab1522611538570c2f40e34285146b054b))


### Bug Fixes

* match the Binance spot grid bot price limits ([d4bc844](https://github.com/Zver64/crypto_scaner/commit/d4bc844d80518c7892f61f0685cdab3272f4e330))

## [0.31.0](https://github.com/Zver64/crypto_scaner/compare/v0.30.0...v0.31.0) (2026-10-03)


### Features

* respect Binance price limits in the spot grid calculator ([0ee0737](https://github.com/Zver64/crypto_scaner/commit/0ee07374f4cde8a05c6fd85d9a5a55311ecb4c6c))

## [0.30.0](https://github.com/Zver64/crypto_scaner/compare/v0.29.0...v0.30.0) (2026-10-02)


### Features

* clear indicator and coin selections ([08aa3c5](https://github.com/Zver64/crypto_scaner/commit/08aa3c5942415a44dd4f2e6aad6e4d06d4b6fc67))
* custom Telegram message for strategies ([796c021](https://github.com/Zver64/crypto_scaner/commit/796c021f62e7e007548ff9e373ac007076f92523))


### Bug Fixes

* draw average deviation in its own pane ([d369e54](https://github.com/Zver64/crypto_scaner/commit/d369e54e18b85905a1590c5cd7b7ea23dcdb000f))


### Performance Improvements

* recalculate strategy indicators once per sync round ([fd80514](https://github.com/Zver64/crypto_scaner/commit/fd80514adba3a82f20fa3f5ce2984cc5710e81c5))

## [0.29.0](https://github.com/Zver64/crypto_scaner/compare/v0.28.0...v0.29.0) (2026-10-02)


### Features

* clear strategy conditions from the import button ([6f7f648](https://github.com/Zver64/crypto_scaner/commit/6f7f6480fbf2b3bb3f26a1aa75afe32dcf3a0398))
* color strategy blocks by nesting depth ([f5e3711](https://github.com/Zver64/crypto_scaner/commit/f5e37117a77e64e212261f757450c0ab1f5348ad))
* compare strategies with another coin ([6a52ab9](https://github.com/Zver64/crypto_scaner/commit/6a52ab94ad9e00ba65aca017b7a4d3dbf0aa44d3))
* extend strategy language and indicator sources ([7367fac](https://github.com/Zver64/crypto_scaner/commit/7367face977e315e5b85c8b6179013cdd12694ca))
* import strategies from an expression ([9dc1d79](https://github.com/Zver64/crypto_scaner/commit/9dc1d79e8c87e20a6404086c901b4acf94314fd0))


### Performance Improvements

* compress live chart WebSocket messages ([0433523](https://github.com/Zver64/crypto_scaner/commit/0433523d74b8fafcae9ad046e21ae110d2decfc8))

## [0.28.0](https://github.com/Zver64/crypto_scaner/compare/v0.27.0...v0.28.0) (2026-10-01)


### Features

* exchange Telegram init data for server sessions ([dece942](https://github.com/Zver64/crypto_scaner/commit/dece942e69ee7b7207104d210e6971c2b265b17c))
* separate overlay and pane indicator values in chart legend ([cdcb3de](https://github.com/Zver64/crypto_scaner/commit/cdcb3dea93c6ba2b06eea009c5336fd35b287596))
* show current price in price alerts ([bfcd57b](https://github.com/Zver64/crypto_scaner/commit/bfcd57b0289faac893564971f36cf9c7a9078404))
* show target percent change in price alerts ([3126029](https://github.com/Zver64/crypto_scaner/commit/31260294d0f2964c908a362a82d2875109fd87b1))

## [0.27.0](https://github.com/Zver64/crypto_scaner/compare/v0.26.0...v0.27.0) (2026-10-01)


### Features

* filter indicator types by group in scanner settings ([1a55d80](https://github.com/Zver64/crypto_scaner/commit/1a55d806490268ed2e1e71f2fe95ba7e573df41d))


### Bug Fixes

* load coin charts faster ([7fc1f67](https://github.com/Zver64/crypto_scaner/commit/7fc1f6793ddc77933cb4d314474a8f8b134845fc))


### Performance Improvements

* calculate background indicators only for enabled strategies ([6a84d9e](https://github.com/Zver64/crypto_scaner/commit/6a84d9ecb879f4bce856c0d632c7e063f053245b))

## [0.26.0](https://github.com/Zver64/crypto_scaner/compare/v0.25.0...v0.26.0) (2026-09-30)


### Features

* add scanner indicators on several periods and clear them all ([011841b](https://github.com/Zver64/crypto_scaner/commit/011841b81cb874e95d8adc4c525cdef043b7714b))
* add strategy screener alerts ([c7b557d](https://github.com/Zver64/crypto_scaner/commit/c7b557d16820a72dd7191f68f7d67c430cc67ab2))
* manage users in the Mini App settings ([e0acf75](https://github.com/Zver64/crypto_scaner/commit/e0acf757a99bf1971358a04eb5d239d4b26e3faf))
* remove the hardcoded RSI filter from market scan ([8517c06](https://github.com/Zver64/crypto_scaner/commit/8517c0692ed78f6eebd67bec1fc657959ef68adc))


### Bug Fixes

* keep the selected interval after adding a scanner indicator ([4764de6](https://github.com/Zver64/crypto_scaner/commit/4764de6b97ace6795fb99e2e8002d0815da26273))
* name the missing strategy name when saving is disabled ([735dd81](https://github.com/Zver64/crypto_scaner/commit/735dd815ff2a3f307b257d51c6e872c0313e6a10))

## [0.25.0](https://github.com/Zver64/crypto_scaner/compare/v0.24.0...v0.25.0) (2026-09-29)


### Features

* let the administrator configure scanner indicators ([4f6ea8a](https://github.com/Zver64/crypto_scaner/commit/4f6ea8a5ab09a56053643212d284187f8bc2cd7d))
* render market tables from backend-defined columns ([880775e](https://github.com/Zver64/crypto_scaner/commit/880775e6e8c2370cda49fe57444631a8c5e98fe3))


### Bug Fixes

* hide overlay indicator values on the price axis ([8a20316](https://github.com/Zver64/crypto_scaner/commit/8a203165bdc313bec7d4f242eb02c39063a52e22))

## [0.24.0](https://github.com/Zver64/crypto_scaner/compare/v0.23.0...v0.24.0) (2026-09-29)


### Features

* calculate every TA-Lib indicator through one generic adapter ([d706354](https://github.com/Zver64/crypto_scaner/commit/d706354a078692c7c1c55ddd949619fe23b7bd04))
* collapse the market scan RSI filter block by default ([151cdf4](https://github.com/Zver64/crypto_scaner/commit/151cdf44b694e1b2881d0fee2fe8b39a9a9ce5e5))
* list instruments with insufficient history in the scan summary tooltip ([bae3e2a](https://github.com/Zver64/crypto_scaner/commit/bae3e2a56a80c9828e3b62a7dd998e2f5e1ae318))


### Bug Fixes

* link libm for TA-Lib on Linux ([37d51e9](https://github.com/Zver64/crypto_scaner/commit/37d51e904a556ac1d130484705f46b30eefd3f05))
* use dark surface colors for tooltips in the dark theme ([ec76ba1](https://github.com/Zver64/crypto_scaner/commit/ec76ba135fe73a580f522a134a4bfbf86ac0810a))

## [0.23.0](https://github.com/Zver64/crypto_scaner/compare/v0.22.0...v0.23.0) (2026-09-28)


### Features

* add daily and weekly max RSI filter to market scan ([1b8ce98](https://github.com/Zver64/crypto_scaner/commit/1b8ce9816861fdbb6fcde85aff91ffea84ca75f6))
* shorten Telegram price alert message ([7de36f3](https://github.com/Zver64/crypto_scaner/commit/7de36f3fee2606a6c71dd7a157224bea89c271f8))

## [0.22.0](https://github.com/Zver64/crypto_scaner/compare/v0.21.0...v0.22.0) (2026-09-28)


### Features

* add symbol filter to top market cap and favorites pages ([f370dc9](https://github.com/Zver64/crypto_scaner/commit/f370dc9879146c18825ebd42055b405f95bf33a6))
* bound market history and harden exchange and alert feeds ([a8d6063](https://github.com/Zver64/crypto_scaner/commit/a8d6063d726cfbec7559a5f49bf4af976b687ed1))
* derive spot grid count from a lower price markup slider ([0608992](https://github.com/Zver64/crypto_scaner/commit/0608992052267d017f7def5d2d4152a4d0119652))


### Bug Fixes

* fit indicator panes to visible values instead of the full range ([aea55fb](https://github.com/Zver64/crypto_scaner/commit/aea55fb373c5530b353c3738a414b39f53869349))

## [0.21.0](https://github.com/Zver64/crypto_scaner/compare/v0.20.0...v0.21.0) (2026-09-28)


### Features

* add backend-configured chart indicators with EMA 20/50/100 ([0335789](https://github.com/Zver64/crypto_scaner/commit/0335789236415ce6f914924c4ac370910bb18e5d))
* hide gross profit from the spot grid profit split ([53aec2b](https://github.com/Zver64/crypto_scaner/commit/53aec2b1f14e8325edc4c46a68e4b9c4640f56dd))
* merge spot grid profit into a single per-trade card ([16f97ca](https://github.com/Zver64/crypto_scaner/commit/16f97ca096573e71f0eeb12bc74df105a5240be2))
* open the coin chart on the daily interval ([64429b3](https://github.com/Zver64/crypto_scaner/commit/64429b38691f6a59408c97f2a4efd752aa32a5e5))
* show the coin symbol in the app header ([b69c102](https://github.com/Zver64/crypto_scaner/commit/b69c102d6e0b431a1c48fbe50da2bedd582ed60f))
* show trading volume on the coin chart ([607459e](https://github.com/Zver64/crypto_scaner/commit/607459e68c348671c37d9bc2ad56274fe00aa494))


### Bug Fixes

* format RSI values with the shared number formatter ([1eb7edd](https://github.com/Zver64/crypto_scaner/commit/1eb7eddcc3417bb0309f701a4875d214da9ae515))
* keep chart price scale stable near zero ([71aa082](https://github.com/Zver64/crypto_scaner/commit/71aa08240277f86bbfa882bceb65bc743586d594))
* removed irrelevant text ([86733a0](https://github.com/Zver64/crypto_scaner/commit/86733a0edbf59adace4a1c2342da5cd9c7ef01a2))

## [0.20.0](https://github.com/Zver64/crypto_scaner/compare/v0.19.0...v0.20.0) (2026-09-26)


### Features

* show background-tracked daily RSI in all coin tables ([57d3832](https://github.com/Zver64/crypto_scaner/commit/57d3832d7d23a418fbb547329c0a3fd7a6dc2c1e))
* show background-tracked weekly RSI in coin tables ([c48cfed](https://github.com/Zver64/crypto_scaner/commit/c48cfeda1da6465af65a232701903ea865181ed8))
* stream chart candles and indicators from a shared engine ([09e9752](https://github.com/Zver64/crypto_scaner/commit/09e97520ed124ed355f6c191aa60632d27bb7755))


### Bug Fixes

* keep live charts updating at range limits and delayed closes ([614ece5](https://github.com/Zver64/crypto_scaner/commit/614ece5717513216fa45ca0e7cbaefcaa6efcec0))
* recalculate chart RSI across expanded history ([739a3cf](https://github.com/Zver64/crypto_scaner/commit/739a3cfbdc8b1affd76726637e1dda396c7ec781))
* restructure backend modules and fix live data defects ([b44d8c4](https://github.com/Zver64/crypto_scaner/commit/b44d8c47960d9774d53cc9ca7d5c3f8601c8413e))

## [0.19.0](https://github.com/Zver64/crypto_scaner/compare/v0.18.1...v0.19.0) (2026-09-25)


### Features

* show visible min and max prices on shared chart axis ([d8aea25](https://github.com/Zver64/crypto_scaner/commit/d8aea259a115956abd19830771125a7d81a6ac55))


### Bug Fixes

* add shared analysis settings to favorites ([1bc3a9e](https://github.com/Zver64/crypto_scaner/commit/1bc3a9e3bb001e54f3357ba8c7ff06ba1420b51d))
* better layout ([e3301e4](https://github.com/Zver64/crypto_scaner/commit/e3301e469f3da608baec8ca3c1ccf31988fac30c))
* monitor live trades only for favorite symbols ([4f5ba03](https://github.com/Zver64/crypto_scaner/commit/4f5ba03995f927b639826e9f4bfbc239e26ccc71))

## [0.18.1](https://github.com/Zver64/crypto_scaner/compare/v0.18.0...v0.18.1) (2026-09-24)


### Bug Fixes

* better chart performance and isolation ([acb92fe](https://github.com/Zver64/crypto_scaner/commit/acb92fe1a5e360b95f31349da1962fb00761cf36))
* clear stale live candle errors on reconnect ([5de3318](https://github.com/Zver64/crypto_scaner/commit/5de331836c94c91b6b16eebc6e152f065316820b))

## [0.18.0](https://github.com/Zver64/crypto_scaner/compare/v0.17.1...v0.18.0) (2026-09-23)


### Features

* add favorites and price alerts ([5721ccb](https://github.com/Zver64/crypto_scaner/commit/5721ccb5f26b13a67afb9786b5168c0a7e29c634))


### Bug Fixes

* load instrument route eagerly ([56a6ad9](https://github.com/Zver64/crypto_scaner/commit/56a6ad9ef66b237baa45ae53634c05823365ea92))

## [0.17.1](https://github.com/Zver64/crypto_scaner/compare/v0.17.0...v0.17.1) (2026-09-22)


### Bug Fixes

* cache after new release ([074f1b1](https://github.com/Zver64/crypto_scaner/commit/074f1b16484064b855da1ac2ad1f4f1d0703510d))

## [0.17.0](https://github.com/Zver64/crypto_scaner/compare/v0.16.1...v0.17.0) (2026-09-22)


### Features

* add real-time candle streaming ([741fd3f](https://github.com/Zver64/crypto_scaner/commit/741fd3f8efeb334864b9ef160f8b905a54df78e0))


### Bug Fixes

* repair candle history depth ([33bea8e](https://github.com/Zver64/crypto_scaner/commit/33bea8e6d55e762469d7ed46c40bf896abb21bfa))

## [0.16.1](https://github.com/Zver64/crypto_scaner/compare/v0.16.0...v0.16.1) (2026-09-20)


### Bug Fixes

* ui ([a790e56](https://github.com/Zver64/crypto_scaner/commit/a790e56fda918beeecc03af281e2315d48a90624))

## [0.16.0](https://github.com/Zver64/crypto_scaner/compare/v0.15.0...v0.16.0) (2026-09-20)


### Features

* add RSI chart indicator ([7102ec6](https://github.com/Zver64/crypto_scaner/commit/7102ec6e71d5ad552e9b89b699b46c85bcff5d13))


### Bug Fixes

* refine market scan table columns ([cf47a96](https://github.com/Zver64/crypto_scaner/commit/cf47a96f7d3b8211e6219a2589469dec581862a6))
* size table columns to content ([769db93](https://github.com/Zver64/crypto_scaner/commit/769db9352e1a75e441cd8c4e329abd6414f4dd29))

## [0.15.0](https://github.com/Zver64/crypto_scaner/compare/v0.14.0...v0.15.0) (2026-09-18)


### Features

* configure top market cap result limit ([74c9adf](https://github.com/Zver64/crypto_scaner/commit/74c9adfd34eed9cfd2e304f63dc727a4e25d7fe8))
* sync coin metadata and retain page searches ([944561a](https://github.com/Zver64/crypto_scaner/commit/944561a6be6847a40a247ce8d618a998d252ffd9))


### Bug Fixes

* size symbol column to content ([c83797c](https://github.com/Zver64/crypto_scaner/commit/c83797c4c9c80959a360d53c5a1a6977b1262e85))

## [0.14.0](https://github.com/Zver64/crypto_scaner/compare/v0.13.0...v0.14.0) (2026-09-17)


### Features

* add backend OpenAPI contract ([aaa5dfa](https://github.com/Zver64/crypto_scaner/commit/aaa5dfa54756c8786556bad12fc86ae63f15567a))
* add database-first market selection ([2ed90b0](https://github.com/Zver64/crypto_scaner/commit/2ed90b064808abdcf4655ee7e9311f2ee0aef9b2))
* configure top market cap volatility ([ffce828](https://github.com/Zver64/crypto_scaner/commit/ffce8288d7add24f599ddf9e04a641b4e472892b))
* generate frontend API client ([a08efff](https://github.com/Zver64/crypto_scaner/commit/a08efff9a1a616991e0d99e93c13aa002f16a800))


### Bug Fixes

* accept large market cap thresholds ([3d8bebb](https://github.com/Zver64/crypto_scaner/commit/3d8bebb8debd2987c66c715db9d86b9898be8ff6))
* correct analysis query caching ([f516ba0](https://github.com/Zver64/crypto_scaner/commit/f516ba057a8499ec81c2a1864e08b72992153e2b))
* persist top market cap table sort in URL ([49df270](https://github.com/Zver64/crypto_scaner/commit/49df2708522c5dfb85c55185cf76863e64e42d1b))
* require complete filter history ([8f5c848](https://github.com/Zver64/crypto_scaner/commit/8f5c84847b397d4777785b2262a32c3aba76a4a6))

## [0.13.0](https://github.com/Zver64/crypto_scaner/compare/v0.12.0...v0.13.0) (2026-09-16)


### Features

* add multi-interval candle history ([791bfc8](https://github.com/Zver64/crypto_scaner/commit/791bfc89b9cb17fa9138dd6b89fd14c47891dc98))

## [0.12.0](https://github.com/Zver64/crypto_scaner/compare/v0.11.1...v0.12.0) (2026-09-15)


### Features

* show grid profit fee breakdown ([7840355](https://github.com/Zver64/crypto_scaner/commit/78403558db16d0e2db857ee244166a7e09e4c642))


### Bug Fixes

* refine mobile profit split layout ([9ec2285](https://github.com/Zver64/crypto_scaner/commit/9ec2285b63ef3e11f4d34f57c671c57e953cd924))

## [0.11.1](https://github.com/Zver64/crypto_scaner/compare/v0.11.0...v0.11.1) (2026-09-11)


### Bug Fixes

* changed percentile ([4332661](https://github.com/Zver64/crypto_scaner/commit/43326615e3d8bde2aba93d227657af1d295a664d))

## [0.11.0](https://github.com/Zver64/crypto_scaner/compare/v0.10.1...v0.11.0) (2026-09-11)


### Features

* add configurable spot grid range ([f31fbc9](https://github.com/Zver64/crypto_scaner/commit/f31fbc9824c694af8f0a688f040f9e8efbb99b2d))


### Bug Fixes

* enable top market cap row navigation ([9b838ca](https://github.com/Zver64/crypto_scaner/commit/9b838ca4e96972a7a3f54d6b1b3dc3ae9eee2d1f))
* reorder instrument analysis statistics ([42f471f](https://github.com/Zver64/crypto_scaner/commit/42f471ff02da7ba684f331fe3e25b952f40e6f26))
* use hourly volatility for spot grid ([520a7c0](https://github.com/Zver64/crypto_scaner/commit/520a7c0129d7a26fb90457a6ceb53b2738ebb338))
* use observed volatility percentile thresholds ([b2ac4de](https://github.com/Zver64/crypto_scaner/commit/b2ac4de2182f14b4fee9965e9e206311503d4e8f))

## [0.10.1](https://github.com/Zver64/crypto_scaner/compare/v0.10.0...v0.10.1) (2026-09-10)


### Bug Fixes

* decouple market scans from URL updates ([5ded4ea](https://github.com/Zver64/crypto_scaner/commit/5ded4eabb5fd045062ee92b1ebf0332dee6e2d99))
* make higher volatility percentiles stricter ([8b8e661](https://github.com/Zver64/crypto_scaner/commit/8b8e661e346a2171ea8aeb9002fcb31aaeae3ee5))

## [0.10.0](https://github.com/Zver64/crypto_scaner/compare/v0.9.0...v0.10.0) (2026-09-09)


### Features

* add top market cap page ([16dbda7](https://github.com/Zver64/crypto_scaner/commit/16dbda7783f0daa39cb21969646ac60e20fd10f8))


### Bug Fixes

* rank top market caps on backend ([d3e3b7f](https://github.com/Zver64/crypto_scaner/commit/d3e3b7f266fe09c313331d316eef314833521d0c))

## [0.9.0](https://github.com/Zver64/crypto_scaner/compare/v0.8.2...v0.9.0) (2026-09-09)


### Features

* better layout ([f176ad9](https://github.com/Zver64/crypto_scaner/commit/f176ad9d4cf24768536099e903274b4547668fe6))
* refine market scan criteria presets ([fbf8410](https://github.com/Zver64/crypto_scaner/commit/fbf84100b839b882a6f83bdc784c62d35c9d14ea))

## [0.8.2](https://github.com/Zver64/crypto_scaner/compare/v0.8.1...v0.8.2) (2026-09-09)


### Bug Fixes

* refine instrument analysis summaries ([c45ae9e](https://github.com/Zver64/crypto_scaner/commit/c45ae9ee7bf91cc0829997b89657a3a5e42c0f41))

## [0.8.1](https://github.com/Zver64/crypto_scaner/compare/v0.8.0...v0.8.1) (2026-09-09)


### Bug Fixes

* disable mobile viewport zoom ([20b26a2](https://github.com/Zver64/crypto_scaner/commit/20b26a2d6f8d26c462d7b9a757dde37f6808beec))
* keep spot grid inputs in two columns ([89f7fc1](https://github.com/Zver64/crypto_scaner/commit/89f7fc18deb6bca6b855683e564aeb0e1d9e5d17))
* move candle count columns to table end ([4a8e343](https://github.com/Zver64/crypto_scaner/commit/4a8e34318dfac4aa7ac2ac59e084098a1f19a6d8))
* refine market scan table labels and alignment ([f7a1639](https://github.com/Zver64/crypto_scaner/commit/f7a163934ad91c4b71549d56595ada18eff8cd01))

## [0.8.0](https://github.com/Zver64/crypto_scaner/compare/v0.7.1...v0.8.0) (2026-09-09)


### Features

* add geometric spot grid calculator ([0238ad4](https://github.com/Zver64/crypto_scaner/commit/0238ad412aa1490663f27d7518869ac640eea775))
* add market scan filter presets ([1b5231b](https://github.com/Zver64/crypto_scaner/commit/1b5231b0640f96a796a98b688bb83e18d45e20d5))
* add spot grid calculator ([35b25d5](https://github.com/Zver64/crypto_scaner/commit/35b25d5a7c8922eb8ee276d668121643fd34cfb8))
* add thirty-day instrument candlestick charts ([a42a8bb](https://github.com/Zver64/crypto_scaner/commit/a42a8bb12391298218db5a8b58e3979742086919))
* group spot grid results and display grid step ([b504fb3](https://github.com/Zver64/crypto_scaner/commit/b504fb31f60ae66b7262ebb5ecf714a7ba037bed))
* prefill spot grid calculator ([67a27b3](https://github.com/Zver64/crypto_scaner/commit/67a27b361955cc320f3e10f819d6c68e1f4c0cd9))
* reorder blocks ([bd606d8](https://github.com/Zver64/crypto_scaner/commit/bd606d81b97b5bd461607086bc2ce3991f302ff6))
* show seven-day change on coin page ([7eb5b5f](https://github.com/Zver64/crypto_scaner/commit/7eb5b5f8efadcd492382c0034d974f11bf106fbe))


### Bug Fixes

* prevent iOS input zoom ([756e563](https://github.com/Zver64/crypto_scaner/commit/756e5637e1e25611c9f86ab3d5e1e7a9d0b13f01))

## [0.7.1](https://github.com/Zver64/crypto_scaner/compare/v0.7.0...v0.7.1) (2026-09-06)


### Bug Fixes

* simplify backend startup provisioning ([88cad37](https://github.com/Zver64/crypto_scaner/commit/88cad372b0d23e1cec6c93819242d8451f3972ff))

## [0.7.0](https://github.com/Zver64/crypto_scaner/compare/v0.6.0...v0.7.0) (2026-09-05)


### Features

* add sortable seven-day change percentage ([a216d7c](https://github.com/Zver64/crypto_scaner/commit/a216d7c065cf7a9dc58e945144429469f27d3888))


### Bug Fixes

* close Telegram access flow and validate bot startup ([e568b0b](https://github.com/Zver64/crypto_scaner/commit/e568b0b8ecc57fbfddb3af94606f98132a575dda))
* keep first table column visible while scrolling ([c45e953](https://github.com/Zver64/crypto_scaner/commit/c45e9533d9834bde8533a7f4d6700716b5ddbec5))
* retain market cap metrics at zero threshold ([3bd8f83](https://github.com/Zver64/crypto_scaner/commit/3bd8f8319ac248a188066916ed1ecd03f07279d9))

## [0.6.0](https://github.com/Zver64/crypto_scaner/compare/v0.5.0...v0.6.0) (2026-09-04)


### Features

* show detail price history ([22a083b](https://github.com/Zver64/crypto_scaner/commit/22a083b22c01a3df9ceee5a3029ff5a63a694130))
* telegram bot access management for the Administrator ([3006322](https://github.com/Zver64/crypto_scaner/commit/300632204530a82b6284edd239b46c5f38422e28)), closes [#24](https://github.com/Zver64/crypto_scaner/issues/24)

## [0.5.0](https://github.com/Zver64/crypto_scaner/compare/v0.4.0...v0.5.0) (2026-09-04)


### Features

* show grid step recommendations ([3a329ea](https://github.com/Zver64/crypto_scaner/commit/3a329ea6f5cce877c1b8b8bff9b3a94759951dd9))


### Bug Fixes

* remove redundant text ([ff964b8](https://github.com/Zver64/crypto_scaner/commit/ff964b81f88c93514d41f93860927693592daa60))

## [0.4.0](https://github.com/Zver64/crypto_scaner/compare/v0.3.0...v0.4.0) (2026-09-03)


### Features

* add seven-day price charts to analysis results ([#19](https://github.com/Zver64/crypto_scaner/issues/19)) ([0b24d6c](https://github.com/Zver64/crypto_scaner/commit/0b24d6c41fb9da540bc6b7bcd4a55a790aa5f6a9))
* Show instrument range statistics and sample coverage ([365f8ca](https://github.com/Zver64/crypto_scaner/commit/365f8ca401b4cd748bd13774a6bd61c8681589d9))
* tweaket default filter ([09f1c4e](https://github.com/Zver64/crypto_scaner/commit/09f1c4ec1585a85252b8810d1c0753e17647ab70))


### Bug Fixes

* refine responsive scan interface ([cafe41c](https://github.com/Zver64/crypto_scaner/commit/cafe41c7f4edb7943821b3f86e008174dc2ca2a7))
* show units only in scan field labels ([be76167](https://github.com/Zver64/crypto_scaner/commit/be761678fe47b588bee45b1e88e42b6d114d139f))

## [0.3.0](https://github.com/Zver64/crypto_scaner/compare/v0.2.0...v0.3.0) (2026-09-02)


### Features

* add administrator bootstrap command ([dee408a](https://github.com/Zver64/crypto_scaner/commit/dee408afc48517ff1ebd4ab397ae28efc534bad2))
* add average entry price calculators ([4faff51](https://github.com/Zver64/crypto_scaner/commit/4faff51887aa77775e8af6fc0e49b96e390bddeb))
* add candle range percentile analyzer ([1159e7d](https://github.com/Zver64/crypto_scaner/commit/1159e7dce70f4a0b89bf338dda8220542047690f))
* add COIN-M average entry validation ([2db8cdb](https://github.com/Zver64/crypto_scaner/commit/2db8cdb9e9cf26c4baa79d525556d872da7f39bb))
* add COIN-M liquidation calculator ([6a592bf](https://github.com/Zver64/crypto_scaner/commit/6a592bf89737e0b36a509c4a79becaab8ea55dd4))
* add composable analysis criteria ([31434f3](https://github.com/Zver64/crypto_scaner/commit/31434f30e527d481ba34535aa52e473349e8bd0c))
* add conservative liquidation calculator ([518da42](https://github.com/Zver64/crypto_scaner/commit/518da42a0b061a21c0ee711c650534c30e36757b))
* add explicit PostgreSQL migrations ([84dd2a7](https://github.com/Zver64/crypto_scaner/commit/84dd2a7fd340153a4798692240914bc79d278da6))
* add hourly market analysis ([c4005e7](https://github.com/Zver64/crypto_scaner/commit/c4005e78bd8d3e6760cf77bfceeb569b76098e6b))
* add incremental market synchronization ([d6ea3a2](https://github.com/Zver64/crypto_scaner/commit/d6ea3a24118a93c8e7d92577e30a1db39b29a76f))
* add keyed volatility criterion instances ([#2](https://github.com/Zver64/crypto_scaner/issues/2)) ([dabcb52](https://github.com/Zver64/crypto_scaner/commit/dabcb5271877b500e7c8526d7e40d4a6324bcbc7))
* add local Telegram auth workflow ([31b3364](https://github.com/Zver64/crypto_scaner/commit/31b336428f3cc98c1cef23f62905cab4a86bc4ac))
* add market cap analysis ([7aad533](https://github.com/Zver64/crypto_scaner/commit/7aad53337bacfaf8f21044d7801751d277b37e33))
* add persisted Market Scan result sorting ([0712d0c](https://github.com/Zver64/crypto_scaner/commit/0712d0c22b1d188017826c171df4ff37741cc1b1))
* add persisted Market Scan result sorting ([#5](https://github.com/Zver64/crypto_scaner/issues/5)) ([ba6efbf](https://github.com/Zver64/crypto_scaner/commit/ba6efbf3cb7bdaa5f83c22c9b5c14bf5c5a6facc))
* add PostgreSQL stores and readiness ([3e1f642](https://github.com/Zver64/crypto_scaner/commit/3e1f64261487af089a47a435d851e98bd34286c6))
* add runtime lifecycle and liveness ([96ae993](https://github.com/Zver64/crypto_scaner/commit/96ae9932c90d1ea96575669c2ae8c720e0b4f76c))
* add USD-M average entry calculator ([f927825](https://github.com/Zver64/crypto_scaner/commit/f927825de37383995ad363b22575a1bd2015eff0))
* authenticate Telegram Mini App users ([045ed28](https://github.com/Zver64/crypto_scaner/commit/045ed2890645aa4b203e8da6ce8ece8d4eaf5e21))
* backfill closed daily candles ([8c919ac](https://github.com/Zver64/crypto_scaner/commit/8c919ac22105bb397a247cd124b68e12caa33716))
* configure local development ([f2d4a3f](https://github.com/Zver64/crypto_scaner/commit/f2d4a3fc6ad689113defd9df65475e63ff2c32ca))
* expose authenticated percentile analysis ([5a5c8c6](https://github.com/Zver64/crypto_scaner/commit/5a5c8c63267a5be1fbd6ad3ec9cf8201df71b4a9))
* **frontend:** analyze scan instruments ([ea2b7aa](https://github.com/Zver64/crypto_scaner/commit/ea2b7aa516b8f8bb7353d2f4ceb84884f9645022))
* **frontend:** bootstrap mini app shell ([7100552](https://github.com/Zver64/crypto_scaner/commit/7100552e2e2681d04445e8799179f6ce50ec281e))
* **frontend:** link scan results to Binance Spot ([b32cad0](https://github.com/Zver64/crypto_scaner/commit/b32cad0d20d28cab32aceb531522811dfe18e5db))
* **frontend:** refine scan results ([526c685](https://github.com/Zver64/crypto_scaner/commit/526c68582fa9507f7f58317c83cca6eaddf89e35))
* **frontend:** run market scan ([1ec3511](https://github.com/Zver64/crypto_scaner/commit/1ec3511819ef60fc9db6aabb09f733f9aa829bbc))
* initialize Go project foundation ([b7c60bf](https://github.com/Zver64/crypto_scaner/commit/b7c60bf093edc26e1e3ee83a93b8a54a0eb1ab8e))
* launch Mini App from Telegram webhook ([be610e2](https://github.com/Zver64/crypto_scaner/commit/be610e213593cc5bcfe5e16b8bb51d0037f14519))
* open Binance links through Telegram ([076f62b](https://github.com/Zver64/crypto_scaner/commit/076f62bfb078f47be80cf57af8fc9630cf4eb28c))
* show unified pipeline in instrument analysis ([b7b7604](https://github.com/Zver64/crypto_scaner/commit/b7b7604aa018e4ded4455f5a49fceab286f1df68))
* show unified pipeline in instrument analysis ([#4](https://github.com/Zver64/crypto_scaner/issues/4)) ([e423c36](https://github.com/Zver64/crypto_scaner/commit/e423c36d34eeb2206f054d479987d1dbe7623e57))
* sync Binance instrument catalog ([8aeb04e](https://github.com/Zver64/crypto_scaner/commit/8aeb04e956895d780a0eb6ff6eda3bd33b465209))
* unify the Market Scan analysis pipeline ([#3](https://github.com/Zver64/crypto_scaner/issues/3)) ([4cf9844](https://github.com/Zver64/crypto_scaner/commit/4cf9844768a63a848ebe16a42b3a7aedb7bfd1dd))


### Bug Fixes

* analyze partial market history ([0d6c1b3](https://github.com/Zver64/crypto_scaner/commit/0d6c1b3d9090cc28cc6e25a8c8cd8f925caa0314))
* **frontend:** address MVP review ([4456e97](https://github.com/Zver64/crypto_scaner/commit/4456e9726db1ed0909b3ec07223400639674cc47))
* keep market scan sorting local ([558589d](https://github.com/Zver64/crypto_scaner/commit/558589dceac7438df01bc701aa3467435c1e5c88))
* pass release version to frontend ([6ed34dd](https://github.com/Zver64/crypto_scaner/commit/6ed34ddd9cd7518dbdd95976324cbf72bd59ae7d))
* prevent Telegram swipe gesture conflicts ([8f17cd6](https://github.com/Zver64/crypto_scaner/commit/8f17cd6ff3d4300c75a34bcda940df95362f55b9))
* repair migrations and market sync ([e5325e5](https://github.com/Zver64/crypto_scaner/commit/e5325e575a6575b5b21abe9f90af86f43836e62c))

## [0.2.0](https://github.com/Zver64/crypto_scaner/compare/crypto-scanner-v0.1.0...crypto-scanner-v0.2.0) (2026-09-02)


### Features

* add administrator bootstrap command ([dee408a](https://github.com/Zver64/crypto_scaner/commit/dee408afc48517ff1ebd4ab397ae28efc534bad2))
* add average entry price calculators ([4faff51](https://github.com/Zver64/crypto_scaner/commit/4faff51887aa77775e8af6fc0e49b96e390bddeb))
* add candle range percentile analyzer ([1159e7d](https://github.com/Zver64/crypto_scaner/commit/1159e7dce70f4a0b89bf338dda8220542047690f))
* add COIN-M average entry validation ([2db8cdb](https://github.com/Zver64/crypto_scaner/commit/2db8cdb9e9cf26c4baa79d525556d872da7f39bb))
* add COIN-M liquidation calculator ([6a592bf](https://github.com/Zver64/crypto_scaner/commit/6a592bf89737e0b36a509c4a79becaab8ea55dd4))
* add composable analysis criteria ([31434f3](https://github.com/Zver64/crypto_scaner/commit/31434f30e527d481ba34535aa52e473349e8bd0c))
* add conservative liquidation calculator ([518da42](https://github.com/Zver64/crypto_scaner/commit/518da42a0b061a21c0ee711c650534c30e36757b))
* add explicit PostgreSQL migrations ([84dd2a7](https://github.com/Zver64/crypto_scaner/commit/84dd2a7fd340153a4798692240914bc79d278da6))
* add hourly market analysis ([c4005e7](https://github.com/Zver64/crypto_scaner/commit/c4005e78bd8d3e6760cf77bfceeb569b76098e6b))
* add incremental market synchronization ([d6ea3a2](https://github.com/Zver64/crypto_scaner/commit/d6ea3a24118a93c8e7d92577e30a1db39b29a76f))
* add keyed volatility criterion instances ([#2](https://github.com/Zver64/crypto_scaner/issues/2)) ([dabcb52](https://github.com/Zver64/crypto_scaner/commit/dabcb5271877b500e7c8526d7e40d4a6324bcbc7))
* add local Telegram auth workflow ([31b3364](https://github.com/Zver64/crypto_scaner/commit/31b336428f3cc98c1cef23f62905cab4a86bc4ac))
* add market cap analysis ([7aad533](https://github.com/Zver64/crypto_scaner/commit/7aad53337bacfaf8f21044d7801751d277b37e33))
* add persisted Market Scan result sorting ([0712d0c](https://github.com/Zver64/crypto_scaner/commit/0712d0c22b1d188017826c171df4ff37741cc1b1))
* add persisted Market Scan result sorting ([#5](https://github.com/Zver64/crypto_scaner/issues/5)) ([ba6efbf](https://github.com/Zver64/crypto_scaner/commit/ba6efbf3cb7bdaa5f83c22c9b5c14bf5c5a6facc))
* add PostgreSQL stores and readiness ([3e1f642](https://github.com/Zver64/crypto_scaner/commit/3e1f64261487af089a47a435d851e98bd34286c6))
* add runtime lifecycle and liveness ([96ae993](https://github.com/Zver64/crypto_scaner/commit/96ae9932c90d1ea96575669c2ae8c720e0b4f76c))
* add USD-M average entry calculator ([f927825](https://github.com/Zver64/crypto_scaner/commit/f927825de37383995ad363b22575a1bd2015eff0))
* authenticate Telegram Mini App users ([045ed28](https://github.com/Zver64/crypto_scaner/commit/045ed2890645aa4b203e8da6ce8ece8d4eaf5e21))
* backfill closed daily candles ([8c919ac](https://github.com/Zver64/crypto_scaner/commit/8c919ac22105bb397a247cd124b68e12caa33716))
* configure local development ([f2d4a3f](https://github.com/Zver64/crypto_scaner/commit/f2d4a3fc6ad689113defd9df65475e63ff2c32ca))
* expose authenticated percentile analysis ([5a5c8c6](https://github.com/Zver64/crypto_scaner/commit/5a5c8c63267a5be1fbd6ad3ec9cf8201df71b4a9))
* **frontend:** analyze scan instruments ([ea2b7aa](https://github.com/Zver64/crypto_scaner/commit/ea2b7aa516b8f8bb7353d2f4ceb84884f9645022))
* **frontend:** bootstrap mini app shell ([7100552](https://github.com/Zver64/crypto_scaner/commit/7100552e2e2681d04445e8799179f6ce50ec281e))
* **frontend:** link scan results to Binance Spot ([b32cad0](https://github.com/Zver64/crypto_scaner/commit/b32cad0d20d28cab32aceb531522811dfe18e5db))
* **frontend:** refine scan results ([526c685](https://github.com/Zver64/crypto_scaner/commit/526c68582fa9507f7f58317c83cca6eaddf89e35))
* **frontend:** run market scan ([1ec3511](https://github.com/Zver64/crypto_scaner/commit/1ec3511819ef60fc9db6aabb09f733f9aa829bbc))
* initialize Go project foundation ([b7c60bf](https://github.com/Zver64/crypto_scaner/commit/b7c60bf093edc26e1e3ee83a93b8a54a0eb1ab8e))
* launch Mini App from Telegram webhook ([be610e2](https://github.com/Zver64/crypto_scaner/commit/be610e213593cc5bcfe5e16b8bb51d0037f14519))
* show unified pipeline in instrument analysis ([b7b7604](https://github.com/Zver64/crypto_scaner/commit/b7b7604aa018e4ded4455f5a49fceab286f1df68))
* show unified pipeline in instrument analysis ([#4](https://github.com/Zver64/crypto_scaner/issues/4)) ([e423c36](https://github.com/Zver64/crypto_scaner/commit/e423c36d34eeb2206f054d479987d1dbe7623e57))
* sync Binance instrument catalog ([8aeb04e](https://github.com/Zver64/crypto_scaner/commit/8aeb04e956895d780a0eb6ff6eda3bd33b465209))
* unify the Market Scan analysis pipeline ([#3](https://github.com/Zver64/crypto_scaner/issues/3)) ([4cf9844](https://github.com/Zver64/crypto_scaner/commit/4cf9844768a63a848ebe16a42b3a7aedb7bfd1dd))


### Bug Fixes

* analyze partial market history ([0d6c1b3](https://github.com/Zver64/crypto_scaner/commit/0d6c1b3d9090cc28cc6e25a8c8cd8f925caa0314))
* **frontend:** address MVP review ([4456e97](https://github.com/Zver64/crypto_scaner/commit/4456e9726db1ed0909b3ec07223400639674cc47))
* prevent Telegram swipe gesture conflicts ([8f17cd6](https://github.com/Zver64/crypto_scaner/commit/8f17cd6ff3d4300c75a34bcda940df95362f55b9))
* repair migrations and market sync ([e5325e5](https://github.com/Zver64/crypto_scaner/commit/e5325e575a6575b5b21abe9f90af86f43836e62c))

## Changelog

All notable changes to this project will be documented in this file.
