// Reusable Modal System for Pantopix Corporate Identity

/**
 * Show a message modal (replaces alert())
 * @param {string} title - Modal title
 * @param {string} message - Message to display
 * @param {string} type - Modal type: 'info', 'success', 'error', 'warning'
 * @param {function} onClose - Optional callback when modal is closed
 */
function showModal(title, message, type = 'info', onClose = null) {
	// Get or create modal
	let modal = document.getElementById('message-modal');
	if (!modal) {
		modal = document.createElement('div');
		modal.id = 'message-modal';
		modal.className = 'modal';
		document.body.appendChild(modal);
	}

	// Determine icon and color based on type
	let icon = '';
	let iconColor = '';
	switch (type) {
		case 'success':
			icon = '✓';
			iconColor = 'var(--success)';
			break;
		case 'error':
			icon = '✗';
			iconColor = 'var(--error)';
			break;
		case 'warning':
			icon = '⚠';
			iconColor = 'var(--warning)';
			break;
		case 'info':
		default:
			icon = 'ℹ';
			iconColor = 'var(--primary)';
			break;
	}

	// Build modal content
	const modalHTML = `
		<div class="modal-content" style="max-width: 500px;">
			<div class="modal-header">
				<div style="display: flex; align-items: center; gap: 0.75rem;">
					<span style="color: ${iconColor}; font-size: 1.5rem; font-weight: 700;">${icon}</span>
					<h2 style="margin: 0;">${title}</h2>
				</div>
				<span class="modal-close" onclick="closeMessageModal()">×</span>
			</div>
			<div class="modal-body">
				<p style="margin: 0; color: var(--text-primary); line-height: 1.6;">${message}</p>
			</div>
			<div style="padding: 1rem 1.5rem; border-top: 1px solid var(--gray-200); display: flex; justify-content: flex-end;">
				<button onclick="closeMessageModal()" style="padding: 0.5rem 1.5rem;">OK</button>
			</div>
		</div>
	`;

	modal.innerHTML = modalHTML;
	modal.style.display = 'block';

	// Store the callback
	if (onClose) {
		modal.dataset.onCloseCallback = 'pending';
		window._modalCloseCallback = onClose;
	}
}

/**
 * Show a confirmation modal (replaces confirm())
 * @param {string} title - Modal title
 * @param {string} message - Confirmation message
 * @param {function} onConfirm - Callback when user confirms
 * @param {function} onCancel - Optional callback when user cancels
 */
function showConfirmModal(title, message, onConfirm, onCancel = null) {
	// Get or create modal
	let modal = document.getElementById('confirm-modal');
	if (!modal) {
		modal = document.createElement('div');
		modal.id = 'confirm-modal';
		modal.className = 'modal';
		document.body.appendChild(modal);
	}

	// Build modal content
	const modalHTML = `
		<div class="modal-content" style="max-width: 500px;">
			<div class="modal-header">
				<div style="display: flex; align-items: center; gap: 0.75rem;">
					<span style="color: var(--warning); font-size: 1.5rem; font-weight: 700;">⚠</span>
					<h2 style="margin: 0;">${title}</h2>
				</div>
				<span class="modal-close" onclick="closeConfirmModal(false)">×</span>
			</div>
			<div class="modal-body">
				<p style="margin: 0; color: var(--text-primary); line-height: 1.6;">${message}</p>
			</div>
			<div style="padding: 1rem 1.5rem; border-top: 1px solid var(--gray-200); display: flex; justify-content: flex-end; gap: 0.75rem;">
				<button onclick="closeConfirmModal(false)" style="background: var(--gray-300); color: var(--gray-800); padding: 0.5rem 1.5rem;">Cancel</button>
				<button onclick="closeConfirmModal(true)" style="padding: 0.5rem 1.5rem;">Confirm</button>
			</div>
		</div>
	`;

	modal.innerHTML = modalHTML;
	modal.style.display = 'block';

	// Store the callbacks
	window._modalConfirmCallback = onConfirm;
	window._modalCancelCallback = onCancel;
}

/**
 * Close the message modal
 */
function closeMessageModal() {
	const modal = document.getElementById('message-modal');
	if (modal) {
		modal.style.display = 'none';

		// Execute callback if present
		if (modal.dataset.onCloseCallback === 'pending' && window._modalCloseCallback) {
			const callback = window._modalCloseCallback;
			delete modal.dataset.onCloseCallback;
			delete window._modalCloseCallback;
			callback();
		}
	}
}

/**
 * Close the confirmation modal
 * @param {boolean} confirmed - Whether user confirmed or cancelled
 */
function closeConfirmModal(confirmed) {
	const modal = document.getElementById('confirm-modal');
	if (modal) {
		modal.style.display = 'none';

		// Execute appropriate callback
		if (confirmed && window._modalConfirmCallback) {
			const callback = window._modalConfirmCallback;
			delete window._modalConfirmCallback;
			delete window._modalCancelCallback;
			callback();
		} else if (!confirmed && window._modalCancelCallback) {
			const callback = window._modalCancelCallback;
			delete window._modalConfirmCallback;
			delete window._modalCancelCallback;
			callback();
		} else {
			// Clean up callbacks
			delete window._modalConfirmCallback;
			delete window._modalCancelCallback;
		}
	}
}

// Close modals when clicking outside
window.addEventListener('click', function(event) {
	const messageModal = document.getElementById('message-modal');
	if (event.target === messageModal) {
		closeMessageModal();
	}

	const confirmModal = document.getElementById('confirm-modal');
	if (event.target === confirmModal) {
		closeConfirmModal(false);
	}
});

// Close modals with Escape key
window.addEventListener('keydown', function(event) {
	if (event.key === 'Escape') {
		const messageModal = document.getElementById('message-modal');
		if (messageModal && messageModal.style.display === 'block') {
			closeMessageModal();
		}

		const confirmModal = document.getElementById('confirm-modal');
		if (confirmModal && confirmModal.style.display === 'block') {
			closeConfirmModal(false);
		}
	}
});
