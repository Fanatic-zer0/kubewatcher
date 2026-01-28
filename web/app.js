let currentTab = 'deployments';
let autoRefreshTimer;
let countdownTimer;
let countdown = 10;
let currentPage = 1;
let pageSize = 50;
let totalCount = 0;

// Helper function to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    loadData();
    startAutoRefresh();
});

// Switch tabs
function switchTab(tab) {
    currentTab = tab;
    
    // Update tab buttons
    document.querySelectorAll('.tab-button').forEach(btn => {
        btn.classList.remove('active', 'border-blue-600', 'text-blue-600');
        btn.classList.add('border-transparent', 'text-gray-600');
    });
    
    const activeTab = document.getElementById(`tab-${tab}`);
    if (activeTab) {
        activeTab.classList.add('active', 'border-blue-600', 'text-blue-600');
        activeTab.classList.remove('border-transparent', 'text-gray-600');
    }
    
    applyFilters();
}

// Load all data
async function loadData() {
    await Promise.all([
        loadStats(),
        loadEvents()
    ]);
}

// Load statistics
async function loadStats() {
    try {
        const response = await fetch('/api/stats');
        const stats = await response.json();
        
        document.getElementById('totalChanges').textContent = stats.total_changes || 0;
        document.getElementById('changes24h').textContent = stats.changes_last_24h || 0;
        document.getElementById('changesPerHour').textContent = (stats.changes_per_hour || 0).toFixed(1);
        document.getElementById('recentImagesCount').textContent = (stats.recent_images || []).length;
        
        // Top modified apps
        const topApps = stats.top_modified_apps || [];
        const topAppsHTML = topApps.length > 0 
            ? topApps.map(app => `
                <div class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-900 rounded">
                    <span class="text-sm font-medium text-gray-900 dark:text-white">${app.name}</span>
                    <span class="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded text-xs">${app.count} changes</span>
                </div>
            `).join('')
            : '<div class="text-gray-500 dark:text-gray-400 text-sm">No data</div>';
        document.getElementById('topApps').innerHTML = topAppsHTML;
        
        // Recent images
        const recentImages = stats.recent_images || [];
        const imagesHTML = recentImages.length > 0
            ? recentImages.map(image => `
                <div class="p-2 bg-gray-50 dark:bg-gray-900 rounded">
                    <code class="text-xs text-gray-700 dark:text-gray-300">${image}</code>
                </div>
            `).join('')
            : '<div class="text-gray-500 dark:text-gray-400 text-sm">No data</div>';
        document.getElementById('recentImages').innerHTML = imagesHTML;
        
    } catch (error) {
        console.error('Error loading stats:', error);
    }
}

