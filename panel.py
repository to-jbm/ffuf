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
import re
from pathlib import Path
from datetime import datetime
from threading import Timer
from flask import Flask, render_template_string, jsonify, request

# Configuration
PORT = 8080
DATA_DIR = Path(__file__).parent
APP_NAME = "FFUF Results Dashboard"

def format_duration(ms):
    """Format duration in milliseconds to human readable string."""
    if ms < 1000:
        return f"{ms}ms"
    elif ms < 60000:
        return f"{ms/1000:.1f}s"
    else:
        return f"{ms/60000:.1f}m"

app = Flask(__name__)
app.jinja_env.filters['format_duration'] = format_duration

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
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .header h1 {
            font-size: 24px;
            font-weight: 600;
            letter-spacing: -0.5px;
        }
        
        .container {
            max-width: 1600px;
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
        
        /* Scans Table */
        .scans-section {
            background: white;
            border-radius: 12px;
            padding: 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
            margin-bottom: 25px;
        }
        
        .section-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        
        .section-title {
            font-size: 20px;
            font-weight: 600;
            color: #2d3748;
        }
        
        .search-box {
            width: 300px;
            padding: 10px 15px;
            border: 2px solid #e2e8f0;
            border-radius: 8px;
            font-size: 14px;
            transition: all 0.2s;
        }
        
        .search-box:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        
        .data-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 14px;
        }
        
        .data-table th {
            background: #f7fafc;
            padding: 12px;
            text-align: left;
            font-weight: 600;
            color: #4a5568;
            border-bottom: 2px solid #e2e8f0;
            cursor: pointer;
            user-select: none;
        }
        
        .data-table th:hover {
            background: #edf2f7;
        }
        
        .data-table th .sort-icon {
            margin-left: 5px;
            opacity: 0.5;
        }
        
        .data-table th.sort-asc .sort-icon::after { content: "▲"; opacity: 1; }
        .data-table th.sort-desc .sort-icon::after { content: "▼"; opacity: 1; }
        
        .data-table td {
            padding: 12px;
            border-bottom: 1px solid #edf2f7;
        }
        
        .data-table tr:hover {
            background: #f7fafc;
        }
        
        .data-table tr.clickable {
            cursor: pointer;
        }
        
        .scan-row:hover {
            background: #edf2f7 !important;
        }
        
        .progress-bar {
            height: 6px;
            background: #e2e8f0;
            border-radius: 3px;
            overflow: hidden;
            width: 100px;
        }
        
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 3px;
            transition: width 0.3s;
        }
        
        .scan-badge {
            background: #667eea;
            color: white;
            padding: 3px 10px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 600;
        }
        
        /* Scan Detail Card */
        .scan-detail {
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
            margin-bottom: 25px;
            display: none;
        }
        
        .scan-detail.active {
            display: block;
        }
        
        .scan-card-header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px 25px;
            border-radius: 12px 12px 0 0;
            display: flex;
            justify-content: space-between;
            align-items: start;
        }
        
        .scan-card-title {
            font-size: 18px;
            font-weight: 600;
            word-break: break-all;
        }
        
        .scan-card-meta {
            font-size: 13px;
            opacity: 0.9;
            margin-top: 8px;
        }
        
        .close-btn {
            background: rgba(255,255,255,0.2);
            border: none;
            color: white;
            width: 32px;
            height: 32px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 18px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s;
        }
        
        .close-btn:hover {
            background: rgba(255,255,255,0.3);
        }
        
        .scan-card-body {
            padding: 20px 25px;
        }
        
        .info-row {
            display: flex;
            margin-bottom: 12px;
        }
        
        .info-label {
            font-weight: 600;
            color: #4a5568;
            min-width: 120px;
        }
        
        .info-value {
            color: #2d3748;
            word-break: break-all;
        }
        
        .full-command {
            background: #f7fafc;
            padding: 12px;
            border-radius: 6px;
            font-family: 'Consolas', 'Monaco', monospace;
            font-size: 12px;
            word-break: break-all;
            border: 1px solid #e2e8f0;
        }
        
        /* Results Section */
        .results-section {
            background: white;
            border-radius: 12px;
            padding: 25px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.05);
        }
        
        .results-section.hidden {
            display: none;
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
        
        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 6px;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.2s;
        }
        
        .btn-primary {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        
        .btn-primary:hover {
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
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
        
        .per-page-select {
            padding: 6px 12px;
            border: 1px solid #e2e8f0;
            border-radius: 6px;
            font-size: 13px;
        }
        
        /* Column Filters */
        .column-filters {
            display: flex;
            gap: 8px;
            margin-bottom: 15px;
            flex-wrap: wrap;
        }
        
        .col-filter {
            padding: 6px 10px;
            border: 1px solid #e2e8f0;
            border-radius: 6px;
            font-size: 12px;
            width: 120px;
        }
        
        /* Results Table */
        .col-checkbox { width: 40px; }
        .col-position { width: 80px; }
        .col-status { width: 80px; }
        .col-length { width: 90px; }
        .col-words { width: 80px; }
        .col-lines { width: 80px; }
        .col-duration { width: 100px; }
        .col-url { min-width: 200px; max-width: 400px; }
        .col-input { min-width: 150px; max-width: 300px; }
        
        .url-cell, .input-cell {
            word-wrap: break-word;
            white-space: pre-wrap;
            max-width: 400px;
            font-size: 13px;
            line-height: 1.4;
        }
        
        .url-cell a {
            color: #667eea;
            text-decoration: none;
        }
        
        .url-cell a:hover {
            text-decoration: underline;
        }
        
        .input-tags {
            display: flex;
            gap: 4px;
            flex-wrap: wrap;
        }
        
        .input-tag {
            background: #e2e8f0;
            padding: 2px 6px;
            border-radius: 4px;
            font-size: 11px;
            color: #4a5568;
        }
        
        .status-badge {
            padding: 4px 10px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }
        
        .status-200 { background: #c6f6d5; color: #22543d; }
        .status-301, .status-302, .status-307 { background: #bee3f8; color: #2b6cb0; }
        .status-400, .status-401, .status-403 { background: #feebc8; color: #7c2d12; }
        .status-500, .status-502, .status-503, .status-504 { background: #fed7d7; color: #742a2a; }
        .status-other { background: #e2e8f0; color: #4a5568; }
        
        /* Pagination */
        .pagination {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 8px;
            margin-top: 20px;
            padding-top: 15px;
            border-top: 1px solid #e2e8f0;
        }
        
        .page-btn {
            padding: 6px 12px;
            border: 1px solid #e2e8f0;
            background: white;
            border-radius: 6px;
            cursor: pointer;
            font-size: 13px;
        }
        
        .page-btn:hover:not(:disabled) {
            background: #f7fafc;
        }
        
        .page-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        
        .page-btn.active {
            background: #667eea;
            color: white;
            border-color: #667eea;
        }
        
        .page-info {
            color: #718096;
            font-size: 13px;
        }
        
        /* Checkbox styling */
        input[type="checkbox"] {
            width: 16px;
            height: 16px;
            cursor: pointer;
        }
        
        .hidden-row {
            display: none !important;
        }
        
        .hidden-count {
            color: #e53e3e;
            font-weight: 600;
            font-size: 13px;
        }
        
        .empty-state {
            text-align: center;
            padding: 60px;
            color: #718096;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 FFUF Results Dashboard</h1>
        <div style="font-size: 13px; opacity: 0.9;">{{ total_scans }} scans • {{ total_results }} results</div>
    </div>
    
    <div class="container">
        <!-- Scans List Table -->
        <div class="scans-section" id="scansList">
            <div class="section-header">
                <div class="section-title">Scans</div>
                <input type="text" class="search-box" id="scanSearch" placeholder="Search scans..." onkeyup="filterScans()">
            </div>
            
            <table class="data-table" id="scansTable">
                <thead>
                    <tr>
                        <th onclick="sortScans('url')">Target URL <span class="sort-icon">↕</span></th>
                        <th onclick="sortScans('filename')">Filename <span class="sort-icon">↕</span></th>
                        <th onclick="sortScans('time')">Time <span class="sort-icon">↕</span></th>
                        <th onclick="sortScans('result_count')">Results <span class="sort-icon">↕</span></th>
                        <th>Progress</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody id="scansTableBody">
                    {% for scan in scans %}
                    <tr class="scan-row clickable" data-url="{{ scan.url }}" data-filename="{{ scan.filename }}" onclick="selectScan('{{ scan.filename }}')">
                        <td>{{ scan.url }}</td>
                        <td>{{ scan.filename }}</td>
                        <td>{{ scan.time }}</td>
                        <td><span class="scan-badge">{{ scan.result_count }}</span></td>
                        <td>
                            {% if scan.total_positions > 0 %}
                            <div style="display: flex; align-items: center; gap: 8px;">
                                <div class="progress-bar">
                                    <div class="progress-fill" style="width: {{ (scan.last_position / scan.total_positions * 100) if scan.total_positions else 0 }}%"></div>
                                </div>
                                <small style="color: #718096;">{{ scan.last_position }}/{{ scan.total_positions }}</small>
                            </div>
                            {% else %}
                            <span style="color: #a0aec0;">-</span>
                            {% endif %}
                        </td>
                        <td><button class="btn btn-primary" style="padding: 4px 12px; font-size: 12px;">View</button></td>
                    </tr>
                    {% endfor %}
                </tbody>
            </table>
        </div>
        
        <!-- Scan Detail Card -->
        <div class="scan-detail" id="scanDetail">
            <div class="scan-card-header">
                <div>
                    <div class="scan-card-title" id="detailTitle">Scan Details</div>
                    <div class="scan-card-meta" id="detailMeta"></div>
                </div>
                <button class="close-btn" onclick="closeScanDetail()" title="Close">✕</button>
            </div>
            <div class="scan-card-body">
                <div class="info-row">
                    <div class="info-label">Target:</div>
                    <div class="info-value" id="detailUrl"></div>
                </div>
                <div class="info-row" id="fullCmdRow">
                    <div class="info-label">Full Command:</div>
                    <div class="info-value">
                        <div class="full-command" id="detailFullCmd"></div>
                    </div>
                </div>
                <div class="info-row">
                    <div class="info-label">Results:</div>
                    <div class="info-value"><span id="detailResults"></span> found</div>
                </div>
                <div class="info-row" id="progressRow">
                    <div class="info-label">Progress:</div>
                    <div class="info-value">
                        <div style="display: flex; align-items: center; gap: 10px;">
                            <div class="progress-bar" style="width: 200px;">
                                <div class="progress-fill" id="detailProgress"></div>
                            </div>
                            <span id="detailProgressText"></span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Results Section -->
        <div class="results-section hidden" id="resultsSection">
            <div class="section-header">
                <div class="section-title">Results</div>
                <div style="display: flex; gap: 10px; align-items: center;">
                    <span class="hidden-count" id="hiddenCount"></span>
                    <select class="per-page-select" id="perPage" onchange="changePerPage()">
                        <option value="50">50 / page</option>
                        <option value="100" selected>100 / page</option>
                        <option value="250">250 / page</option>
                        <option value="500">500 / page</option>
                    </select>
                </div>
            </div>
            
            <div class="controls-bar">
                <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                    <input type="checkbox" id="selectAllVisible" onchange="toggleSelectAll()">
                    <span>Select all visible</span>
                </label>
                <button class="btn btn-danger" onclick="hideSelected()">Hide Selected</button>
                <button class="btn btn-secondary" onclick="resetHidden()">Reset All Hidden</button>
            </div>
            
            <!-- Column Filters -->
            <div class="column-filters">
                <input type="text" class="col-filter" id="filterPosition" placeholder="Position..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterStatus" placeholder="Status..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterLength" placeholder="Length..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterWords" placeholder="Words..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterLines" placeholder="Lines..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterDuration" placeholder="Duration..." onkeyup="applyFilters()">
                <input type="text" class="col-filter" id="filterUrl" placeholder="URL..." onkeyup="applyFilters()" style="width: 200px;">
                <button class="btn btn-secondary" onclick="clearFilters()">Clear Filters</button>
            </div>
            
            <div id="resultsTableContainer">
                <!-- Results table injected here -->
            </div>
            
            <div class="pagination" id="pagination"></div>
        </div>
    </div>
    
    <script>
        let currentScan = null;
        let allResults = [];
        let filteredResults = [];
        let hiddenIds = new Set();
        let currentPage = 1;
        let perPage = 100;
        let sortColumn = null;
        let sortDirection = 'asc';
        
        // Scan list sorting
        let scanSortCol = null;
        let scanSortDir = 'asc';
        
        function filterScans() {
            const search = document.getElementById('scanSearch').value.toLowerCase();
            const rows = document.querySelectorAll('.scan-row');
            rows.forEach(row => {
                const text = row.textContent.toLowerCase();
                row.style.display = text.includes(search) ? '' : 'none';
            });
        }
        
        function sortScans(column) {
            if (scanSortCol === column) {
                scanSortDir = scanSortDir === 'asc' ? 'desc' : 'asc';
            } else {
                scanSortCol = column;
                scanSortDir = 'asc';
            }
            
            // Update header icons
            document.querySelectorAll('#scansTable th').forEach(th => {
                th.classList.remove('sort-asc', 'sort-desc');
            });
            
            // Reload page with sorted data (server-side would be better, but client-side for now)
            // For simplicity, we'll just alert - in production this would re-fetch or sort client-side array
            const rows = Array.from(document.querySelectorAll('.scan-row'));
            const tbody = document.getElementById('scansTableBody');
            
            rows.sort((a, b) => {
                let valA, valB;
                switch(column) {
                    case 'url': valA = a.cells[0].textContent; valB = b.cells[0].textContent; break;
                    case 'filename': valA = a.cells[1].textContent; valB = b.cells[1].textContent; break;
                    case 'time': valA = a.cells[2].textContent; valB = b.cells[2].textContent; break;
                    case 'result_count': valA = parseInt(a.cells[3].textContent); valB = parseInt(b.cells[3].textContent); break;
                    default: return 0;
                }
                
                if (valA < valB) return scanSortDir === 'asc' ? -1 : 1;
                if (valA > valB) return scanSortDir === 'asc' ? 1 : -1;
                return 0;
            });
            
            rows.forEach(row => tbody.appendChild(row));
        }
        
        function selectScan(filename) {
            fetch('/api/scan/' + encodeURIComponent(filename))
                .then(r => r.json())
                .then(data => {
                    currentScan = data;
                    allResults = data.results || [];
                    filteredResults = [...allResults];
                    hiddenIds.clear();
                    currentPage = 1;
                    
                    // Hide scans list, show detail + results
                    document.getElementById('scansList').style.display = 'none';
                    document.getElementById('scanDetail').classList.add('active');
                    document.getElementById('resultsSection').classList.remove('hidden');
                    
                    // Populate detail card
                    document.getElementById('detailTitle').textContent = data.url || 'Scan Details';
                    document.getElementById('detailMeta').textContent = data.filename + ' • ' + (data.time || '');
                    document.getElementById('detailUrl').textContent = data.url || 'Unknown';
                    document.getElementById('detailResults').textContent = data.result_count || 0;
                    
                    if (data.commandline || data.full_command) {
                        cmd = data.full_command ? data.full_command : data.commandline;
                        if(!cmd.includes(data.commandline)) {
                            cmd = cmd + ' | ' + data.commandline;
                        }
                        document.getElementById('detailFullCmd').textContent = cmd;
                        document.getElementById('fullCmdRow').style.display = 'flex';
                    } else {
                        document.getElementById('fullCmdRow').style.display = 'none';
                    }
                    
                    if (data.total_positions > 0) {
                        const pct = (data.last_position / data.total_positions * 100).toFixed(1);
                        document.getElementById('detailProgress').style.width = pct + '%';
                        document.getElementById('detailProgressText').textContent = data.last_position + ' / ' + data.total_positions + ' (' + pct + '%)';
                        document.getElementById('progressRow').style.display = 'flex';
                    } else {
                        document.getElementById('progressRow').style.display = 'none';
                    }
                    
                    renderResults();
                });
        }
        
        function closeScanDetail() {
            document.getElementById('scansList').style.display = 'block';
            document.getElementById('scanDetail').classList.remove('active');
            document.getElementById('resultsSection').classList.add('hidden');
            currentScan = null;
            allResults = [];
            filteredResults = [];
        }
        
        function renderResults() {
            applyFilters(false);
        }
        
        function applyFilters(updatePagination = true) {
            if (!currentScan) return;
            
            const filters = {
                position: document.getElementById('filterPosition').value.toLowerCase(),
                status: document.getElementById('filterStatus').value.toLowerCase(),
                length: document.getElementById('filterLength').value.toLowerCase(),
                words: document.getElementById('filterWords').value.toLowerCase(),
                lines: document.getElementById('filterLines').value.toLowerCase(),
                duration: document.getElementById('filterDuration').value.toLowerCase(),
                url: document.getElementById('filterUrl').value.toLowerCase()
            };
            
            filteredResults = allResults.filter(r => {
                if (hiddenIds.has(r.id)) return false;
                
                if (filters.position && String(r.position) != (filters.position)) return false;
                if (filters.status && String(r.status) != (filters.status)) return false;
                if (filters.length && String(r.length) != (filters.length)) return false;
                if (filters.words && String(r.words) != (filters.words)) return false;
                if (filters.lines && String(r.lines) != (filters.lines)) return false;
                if (filters.duration && !formatDuration(r.duration).toLowerCase().includes(filters.duration)) return false;
                if (filters.url && !r.url.toLowerCase().includes(filters.url)) return false;
                
                return true;
            });
            
            // Sort
            if (sortColumn) {
                filteredResults.sort((a, b) => {
                    let valA = a[sortColumn];
                    let valB = b[sortColumn];
                    
                    if (typeof valA === 'string') {
                        valA = valA.toLowerCase();
                        valB = valB.toLowerCase();
                    }
                    
                    if (valA < valB) return sortDirection === 'asc' ? -1 : 1;
                    if (valA > valB) return sortDirection === 'asc' ? 1 : -1;
                    return 0;
                });
            }
            
            if (updatePagination) {
                currentPage = 1;
            }
            
            renderTable();
            renderPagination();
            updateHiddenCount();
        }
        
        function formatDuration(ms) {
            if (ms < 1000) return ms + 'ms';
            if (ms < 60000) return (ms/1000).toFixed(1) + 's';
            return (ms/60000).toFixed(1) + 'm';
        }
        
        function renderTable() {
            const container = document.getElementById('resultsTableContainer');
            
            if (filteredResults.length === 0) {
                container.innerHTML = '<div class="empty-state">No results match your filters</div>';
                return;
            }
            
            const start = (currentPage - 1) * perPage;
            const end = Math.min(start + perPage, filteredResults.length);
            const pageResults = filteredResults.slice(start, end);
            
            let html = '<table class="data-table"><thead><tr>';
            html += '<th class="col-checkbox"><input type="checkbox" id="selectAllHeader" onchange="toggleSelectAll()"></th>';
            html += '<th class="col-position" onclick="sortResults(\\\'position\\\')">Position <span class="sort-icon">↕</span></th>';
            html += '<th class="col-status" onclick="sortResults(\\\'status\\\')">Status <span class="sort-icon">↕</span></th>';
            html += '<th class="col-length" onclick="sortResults(\\\'length\\\')">Length <span class="sort-icon">↕</span></th>';
            html += '<th class="col-words" onclick="sortResults(\\\'words\\\')">Words <span class="sort-icon">↕</span></th>';
            html += '<th class="col-lines" onclick="sortResults(\\\'lines\\\')">Lines <span class="sort-icon">↕</span></th>';
            html += '<th class="col-duration" onclick="sortResults(\\\'duration\\\')">Duration <span class="sort-icon">↕</span></th>';
            html += '<th class="col-url" onclick="sortResults(\\\'url\\\')">URL <span class="sort-icon">↕</span></th>';
            html += '<th class="col-input">Input</th>';
            html += '</tr></thead><tbody>';
            
            pageResults.forEach(r => {
                const statusClass = r.status >= 200 && r.status < 300 ? 'status-200' :
                                   r.status >= 300 && r.status < 400 ? 'status-301' :
                                   r.status >= 400 && r.status < 500 ? 'status-400' :
                                   r.status >= 500 ? 'status-500' : 'status-other';
                
                let inputHtml = '';
                if (r.input) {
                    inputHtml = '<div class="input-tags">';
                    for (const [k, v] of Object.entries(r.input)) {
                        if (k !== 'FFUFHASH') {
                            const val = typeof v === 'string' ? v : new TextDecoder().decode(v);
                            inputHtml += '<span class="input-tag">' + escapeHtml(k) + ': ' + escapeHtml(val.substring(0, 50)) + '</span>';
                        }
                    }
                    inputHtml += '</div>';
                }
                
                html += '<tr id="row-' + r.id + '">';
                html += '<td><input type="checkbox" class="row-checkbox" data-id="' + r.id + '"></td>';
                html += '<td>' + r.position + '</td>';
                html += '<td><span class="status-badge ' + statusClass + '">' + r.status + '</span></td>';
                html += '<td>' + r.length.toLocaleString() + '</td>';
                html += '<td>' + r.words + '</td>';
                html += '<td>' + r.lines + '</td>';
                html += '<td>' + (r.duration/1000000).toFixed(2) + 's</td>';
                html += '<td class="url-cell"><a href="' + escapeHtml(r.url) + '" target="_blank">' + escapeHtml(r.url) + '</a></td>';
                html += '<td class="input-cell">' + inputHtml + '</td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
            
            // Update sort indicators
            document.querySelectorAll('#resultsTableContainer th').forEach(th => {
                th.classList.remove('sort-asc', 'sort-desc');
            });
            
            if (sortColumn) {
                const thMap = {
                    'position': 1, 'status': 2, 'length': 3, 'words': 4,
                    'lines': 5, 'duration': 6, 'url': 7
                };
                const idx = thMap[sortColumn];
                if (idx !== undefined) {
                    const th = document.querySelectorAll('#resultsTableContainer th')[idx];
                    th.classList.add(sortDirection === 'asc' ? 'sort-asc' : 'sort-desc');
                }
            }
        }
        
        function escapeHtml(text) {
            if (!text) return '';
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        function sortResults(column) {
            if (sortColumn === column) {
                sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                sortColumn = column;
                sortDirection = 'asc';
            }
            applyFilters(false);
        }
        
        function renderPagination() {
            const total = filteredResults.length;
            const totalPages = Math.ceil(total / perPage) || 1;
            
            let html = '<span class="page-info">Page ' + currentPage + ' of ' + totalPages + ' (' + total + ' results)</span>';
            
            html += '<button class="page-btn" onclick="goToPage(1)" ' + (currentPage === 1 ? 'disabled' : '') + '>First</button>';
            html += '<button class="page-btn" onclick="goToPage(' + (currentPage - 1) + ')" ' + (currentPage === 1 ? 'disabled' : '') + '>← Prev</button>';
            
            let startPage = Math.max(1, currentPage - 2);
            let endPage = Math.min(totalPages, startPage + 4);
            if (endPage - startPage < 4) {
                startPage = Math.max(1, endPage - 4);
            }
            
            for (let i = startPage; i <= endPage; i++) {
                html += '<button class="page-btn ' + (i === currentPage ? 'active' : '') + '" onclick="goToPage(' + i + ')">' + i + '</button>';
            }
            
            html += '<button class="page-btn" onclick="goToPage(' + (currentPage + 1) + ')" ' + (currentPage >= totalPages ? 'disabled' : '') + '>Next →</button>';
            html += '<button class="page-btn" onclick="goToPage(' + totalPages + ')" ' + (currentPage >= totalPages ? 'disabled' : '') + '>Last</button>';
            
            document.getElementById('pagination').innerHTML = html;
        }
        
        function goToPage(page) {
            currentPage = page;
            renderTable();
            renderPagination();
        }
        
        function changePerPage() {
            perPage = parseInt(document.getElementById('perPage').value);
            currentPage = 1;
            renderTable();
            renderPagination();
        }
        
        function toggleSelectAll() {
            const checked = document.getElementById('selectAllVisible').checked || document.getElementById('selectAllHeader')?.checked;
            document.querySelectorAll('.row-checkbox').forEach(cb => {
                cb.checked = checked;
            });
        }
        
        function hideSelected() {
            document.querySelectorAll('.row-checkbox:checked').forEach(cb => {
                const id = parseInt(cb.getAttribute('data-id'));
                hiddenIds.add(id);
            });
            document.getElementById('selectAllVisible').checked = false;
            if (document.getElementById('selectAllHeader')) {
                document.getElementById('selectAllHeader').checked = false;
            }
            applyFilters();
        }
        
        function resetHidden() {
            hiddenIds.clear();
            applyFilters();
        }
        
        function clearFilters() {
            document.querySelectorAll('.col-filter').forEach(f => f.value = '');
            applyFilters();
        }
        
        function updateHiddenCount() {
            const count = hiddenIds.size;
            const el = document.getElementById('hiddenCount');
            if (count > 0) {
                el.textContent = count + ' hidden';
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
