// Migration Logs Page JavaScript

function filterLogs(event) {
	event.preventDefault();
	const form = event.target;
	const formData = new FormData(form);
	const params = new URLSearchParams(formData);

	const url = `/admin/migrations/list?${params.toString()}`;
	htmx.ajax('GET', url, {target: '#sessions-list', swap: 'innerHTML'});
}

function clearFilters() {
	document.getElementById('filter-form').reset();
	document.getElementById('filter-date').value = new Date().toISOString().split('T')[0];
	filterLogs({preventDefault: () => {}, target: document.getElementById('filter-form')});
}

async function rotateOldLogs() {
	if (!confirm('Are you sure you want to rotate old migration logs? This will archive logs older than the retention period.')) {
		return;
	}

	try {
		const response = await fetch('/admin/migrations/rotate', {
			method: 'POST'
		});

		if (response.ok) {
			alert('Old logs rotated successfully');
		} else {
			const error = await response.json();
			alert('Failed to rotate logs: ' + (error.error || 'Unknown error'));
		}
	} catch (err) {
		alert('Failed to rotate logs: ' + err.message);
	}
}

function viewSession(sessionId) {
	fetch(`/admin/migrations/session/${sessionId}`)
		.then(response => response.json())
		.then(session => {
			showSessionDetails(session);
		})
		.catch(err => {
			alert('Failed to load session details: ' + err.message);
		});
}

function showSessionDetails(session) {
	// Get or create modal
	let modal = document.getElementById('session-modal');
	if (!modal) {
		modal = document.createElement('div');
		modal.id = 'session-modal';
		modal.className = 'modal';
		document.body.appendChild(modal);
	}

	// Format the request JSON
	let requestJSON = 'Not available';
	if (session.metadata && session.metadata.request_json) {
		try {
			const jsonObj = JSON.parse(session.metadata.request_json);
			requestJSON = JSON.stringify(jsonObj, null, 2);
		} catch (e) {
			requestJSON = session.metadata.request_json;
		}
	}

	// Build modal content
	const pre = document.createElement('pre');
	pre.style.cssText = 'background: var(--gray-100); padding: 1rem; border-radius: 0.375rem; overflow-x: auto; max-height: 400px; font-size: 0.875rem;';
	pre.textContent = requestJSON;

	const modalHTML = '<div class="modal-content">' +
		'<div class="modal-header">' +
		'<h2 style="margin: 0;">Migration Session Details</h2>' +
		'<span class="modal-close" onclick="closeSessionModal()">×</span>' +
		'</div>' +
		'<div class="modal-body">' +
		'<div style="margin-bottom: 1.5rem;">' +
		'<h3 style="font-size: 1rem; margin-bottom: 0.5rem; color: var(--text-primary);">Session Information</h3>' +
		'<div style="display: grid; grid-template-columns: 150px 1fr; gap: 0.5rem; font-size: 0.875rem;">' +
		'<strong>Session ID:</strong><span>' + session.id + '</span>' +
		'<strong>User:</strong><span>' + session.username + '</span>' +
		'<strong>Status:</strong><span>' + session.status + '</span>' +
		'<strong>Total Tasks:</strong><span>' + session.total_tasks + '</span>' +
		'<strong>Completed:</strong><span>' + session.completed_tasks + '</span>' +
		'<strong>Failed:</strong><span>' + session.failed_tasks + '</span>' +
		'<strong>Duration:</strong><span>' + (session.duration_ms / 1000 / 60).toFixed(2) + ' min</span>' +
		'<strong>Data Size:</strong><span>' + formatBytes(session.total_data_size_bytes) + '</span>' +
		'</div>' +
		'</div>' +
		'<div id="json-container">' +
		'<h3 style="font-size: 1rem; margin-bottom: 0.5rem; color: var(--text-primary);">Original Request JSON</h3>' +
		'</div>' +
		'</div>' +
		'</div>';

	modal.innerHTML = modalHTML;
	document.getElementById('json-container').appendChild(pre);
	modal.style.display = 'block';
}

function closeSessionModal() {
	const modal = document.getElementById('session-modal');
	if (modal) {
		modal.style.display = 'none';
	}
}

// Close modal when clicking outside
window.onclick = function(event) {
	const modal = document.getElementById('session-modal');
	if (event.target === modal) {
		closeSessionModal();
	}
}

function formatBytes(bytes) {
	// Always display in MB (Megabytes)
	const megabyte = 1024 * 1024;
	const megabytes = bytes / megabyte;
	return megabytes.toFixed(2) + ' MB';
}
