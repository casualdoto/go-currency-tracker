document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
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
    
    // Variables to store current analysis parameters
    let currentDataSource = 'cbr';
    let currentCurrencyCode = '';
    let currentStartDate = '';
    let currentEndDate = '';
    
    // Chart context
    const ctx = document.getElementById('currency-chart').getContext('2d');
    let currencyChart = null;
    
    // Initialize date inputs with reasonable defaults
    const today = new Date();
    endDateInput.value = formatDate(today);
    
    // Default start date is 30 days ago
    const defaultStartDate = new Date();
    defaultStartDate.setDate(today.getDate() - 30);
    startDateInput.value = formatDate(defaultStartDate);
    
    // Load currencies list when page loads
    loadCurrencies();
    
    // Data source change handler
    dataSourceSelect.addEventListener('change', function() {
        currentDataSource = this.value;
        
        // Clear current currency selection
        currencySelect.innerHTML = '<option value="" selected disabled>Loading...</option>';
        
        // Reset form and chart
        resetMetrics();
        if (currencyChart) {
            currencyChart.destroy();
            currencyChart = null;
        }
        
        // Update UI labels based on data source
        updateUILabels();
        
        // Load appropriate data based on source
        if (currentDataSource === 'cbr') {
            loadCurrencies();
        } else if (currentDataSource === 'crypto') {
            loadCryptoSymbols();
        }
    });
    
    // Enable Excel download button when currency or period changes
    currencySelect.addEventListener('change', updateExcelDownloadButton);
    periodSelect.addEventListener('change', function() {
        if (this.value === 'custom') {
            customPeriodDiv.classList.remove('d-none');
        } else {
            customPeriodDiv.classList.add('d-none');
        }
        updateExcelDownloadButton();
    });
    
    // Update Excel download button when custom dates change
    startDateInput.addEventListener('change', updateExcelDownloadButton);
    endDateInput.addEventListener('change', updateExcelDownloadButton);
    
    // Function to update Excel download button state
    function updateExcelDownloadButton() {
        const currencyCode = currencySelect.value;
        
        if (currencyCode) {
            if (periodSelect.value === 'custom') {
                // Custom date range
                const startDate = new Date(startDateInput.value);
                const endDate = new Date(endDateInput.value);
                
                // Validate dates
                if (isNaN(startDate.getTime()) || isNaN(endDate.getTime()) || startDate > endDate) {
                    downloadExcelBtn.disabled = true;
                    return;
                }
                
                // Calculate days between dates
                const daysDiff = Math.round((endDate - startDate) / (1000 * 60 * 60 * 24)) + 1;
                
                // Limit to 365 days
                if (daysDiff > 365) {
                    downloadExcelBtn.disabled = true;
                    return;
                }
                
                // Save current parameters for Excel download
                currentCurrencyCode = currencyCode;
                currentStartDate = formatDate(startDate);
                currentEndDate = formatDate(endDate);
            } else {
                // Standard period
                const period = periodSelect.value;
                
                // Calculate start date based on period
                const endDate = new Date();
                const startDate = new Date();
                startDate.setDate(endDate.getDate() - parseInt(period));
                
                // Save current parameters for Excel download
                currentCurrencyCode = currencyCode;
                currentStartDate = formatDate(startDate);
                currentEndDate = formatDate(endDate);
            }
            
            // Enable Excel download button
            downloadExcelBtn.disabled = false;
        } else {
            downloadExcelBtn.disabled = true;
        }
    }
    
    // Form submit handler
    currencyForm.addEventListener('submit', function(e) {
        e.preventDefault();
        const currencyCode = currencySelect.value;
        
        if (currencyCode) {
            if (periodSelect.value === 'custom') {
                // Custom date range
                const startDate = new Date(startDateInput.value);
                const endDate = new Date(endDateInput.value);
                
                // Validate dates
                if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) {
                    alert('Please enter valid dates');
                    return;
                }
                
                if (startDate > endDate) {
                    alert('Start date must be before end date');
                    return;
                }
                
                // Calculate days between dates
                const daysDiff = Math.round((endDate - startDate) / (1000 * 60 * 60 * 24)) + 1;
                
                // Limit to 365 days
                if (daysDiff > 365) {
                    alert('Custom period cannot exceed 365 days');
                    return;
                }
                
                // Save current parameters for Excel download
                currentCurrencyCode = currencyCode;
                currentStartDate = formatDate(startDate);
                currentEndDate = formatDate(endDate);
                
                // Load data for custom period based on source
                if (currentDataSource === 'cbr') {
                    loadCurrencyHistoryCustom(currencyCode, startDate, endDate);
                } else if (currentDataSource === 'crypto') {
                    loadCryptoHistoryCustom(currencyCode, startDate, endDate);
                }
            } else {
                // Standard period
                const period = periodSelect.value;
                
                // Calculate start date based on period
                const endDate = new Date();
                const startDate = new Date();
                startDate.setDate(endDate.getDate() - parseInt(period));
                
                // Save current parameters for Excel download
                currentCurrencyCode = currencyCode;
                currentStartDate = formatDate(startDate);
                currentEndDate = formatDate(endDate);
                
                // Load data based on source
                if (currentDataSource === 'cbr') {
                    loadCurrencyHistory(currencyCode, period);
                } else if (currentDataSource === 'crypto') {
                    loadCryptoHistory(currencyCode, period);
                }
            }
        }
    });
    
    // Excel download button handler
    downloadExcelBtn.addEventListener('click', function() {
        if (currentCurrencyCode && currentStartDate && currentEndDate) {
            let excelUrl;
            if (currentDataSource === 'cbr') {
                excelUrl = `/rates/cbr/history/range/excel?code=${currentCurrencyCode}&start_date=${currentStartDate}&end_date=${currentEndDate}`;
            } else if (currentDataSource === 'crypto') {
                excelUrl = `/rates/crypto/history/range/excel?symbol=${currentCurrencyCode}&start_date=${currentStartDate}&end_date=${currentEndDate}`;
            }
            
            if (excelUrl) {
                window.location.href = excelUrl;
            }
        }
    });
    
    // Load available currencies
    async function loadCurrencies() {
        try {
            const response = await fetch('/rates/cbr');
            const data = await response.json();
            
            if (data.success && data.data) {
                // Clear dropdown
                currencySelect.innerHTML = '';
                
                // Add options for each currency
                const currencies = Object.entries(data.data).sort((a, b) => {
                    return a[1].Name.localeCompare(b[1].Name);
                });
                
                // Store currency data globally for later use
                window.currencyData = {};
                
                currencies.forEach(([code, currency]) => {
                    const option = document.createElement('option');
                    option.value = code;
                    
                    // Store currency data for later use
                    window.currencyData[code] = {
                        name: currency.Name,
                        nominal: currency.Nominal
                    };
                    
                    // Format currency name with correct nominal form
                    let currencyNameForm = getCurrencyNameForm(currency.Name, currency.Nominal);
                    option.textContent = `${currency.Nominal} ${currencyNameForm} (${code})`;
                    
                    // Store nominal in data attribute for easy access
                    option.dataset.nominal = currency.Nominal;
                    currencySelect.appendChild(option);
                });
                
                // Select USD by default
                if (data.data.USD) {
                    currencySelect.value = 'USD';
                    
                    // Update Excel download button state
                    updateExcelDownloadButton();
                    
                    // Load data for USD for a week by default
                    loadCurrencyHistory('USD', 7);
                }
            }
        } catch (error) {
            console.error('Error loading currencies list:', error);
            alert('Failed to load currencies list. Please try again later.');
        }
    }
    
    // Load available crypto symbols - using fixed list of popular cryptocurrencies
    async function loadCryptoSymbols() {
        try {
            // Fixed list of popular cryptocurrencies
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
                { symbol: 'NEAR', name: 'NEAR Protocol' }
            ];
            
            // Clear dropdown
            currencySelect.innerHTML = '';
            
            // Store crypto data globally for later use
            window.cryptoData = {};
            
            popularCryptos.forEach(crypto => {
                const option = document.createElement('option');
                option.value = crypto.symbol;
                option.textContent = `${crypto.name} (${crypto.symbol})`;
                
                // Store symbol data for later use
                window.cryptoData[crypto.symbol] = {
                    name: crypto.name,
                    symbol: crypto.symbol,
                    type: 'crypto'
                };
                
                currencySelect.appendChild(option);
            });
            
            // Select BTC by default
            currencySelect.value = 'BTC';
            
            // Update Excel download button state
            updateExcelDownloadButton();
            
            // Load data for BTC for a week by default
            loadCryptoHistory('BTC', 7);
        } catch (error) {
            console.error('Error loading crypto symbols:', error);
            alert('Failed to load crypto symbols. Please try again later.');
        }
    }
    
    // Function to get the correct form of currency name based on nominal
    function getCurrencyNameForm(name, nominal) {
        // Simple mapping of Russian currency names to English
        const currencyNameMap = {
            'Доллар США': 'US Dollar',
            'Евро': 'Euro',
            'Российский рубль': 'Russian Ruble',
            'Фунт стерлингов': 'Pound Sterling',
            'Швейцарский франк': 'Swiss Franc',
            'Вона Республики Корея': 'Korean Won',
            'Вон': 'Won',
            'Сомони': 'Somoni',
            'Вьетнамских донгов': 'Vietnamese Dong',
            'Донгов': 'Dong',
            'Индонезийских рупий': 'Indonesian Rupiah',
            'Венгерских форинтов': 'Hungarian Forint',
            'Форинтов': 'Forint',
            'Казахстанских тенге': 'Kazakhstani Tenge',
            'Тенге': 'Tenge',
            'Индийских рупий': 'Indian Rupee',
            'Рупий': 'Rupiah',
            'Сомов': 'Som',
            'Киргизских сомов': 'Kyrgyzstani Som',
            'Молдавских леев': 'Moldovan Leu',
            'Чешских крон': 'Czech Koruna',
            'Украинских гривен': 'Ukrainian Hryvnia',
            'Гривен': 'Hryvnia',
            'Батов': 'Baht',
            'Таиландских батов': 'Thai Baht',
            'Норвежских крон': 'Norwegian Krone',
            'Шведских крон': 'Swedish Krona',
            'Таджикских сомони': 'Tajikistani Somoni',
            'Узбекских сумов': 'Uzbekistani Som',
            'Сербских динаров': 'Serbian Dinar',
            'Иен': 'Yen',
            'Японских иен': 'Japanese Yen',
            'Египетских фунтов': 'Egyptian Pound',
            'Гонконгских долларов': 'Hong Kong Dollar',
            'Южноафриканских рэндов': 'South African Rand',
            'Рэндов': 'Rand',
            'Турецких лир': 'Turkish Lira',
            'Турецкая лира': 'Turkish Lira',
            'Армянских драмов': 'Armenian Dram',
            'Китайский юань': 'Chinese Yuan',
            'Юань': 'Chinese Yuan',
            'Австралийский доллар': 'Australian Dollar',
            'Канадский доллар': 'Canadian Dollar',
            'Сингапурский доллар': 'Singapore Dollar',
            'Гонконгский доллар': 'Hong Kong Dollar',
            'Новозеландский доллар': 'New Zealand Dollar',
            'Датская крона': 'Danish Krone',
            'Болгарский лев': 'Bulgarian Lev',
            'Польский злотый': 'Polish Zloty',
            'Злотый': 'Polish Zloty',
            'Бразильский реал': 'Brazilian Real',
            'Белорусский рубль': 'Belarusian Ruble',
            'Азербайджанский манат': 'Azerbaijani Manat',
            'Дирхам ОАЭ': 'UAE Dirham',
            'Катарский риал': 'Qatari Riyal',
            'Румынский лей': 'Romanian Leu',
            'Лари': 'Georgian Lari',
            'Новый туркменский манат': 'Turkmenistan Manat',
            'СДР (специальные права заимствования)': 'SDR (Special Drawing Rights)'
        };
        
        // Get English name if available, otherwise use the original name
        const englishName = currencyNameMap[name] || name;
        
        // For currencies with specific nominals, we don't need to handle plurals
        const nonPluralCurrencies = [
            'Won', 'Yen', 'Japanese Yen', 'Euro', 'Tenge', 'Somoni', 
            'Dong', 'Vietnamese Dong', 'Indonesian Rupiah', 'Rupiah',
            'Chinese Yuan', 'Baht', 'Thai Baht', 'Armenian Dram'
        ];
        
        // If nominal is 1 or the currency doesn't have plural form, return as is
        if (nominal === 1 || nonPluralCurrencies.includes(englishName)) {
            return englishName;
        }
        
        // Special plural forms
        const specialPlurals = {
            'Pound Sterling': 'Pounds Sterling',
            'Czech Koruna': 'Czech Korunas',
            'Norwegian Krone': 'Norwegian Kroner',
            'Swedish Krona': 'Swedish Kronor',
            'Moldovan Leu': 'Moldovan Lei',
            'Romanian Leu': 'Romanian Lei'
        };
        
        // Check for special plural forms
        if (specialPlurals[englishName]) {
            return specialPlurals[englishName];
        }
        
        // Default plural form (add 's')
        return englishName + 's';
    }
    
    // Load historical data for a specific number of days
    async function loadCurrencyHistory(currencyCode, days) {
        try {
            // Reset metrics and destroy existing chart
            resetMetrics();
            if (currencyChart) {
                currencyChart.destroy();
                currencyChart = null;
            }
            
            // Show loading indicator
            loadingIndicator.classList.remove('d-none');
            document.getElementById('currency-chart').classList.add('d-none');
            document.getElementById('loading-progress').style.width = '10%';
            document.getElementById('loading-status').textContent = 'Retrieving currency data from database...';
            
            // Calculate dates for API request
            const endDate = new Date();
            const startDate = new Date();
            startDate.setDate(endDate.getDate() - parseInt(days));
            
            const startDateStr = formatDate(startDate);
            const endDateStr = formatDate(endDate);
            
            // Request to API for historical data
            const response = await fetch(`/rates/cbr/history/range?code=${currencyCode}&start_date=${startDateStr}&end_date=${endDateStr}`);
            const data = await response.json();
            
            if (data.success && data.data && data.data.length > 0) {
                // Update loading progress
                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';
                
                // Check if we have data for all days in the range (excluding weekends/holidays)
                const expectedDaysCount = getExpectedBusinessDays(startDate, endDate);
                const actualDaysCount = data.data.length;
                
                // If we don't have enough data, wait a bit and try again
                if (actualDaysCount < expectedDaysCount * 0.9) { // Allow for 10% missing days
                    document.getElementById('loading-status').textContent = `Loading more data (${actualDaysCount}/${expectedDaysCount} days)...`;
                    document.getElementById('loading-progress').style.width = `${(actualDaysCount / expectedDaysCount) * 80}%`;
                    
                    setTimeout(() => {
                        loadCurrencyHistory(currencyCode, days);
                    }, 1000); // Wait 1 second before retrying
                    return;
                }
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '80%';
                document.getElementById('loading-status').textContent = 'Generating chart...';
                
                // Extract dates and values from response
                const history = data.data;
                
                // Sort by date (ascending)
                history.sort((a, b) => new Date(a.date) - new Date(b.date));
                
                // Check if nominal changes in the data
                const nominalChanges = checkNominalChanges(history);
                
                const dates = history.map(item => item.date);
                
                // Normalize values based on nominal to ensure consistent comparison
                const values = history.map(item => {
                    // Calculate the value per unit of currency
                    return item.value / item.nominal;
                });
                
                // Get currency info for chart display - use the most recent nominal
                const mostRecentItem = history[history.length - 1];
                const currencyInfo = {
                    code: currencyCode,
                    nominal: mostRecentItem.nominal,
                    name: mostRecentItem.name,
                    nominalChanged: nominalChanges.changed,
                    nominalChangeDates: nominalChanges.dates
                };
                
                // Calculate metrics based on normalized values
                calculateMetrics(values, currencyInfo.nominal);
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';
                
                // Small delay to show the 100% progress
                setTimeout(() => {
                    // Hide loading indicator
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    
                    // Render chart with normalized values
                    renderChart(dates, values, currencyInfo);
                    
                    // Show warning if nominal changed
                    if (nominalChanges.changed) {
                        const warningDates = nominalChanges.dates.map(d => d.date).join(', ');
                        alert(`Note: The nominal value for ${currencyCode} changed on the following dates: ${warningDates}. The chart has been normalized to ensure accurate comparison.`);
                    }
                }, 500);
                
                // Enable Excel download button
                downloadExcelBtn.disabled = false;
            } else {
                // Hide loading indicator
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                
                // Disable Excel download button
                downloadExcelBtn.disabled = true;
                
                alert('No data available for the selected period. Try a different period.');
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            // Disable Excel download button
            downloadExcelBtn.disabled = true;
            
            console.error('Error loading historical data:', error);
            alert('Failed to load historical data. Please try again later.');
            resetMetrics();
        }
    }
    
    // Load historical data for custom date range
    async function loadCurrencyHistoryCustom(currencyCode, startDate, endDate) {
        try {
            // Show loading indicator
            resetMetrics();
            if (currencyChart) {
                currencyChart.destroy();
                currencyChart = null;
            }
            
            // Show loading indicator
            loadingIndicator.classList.remove('d-none');
            document.getElementById('currency-chart').classList.add('d-none');
            document.getElementById('loading-progress').style.width = '10%';
            document.getElementById('loading-status').textContent = 'Retrieving currency data from database...';
            
            // Format dates for API request
            const startDateStr = formatDate(startDate);
            const endDateStr = formatDate(endDate);
            
            // Request to API for historical data using the new endpoint
            const response = await fetch(`/rates/cbr/history/range?code=${currencyCode}&start_date=${startDateStr}&end_date=${endDateStr}`);
            const data = await response.json();
            
            if (data.success && data.data && data.data.length > 0) {
                // Update loading progress
                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';
                
                // Check if we have data for all days in the range (excluding weekends/holidays)
                const expectedDaysCount = getExpectedBusinessDays(startDate, endDate);
                const actualDaysCount = data.data.length;
                
                // If we don't have enough data, wait a bit and try again
                if (actualDaysCount < expectedDaysCount * 0.9) { // Allow for 10% missing days
                    document.getElementById('loading-status').textContent = `Loading more data (${actualDaysCount}/${expectedDaysCount} days)...`;
                    document.getElementById('loading-progress').style.width = `${(actualDaysCount / expectedDaysCount) * 80}%`;
                    
                    setTimeout(() => {
                        loadCurrencyHistoryCustom(currencyCode, startDate, endDate);
                    }, 1000); // Wait 1 second before retrying
                    return;
                }
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '80%';
                document.getElementById('loading-status').textContent = 'Generating chart...';
                
                // Extract dates and values from response
                const history = data.data;
                
                // Sort by date (ascending)
                history.sort((a, b) => new Date(a.date) - new Date(b.date));
                
                // Check if nominal changes in the data
                const nominalChanges = checkNominalChanges(history);
                
                const dates = history.map(item => item.date);
                
                // Normalize values based on nominal to ensure consistent comparison
                const values = history.map(item => {
                    // Calculate the value per unit of currency
                    return item.value / item.nominal;
                });
                
                // Get currency info for chart display - use the most recent nominal
                const mostRecentItem = history[history.length - 1];
                const currencyInfo = {
                    code: currencyCode,
                    nominal: mostRecentItem.nominal,
                    name: mostRecentItem.name,
                    nominalChanged: nominalChanges.changed,
                    nominalChangeDates: nominalChanges.dates
                };
                
                // Calculate metrics based on normalized values
                calculateMetrics(values, currencyInfo.nominal);
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';
                
                // Small delay to show the 100% progress
                setTimeout(() => {
                    // Hide loading indicator
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    
                    // Render chart with normalized values
                    renderChart(dates, values, currencyInfo);
                    
                    // Show warning if nominal changed
                    if (nominalChanges.changed) {
                        const warningDates = nominalChanges.dates.map(d => d.date).join(', ');
                        alert(`Note: The nominal value for ${currencyCode} changed on the following dates: ${warningDates}. The chart has been normalized to ensure accurate comparison.`);
                    }
                }, 500);
                
                // Enable Excel download button
                downloadExcelBtn.disabled = false;
            } else {
                // Hide loading indicator
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                
                // Disable Excel download button
                downloadExcelBtn.disabled = true;
                
                alert('No data available for the selected date range. Try a different period.');
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            // Disable Excel download button
            downloadExcelBtn.disabled = true;
            
            console.error('Error loading historical data:', error);
            alert('Failed to load historical data. Please try again later.');
            resetMetrics();
        }
    }
    
    // Load crypto historical data
    async function loadCryptoHistory(symbol, days) {
        try {
            // Show loading indicator
            resetMetrics();
            if (currencyChart) {
                currencyChart.destroy();
                currencyChart = null;
            }
            
            // Show loading indicator
            loadingIndicator.classList.remove('d-none');
            document.getElementById('currency-chart').classList.add('d-none');
            document.getElementById('loading-progress').style.width = '10%';
            document.getElementById('loading-status').textContent = 'Retrieving crypto data...';
            
            // Request to API for historical data
            const response = await fetch(`/rates/crypto/history?symbol=${symbol}&days=${days}`);
            const data = await response.json();
            
            console.log('Crypto history API response:', data);
            
            if (data.success && data.data && data.data.length > 0) {
                // Update loading progress
                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';
                
                // Extract dates and values from response
                const history = data.data;
                
                // Sort by date (ascending)
                history.sort((a, b) => new Date(a.date) - new Date(b.date));
                
                console.log('Processing crypto history data, first item:', history[0]);
                
                const dates = history.map(item => {
                    // Convert date format from "2006-01-02 15:04:05" to "2006-01-02"
                    return item.date.split(' ')[0];
                });
                const values = history.map(item => item.close); // Use closing price (now in RUB)
                
                console.log('Processed dates:', dates.slice(0, 3), 'values:', values.slice(0, 3));
                
                // Crypto info for chart display
                const cryptoInfo = {
                    code: symbol,
                    name: window.cryptoData[symbol]?.name || symbol,
                    type: 'crypto',
                    data: history // Store full OHLC data
                };
                
                // Calculate metrics based on closing prices
                calculateCryptoMetrics(history);
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';
                
                // Small delay to show the 100% progress
                setTimeout(() => {
                    // Hide loading indicator
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    
                    // Render chart with crypto data
                    renderChart(dates, values, cryptoInfo);
                }, 500);
                
                // Enable Excel download button
                downloadExcelBtn.disabled = false;
            } else {
                // Hide loading indicator
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                
                // Disable Excel download button
                downloadExcelBtn.disabled = true;
                
                console.log('No crypto data available:', data);
                alert(`No crypto data available for ${symbol} for the selected period. Error: ${data.error || 'Unknown error'}`);
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            // Disable Excel download button
            downloadExcelBtn.disabled = true;
            
            console.error('Error loading crypto historical data:', error);
            alert(`Failed to load crypto historical data for ${symbol}. Error: ${error.message}`);
            resetMetrics();
        }
    }
    
    // Load crypto historical data for custom date range
    async function loadCryptoHistoryCustom(symbol, startDate, endDate) {
        try {
            // Show loading indicator
            resetMetrics();
            if (currencyChart) {
                currencyChart.destroy();
                currencyChart = null;
            }
            
            // Show loading indicator
            loadingIndicator.classList.remove('d-none');
            document.getElementById('currency-chart').classList.add('d-none');
            document.getElementById('loading-progress').style.width = '10%';
            document.getElementById('loading-status').textContent = 'Retrieving crypto data...';
            
            // Format dates for API request
            const startDateStr = formatDate(startDate);
            const endDateStr = formatDate(endDate);
            
            // Request to API for historical data using the new endpoint
            const response = await fetch(`/rates/crypto/history/range?symbol=${symbol}&start_date=${startDateStr}&end_date=${endDateStr}`);
            const data = await response.json();
            
            console.log('Crypto history range API response:', data);
            
            if (data.success && data.data && data.data.length > 0) {
                // Update loading progress
                document.getElementById('loading-progress').style.width = '50%';
                document.getElementById('loading-status').textContent = 'Processing data...';
                
                // Extract dates and values from response
                const history = data.data;
                
                // Sort by date (ascending)
                history.sort((a, b) => new Date(a.date) - new Date(b.date));
                
                const dates = history.map(item => {
                    // Convert date format from "2006-01-02 15:04:05" to "2006-01-02"
                    return item.date.split(' ')[0];
                });
                const values = history.map(item => item.close); // Use closing price (now in RUB)
                
                // Crypto info for chart display
                const cryptoInfo = {
                    code: symbol,
                    name: window.cryptoData[symbol]?.name || symbol,
                    type: 'crypto',
                    data: history // Store full OHLC data
                };
                
                // Calculate metrics based on closing prices
                calculateCryptoMetrics(history);
                
                // Update loading progress
                document.getElementById('loading-progress').style.width = '100%';
                document.getElementById('loading-status').textContent = 'Completed!';
                
                // Small delay to show the 100% progress
                setTimeout(() => {
                    // Hide loading indicator
                    loadingIndicator.classList.add('d-none');
                    document.getElementById('currency-chart').classList.remove('d-none');
                    
                    // Render chart with crypto data
                    renderChart(dates, values, cryptoInfo);
                }, 500);
                
                // Enable Excel download button
                downloadExcelBtn.disabled = false;
            } else {
                // Hide loading indicator
                loadingIndicator.classList.add('d-none');
                document.getElementById('currency-chart').classList.remove('d-none');
                
                // Disable Excel download button
                downloadExcelBtn.disabled = true;
                
                console.log('No crypto data available for date range:', data);
                alert(`No crypto data available for ${symbol} for the selected date range. Error: ${data.error || 'Unknown error'}`);
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            // Disable Excel download button
            downloadExcelBtn.disabled = true;
            
            console.error('Error loading crypto historical data for date range:', error);
            alert(`Failed to load crypto historical data for ${symbol} date range. Error: ${error.message}`);
            resetMetrics();
        }
    }
    
    // Check if nominal changes in the historical data
    function checkNominalChanges(history) {
        if (!history || history.length <= 1) {
            return { changed: false, dates: [] };
        }
        
        let prevNominal = history[0].nominal;
        const changes = [];
        
        for (let i = 1; i < history.length; i++) {
            const currentNominal = history[i].nominal;
            if (currentNominal !== prevNominal) {
                changes.push({
                    date: history[i].date,
                    oldNominal: prevNominal,
                    newNominal: currentNominal
                });
                prevNominal = currentNominal;
            }
        }
        
        return {
            changed: changes.length > 0,
            dates: changes
        };
    }
    
    // Format date to YYYY-MM-DD
    function formatDate(date) {
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
    }
    
    // Calculate metrics based on historical data
    function calculateMetrics(values, nominal) {
        if (values.length === 0) {
            resetMetrics();
            return;
        }
        
        // Average value
        const avg = values.reduce((sum, val) => sum + val, 0) / values.length;
        
        // Minimum and maximum values
        const min = Math.min(...values);
        const max = Math.max(...values);
        
        // Standard deviation
        const squaredDiffs = values.map(val => Math.pow(val - avg, 2));
        const variance = squaredDiffs.reduce((sum, val) => sum + val, 0) / values.length;
        const std = Math.sqrt(variance);
        
        // Volatility (standard deviation / average * 100%)
        const volatility = (std / avg) * 100;
        
        // Adjust metrics for display based on nominal
        const displayAvg = avg * nominal;
        const displayStd = std * nominal;
        const displayMin = min * nominal;
        const displayMax = max * nominal;
        
        // Display metrics
        metricAvg.textContent = displayAvg.toFixed(4) + ' ₽';
        metricStd.textContent = displayStd.toFixed(4) + ' ₽';
        metricMin.textContent = displayMin.toFixed(4) + ' ₽';
        metricMax.textContent = displayMax.toFixed(4) + ' ₽';
        metricVolatility.textContent = volatility.toFixed(2) + '%';
    }
    
    // Reset metrics
    function resetMetrics() {
        metricAvg.textContent = '-';
        metricStd.textContent = '-';
        metricMin.textContent = '-';
        metricMax.textContent = '-';
        metricVolatility.textContent = '-';
    }
    
    // Update UI labels based on data source
    function updateUILabels() {
        const currencyLabel = document.querySelector('label[for="currency-select"]');
        
        if (currentDataSource === 'cbr') {
            currencyLabel.textContent = 'Select Currency:';
            currencySelect.innerHTML = '<option value="" selected disabled>Loading currencies...</option>';
        } else if (currentDataSource === 'crypto') {
            currencyLabel.textContent = 'Select Cryptocurrency:';
            currencySelect.innerHTML = '<option value="" selected disabled>Loading cryptocurrencies...</option>';
        }
    }
    
    // Calculate metrics for crypto data
    function calculateCryptoMetrics(history) {
        if (history.length === 0) {
            resetMetrics();
            return;
        }
        
        // Extract closing prices (now in RUB)
        const closingPrices = history.map(item => item.close);
        
        // Calculate basic metrics
        const avg = closingPrices.reduce((sum, val) => sum + val, 0) / closingPrices.length;
        const min = Math.min(...closingPrices);
        const max = Math.max(...closingPrices);
        
        // Standard deviation
        const squaredDiffs = closingPrices.map(val => Math.pow(val - avg, 2));
        const variance = squaredDiffs.reduce((sum, val) => sum + val, 0) / closingPrices.length;
        const std = Math.sqrt(variance);
        
        // Volatility (coefficient of variation as percentage)
        const volatility = (std / avg) * 100;
        
        // Format price in RUB
        const formatPrice = (price) => {
            return price.toFixed(2);
        };
        
        // Update metrics display for crypto in RUB
        metricAvg.textContent = `${formatPrice(avg)} ₽`;
        metricStd.textContent = `${formatPrice(std)} ₽`;
        metricMin.textContent = `${formatPrice(min)} ₽`;
        metricMax.textContent = `${formatPrice(max)} ₽`;
        metricVolatility.textContent = `${volatility.toFixed(2)}%`;
    }
    
    // Render currency rate chart
    function renderChart(dates, values, currencyInfo) {
        // If chart already exists, destroy it
        if (currencyChart) {
            currencyChart.destroy();
        }
        
        // Check if this is crypto data
        const isCrypto = currencyInfo.type === 'crypto';
        
        let chartLabel, displayValues, yAxisLabel, tooltipCallback;
        
        if (isCrypto) {
            // Crypto chart configuration
            chartLabel = `${currencyInfo.code} Price (RUB)`;
            displayValues = values; // Use values as-is for crypto (now in RUB)
            yAxisLabel = 'Price (RUB)';
            
            tooltipCallback = function(context) {
                const price = context.raw;
                const formatPrice = (price) => {
                    return price.toFixed(2);
                };
                return `Price: ${formatPrice(price)} ₽`;
            };
        } else {
            // Traditional currency chart configuration
            const currencyNameForm = getCurrencyNameForm(currencyInfo.name, currencyInfo.nominal);
            
            chartLabel = `${currencyInfo.nominal} ${currencyNameForm} to RUB`;
            if (currencyInfo.nominalChanged) {
                chartLabel += ' (normalized)';
            }
            
            displayValues = values.map(value => value * currencyInfo.nominal);
            yAxisLabel = 'Rate (₽)';
            
            tooltipCallback = function(context) {
                let label = `Rate: ${context.raw.toFixed(4)} ₽ for ${currencyInfo.nominal} ${currencyNameForm}`;
                
                // Add information about normalization if nominal changed
                if (currencyInfo.nominalChanged) {
                    const originalDateIndex = chartDates.indexOf(context.label);
                    if (originalDateIndex >= 0) {
                        const originalDate = originalDates[originalDateIndex];
                        const changeInfo = currencyInfo.nominalChangeDates.find(d => d.date === originalDate);
                        
                        if (changeInfo) {
                            label += ` (Nominal changed: ${changeInfo.oldNominal} → ${changeInfo.newNominal})`;
                        }
                    }
                }
                
                return label;
            };
        }
        
        // Format dates for better display
        const formattedDates = dates.map(formatDateForDisplay);
        
        // Reduce data points if there are too many for better readability
        let chartDates = formattedDates;
        let chartValues = displayValues;
        let originalDates = dates;
        
        if (dates.length > 60) {
            const { reducedLabels, reducedData, reducedOriginals } = reduceDataPoints(formattedDates, displayValues, dates);
            chartDates = reducedLabels;
            chartValues = reducedData;
            originalDates = reducedOriginals;
        }
        
        // Create new chart
        currencyChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: chartDates,
                datasets: [{
                    label: chartLabel,
                    data: chartValues,
                    borderColor: isCrypto ? '#ff6b35' : '#0d6efd',
                    backgroundColor: isCrypto ? 'rgba(255, 107, 53, 0.1)' : 'rgba(13, 110, 253, 0.1)',
                    borderWidth: 2,
                    fill: true,
                    tension: 0.1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'top',
                    },
                    tooltip: {
                        callbacks: {
                            label: tooltipCallback
                        }
                    }
                },
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: 'Date'
                        },
                        ticks: {
                            maxRotation: 45,
                            minRotation: 45,
                            autoSkip: true,
                            maxTicksLimit: 20
                        },
                        reverse: false // Ensure left-to-right display (oldest to newest)
                    },
                    y: {
                        title: {
                            display: true,
                            text: yAxisLabel
                        },
                        ticks: {
                            callback: function(value) {
                                if (isCrypto) {
                                    return value.toFixed(2) + ' ₽';
                                } else {
                                    return value.toFixed(2) + ' ₽';
                                }
                            }
                        },
                        // Add grid lines for better readability
                        grid: {
                            color: 'rgba(0, 0, 0, 0.1)'
                        }
                    }
                },
                // Improve interaction
                interaction: {
                    intersect: false,
                    mode: 'index'
                }
            }
        });
    }
    
    // Reduce number of data points for better chart readability
    function reduceDataPoints(labels, data, originalDates) {
        // If we have more than 60 points, reduce them
        if (labels.length <= 60) {
            return { reducedLabels: labels, reducedData: data, reducedOriginals: originalDates };
        }
        
        // Calculate step size to reduce to about 30-60 points
        const stepSize = Math.ceil(labels.length / 60);
        
        const reducedLabels = [];
        const reducedData = [];
        const reducedOriginals = [];
        
        // Always include first and last point
        for (let i = 0; i < labels.length; i += stepSize) {
            reducedLabels.push(labels[i]);
            reducedData.push(data[i]);
            reducedOriginals.push(originalDates[i]);
        }
        
        // Make sure to include the last point if it wasn't included
        const lastIndex = labels.length - 1;
        if ((labels.length - 1) % stepSize !== 0) {
            reducedLabels.push(labels[lastIndex]);
            reducedData.push(data[lastIndex]);
            reducedOriginals.push(originalDates[lastIndex]);
        }
        
        return { reducedLabels, reducedData, reducedOriginals };
    }
    
    // Format date for display on chart
    function formatDateForDisplay(dateStr) {
        const date = new Date(dateStr);
        const day = date.getDate().toString().padStart(2, '0');
        const month = (date.getMonth() + 1).toString().padStart(2, '0');
        const year = date.getFullYear();
        
        return `${day}.${month}.${year}`;
    }
    
    // Helper function to estimate expected business days in a date range
    function getExpectedBusinessDays(startDate, endDate) {
        // Clone dates to avoid modifying the originals
        const start = new Date(startDate);
        const end = new Date(endDate);
        
        // Set time to midnight to avoid time issues
        start.setHours(0, 0, 0, 0);
        end.setHours(0, 0, 0, 0);
        
        let count = 0;
        const curDate = new Date(start);
        
        while (curDate <= end) {
            const dayOfWeek = curDate.getDay();
            // Skip weekends (0 = Sunday, 6 = Saturday)
            if (dayOfWeek !== 0 && dayOfWeek !== 6) {
                count++;
            }
            curDate.setDate(curDate.getDate() + 1);
        }
        
        return count;
    }
}); 