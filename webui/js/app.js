// ChronosDB Web UI - Main Application

const API_BASE = 'http://localhost:8080/v1/db/chronosdb';

async function checkHealth() {
    try {
        const response = await fetch(`${API_BASE}/health`);
        if (response.ok) {
            document.getElementById('statusBadge').className = 'badge bg-success';
            document.getElementById('statusBadge').innerHTML = '<i class="bi bi-check-circle"></i> Connected';
            document.getElementById('dbStatus').innerHTML = '● Healthy';
        } else {
            throw new Error('Health check failed');
        }
    } catch (error) {
        document.getElementById('statusBadge').className = 'badge bg-danger';
        document.getElementById('statusBadge').innerHTML = '<i class="bi bi-exclamation-circle"></i> Disconnected';
        document.getElementById('dbStatus').innerHTML = '● Offline';
    }
}

async function executeQuery() {
    const query = document.getElementById('queryInput').value;
    if (!query.trim()) {
        showError('Please enter a query');
        return;
    }

    const resultsDiv = document.getElementById('results');
    resultsDiv.innerHTML = '<div class="text-center"><div class="spinner-border text-primary" role="status"></div><br>Executing...</div>';

    try {
        const response = await fetch(`${API_BASE}/query`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ query: query })
        });

        const data = await response.json();

        if (response.ok) {
            displayResults(data);
        } else {
            showError(data.error || 'Query execution failed');
        }
    } catch (error) {
        showError('Connection error: ' + error.message);
    }
}

function displayResults(data) {
    const resultsDiv = document.getElementById('results');
    
    if (!data.results || data.results.length === 0) {
        resultsDiv.innerHTML = '<div class="alert alert-info">No results returned</div>';
        return;
    }

    let html = '<table class="table table-sm table-striped">';
    html += '<thead><tr>';
    
    // Get all unique keys from results
    const allKeys = new Set();
    data.results.forEach(result => {
        if (result.data) {
            Object.keys(result.data).forEach(key => allKeys.add(key));
        }
    });
    
    if (allKeys.size > 0) {
        allKeys.forEach(key => {
            html += `<th>${key}</th>`;
        });
    } else {
        html += '<th>Type</th><th>Data</th>';
    }
    html += '</tr></thead><tbody>';
    
    data.results.forEach(result => {
        html += '<tr>';
        if (allKeys.size > 0) {
            allKeys.forEach(key => {
                let value = result.data?.[key];
                if (typeof value === 'object') {
                    value = JSON.stringify(value);
                }
                html += `<td>${value || '-'}</td>`;
            });
        } else {
            html += `<td>${result.type || 'result'}</td>`;
            html += `<td>${JSON.stringify(result.data || {})}</td>`;
        }
        html += '</tr>';
    });
    
    html += '</tbody></table>';
    html += `<div class="mt-2 text-muted small">${data.results.length} row(s) returned</div>`;
    resultsDiv.innerHTML = html;
}

function showError(message) {
    const resultsDiv = document.getElementById('results');
    resultsDiv.innerHTML = `<div class="alert alert-danger"><i class="bi bi-exclamation-triangle"></i> ${message}</div>`;
}

function clearResults() {
    document.getElementById('results').innerHTML = '<p class="text-muted text-center">Results cleared</p>';
    document.getElementById('queryInput').value = '';
}

function loadQuery(query) {
    document.getElementById('queryInput').value = query;
    executeQuery();
}

function showHelp() {
    const helpHtml = `
        <div class="alert alert-info">
            <h5><i class="bi bi-info-circle"></i> ChronoSQL Help</h5>
            <hr>
            <strong>CREATE</strong> - Create nodes and edges<br>
            <strong>MATCH</strong> - Query the graph<br>
            <strong>DELETE</strong> - Soft delete with versioning<br>
            <strong>FORECAST</strong> - Predict future values<br>
            <strong>AS OF</strong> - Time travel to specific timestamp<br>
            <strong>BETWEEN</strong> - Query time ranges<br>
        </div>
    `;
    const resultsDiv = document.getElementById('results');
    resultsDiv.innerHTML = helpHtml;
    setTimeout(() => {
        if (resultsDiv.innerHTML === helpHtml) {
            resultsDiv.innerHTML = '<p class="text-muted text-center">Help displayed</p>';
        }
    }, 5000);
}

// Auto-refresh stats
async function updateStats() {
    try {
        const response = await fetch(`${API_BASE}/query`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: 'MATCH (n) RETURN n' })
        });
        const data = await response.json();
        if (data.results) {
            document.getElementById('nodeCount').innerHTML = data.results.length;
        }
    } catch (error) {
        console.error('Stats update failed:', error);
    }
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    checkHealth();
    updateStats();
    setInterval(checkHealth, 30000);
    setInterval(updateStats, 60000);
});
