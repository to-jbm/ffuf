#!/usr/bin/env python3
"""
FFUF Results Dashboard
A modern web interface for browsing, searching and managing ffuf scan results.
Auto-discovers output files from ffuf_out/ directory.
"""

import os
import sys
import json
import glob
import psutil
import webbrowser
import socket
from pathlib import Path
from datetime import datetime
from threading import Timer
from flask import Flask, render_template_string, jsonify, request

# Configuration
PORT = 8080
DATA_DIR = Path(__file__).parent
APP_NAME = "FFUF Results Dashboard"

app = Flask(__name__)

# Kill existing instances on the same port
def kill_existing_instances():
    """Terminate any other processes using the same port."""
    current_pid = os.getpid()
    for proc in psutil.process_iter(['pid', 'name', 'connections']):
        try:
            if proc.pid == current_pid:
                continue
            for conn in proc.connections(kind='inet'):
                if conn.laddr.port == PORT:
                    print(f"Killing existing process {proc.pid} using port {PORT}")
                    proc.terminate()
                    proc.wait(timeout=3)
        except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.TimeoutExpired):
            pass

# Auto-open browser
def open_browser():
    """Launch the default web browser."""
    webbrowser.open(f'http://127.0.0.1:{PORT}')

# Parse different ffuf output formats
def parse_ffuf_file(filepath):
    """Parse ffuf JSON/EJSON output file and extract metadata + results."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            data = json.load(f)
        
        # Extract command info
        cmdline = data.get('commandline', '')
        full_cmd = data.get('full_command', '')
        url = extract_url_from_cmd(cmdline)
        
        # Extract results
        results = []
        raw_results = data.get('results', [])
        
        for idx, r in enumerate(raw_results):
            result = {
                'id': idx,
                'position': r.get('position', 0),
                'status': r.get('status', 0),
                'length': r.get('length', 0),
                'words': r.get('words', 0),
                'lines': r.get('lines', 0),
                'url': r.get('url', ''),
                'duration': r.get('duration', 0),
                'input': r.get('input', {}),
                'redirectlocation': r.get('redirectlocation', ''),
                'content_type': r.get('content-type', '')
            }
            results.append(result)
        
        return {
            'filename': Path(filepath).name,
            'filepath': str(filepath),
            'url': url,
            'commandline': cmdline,
            'full_command': full_cmd,
            'time': data.get('time', ''),
            'last_position': data.get('last_position', 0),
            'total_positions': data.get('total_positions', 0),
            'results': results,
            'result_count': len(results)
        }
    except Exception as e:
        return {
            'filename': Path(filepath).name,
            'filepath': str(filepath),
            'url': 'Unknown',
            'error': str(e),
            'results': [],
            'result_count': 0
        }

def extract_url_from_cmd(cmdline):
    """Extract the target URL from ffuf command line."""
    if '-u ' in cmdline:
        parts = cmdline.split('-u ')
        if len(parts) > 1:
            url_part = parts[1].split()[0]
            return url_part
    return 'Unknown'

def get_all_scans():
    """Discover and parse all ffuf output files."""
    scans = []
    patterns = ['*.json', '*.ejson']
    
    for pattern in patterns:
        for filepath in DATA_DIR.glob(pattern):
            scan = parse_ffuf_file(filepath)
            scans.append(scan)
    
    # Sort by modification time (newest first)
    scans.sort(key=lambda x: os.path.getmtime(x['filepath']) if os.path.exists(x['filepath']) else 0, reverse=True)
    return scans

# HTML Template with modern light theme
HTML_TEMPLATE = '''
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ app_name }}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ec 100%);
            min-height: 100vh;
            color: #2d3748;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px 30px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
        }
        
        .header h1 {
            font-size: 24px;
            font-weight: 600;
            letter-spacing: -0.5px;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 30px;
        }
        
        .stats-bar {
            display: flex;
            gap: 20px;
            margin-bottom: 30px;
            flex-wrap: wrap;
        }
        
        .stat-card {
            background: white;
            border-radius: 12px;
            padding: 20px 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
            min-width: 150px;
        }
        
        .stat-card .number {
            font-size: 32px;
            font-weight: 700;
            color: #667eea;
        }
        
        .stat-card .label {
            font-size: 13px;
            color: #718096;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        .search-section {
            background: white;
            border-radius: 12px;
            padding: 25px;
            margin-bottom: 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
        }
        
        .search-box {
            width: 100%;
            padding: 14px 20px;
            border: 2px solid #e2e8f0;
            border-radius: 10px;
            font-size: 15px;
            transition: all 0.2s;
        }
        
        .search-box:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        
        .scans-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
            gap: 25px;
        }
        
        .scan-card {
            background: white;
            border-radius: 12px;
            padding: 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
            cursor: pointer;
            transition: all 0.2s;
            border: 2px solid transparent;
        }
        
        .scan-card:hover {
            transform: translateY(-3px);
            box-shadow: 0 8px 25px rgba(0,0,0,0.1);
            border-color: #667eea;
        }
        
        .scan-card.selected {
            border-color: #667eea;
            background: #f8f9ff;
        }
        
        .scan-header {
            display: flex;
            justify-content: space-between;
            align-items: start;
            margin-bottom: 15px;
        }
        
        .scan-url {
            font-size: 14px;
            font-weight: 600;
            color: #2d3748;
            word-break: break-all;
        }
        
        .scan-badge {
            background: #667eea;
            color: white;
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
        }
        
        .scan-meta {
            font-size: 12px;
            color: #718096;
            margin-top: 8px;
        }
        
        .scan-progress {
            margin-top: 15px;
        }
        
        .progress-bar {
            height: 6px;
            background: #e2e8f0;
            border-radius: 3px;
            overflow: hidden;
        }
        
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 3px;
            transition: width 0.3s;
        }
        
        .results-panel {
            background: white;
            border-radius: 12px;
            padding: 25px;
            margin-top: 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
        }
        
        .results-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
            flex-wrap: wrap;
            gap: 15px;
        }
        
        .results-title {
            font-size: 20px;
            font-weight: 600;
            color: #2d3748;
        }
        
        .btn {
            padding: 10px 20px;
            border: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.2s;
        }
        
        .btn-primary {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        
        .btn-primary:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
        }
        
        .btn-secondary {
            background: #e2e8f0;
            color: #4a5568;
        }
        
        .btn-secondary:hover {
            background: #cbd5e0;
        }
        
        .btn-danger {
            background: #fc8181;
            color: white;
        }
        
        .btn-danger:hover {
            background: #f56565;
        }
        
        .controls-bar {
            display: flex;
            gap: 10px;
            align-items: center;
            flex-wrap: wrap;
            padding: 15px 0;
            border-bottom: 1px solid #e2e8f0;
            margin-bottom: 15px;
        }
        
        .checkbox-wrapper {
            display: flex;
            align-items: center;
            gap: 8px;
            cursor: pointer;
        }
        
        .checkbox-wrapper input[type="checkbox"] {
            width: 18px;
            height: 18px;
            cursor: pointer;
        }
        
        .results-table {
            width: 100%;
            border-collapse: collapse;
        }
        
        .results-table th {
            background: #f7fafc;
            padding: 12px;
            text-align: left;
            font-size: 12px;
            font-weight: 600;
            color: #4a5568;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            border-bottom: 2px solid #e2e8f0;
        }
        
        .results-table td {
            padding: 12px;
            border-bottom: 1px solid #edf2f7;
            font-size: 14px;
        }
        
        .results-table tr:hover {
            background: #f7fafc;
        }
        
        .results-table tr.hidden-row {
            display: none;
        }
        
        .status-badge {
            padding: 4px 10px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }
        
        .status-200 { background: #9ae6b4; color: #22543d; }
        .status-301, .status-302 { background: #90cdf4; color: #2c5282; }
        .status-400, .status-401, .status-403 { background: #fbd38d; color: #7c2d12; }
        .status-500 { background: #fc8181; color: #742a2a; }
        
        .empty-state {
            text-align: center;
            padding: 60px;
            color: #718096;
        }
        
        .empty-state-icon {
            font-size: 48px;
            margin-bottom: 20px;
        }
        
        .hidden-count {
            color: #a0aec0;
            font-size: 13px;
        }
        
        .url-link {
            color: #667eea;
            text-decoration: none;
            word-break: break-all;
        }
        
        .url-link:hover {
            text-decoration: underline;
        }
        
        .input-tags {
            display: flex;
            gap: 5px;
            flex-wrap: wrap;
        }
        
        .input-tag {
            background: #e2e8f0;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            color: #4a5568;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 FFUF Results Dashboard</h1>
    </div>
    
    <div class="container">
        <div class="stats-bar">
            <div class="stat-card">
                <div class="number">{{ total_scans }}</div>
                <div class="label">Total Scans</div>
            </div>
            <div class="stat-card">
                <div class="number">{{ total_results }}</div>
                <div class="label">Total Results</div>
            </div>
        </div>
        
        <div class="search-section">
            <input type="text" class="search-box" id="urlSearch" placeholder="🔍 Search by target URL..." onkeyup="filterScans()">
        </div>
        
        <div class="scans-grid" id="scansGrid">
            {% for scan in scans %}
            <div class="scan-card" data-url="{{ scan.url }}" data-filename="{{ scan.filename }}" onclick="selectScan('{{ scan.filename }}')">
                <div class="scan-header">
                    <div class="scan-url">{{ scan.url }}</div>
                    <span class="scan-badge">{{ scan.result_count }}</span>
                </div>
                <div class="scan-meta">{{ scan.filename }}</div>
                <div class="scan-meta">{{ scan.time }}</div>
                {% if scan.total_positions > 0 %}
                <div class="scan-progress">
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: {{ (scan.last_position / scan.total_positions * 100) }}%"></div>
                    </div>
                    <div class="scan-meta">Progress: {{ scan.last_position }} / {{ scan.total_positions }}</div>
                </div>
                {% endif %}
            </div>
            {% endfor %}
        </div>
        
        <div class="results-panel" id="resultsPanel" style="display: none;">
            <div class="results-header">
                <div class="results-title" id="resultsTitle">Results</div>
                <div>
                    <span class="hidden-count" id="hiddenCount"></span>
                </div>
            </div>
            
            <input type="text" class="search-box" id="resultSearch" placeholder="🔍 Search results (URL, status, content type...)" onkeyup="filterResults()">
            
            <div class="controls-bar">
                <label class="checkbox-wrapper">
                    <input type="checkbox" id="selectAll" onchange="toggleSelectAll()">
                    <span>Select all visible</span>
                </label>
                <button class="btn btn-danger" onclick="hideSelected()">Hide Selected</button>
                <button class="btn btn-secondary" onclick="resetHidden()">Reset All Hidden</button>
            </div>
            
            <div id="resultsTableContainer">
                <!-- Results table injected here -->
            </div>
        </div>
    </div>
    
    <script>
        let currentScanData = null;
        let hiddenRows = new Set();
        
        function filterScans() {
            const search = document.getElementById('urlSearch').value.toLowerCase();
            const cards = document.querySelectorAll('.scan-card');
            cards.forEach(card => {
                const url = card.getAttribute('data-url').toLowerCase();
                const filename = card.getAttribute('data-filename').toLowerCase();
                if (url.includes(search) || filename.includes(search)) {
                    card.style.display = '';
                } else {
                    card.style.display = 'none';
                }
            });
        }
        
        function selectScan(filename) {
            document.querySelectorAll('.scan-card').forEach(c => c.classList.remove('selected'));
            event.currentTarget.classList.add('selected');
            
            fetch('/api/scan/' + encodeURIComponent(filename))
                .then(r => r.json())
                .then(data => {
                    currentScanData = data;
                    hiddenRows.clear();
                    showResults(data);
                });
        }
        
        function showResults(data) {
            document.getElementById('resultsPanel').style.display = 'block';
            document.getElementById('resultsTitle').textContent = 'Results: ' + data.url;
            renderResultsTable(data.results);
            updateHiddenCount();
        }
        
        function renderResultsTable(results) {
            const container = document.getElementById('resultsTableContainer');
            if (!results || results.length === 0) {
                container.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📭</div><p>No results found</p></div>';
                return;
            }
            
            let html = '<table class="results-table"><thead><tr>';
            html += '<th><input type="checkbox" id="selectAllHeader" onchange="toggleSelectAll()"></th>';
            html += '<th>Position</th><th>Status</th><th>Length</th><th>Words</th><th>Lines</th><th>Duration</th><th>URL</th><th>Input</th>';
            html += '</tr></thead><tbody>';
            
            results.forEach(r => {
                const statusClass = r.status >= 200 && r.status < 300 ? 'status-200' : 
                                   r.status >= 300 && r.status < 400 ? 'status-301' :
                                   r.status >= 400 && r.status < 500 ? 'status-400' : 'status-500';
                
                let inputHtml = '';
                if (r.input) {
                    inputHtml = '<div class="input-tags">';
                    for (const [k, v] of Object.entries(r.input)) {
                        if (k !== 'FFUFHASH') {
                            inputHtml += '<span class="input-tag">' + k + ': ' + v + '</span>';
                        }
                    }
                    inputHtml += '</div>';
                }
                
                html += '<tr id="row-' + r.id + '" class="result-row">';
                html += '<td><input type="checkbox" class="row-checkbox" data-id="' + r.id + '"></td>';
                html += '<td>' + r.position + '</td>';
                html += '<td><span class="status-badge ' + statusClass + '">' + r.status + '</span></td>';
                html += '<td>' + r.length + '</td>';
                html += '<td>' + r.words + '</td>';
                html += '<td>' + r.lines + '</td>';
                html += '<td>' + r.duration + 'ms</td>';
                html += '<td><a href="' + r.url + '" target="_blank" class="url-link">' + r.url + '</a></td>';
                html += '<td>' + inputHtml + '</td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        }
        
        function filterResults() {
            const search = document.getElementById('resultSearch').value.toLowerCase();
            const rows = document.querySelectorAll('.result-row');
            rows.forEach(row => {
                if (hiddenRows.has(parseInt(row.id.replace('row-', '')))) {
                    return;
                }
                const text = row.textContent.toLowerCase();
                if (text.includes(search)) {
                    row.style.display = '';
                } else {
                    row.style.display = 'none';
                }
            });
        }
        
        function toggleSelectAll() {
            const checkboxes = document.querySelectorAll('.row-checkbox');
            const selectAll = document.getElementById('selectAll').checked || document.getElementById('selectAllHeader')?.checked;
            checkboxes.forEach(cb => {
                const row = document.getElementById('row-' + cb.getAttribute('data-id'));
                if (row && row.style.display !== 'none') {
                    cb.checked = selectAll;
                }
            });
        }
        
        function hideSelected() {
            const checkboxes = document.querySelectorAll('.row-checkbox:checked');
            checkboxes.forEach(cb => {
                const id = parseInt(cb.getAttribute('data-id'));
                hiddenRows.add(id);
                const row = document.getElementById('row-' + id);
                if (row) row.classList.add('hidden-row');
            });
            document.getElementById('selectAll').checked = false;
            if (document.getElementById('selectAllHeader')) {
                document.getElementById('selectAllHeader').checked = false;
            }
            updateHiddenCount();
        }
        
        function resetHidden() {
            hiddenRows.clear();
            document.querySelectorAll('.hidden-row').forEach(row => {
                row.classList.remove('hidden-row');
            });
            filterResults();
            updateHiddenCount();
        }
        
        function updateHiddenCount() {
            const count = hiddenRows.size;
            const el = document.getElementById('hiddenCount');
            if (count > 0) {
                el.textContent = count + ' row' + (count > 1 ? 's' : '') + ' hidden';
            } else {
                el.textContent = '';
            }
        }
    </script>
</body>
</html>
'''

@app.route('/')
def index():
    scans = get_all_scans()
    total_results = sum(s['result_count'] for s in scans)
    return render_template_string(HTML_TEMPLATE, 
                                   app_name=APP_NAME,
                                   scans=scans,
                                   total_scans=len(scans),
                                   total_results=total_results)

@app.route('/api/scan/<path:filename>')
def get_scan(filename):
    filepath = DATA_DIR / filename
    if not filepath.exists():
        return jsonify({'error': 'File not found'}), 404
    return jsonify(parse_ffuf_file(filepath))

@app.route('/api/scans')
def list_scans():
    return jsonify(get_all_scans())

def is_port_available(port):
    """Check if a port is available."""
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.bind(('127.0.0.1', port))
        sock.close()
        return True
    except OSError:
        return False

if __name__ == '__main__':
    # Kill existing instances
    kill_existing_instances()
    
    # Check port availability
    if not is_port_available(PORT):
        print(f"Port {PORT} is still in use. Trying to free it...")
        kill_existing_instances()
        import time
        time.sleep(1)
    
    # Open browser after a short delay
    Timer(1.0, open_browser).start()
    
    print(f"🚀 Starting {APP_NAME} on http://127.0.0.1:{PORT}")
    print(f"📁 Watching directory: {DATA_DIR.absolute()}")
    
    # Run Flask
    app.run(host='127.0.0.1', port=PORT, debug=False)
