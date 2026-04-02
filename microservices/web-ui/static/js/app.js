/**
 * Web UI for microservices API Gateway.
 * Layout and behaviour mirror monolith/web/js/app.js; API paths match /rates/* on the gateway.
 */
(function () {
    const API_BASE = window.API_BASE || 'http://localhost:8080';

    document.addEventListener('DOMContentLoaded', function () {
        const dataSourceSelect = document.getElementById('data-source');
        const currencySelect = document.getElementById('currency-select');
        const periodSelect = document.getElementById('period-select');
        const customPeriodDiv = document.getElementById('custom-period');
        const startDateInput = document.getElementById('start-date');
        const endDateInput = document.getElementById('end-date');
        const currencyForm = document.getElementById('currency-form');
        const metricAvg = document.getElementById('metric-avg');
        const metricStd = document.getElementById('metric-std');
        const metricMin = document.getElementById('metric-min');
        const metricMax = document.getElementById('metric-max');
        const metricVolatility = document.getElementById('metric-volatility');
        const loadingIndicator = document.getElementById('loading-indicator');
        const downloadExcelBtn = document.getElementById('download-excel');

        let currentDataSource = 'cbr';
        let currentCurrencyCode = '';
        let currentStartDate = '';
        let currentEndDate = '';

        const ctx = document.getElementById('currency-chart').getContext('2d');
        let currencyChart = null;

        function startOfLocalDay(d) {
            return new Date(d.getFullYear(), d.getMonth(), d.getDate());
        }

        function parseISODateInput(value) {
            const parts = String(value)
                .split('-')
                .map((x) => parseInt(x, 10));
            if (parts.length !== 3 || parts.some((n) => !Number.isFinite(n))) {
                return new Date(NaN);
            }
            return new Date(parts[0], parts[1] - 1, parts[2]);
        }

        function parsePeriodDays(raw) {
            const n = parseInt(String(raw), 10);
            return Number.isFinite(n) && n >= 1 ? n : 7;
        }

        function cbrDateISOFromAPI(d) {
            if (d == null) return '';
            if (typeof d === 'string') {
                return d.length >= 10 ? d.slice(0, 10) : d;
            }
            return formatDate(new Date(d));
        }

        function formatDateForDisplayYMD(ymd) {
            const s = String(ymd).slice(0, 10);
            if (s.length >= 10 && s[4] === '-' && s[7] === '-') {
                const [y, m, d] = s.split('-');
                return `${d}.${m}.${y}`;
            }
            const date = new Date(ymd);
            if (isNaN(date.getTime())) return String(ymd);
            const day = date.getDate().toString().padStart(2, '0');
            const month = (date.getMonth() + 1).toString().padStart(2, '0');
            const year = date.getFullYear();
            return `${day}.${month}.${year}`;
        }

        let analysisTimer = null;
        let latestHistoryRequest = 0;

        function beginHistoryLoad() {
            return ++latestHistoryRequest;
        }

        function isStaleHistoryRequest(reqId) {
            return reqId !== latestHistoryRequest;
        }

        function scheduleAnalysis() {
            if (analysisTimer) clearTimeout(analysisTimer);
            analysisTimer = setTimeout(() => runAnalysis(), 250);
        }

        const todayStart = startOfLocalDay(new Date());
        endDateInput.value = formatDate(todayStart);
        const defaultStart = new Date(todayStart);
        defaultStart.setDate(defaultStart.getDate() - 30);
        startDateInput.value = formatDate(defaultStart);

        loadCurrencies();

        dataSourceSelect.addEventListener('change', function () {
            currentDataSource = this.value;
            currencySelect.innerHTML = '<option value="" selected disabled>Loading...</option>';
            resetMetrics();
            if (currencyChart) {
                currencyChart.destroy();
                currencyChart = null;
            }
            updateUILabels();
            downloadExcelBtn.disabled = true;
            if (currentDataSource === 'cbr') {
                loadCurrencies();
            } else {
                loadCryptoSymbols();
            }
        });

        currencySelect.addEventListener('change', function () {
            updateDownloadButtonState();
            scheduleAnalysis();
        });

        periodSelect.addEventListener('change', function () {
            if (this.value === 'custom') {
                customPeriodDiv.classList.remove('d-none');
            } else {
                customPeriodDiv.classList.add('d-none');
            }
            updateDownloadButtonState();
            scheduleAnalysis();
        });

        function onCustomDateFieldUpdate() {
            updateDownloadButtonState();
            if (periodSelect.value !== 'custom') return;
            // "input" fires when picking a date in the native picker; "change" alone is unreliable on some browsers.
            scheduleAnalysis();
        }
        startDateInput.addEventListener('change', onCustomDateFieldUpdate);
        startDateInput.addEventListener('input', onCustomDateFieldUpdate);
        endDateInput.addEventListener('change', onCustomDateFieldUpdate);
        endDateInput.addEventListener('input', onCustomDateFieldUpdate);

        function updateDownloadButtonState() {
            const val = currencySelect.value;
            if (!val) {
                downloadExcelBtn.disabled = true;
                return;
            }
            if (periodSelect.value === 'custom') {
                const startDate = parseISODateInput(startDateInput.value);
                const endDate = parseISODateInput(endDateInput.value);
                if (isNaN(startDate.getTime()) || isNaN(endDate.getTime()) || startDate > endDate) {
                    downloadExcelBtn.disabled = true;
                    return;
                }
                const daysDiff = Math.round((endDate - startDate) / (1000 * 60 * 60 * 24)) + 1;
                if (daysDiff > 365) {
                    downloadExcelBtn.disabled = true;
                    return;
                }
            }
            // Export stays disabled until a successful history load sets __lastExport
        }

        currencyForm.addEventListener('submit', function (e) {
            e.preventDefault();
            if (analysisTimer) clearTimeout(analysisTimer);
            runAnalysis();
        });

        downloadExcelBtn.addEventListener('click', function () {
            exportReportToCSV();
        });

        function runAnalysis() {
            const currencyVal = currencySelect.value;
            if (!currencyVal) return;

            if (periodSelect.value === 'custom') {
                const startDate = parseISODateInput(startDateInput.value);
                const endDate = parseISODateInput(endDateInput.value);
                if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) {
                    alert('Please enter valid dates');
                    return;
                }
                if (startDate > endDate) {
                    alert('Start date must be before end date');
                    return;
                }
                const daysDiff = Math.round((endDate - startDate) / (1000 * 60 * 60 * 24)) + 1;
                if (daysDiff > 365) {
                    alert('Custom period cannot exceed 365 days');
                    return;
                }
                currentCurrencyCode = currentDataSource === 'cbr' ? currencyVal : (currencySelect.selectedOptions[0]?.dataset.base || stripUsdt(currencyVal));
                currentStartDate = formatDate(startDate);
                currentEndDate = formatDate(endDate);
                if (currentDataSource === 'cbr') {
                    loadCurrencyHistoryCustom(currencyVal, startDate, endDate);
                } else {
                    loadCryptoHistoryCustom(currencyVal, startDate, endDate);
                }
                return;
            }

            const period = periodSelect.value;
            const n = parsePeriodDays(period);
            const endDay = startOfLocalDay(new Date());
            const startDay = new Date(endDay);
            startDay.setDate(startDay.getDate() - n);
            currentStartDate = formatDate(startDay);
            currentEndDate = formatDate(endDay);
            currentCurrencyCode = currentDataSource === 'cbr' ? currencyVal : (currencySelect.selectedOptions[0]?.dataset.base || stripUsdt(currencyVal));

            if (currentDataSource === 'cbr') {
                loadCurrencyHistory(currencyVal, period);
            } else {
                loadCryptoHistory(currencyVal, period);
            }
        }

        async function loadCurrencies() {
            try {
                let rates = [];
                for (let i = 0; i <= 7; i++) {
                    const d = new Date(Date.now() - i * 86400000);
                    const ds = formatDate(d);
                    const response = await fetch(`${API_BASE}/rates/cbr?date=${ds}`);
                    const data = await response.json();
                    if (data && data.error) continue;
                    if (Array.isArray(data) && data.length > 0) {
                        rates = data;
                        break;
                    }
                }

                currencySelect.innerHTML = '';
                window.currencyData = {};

                if (rates.length === 0) {
                    const opt = document.createElement('option');
                    opt.value = '';
                    opt.textContent = 'No data (empty database)';
                    opt.disabled = true;
                    currencySelect.appendChild(opt);
                    return;
                }

                rates.sort((a, b) => (a.CurrencyName || '').localeCompare(b.CurrencyName || ''));

                rates.forEach((r) => {
                    const code = r.CurrencyCode;
                    const option = document.createElement('option');
                    option.value = code;
                    window.currencyData[code] = { name: r.CurrencyName, nominal: r.Nominal };
                    const label = `${r.Nominal} ${getCurrencyNameForm(r.CurrencyName, r.Nominal)} (${code})`;
                    option.textContent = label;
                    option.dataset.nominal = String(r.Nominal);
                    currencySelect.appendChild(option);
                });

                if (window.currencyData.USD) {
                    currencySelect.value = 'USD';
                } else {
                    currencySelect.selectedIndex = 0;
                }
                updateDownloadButtonState();
                if (currencySelect.value) {
                    await loadCurrencyHistory(currencySelect.value, periodSelect.value);
                }
            } catch (e) {
                console.error('Error loading currencies list:', e);
                currencySelect.innerHTML = '';
                const opt = document.createElement('option');
                opt.value = '';
                opt.textContent = 'Failed to load currencies';
                opt.disabled = true;
                currencySelect.appendChild(opt);
                alert('Failed to load currencies list. Please try again later.');
            }
        }

        const popularCryptos = [
            { symbol: 'BTC', name: 'Bitcoin' },
            { symbol: 'ETH', name: 'Ethereum' },
            { symbol: 'BNB', name: 'Binance Coin' },
            { symbol: 'SOL', name: 'Solana' },
            { symbol: 'XRP', name: 'XRP' },
            { symbol: 'ADA', name: 'Cardano' },
            { symbol: 'AVAX', name: 'Avalanche' },
            { symbol: 'DOT', name: 'Polkadot' },
            { symbol: 'DOGE', name: 'Dogecoin' },
            { symbol: 'SHIB', name: 'Shiba Inu' },
            { symbol: 'LINK', name: 'Chainlink' },
            { symbol: 'MATIC', name: 'Polygon' },
            { symbol: 'UNI', name: 'Uniswap' },
            { symbol: 'LTC', name: 'Litecoin' },
            { symbol: 'ATOM', name: 'Cosmos' },
            { symbol: 'XTZ', name: 'Tezos' },
            { symbol: 'FIL', name: 'Filecoin' },
            { symbol: 'TRX', name: 'TRON' },
            { symbol: 'ETC', name: 'Ethereum Classic' },
            { symbol: 'NEAR', name: 'NEAR Protocol' },
        ];

        async function loadCryptoSymbols() {
            try {
                const res = await fetch(`${API_BASE}/rates/crypto/symbols`);
                const fromApi = await res.json();
                const apiSet = new Set(
                    Array.isArray(fromApi) ? fromApi.map((s) => String(s).toUpperCase()) : []
                );

                currencySelect.innerHTML = '';
                window.cryptoData = {};

                const list = popularCryptos.filter((c) => apiSet.has(toBinanceSymbol(c.symbol)));

                if (list.length === 0 && Array.isArray(fromApi) && fromApi.length > 0) {
                    fromApi.forEach((sym) => {
                        const s = String(sym).toUpperCase();
                        const base = stripUsdt(s);
                        const option = document.createElement('option');
                        option.value = s;
                        option.dataset.base = base;
                        option.textContent = `${base} (${s})`;
                        window.cryptoData[base] = { name: base, symbol: base, type: 'crypto' };
                        currencySelect.appendChild(option);
                    });
                } else {
                    const useList = list.length > 0 ? list : popularCryptos;
                    useList.forEach((crypto) => {
                        const apiSym = toBinanceSymbol(crypto.symbol);
                        const option = document.createElement('option');
                        option.value = apiSym;
                        option.dataset.base = crypto.symbol;
                        option.textContent = `${crypto.name} (${crypto.symbol})`;
                        window.cryptoData[crypto.symbol] = {
                            name: crypto.name,
                            symbol: crypto.symbol,
                            type: 'crypto',
                        };
                        currencySelect.appendChild(option);
                    });
                }

                const btcPair = toBinanceSymbol('BTC');
                if ([...currencySelect.options].some((o) => o.value === btcPair)) {
                    currencySelect.value = btcPair;
                } else if (currencySelect.options.length) {
                    currencySelect.selectedIndex = 0;
                }

                updateDownloadButtonState();
                if (currencySelect.value) {
                    await loadCryptoHistory(currencySelect.value, periodSelect.value);
                }
            } catch (e) {
                console.error('Error loading crypto symbols:', e);
                alert('Failed to load crypto symbols. Please try again later.');
            }
        }

        function toBinanceSymbol(base) {
            const u = String(base).toUpperCase();
            if (u.endsWith('USDT')) return u;
            return u + 'USDT';
        }

        function stripUsdt(sym) {
            const u = String(sym).toUpperCase();
            return u.endsWith('USDT') ? u.slice(0, -4) : u;
        }

        function getCurrencyNameForm(name, nominal) {
            const currencyNameMap = {
                'Доллар США': 'US Dollar',
                Евро: 'Euro',
                'Российский рубль': 'Russian Ruble',
                'Фунт стерлингов': 'Pound Sterling',
                'Швейцарский франк': 'Swiss Franc',
                'Вона Республики Корея': 'Korean Won',
                Вон: 'Won',
                Сомони: 'Somoni',
                'Вьетнамских донгов': 'Vietnamese Dong',
                Донгов: 'Dong',
                'Индонезийских рупий': 'Indonesian Rupiah',
                'Венгерских форинтов': 'Hungarian Forint',
                Форинтов: 'Forint',
                'Казахстанских тенге': 'Kazakhstani Tenge',
                Тенге: 'Tenge',
                'Индийских рупий': 'Indian Rupee',
                Рупий: 'Rupiah',
                Сомов: 'Som',
                'Киргизских сомов': 'Kyrgyzstani Som',
                'Молдавских леев': 'Moldovan Leu',
                'Чешских крон': 'Czech Koruna',
                'Украинских гривен': 'Ukrainian Hryvnia',
                Гривен: 'Hryvnia',
                Батов: 'Baht',
                'Таиландских батов': 'Thai Baht',
                'Норвежских крон': 'Norwegian Krone',
                'Шведских крон': 'Swedish Krona',
                'Таджикских сомони': 'Tajikistani Somoni',
                'Узбекских сумов': 'Uzbekistani Som',
                'Сербских динаров': 'Serbian Dinar',
                Иен: 'Yen',
                'Японских иен': 'Japanese Yen',
                'Египетских фунтов': 'Egyptian Pound',
                'Гонконгских долларов': 'Hong Kong Dollar',
                'Южноафриканских рэндов': 'South African Rand',
                Рэндов: 'Rand',
                'Турецких лир': 'Turkish Lira',
                'Турецкая лира': 'Turkish Lira',
                'Армянских драмов': 'Armenian Dram',
                'Китайский юань': 'Chinese Yuan',
                Юань: 'Chinese Yuan',
                'Австралийский доллар': 'Australian Dollar',
                'Канадский доллар': 'Canadian Dollar',
                'Сингапурский доллар': 'Singapore Dollar',
                'Гонконгский доллар': 'Hong Kong Dollar',
                'Новозеландский доллар': 'New Zealand Dollar',
                'Датская крона': 'Danish Krone',
                'Болгарский лев': 'Bulgarian Lev',
                'Польский злотый': 'Polish Zloty',
                Злотый: 'Polish Zloty',
                'Бразильский реал': 'Brazilian Real',
                'Белорусский рубль': 'Belarusian Ruble',
                'Азербайджанский манат': 'Azerbaijani Manat',
                'Дирхам ОАЭ': 'UAE Dirham',
                'Катарский риал': 'Qatari Riyal',
                'Румынский лей': 'Romanian Leu',
                Лари: 'Georgian Lari',
                'Новый туркменский манат': 'Turkmenistan Manat',
                'СДР (специальные права заимствования)': 'SDR (Special Drawing Rights)',
            };

            const englishName = currencyNameMap[name] || name;
            const nonPluralCurrencies = [
                'Won',
                'Yen',
                'Japanese Yen',
                'Euro',
                'Tenge',
                'Somoni',
                'Dong',
                'Vietnamese Dong',
                'Indonesian Rupiah',
                'Rupiah',
                'Chinese Yuan',
                'Baht',
                'Thai Baht',
                'Armenian Dram',
            ];

            if (nominal === 1 || nonPluralCurrencies.includes(englishName)) {
                return englishName;
            }

            const specialPlurals = {
                'Pound Sterling': 'Pounds Sterling',
                'Czech Koruna': 'Czech Korunas',
                'Norwegian Krone': 'Norwegian Kroner',
                'Swedish Krona': 'Swedish Kronor',
                'Moldovan Leu': 'Moldovan Lei',
                'Romanian Leu': 'Romanian Lei',
            };

            if (specialPlurals[englishName]) {
                return specialPlurals[englishName];
            }

            return englishName + 's';
        }

        async function loadCurrencyHistory(currencyCode, days) {
            const reqId = beginHistoryLoad();
            try {
                downloadExcelBtn.disabled = true;
                resetMetrics();
                if (currencyChart) {
                    currencyChart.destroy();
                    currencyChart = null;
                }

                loadingIndicator.classList.remove('d-none');
                document.getElementById('currency-chart').classList.add('d-none');
                document.getElementById('loading-progress').style.width = '10%';
                document.getElementById('loading-status').textContent =
                    'Retrieving currency data from database...';

                const n = parsePeriodDays(days);
                const endDay = startOfLocalDay(new Date());
                const startDay = new Date(endDay);
                startDay.setDate(startDay.getDate() - n);
                const startDateStr = formatDate(startDay);
                const endDateStr = formatDate(endDay);
                currentCurrencyCode = currencyCode;
                currentStartDate = startDateStr;
                currentEndDate = endDateStr;

                const response = await fetch(
                    `${API_BASE}/rates/cbr/range?code=${encodeURIComponent(currencyCode)}&from=${startDateStr}&to=${endDateStr}`
                );
                const history = await response.json();

                if (isStaleHistoryRequest(reqId)) return;

                if (!Array.isArray(history) || history.length === 0) {
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    downloadExcelBtn.disabled = true;
                    alert('No data available for the selected period. Try a different period.');
                    resetMetrics();
                    return;
                }

                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';

                history.sort((a, b) =>
                    cbrDateISOFromAPI(a.Date).localeCompare(cbrDateISOFromAPI(b.Date))
                );
                const nominalChanges = checkNominalChanges(history);
                const dates = history.map((item) => cbrDateISOFromAPI(item.Date));
                const values = history.map((item) => item.Value / (item.Nominal || 1));

                const mostRecentItem = history[history.length - 1];
                const currencyInfo = {
                    code: currencyCode,
                    nominal: mostRecentItem.Nominal,
                    name: mostRecentItem.CurrencyName,
                    nominalChanged: nominalChanges.changed,
                    nominalChangeDates: nominalChanges.dates,
                };

                calculateMetrics(values, currencyInfo.nominal);

                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';

                setTimeout(() => {
                    if (isStaleHistoryRequest(reqId)) return;
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    renderChart(dates, values, currencyInfo);
                    window.__lastExport = { kind: 'cbr', rows: history };
                    downloadExcelBtn.disabled = false;
                    if (nominalChanges.changed) {
                        const warningDates = nominalChanges.dates.map((d) => d.date).join(', ');
                        alert(
                            `Note: The nominal value for ${currencyCode} changed on the following dates: ${warningDates}. The chart has been normalized to ensure accurate comparison.`
                        );
                    }
                }, 500);
            } catch (e) {
                if (isStaleHistoryRequest(reqId)) return;
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                downloadExcelBtn.disabled = true;
                console.error('Error loading historical data:', e);
                alert('Failed to load historical data. Please try again later.');
                resetMetrics();
            }
        }

        async function loadCurrencyHistoryCustom(currencyCode, startDate, endDate) {
            const reqId = beginHistoryLoad();
            try {
                downloadExcelBtn.disabled = true;
                resetMetrics();
                if (currencyChart) {
                    currencyChart.destroy();
                    currencyChart = null;
                }

                const startDateStr = formatDate(startDate);
                const endDateStr = formatDate(endDate);
                currentCurrencyCode = currencyCode;
                currentStartDate = startDateStr;
                currentEndDate = endDateStr;

                loadingIndicator.classList.remove('d-none');
                document.getElementById('currency-chart').classList.add('d-none');
                document.getElementById('loading-progress').style.width = '10%';
                document.getElementById('loading-status').textContent =
                    'Retrieving currency data from database...';

                const response = await fetch(
                    `${API_BASE}/rates/cbr/range?code=${encodeURIComponent(currencyCode)}&from=${startDateStr}&to=${endDateStr}`
                );
                const history = await response.json();

                if (isStaleHistoryRequest(reqId)) return;

                if (!Array.isArray(history) || history.length === 0) {
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    downloadExcelBtn.disabled = true;
                    alert('No data available for the selected date range. Try a different period.');
                    resetMetrics();
                    return;
                }

                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';

                history.sort((a, b) =>
                    cbrDateISOFromAPI(a.Date).localeCompare(cbrDateISOFromAPI(b.Date))
                );
                const nominalChanges = checkNominalChanges(history);
                const dates = history.map((item) => cbrDateISOFromAPI(item.Date));
                const values = history.map((item) => item.Value / (item.Nominal || 1));

                const mostRecentItem = history[history.length - 1];
                const currencyInfo = {
                    code: currencyCode,
                    nominal: mostRecentItem.Nominal,
                    name: mostRecentItem.CurrencyName,
                    nominalChanged: nominalChanges.changed,
                    nominalChangeDates: nominalChanges.dates,
                };

                calculateMetrics(values, currencyInfo.nominal);

                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';

                setTimeout(() => {
                    if (isStaleHistoryRequest(reqId)) return;
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    renderChart(dates, values, currencyInfo);
                    window.__lastExport = { kind: 'cbr', rows: history };
                    downloadExcelBtn.disabled = false;
                    if (nominalChanges.changed) {
                        const warningDates = nominalChanges.dates.map((d) => d.date).join(', ');
                        alert(
                            `Note: The nominal value for ${currencyCode} changed on the following dates: ${warningDates}. The chart has been normalized to ensure accurate comparison.`
                        );
                    }
                }, 500);
            } catch (e) {
                if (isStaleHistoryRequest(reqId)) return;
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                downloadExcelBtn.disabled = true;
                console.error('Error loading historical data:', e);
                alert('Failed to load historical data. Please try again later.');
                resetMetrics();
            }
        }

        function pickCryptoPrice(item) {
            if (item.PriceRUB != null && item.PriceRUB !== 0) return item.PriceRUB;
            if (item.price_rub != null && item.price_rub !== 0) return item.price_rub;
            return item.Close;
        }

        async function loadCryptoHistory(apiSymbol, days) {
            const reqId = beginHistoryLoad();
            try {
                downloadExcelBtn.disabled = true;
                resetMetrics();
                if (currencyChart) {
                    currencyChart.destroy();
                    currencyChart = null;
                }

                loadingIndicator.classList.remove('d-none');
                document.getElementById('currency-chart').classList.add('d-none');
                document.getElementById('loading-progress').style.width = '10%';
                document.getElementById('loading-status').textContent = 'Retrieving crypto data...';

                const n = parsePeriodDays(days);
                const endDay = startOfLocalDay(new Date());
                const startDay = new Date(endDay);
                startDay.setDate(startDay.getDate() - n);
                const startDateStr = formatDate(startDay);
                const endDateStr = formatDate(endDay);
                currentCurrencyCode = stripUsdt(apiSymbol);
                currentStartDate = startDateStr;
                currentEndDate = endDateStr;

                const response = await fetch(
                    `${API_BASE}/rates/crypto/history/range?symbol=${encodeURIComponent(apiSymbol)}&from=${startDateStr}&to=${endDateStr}`
                );
                const history = await response.json();

                if (isStaleHistoryRequest(reqId)) return;

                if (!Array.isArray(history) || history.length === 0) {
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    downloadExcelBtn.disabled = true;
                    alert(`No crypto data available for ${apiSymbol} for the selected period.`);
                    resetMetrics();
                    return;
                }

                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';

                history.sort((a, b) => new Date(a.Timestamp) - new Date(b.Timestamp));
                const dates = history.map((item) => {
                    const t = item.Timestamp;
                    if (typeof t === 'string') return t.slice(0, 10);
                    return formatDate(new Date(t));
                });
                const values = history.map((item) => pickCryptoPrice(item));

                const base = stripUsdt(apiSymbol);
                const cryptoInfo = {
                    code: base,
                    name: window.cryptoData[base]?.name || base,
                    type: 'crypto',
                    data: history,
                };

                calculateCryptoMetricsFromValues(values);

                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';

                setTimeout(() => {
                    if (isStaleHistoryRequest(reqId)) return;
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    renderChart(dates, values, cryptoInfo);
                    window.__lastExport = { kind: 'crypto', rows: history };
                    downloadExcelBtn.disabled = false;
                }, 500);
            } catch (e) {
                if (isStaleHistoryRequest(reqId)) return;
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                downloadExcelBtn.disabled = true;
                console.error('Error loading crypto historical data:', e);
                alert(`Failed to load crypto historical data for ${apiSymbol}.`);
                resetMetrics();
            }
        }

        async function loadCryptoHistoryCustom(apiSymbol, startDate, endDate) {
            const reqId = beginHistoryLoad();
            try {
                downloadExcelBtn.disabled = true;
                resetMetrics();
                if (currencyChart) {
                    currencyChart.destroy();
                    currencyChart = null;
                }

                const startDateStr = formatDate(startDate);
                const endDateStr = formatDate(endDate);
                currentCurrencyCode = stripUsdt(apiSymbol);
                currentStartDate = startDateStr;
                currentEndDate = endDateStr;

                loadingIndicator.classList.remove('d-none');
                document.getElementById('currency-chart').classList.add('d-none');
                document.getElementById('loading-progress').style.width = '10%';
                document.getElementById('loading-status').textContent = 'Retrieving crypto data...';

                const response = await fetch(
                    `${API_BASE}/rates/crypto/history/range?symbol=${encodeURIComponent(apiSymbol)}&from=${startDateStr}&to=${endDateStr}`
                );
                const history = await response.json();

                if (isStaleHistoryRequest(reqId)) return;

                if (!Array.isArray(history) || history.length === 0) {
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    downloadExcelBtn.disabled = true;
                    alert(`No crypto data available for ${apiSymbol} for the selected date range.`);
                    resetMetrics();
                    return;
                }

                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';

                history.sort((a, b) => new Date(a.Timestamp) - new Date(b.Timestamp));
                const dates = history.map((item) => {
                    const t = item.Timestamp;
                    if (typeof t === 'string') return t.slice(0, 10);
                    return formatDate(new Date(t));
                });
                const values = history.map((item) => pickCryptoPrice(item));

                const base = stripUsdt(apiSymbol);
                const cryptoInfo = {
                    code: base,
                    name: window.cryptoData[base]?.name || base,
                    type: 'crypto',
                    data: history,
                };

                calculateCryptoMetricsFromValues(values);

                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';

                setTimeout(() => {
                    if (isStaleHistoryRequest(reqId)) return;
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    renderChart(dates, values, cryptoInfo);
                    window.__lastExport = { kind: 'crypto', rows: history };
                    downloadExcelBtn.disabled = false;
                }, 500);
            } catch (e) {
                if (isStaleHistoryRequest(reqId)) return;
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                downloadExcelBtn.disabled = true;
                console.error('Error loading crypto historical data:', e);
                alert(`Failed to load crypto historical data for ${apiSymbol}.`);
                resetMetrics();
            }
        }

        function checkNominalChanges(history) {
            if (!history || history.length <= 1) {
                return { changed: false, dates: [] };
            }
            let prevNominal = history[0].Nominal;
            const changes = [];
            for (let i = 1; i < history.length; i++) {
                const currentNominal = history[i].Nominal;
                if (currentNominal !== prevNominal) {
                    const d = history[i].Date;
                    const dateStr = typeof d === 'string' ? d.slice(0, 10) : formatDate(new Date(d));
                    changes.push({
                        date: dateStr,
                        oldNominal: prevNominal,
                        newNominal: currentNominal,
                    });
                    prevNominal = currentNominal;
                }
            }
            return { changed: changes.length > 0, dates: changes };
        }

        function formatDate(date) {
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            return `${year}-${month}-${day}`;
        }

        function calculateMetrics(values, nominal) {
            if (values.length === 0) {
                resetMetrics();
                return;
            }
            const avg = values.reduce((sum, val) => sum + val, 0) / values.length;
            const min = Math.min(...values);
            const max = Math.max(...values);
            const squaredDiffs = values.map((val) => Math.pow(val - avg, 2));
            const variance = squaredDiffs.reduce((sum, val) => sum + val, 0) / values.length;
            const std = Math.sqrt(variance);
            const volatility = (std / avg) * 100;

            const displayAvg = avg * nominal;
            const displayStd = std * nominal;
            const displayMin = min * nominal;
            const displayMax = max * nominal;

            metricAvg.textContent = displayAvg.toFixed(4) + ' ₽';
            metricStd.textContent = displayStd.toFixed(4) + ' ₽';
            metricMin.textContent = displayMin.toFixed(4) + ' ₽';
            metricMax.textContent = displayMax.toFixed(4) + ' ₽';
            metricVolatility.textContent = volatility.toFixed(2) + '%';
        }

        function calculateCryptoMetricsFromValues(closingPrices) {
            if (closingPrices.length === 0) {
                resetMetrics();
                return;
            }
            const avg = closingPrices.reduce((sum, val) => sum + val, 0) / closingPrices.length;
            const min = Math.min(...closingPrices);
            const max = Math.max(...closingPrices);
            const squaredDiffs = closingPrices.map((val) => Math.pow(val - avg, 2));
            const variance = squaredDiffs.reduce((sum, val) => sum + val, 0) / closingPrices.length;
            const std = Math.sqrt(variance);
            const volatility = (std / avg) * 100;

            const formatPrice = (price) => price.toFixed(2);
            metricAvg.textContent = `${formatPrice(avg)} ₽`;
            metricStd.textContent = `${formatPrice(std)} ₽`;
            metricMin.textContent = `${formatPrice(min)} ₽`;
            metricMax.textContent = `${formatPrice(max)} ₽`;
            metricVolatility.textContent = `${volatility.toFixed(2)}%`;
        }

        function resetMetrics() {
            metricAvg.textContent = '-';
            metricStd.textContent = '-';
            metricMin.textContent = '-';
            metricMax.textContent = '-';
            metricVolatility.textContent = '-';
        }

        function updateUILabels() {
            const currencyLabel = document.querySelector('label[for="currency-select"]');
            if (currentDataSource === 'cbr') {
                currencyLabel.textContent = 'Select Currency:';
                currencySelect.innerHTML =
                    '<option value="" selected disabled>Loading currencies...</option>';
            } else {
                currencyLabel.textContent = 'Select Cryptocurrency:';
                currencySelect.innerHTML =
                    '<option value="" selected disabled>Loading cryptocurrencies...</option>';
            }
        }

        function renderChart(dates, values, currencyInfo) {
            if (currencyChart) {
                currencyChart.destroy();
            }

            const isCrypto = currencyInfo.type === 'crypto';
            let chartLabel;
            let displayValues;
            let yAxisLabel;
            let tooltipCallback;

            let chartDates;
            let chartValues;
            let originalDates;

            if (isCrypto) {
                chartLabel = `${currencyInfo.code} Price (RUB)`;
                displayValues = values;
                yAxisLabel = 'Price (RUB)';
                tooltipCallback = function (context) {
                    const price = context.raw;
                    return `Price: ${price.toFixed(2)} ₽`;
                };
            } else {
                const currencyNameForm = getCurrencyNameForm(currencyInfo.name, currencyInfo.nominal);
                chartLabel = `${currencyInfo.nominal} ${currencyNameForm} to RUB`;
                if (currencyInfo.nominalChanged) {
                    chartLabel += ' (normalized)';
                }
                displayValues = values.map((value) => value * currencyInfo.nominal);
                yAxisLabel = 'Rate (₽)';
                tooltipCallback = function (context) {
                    let label = `Rate: ${context.raw.toFixed(4)} ₽ for ${currencyInfo.nominal} ${currencyNameForm}`;
                    if (currencyInfo.nominalChanged) {
                        const idx = chartDates.indexOf(context.label);
                        if (idx >= 0) {
                            const originalDate = originalDates[idx];
                            const od =
                                typeof originalDate === 'string'
                                    ? originalDate.slice(0, 10)
                                    : formatDate(new Date(originalDate));
                            const changeInfo = currencyInfo.nominalChangeDates.find((d) => d.date === od);
                            if (changeInfo) {
                                label += ` (Nominal changed: ${changeInfo.oldNominal} → ${changeInfo.newNominal})`;
                            }
                        }
                    }
                    return label;
                };
            }

            const formattedDates = dates.map(formatDateForDisplayYMD);
            chartDates = formattedDates;
            chartValues = displayValues;
            originalDates = dates;

            if (dates.length > 60) {
                const reduced = reduceDataPoints(formattedDates, displayValues, dates);
                chartDates = reduced.reducedLabels;
                chartValues = reduced.reducedData;
                originalDates = reduced.reducedOriginals;
            }

            currencyChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: chartDates,
                    datasets: [
                        {
                            label: chartLabel,
                            data: chartValues,
                            borderColor: isCrypto ? '#ff6b35' : '#0d6efd',
                            backgroundColor: isCrypto ? 'rgba(255, 107, 53, 0.1)' : 'rgba(13, 110, 253, 0.1)',
                            borderWidth: 2,
                            fill: true,
                            tension: 0.1,
                        },
                    ],
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { position: 'top' },
                        tooltip: {
                            callbacks: { label: tooltipCallback },
                        },
                    },
                    scales: {
                        x: {
                            title: { display: true, text: 'Date' },
                            ticks: {
                                maxRotation: 45,
                                minRotation: 45,
                                autoSkip: true,
                                maxTicksLimit: 20,
                            },
                            reverse: false,
                        },
                        y: {
                            title: { display: true, text: yAxisLabel },
                            ticks: {
                                callback: function (value) {
                                    return value.toFixed(2) + ' ₽';
                                },
                            },
                            grid: { color: 'rgba(0, 0, 0, 0.1)' },
                        },
                    },
                    interaction: { intersect: false, mode: 'index' },
                },
            });
        }

        function reduceDataPoints(labels, data, originalDatesIn) {
            if (labels.length <= 60) {
                return {
                    reducedLabels: labels,
                    reducedData: data,
                    reducedOriginals: originalDatesIn,
                };
            }
            const stepSize = Math.ceil(labels.length / 60);
            const reducedLabels = [];
            const reducedData = [];
            const reducedOriginals = [];
            for (let i = 0; i < labels.length; i += stepSize) {
                reducedLabels.push(labels[i]);
                reducedData.push(data[i]);
                reducedOriginals.push(originalDatesIn[i]);
            }
            const lastIndex = labels.length - 1;
            if ((labels.length - 1) % stepSize !== 0) {
                reducedLabels.push(labels[lastIndex]);
                reducedData.push(data[lastIndex]);
                reducedOriginals.push(originalDatesIn[lastIndex]);
            }
            return {
                reducedLabels,
                reducedData,
                reducedOriginals,
            };
        }

        function exportReportToCSV() {
            const exp = window.__lastExport;
            if (!exp || !exp.rows || !exp.rows.length) return;

            let rows;
            let filename;
            if (exp.kind === 'cbr') {
                rows = [['Date', 'Code', 'Name', 'Nominal', 'Value (RUB)', 'Previous']];
                exp.rows.forEach((r) => {
                    const d = r.Date;
                    const ds = typeof d === 'string' ? d.slice(0, 10) : formatDate(new Date(d));
                    rows.push([
                        ds,
                        r.CurrencyCode,
                        r.CurrencyName,
                        r.Nominal,
                        r.Value,
                        r.Previous,
                    ]);
                });
                filename = `cbr_${currentCurrencyCode || 'export'}_${currentStartDate}_${currentEndDate}.csv`;
            } else {
                rows = [['Timestamp', 'Symbol', 'Open', 'High', 'Low', 'Close', 'Volume', 'Price (RUB)']];
                exp.rows.forEach((r) => {
                    const ts = r.Timestamp;
                    const tsIso =
                        typeof ts === 'string' ? ts : ts instanceof Date ? ts.toISOString() : String(ts);
                    rows.push([
                        tsIso,
                        r.Symbol,
                        r.Open,
                        r.High,
                        r.Low,
                        r.Close,
                        r.Volume,
                        pickCryptoPrice(r),
                    ]);
                });
                filename = `crypto_${stripUsdt(exp.rows[0].Symbol || 'export')}_${currentStartDate}_${currentEndDate}.csv`;
            }

            const csv = rows
                .map((r) => r.map((v) => `"${String(v ?? '').replace(/"/g, '""')}"`).join(','))
                .join('\n');
            const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8;' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);
        }

        initAuthUI();
    });

    function initAuthUI() {
        let token = localStorage.getItem('token') || '';
        let authMode = 'login';
        const authModalEl = document.getElementById('authModal');
        if (!authModalEl) return;

        const authModal = new bootstrap.Modal(authModalEl);

        window.updateAuthUI = function () {
            const label = document.getElementById('userLabel');
            const btn = document.getElementById('authBtn');
            if (!label || !btn) return;
            if (token) {
                try {
                    const payload = JSON.parse(atob(token.split('.')[1]));
                    label.textContent = payload.email || 'user';
                } catch {
                    label.textContent = 'user';
                }
                btn.textContent = 'Log out';
            } else {
                label.textContent = '';
                btn.textContent = 'Log in';
            }
        };

        window.handleAuthBtn = function () {
            if (token) {
                logout();
            } else {
                authModal.show();
            }
        };

        window.toggleAuthMode = function () {
            authMode = authMode === 'login' ? 'register' : 'login';
            const isLogin = authMode === 'login';
            document.getElementById('authModalTitle').textContent = isLogin ? 'Log in' : 'Register';
            document.getElementById('authToggleText').textContent = isLogin
                ? 'No account?'
                : 'Already have an account?';
            document.getElementById('authToggleLink').textContent = isLogin ? 'Sign up' : 'Log in';
            document.getElementById('authMsg').innerHTML = '';
        };

        window.submitAuth = async function () {
            const email = document.getElementById('authEmail').value.trim();
            const password = document.getElementById('authPassword').value;
            const msgEl = document.getElementById('authMsg');
            if (!email || !password) {
                msgEl.innerHTML =
                    '<div class="alert alert-danger py-2 small mb-2">Please fill in all fields</div>';
                return;
            }
            const endpoint = authMode === 'login' ? '/auth/login' : '/auth/register';
            try {
                const res = await fetch((window.API_BASE || 'http://localhost:8080') + endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ email, password }),
                });
                const data = await res.json();
                if (!res.ok) {
                    msgEl.innerHTML = `<div class="alert alert-danger py-2 small mb-2">${data.error || 'Error'}</div>`;
                    return;
                }
                token = data.token;
                localStorage.setItem('token', token);
                window.updateAuthUI();
                authModal.hide();
            } catch {
                msgEl.innerHTML =
                    '<div class="alert alert-danger py-2 small mb-2">Connection error</div>';
            }
        };

        async function logout() {
            try {
                await fetch((window.API_BASE || 'http://localhost:8080') + '/auth/logout', {
                    method: 'POST',
                    headers: { Authorization: 'Bearer ' + token },
                });
            } catch {}
            token = '';
            localStorage.removeItem('token');
            window.updateAuthUI();
        }

        window.updateAuthUI();
    }
})();
