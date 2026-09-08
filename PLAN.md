# План: информативный график на странице монеты

## Context

На странице инструмента сейчас показывается увеличенный `@microcharts/react` sparkline только по часовым ценам закрытия за фиксированные 7 дней. Пользователь выбрал полноценный свечной график TradingView Lightweight Charts. Компактные sparkline в таблице рынка должны остаться без изменений.

В БД уже хранятся часовые `open`, `high`, `low`, `close`, `volume` и время свечи в `binance_spot.candles`. Поэтому миграция схемы и повторная синхронизация не нужны: изменение базы ограничивается новой SQL-выборкой уже существующих OHLC-полей.

## Approach

Использовать `lightweight-charts` 5.x и готовую `CandlestickSeries`. Backend возвращает для страницы инструмента до 30 дней: 721 закрытый часовой слот от `price_history_window.from` до `to`, где существующий слот содержит UTC-время и OHLC, а отсутствующий — `null`. Frontend преобразует `null` в штатный `WhitespaceData`, чтобы пропуски не сжимали временную шкалу и не изображались свечами. При первом открытии видны последние 7 дней, а пользователь может отдалить график до полного 30-дневного диапазона.

Для market scan сохранить текущий лёгкий `price_history` из close-цен. В instrument response заменить дублирующий ряд закрытий на `candle_history`; 7d change вычислять из первой и последней доступной `close`. Это не раздувает ответ двумя копиями одной истории.

График создаётся и удаляется через React effect по официальному паттерну библиотеки, адаптируется через `ResizeObserver`, использует тему Mantine для цветов фона/текста/сетки и открывается на последних семи днях 30-дневного диапазона. Crosshair и touch предоставляет библиотека; над графиком показывается OHLC выбранной crosshair-свечи, по умолчанию — последняя доступная свеча.

## Files to modify

### Откат незавершённого варианта Mantine Charts

- `frontend/package.json`, `frontend/package-lock.json` — удалить добавленные `@mantine/charts` и `recharts`, добавить `lightweight-charts`.
- `frontend/src/main.tsx` — удалить промежуточный импорт `@mantine/charts/styles.css`.
- `frontend/src/features/instrument-analysis/price-history-chart/` — заменить незавершённый Mantine `LineChart` реализацией Lightweight Charts и актуальными helpers/tests.

### Backend и SQL

- `backend/internal/market/price_history.go` — модель часовой OHLC-свечи для presentation history.
- `backend/internal/market/store.go`, `backend/internal/analysis/service.go` — контракт чтения свечей и `SymbolResult.CandleHistory`.
- `backend/internal/analysis/price_history.go` — построение фиксированного 721-слотового OHLC-ряда за 30 дней для одного инструмента; текущий семидневный close-only ряд market scan сохранить.
- `backend/internal/storage/postgres/queries/candles.sql` — выборка `open_time`, OHLC для закрытых часовых свечей.
- `backend/internal/storage/postgres/sqlc/` — перегенерированные sqlc-файлы, не редактировать вручную.
- `backend/internal/storage/postgres/store.go` — безопасный parse числовых OHLC значений.
- `backend/internal/httpapi/analysis.go` — `candle_history` в instrument response; market response не менять.
- Соответствующие backend unit/integration/HTTP contract tests и stubs нового Store-метода.

### Frontend

- `frontend/src/api/client.ts` — тип и строгий parser `candle_history`: не более 721 слота, RFC3339 UTC, конечные OHLC, `low <= open/close <= high`, timestamp соответствует позиции в окне.
- `frontend/src/api/client.test.ts`, `frontend/src/api/price-history.test.ts`, `frontend/src/api/analysis-contract.test.ts` — обновлённый instrument-контракт и malformed payload cases.
- `frontend/src/features/instrument-analysis/instrument-analysis-screen.tsx` — новый график и расчёт 7d change по close.
- `frontend/src/features/instrument-analysis/price-history-chart/index.tsx` — lifecycle TradingView chart, CandlestickSeries, resize, theme, crosshair OHLC readout и empty state.
- `frontend/src/features/instrument-analysis/price-history-chart/utils.ts` — преобразование API-слотов в `CandlestickData | WhitespaceData`, форматирование UTC и OHLC.
- Тесты нового компонента/helpers и обновление `instrument-analysis-screen.test.tsx`.

## Reuse

- `market.SevenDayWindow` и `SevenDayPriceSlots` в `backend/internal/market/price_history.go` — единая граница и размер ряда.
- Условия «только закрытые свечи» из `ListHourlyPrices` в `backend/internal/storage/postgres/queries/candles.sql`.
- Полная OHLC-модель `market.Candle` и существующее безопасное преобразование PostgreSQL numeric в `backend/internal/storage/postgres/store.go`.
- Текущий `priceHistories` — оставить для компактных market-scan sparkline.
- Строгие проверки `parsePriceHistoryWindow`/`parsePriceHistory` в `frontend/src/api/client.ts` как паттерн нового parser.
- `sevenDayChangePercent` — передавать ему close-значения, извлечённые из свечного ряда, вместо дублирования формулы.
- Mantine color scheme/theme — источник цветов TradingView chart; Mantine Charts не используется.

## Steps

- [x] Удалить незавершённые зависимости и stylesheet Mantine Charts/Recharts; установить `lightweight-charts` 5.x.
- [x] Добавить SQL/store-метод чтения закрытых часовых OHLC-свечей из существующей таблицы и перегенерировать sqlc; миграцию БД не создавать.
- [x] Построить в analysis фиксированный `candle_history` из 721 OHLC/null слота (30 дней) для `AnalyzeSymbol`, сохранив текущий семидневный close-only `price_history` для `Search`.
- [x] Изменить instrument HTTP/API-контракт на `candle_history`, добавить строгую frontend-валидацию времени, размера, конечности и согласованности OHLC.
- [x] Реализовать TradingView `CandlestickSeries`: responsive resize, Mantine light/dark colors, UTC time scale, grid/crosshair, начальный вид последних 7 дней, корректный cleanup и whitespace для пропусков.
- [x] Добавить OHLC-readout для активной crosshair-свечи, fallback на последнюю свечу, доступное summary и понятный empty state; считать 7d change по close.
- [x] Обновить backend/frontend тесты, включая полную, частичную, разорванную, плоскую и пустую историю, malformed API, chart lifecycle/cleanup и отсутствие изменений market-scan sparkline.

## Verification

- Перегенерировать SQL-код штатной командой проекта и убедиться, что generated-файлы изменены только генератором.
- Запустить `go -C backend vet ./...` и `go -C backend test ./...`.
- Запустить `npm -C frontend run quality`, `npm -C frontend run test`, `npm -C frontend run build`.
- Вручную проверить desktop/mobile Telegram viewport и светлую/тёмную темы: читаемые свечи/шкалы, crosshair OHLC, touch, resize и отсутствие обрезки.
- Проверить полную, частичную и пустую историю; отсутствующие часы должны сохранять место на временной шкале и не создавать свечи.
- Убедиться по network response, что instrument получает один OHLC-ряд, market scan сохраняет прежний close-only контракт, новых запросов нет.