// Load events
async function loadEvents() {
    const namespace = document.getElementById('filterNamespace').value;
    const name = document.getElementById('filterName').value;
    const action = document.getElementById('filterAction').value;
    
    // Map tab names to K8s resource kinds
    const kindMap = {
        'deployments': 'Deployment',
        'statefulsets': 'StatefulSet',
        'daemonsets': 'DaemonSet',
        'services': 'Service',
        'ingresses': 'Ingress',
        'cronjobs': 'CronJob',
        'jobs': 'Job',
        'configmaps': 'ConfigMap',
        'secrets': 'Secret'
    };
    
    const kind = kindMap[currentTab] || 'Deployment';
    const offset = (currentPage - 1) * pageSize;
    let url = `/api/events?kind=${kind}&limit=${pageSize}&offset=${offset}`;
    if (namespace) url += `&namespace=${encodeURIComponent(namespace)}`;
    if (name) url += `&name=${encodeURIComponent(name)}`;
    if (action) url += `&action=${encodeURIComponent(action)}`;
    
    try {
        const response = await fetch(url);
        const data = await response.json();
        const events = data.events || [];
        totalCount = data.total_count || 0;
        
        const tbody = document.getElementById('eventsTable');
        if (events.length === 0) {
            tbody.innerHTML = `
                <tr>
                    <td colspan="4" class="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                        No events found
                    </td>
                </tr>
            `;
            updatePagination();
            return;
        }
        
        tbody.innerHTML = events.map(event => {
            const actionColor = event.action === 'ADDED' ? 'green' : event.action === 'DELETED' ? 'red' : 'blue';
            const changeText = event.diff || event.action;
            
            // Extract summary (first line only) for table display
            const diffLines = changeText.split('\n');
            const summaryText = diffLines[0];
            
            // Format details based on change type
            let details = '';
            if (event.image_before && event.image_after && event.image_before !== event.image_after) {
                details = `<div class="text-xs"><span class="text-red-600">${event.image_before}</span> → <span class="text-green-600">${event.image_after}</span></div>`;
            } else if (event.image_after) {
                details = `<code class="text-xs">${event.image_after}</code>`;
            } else {
                const metadata = event.metadata ? JSON.parse(event.metadata) : {};
                if (metadata.replicas) {
                    details = `<span class="text-xs text-gray-600 dark:text-gray-400">Replicas: ${metadata.replicas}</span>`;
                }
            }
            
            return `
                <tr class="hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer" onclick="showTimeline('${event.namespace}', '${event.kind}', '${event.name}')">
                    <td class="px-6 py-4 whitespace-nowrap">
                        <span class="px-2 py-1 bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 rounded text-xs">${event.namespace}</span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">${event.name}</td>
                    <td class="px-6 py-4 text-sm text-gray-900 dark:text-white">
                        <div class="flex items-center gap-2">
                            <span class="px-2 py-1 bg-${actionColor}-100 dark:bg-${actionColor}-900 text-${actionColor}-800 dark:text-${actionColor}-200 rounded text-xs">
                                ${event.action}
                            </span>
                            <span class="text-sm">${summaryText}</span>
                        </div>
                    </td>
                    <td class="px-6 py-4 text-sm text-gray-600 dark:text-gray-400">
                        ${details}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-right text-sm">
                        <button class="text-blue-600 hover:text-blue-800" onclick="event.stopPropagation(); showTimeline('${event.namespace}', '${event.kind}', '${event.name}')">
                            View Timeline →
                        </button>
                    </td>
                </tr>
            `;
        }).join('');
        
        updatePagination();
        
    } catch (error) {
        console.error('Error loading events:', error);
        document.getElementById('eventsTable').innerHTML = `
            <tr>
                <td colspan="6" class="px-6 py-12 text-center text-red-600">
                    Error loading events: ${error.message}
                </td>
            </tr>
        `;
    }
}

// Show timeline for a resource
async function showTimeline(namespace, kind, name) {
    document.getElementById('timelineModal').classList.remove('hidden');
    document.getElementById('timelineModal').classList.add('flex');
    document.getElementById('timelineTitle').textContent = `Timeline: ${namespace}/${name}`;
    document.getElementById('timelineContent').innerHTML = '<div class="text-center text-gray-500 dark:text-gray-400">Loading timeline...</div>';
    
    try {
        const response = await fetch(`/api/timeline/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`);
        const data = await response.json();
        const timeline = data.timeline || [];
        
        if (timeline.length === 0) {
            document.getElementById('timelineContent').innerHTML = '<div class="text-center text-gray-500 dark:text-gray-400">No timeline data available</div>';
            return;
        }
        
        const timelineHTML = timeline.map((event, index) => {
            const timestamp = new Date(event.timestamp).toLocaleString();
            const actionColor = event.action === 'ADDED' ? 'green' : event.action === 'DELETED' ? 'red' : 'blue';
            const hasImageChange = event.image_before && event.image_after && event.image_before !== event.image_after;
            
            // Split diff into lines for detailed display
            const diffLines = (event.diff || event.action).split('\n');
            const summary = diffLines[0];
            const details = diffLines.slice(1).join('\n').trim();
            
            return `
                <div class="relative pl-8 pb-8 ${index === timeline.length - 1 ? '' : 'border-l-2 border-gray-300 dark:border-gray-600'}">
                    <div class="absolute left-0 top-0 w-4 h-4 rounded-full bg-${actionColor}-500 -ml-2"></div>
                    <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                        <div class="flex items-center justify-between mb-2">
                            <span class="px-2 py-1 bg-${actionColor}-100 dark:bg-${actionColor}-900 text-${actionColor}-800 dark:text-${actionColor}-200 rounded text-sm font-medium">
                                ${event.action}
                            </span>
                            <span class="text-sm text-gray-600 dark:text-gray-400">${timestamp}</span>
                        </div>
                        <div class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                            ${summary}
                        </div>
                        ${details ? `
                            <div class="mt-3 p-3 bg-gray-100 dark:bg-gray-800 rounded font-mono text-xs overflow-x-auto">
                                ${details.split('\n').map(line => {
                                    if (line.startsWith('- ')) {
                                        return `<div class="text-red-600 dark:text-red-400">${escapeHtml(line)}</div>`;
                                    } else if (line.startsWith('+ ')) {
                                        return `<div class="text-green-600 dark:text-green-400">${escapeHtml(line)}</div>`;
                                    } else if (line.startsWith('[') && line.endsWith(']')) {
                                        return `<div class="text-blue-600 dark:text-blue-400 font-bold mt-2">${escapeHtml(line)}</div>`;
                                    } else {
                                        return `<div class="text-gray-600 dark:text-gray-400">${escapeHtml(line)}</div>`;
                                    }
                                }).join('')}
                            </div>
                        ` : ''}
                        ${hasImageChange ? `
                            <div class="mt-3 p-3 bg-gray-100 dark:bg-gray-800 rounded font-mono text-xs overflow-x-auto">
                                <div class="text-red-600 dark:text-red-400">- ${escapeHtml(event.image_before)}</div>
                                <div class="text-green-600 dark:text-green-400">+ ${escapeHtml(event.image_after)}</div>
                            </div>
                        ` : event.image_after && event.action === 'ADDED' ? `
                            <div class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                                <code class="text-xs">${escapeHtml(event.image_after)}</code>
                            </div>
                        ` : ''}
                    </div>
                </div>
            `;
        }).join('');
        
        document.getElementById('timelineContent').innerHTML = timelineHTML;
        
    } catch (error) {
        console.error('Error loading timeline:', error);
        document.getElementById('timelineContent').innerHTML = `<div class="text-center text-red-600">Error loading timeline: ${error.message}</div>`;
    }
}

