document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
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
    
    // Show/hide custom period inputs when period selection changes
    periodSelect.addEventListener('change', function() {
        if (this.value === 'custom') {
            customPeriodDiv.classList.remove('d-none');
        } else {
            customPeriodDiv.classList.add('d-none');
        }
    });
    
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
                
                // Load data for custom period
                loadCurrencyHistoryCustom(currencyCode, startDate, endDate);
            } else {
                // Standard period
                const period = periodSelect.value;
                loadCurrencyHistory(currencyCode, period);
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
                
                currencies.forEach(([code, currency]) => {
                    const option = document.createElement('option');
                    option.value = code;
                    option.textContent = `${currency.Name} (${code})`;
                    currencySelect.appendChild(option);
                });
                
                // Select USD by default
                if (data.data.USD) {
                    currencySelect.value = 'USD';
                    // Load data for USD for a week by default
                    loadCurrencyHistory('USD', 7);
                }
            }
        } catch (error) {
            console.error('Error loading currencies list:', error);
            alert('Failed to load currencies list. Please try again later.');
        }
    }
    
    // Load historical data for a currency
    async function loadCurrencyHistory(currencyCode, days) {
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
            
            // Request to API for historical data
            const response = await fetch(`/rates/cbr/history?code=${currencyCode}&days=${days}`);
            const data = await response.json();
            
            // Hide loading indicator
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            if (data.success && data.data && data.data.length > 0) {
                // Extract dates and values from response
                const history = data.data;
                const dates = history.map(item => item.date);
                const values = history.map(item => item.value / item.nominal);
                
                // Calculate metrics
                calculateMetrics(values);
                
                // Render chart
                renderChart(dates, values, currencyCode);
            } else {
                alert('Failed to get historical data for the selected currency.');
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
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
            
            // Format dates for API request
            const startDateStr = formatDate(startDate);
            const endDateStr = formatDate(endDate);
            
            // Request to API for historical data using the new endpoint
            const response = await fetch(`/rates/cbr/history/range?code=${currencyCode}&start_date=${startDateStr}&end_date=${endDateStr}`);
            const data = await response.json();
            
            // Hide loading indicator
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            if (data.success && data.data && data.data.length > 0) {
                // Extract dates and values from response
                const history = data.data;
                
                // Sort by date (ascending)
                history.sort((a, b) => new Date(a.date) - new Date(b.date));
                
                const dates = history.map(item => item.date);
                const values = history.map(item => item.value / item.nominal);
                
                // Calculate metrics
                calculateMetrics(values);
                
                // Render chart
                renderChart(dates, values, currencyCode);
            } else {
                alert('No data available for the selected date range. Try a different period.');
                resetMetrics();
            }
        } catch (error) {
            // Hide loading indicator in case of error
            loadingIndicator.classList.add('d-none');
            document.getElementById('currency-chart').classList.remove('d-none');
            
            console.error('Error loading historical data:', error);
            alert('Failed to load historical data. Please try again later.');
            resetMetrics();
        }
    }
    
    // Format date to YYYY-MM-DD
    function formatDate(date) {
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
    }
    
    // Calculate metrics based on historical data
    function calculateMetrics(values) {
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
        
        // Display metrics
        metricAvg.textContent = avg.toFixed(4);
        metricStd.textContent = std.toFixed(4);
        metricMin.textContent = min.toFixed(4);
        metricMax.textContent = max.toFixed(4);
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
    
    // Render currency rate chart
    function renderChart(dates, values, currencyCode) {
        // If chart already exists, destroy it
        if (currencyChart) {
            currencyChart.destroy();
        }
        
        // Create new chart
        currencyChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: dates,
                datasets: [{
                    label: `${currencyCode} to RUB Rate`,
                    data: values,
                    borderColor: '#0d6efd',
                    backgroundColor: 'rgba(13, 110, 253, 0.1)',
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
                            label: function(context) {
                                return `Rate: ${context.raw.toFixed(4)} ₽`;
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: 'Date'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'Rate (₽)'
                        },
                        ticks: {
                            callback: function(value) {
                                return value.toFixed(2) + ' ₽';
                            }
                        }
                    }
                }
            }
        });
    }
}); 