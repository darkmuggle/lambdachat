document.addEventListener('DOMContentLoaded', () => {
    // Elements - Main UI
    const logsContainer = document.getElementById('logs-container');
    const emptyState = document.getElementById('empty-state');
    const statusIndicator = document.getElementById('status-indicator');
    const statusText = document.getElementById('status-text');
    const logCount = document.getElementById('log-count');
    const connectionInfo = document.getElementById('connection-info');
    const lastTimestamp = document.getElementById('last-timestamp');
    
    // Elements - Filter Controls
    const filterAll = document.getElementById('filter-all');
    const filterLogs = document.getElementById('filter-logs');
    const filterUser = document.getElementById('filter-user');
    const filterResponse = document.getElementById('filter-response');
    
    // Elements - Buttons
    const clearLogsBtn = document.getElementById('clear-logs');
    const autoScrollToggle = document.getElementById('auto-scroll-toggle');
    
    // Elements - Stats
    const totalCount = document.getElementById('total-count');
    const logsCount = document.getElementById('logs-count');
    const userCount = document.getElementById('user-count');
    const responseCount = document.getElementById('response-count');
    const connectionStatus = document.getElementById('connection-status');
    const websocketStatus = document.getElementById('websocket-status');
    
    // Elements - Lists
    const channelsList = document.getElementById('channels-list');
    const usersList = document.getElementById('users-list');
    
    // Variables
    let entries = [];
    let activeFilter = 'all';
    let isAutoScrollEnabled = true;
    let activeChannels = new Set();
    let activeUsers = new Set();
    let stats = {
        total: 0,
        logs: 0,
        userInputs: 0,
        responses: 0
    };
    
    // Function to format timestamp
    function formatTimestamp(timestamp) {
        const date = new Date(timestamp);
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }
    
    // Function to format date for last timestamp
    function formatDateTimestamp(timestamp) {
        const date = new Date(timestamp);
        return date.toLocaleString([], { 
            year: 'numeric', 
            month: 'short', 
            day: 'numeric',
            hour: '2-digit', 
            minute: '2-digit', 
            second: '2-digit' 
        });
    }
    
    // Function to create a log entry element
    function createLogEntryElement(entry) {
        const logEntry = document.createElement('div');
        logEntry.className = `log-entry ${entry.type}`;
        logEntry.dataset.type = entry.type;
        
        const logEntryHeader = document.createElement('div');
        logEntryHeader.className = 'log-entry-header';
        
        const logEntryType = document.createElement('span');
        logEntryType.className = `log-entry-type ${entry.type}`;
        
        let typeText;
        switch (entry.type) {
            case 'log':
                typeText = 'LOG';
                break;
            case 'user-input':
                typeText = 'USER INPUT';
                break;
            case 'response':
                typeText = 'RESPONSE';
                break;
            default:
                typeText = entry.type.toUpperCase();
        }
        logEntryType.textContent = typeText;
        
        const logEntryTime = document.createElement('span');
        logEntryTime.className = 'log-entry-time';
        logEntryTime.textContent = formatTimestamp(entry.timestamp);
        
        logEntryHeader.appendChild(logEntryType);
        logEntryHeader.appendChild(logEntryTime);
        
        const logEntryContent = document.createElement('div');
        logEntryContent.className = 'log-entry-content';
        logEntryContent.textContent = entry.content;
        
        logEntry.appendChild(logEntryHeader);
        logEntry.appendChild(logEntryContent);
        
        // Add user and channel info for user-input and response types
        if ((entry.type === 'user-input' || entry.type === 'response') && (entry.user || entry.channel)) {
            const userDetails = document.createElement('div');
            userDetails.className = 'user-details';
            
            // Create separate spans for user and channel for better styling
            if (entry.user) {
                const userSpan = document.createElement('span');
                userSpan.textContent = `User: ${entry.user}`;
                userDetails.appendChild(userSpan);
                
                // Add to active users
                if (entry.user) {
                    addActiveUser(entry.user);
                }
            }
            
            if (entry.channel) {
                const channelSpan = document.createElement('span');
                channelSpan.textContent = `Channel: ${entry.channel}`;
                userDetails.appendChild(channelSpan);
                
                // Add to active channels
                if (entry.channel) {
                    addActiveChannel(entry.channel);
                }
            }
            
            logEntry.appendChild(userDetails);
        }
        
        return logEntry;
    }
    
    // Function to filter the log entries
    function filterEntries(type) {
        const allEntries = logsContainer.querySelectorAll('.log-entry');
        
        if (type === 'all') {
            allEntries.forEach(entry => {
                entry.style.display = 'block';
            });
        } else {
            allEntries.forEach(entry => {
                if (entry.dataset.type === type) {
                    entry.style.display = 'block';
                } else {
                    entry.style.display = 'none';
                }
            });
        }
        
        // Update active filter buttons
        updateFilterButtons(type);
    }
    
    // Function to update the filter buttons state
    function updateFilterButtons(activeType) {
        filterAll.classList.remove('active');
        filterLogs.classList.remove('active');
        filterUser.classList.remove('active');
        filterResponse.classList.remove('active');
        
        switch (activeType) {
            case 'all':
                filterAll.classList.add('active');
                break;
            case 'log':
                filterLogs.classList.add('active');
                break;
            case 'user-input':
                filterUser.classList.add('active');
                break;
            case 'response':
                filterResponse.classList.add('active');
                break;
        }
    }
    
    // Function to add an active channel to the sidebar
    function addActiveChannel(channel) {
        if (!activeChannels.has(channel)) {
            activeChannels.add(channel);
            updateChannelsList();
        }
    }
    
    // Function to add an active user to the sidebar
    function addActiveUser(user) {
        if (!activeUsers.has(user)) {
            activeUsers.add(user);
            updateUsersList();
        }
    }
    
    // Function to update the channels list
    function updateChannelsList() {
        channelsList.innerHTML = '';
        
        if (activeChannels.size === 0) {
            const emptyItem = document.createElement('li');
            emptyItem.className = 'empty-channel-msg';
            emptyItem.textContent = 'No active channels';
            channelsList.appendChild(emptyItem);
        } else {
            activeChannels.forEach(channel => {
                const li = document.createElement('li');
                li.textContent = channel;
                channelsList.appendChild(li);
            });
        }
    }
    
    // Function to update the users list
    function updateUsersList() {
        usersList.innerHTML = '';
        
        if (activeUsers.size === 0) {
            const emptyItem = document.createElement('li');
            emptyItem.className = 'empty-user-msg';
            emptyItem.textContent = 'No active users';
            usersList.appendChild(emptyItem);
        } else {
            activeUsers.forEach(user => {
                const li = document.createElement('li');
                li.textContent = user;
                usersList.appendChild(li);
            });
        }
    }
    
    // Function to update statistics
    function updateStats() {
        totalCount.textContent = stats.total;
        logsCount.textContent = stats.logs;
        userCount.textContent = stats.userInputs;
        responseCount.textContent = stats.responses;
        logCount.textContent = `${stats.total} entries`;
    }
    
    // Function to add an entry to the logs container
    function addEntryToLogs(entry) {
        // Hide empty state
        if (entries.length === 0) {
            emptyState.classList.add('hidden');
        }
        
        // Add to entries array
        entries.push(entry);
        
        // Update stats
        stats.total++;
        switch (entry.type) {
            case 'log':
                stats.logs++;
                break;
            case 'user-input':
                stats.userInputs++;
                break;
            case 'response':
                stats.responses++;
                break;
        }
        updateStats();
        
        // Create and append the log entry element
        const logEntryElement = createLogEntryElement(entry);
        
        // Apply current filter
        if (activeFilter !== 'all' && entry.type !== activeFilter) {
            logEntryElement.style.display = 'none';
        }
        
        logsContainer.appendChild(logEntryElement);
        
        // Auto-scroll if enabled
        if (isAutoScrollEnabled) {
            logsContainer.scrollTop = logsContainer.scrollHeight;
        }
        
        // Update last timestamp
        lastTimestamp.textContent = formatDateTimestamp(entry.timestamp);
    }
    
    // Function to clear logs
    function clearLogs() {
        entries = [];
        activeChannels.clear();
        activeUsers.clear();
        stats = {
            total: 0,
            logs: 0,
            userInputs: 0,
            responses: 0
        };
        
        // Clear DOM
        logsContainer.innerHTML = '';
        logsContainer.appendChild(emptyState);
        emptyState.classList.remove('hidden');
        
        // Update UI
        updateStats();
        updateChannelsList();
        updateUsersList();
        lastTimestamp.textContent = '--';
    }
    
    // Function to toggle auto-scroll
    function toggleAutoScroll() {
        isAutoScrollEnabled = !isAutoScrollEnabled;
        autoScrollToggle.classList.toggle('active', isAutoScrollEnabled);
        
        if (isAutoScrollEnabled) {
            logsContainer.scrollTop = logsContainer.scrollHeight;
        }
    }
    
    // Function to update connection status
    function updateConnectionStatus(isConnected) {
        if (isConnected) {
            statusIndicator.className = 'connected';
            statusText.textContent = 'Connected';
            connectionInfo.textContent = 'WebSocket: connected';
            connectionStatus.textContent = 'Connected';
            connectionStatus.className = 'connection-value connected';
            websocketStatus.textContent = 'Active';
            websocketStatus.className = 'connection-value connected';
        } else {
            statusIndicator.className = 'disconnected';
            statusText.textContent = 'Disconnected';
            connectionInfo.textContent = 'WebSocket: disconnected';
            connectionStatus.textContent = 'Disconnected';
            connectionStatus.className = 'connection-value disconnected';
            websocketStatus.textContent = 'Inactive';
            websocketStatus.className = 'connection-value disconnected';
        }
    }
    
    // Function to connect to WebSocket
    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        const ws = new WebSocket(wsUrl);
        
        ws.onopen = () => {
            console.log('Connected to WebSocket');
            updateConnectionStatus(true);
        };
        
        ws.onmessage = (event) => {
            try {
                const entry = JSON.parse(event.data);
                addEntryToLogs(entry);
            } catch (error) {
                console.error('Error parsing WebSocket message:', error);
            }
        };
        
        ws.onclose = () => {
            console.log('Disconnected from WebSocket');
            updateConnectionStatus(false);
            
            // Attempt to reconnect after a delay
            setTimeout(() => {
                console.log('Attempting to reconnect...');
                connectWebSocket();
            }, 3000);
        };
        
        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            updateConnectionStatus(false);
        };
    }
    
    // Event Listeners
    
    // Filter buttons
    filterAll.addEventListener('click', () => {
        activeFilter = 'all';
        filterEntries('all');
    });
    
    filterLogs.addEventListener('click', () => {
        activeFilter = 'log';
        filterEntries('log');
    });
    
    filterUser.addEventListener('click', () => {
        activeFilter = 'user-input';
        filterEntries('user-input');
    });
    
    filterResponse.addEventListener('click', () => {
        activeFilter = 'response';
        filterEntries('response');
    });
    
    // Action buttons
    clearLogsBtn.addEventListener('click', clearLogs);
    autoScrollToggle.addEventListener('click', toggleAutoScroll);
    
    // Listen for scroll events to toggle auto-scroll
    logsContainer.addEventListener('scroll', () => {
        const { scrollTop, scrollHeight, clientHeight } = logsContainer;
        const wasAutoScrollEnabled = isAutoScrollEnabled;
        isAutoScrollEnabled = Math.abs(scrollHeight - clientHeight - scrollTop) < 10;
        
        if (wasAutoScrollEnabled !== isAutoScrollEnabled) {
            autoScrollToggle.classList.toggle('active', isAutoScrollEnabled);
        }
    });
    
    // Initialize
    updateConnectionStatus(false);
    updateStats();
    updateChannelsList();
    updateUsersList();
    
    // Initialize WebSocket connection
    connectWebSocket();
});