// Close timeline modal
function closeTimeline() {
    document.getElementById('timelineModal').classList.add('hidden');
    document.getElementById('timelineModal').classList.remove('flex');
}

// Apply filters
function applyFilters() {
    currentPage = 1; // Reset to first page when filters change
    loadEvents();
}

// Clear filters
function clearFilters() {
    document.getElementById('filterNamespace').value = '';
    document.getElementById('filterName').value = '';
    document.getElementById('filterAction').value = '';
    currentPage = 1;
    applyFilters();
}

// Pagination functions
function updatePagination() {
    const totalPages = Math.ceil(totalCount / pageSize);
    const pagination = document.getElementById('pagination');
    
    if (totalPages <= 1) {
        pagination.innerHTML = '';
        return;
    }
    
    const startItem = (currentPage - 1) * pageSize + 1;
    const endItem = Math.min(currentPage * pageSize, totalCount);
    
    pagination.innerHTML = `
        <div class="flex items-center justify-between px-6 py-3 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700">
            <div class="text-sm text-gray-700 dark:text-gray-300">
                Showing <span class="font-medium">${startItem}</span> to <span class="font-medium">${endItem}</span> of <span class="font-medium">${totalCount}</span> results
            </div>
            <div class="flex gap-2">
                <button onclick="changePage(${currentPage - 1})" ${currentPage === 1 ? 'disabled' : ''} 
                    class="px-3 py-1 rounded ${currentPage === 1 ? 'bg-gray-100 text-gray-400 cursor-not-allowed' : 'bg-blue-600 text-white hover:bg-blue-700'}">
                    Previous
                </button>
                <span class="px-3 py-1 text-gray-700 dark:text-gray-300">
                    Page ${currentPage} of ${totalPages}
                </span>
                <button onclick="changePage(${currentPage + 1})" ${currentPage === totalPages ? 'disabled' : ''} 
                    class="px-3 py-1 rounded ${currentPage === totalPages ? 'bg-gray-100 text-gray-400 cursor-not-allowed' : 'bg-blue-600 text-white hover:bg-blue-700'}">
                    Next
                </button>
            </div>
        </div>
    `;
}

function changePage(page) {
    const totalPages = Math.ceil(totalCount / pageSize);
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    loadEvents();
}

// Auto-refresh functionality
function startAutoRefresh() {
    countdown = 10;
    
    // Clear existing timers
    if (autoRefreshTimer) clearInterval(autoRefreshTimer);
    if (countdownTimer) clearInterval(countdownTimer);
    
    // Countdown timer
    countdownTimer = setInterval(() => {
        countdown--;
        document.getElementById('countdown').textContent = countdown;
        
        if (countdown <= 0) {
            countdown = 10;
            loadData();
        }
    }, 1000);
}

// Utility function to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Close modal on Escape key
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        closeTimeline();
    }
});

// Dark mode toggle
function toggleDarkMode() {
    const html = document.documentElement;
    if (html.classList.contains('dark')) {
        html.classList.remove('dark');
        localStorage.theme = 'light';
    } else {
        html.classList.add('dark');
        localStorage.theme = 'dark';
    }
}
