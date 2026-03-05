// Upload Handler
class UploadHandler {
	constructor() {
		this.uploadForm = document.getElementById('upload-form');
		this.fileInput = document.getElementById('file-input');
		this.dropZone = document.getElementById('drop-zone');
		this.uploadQueue = [];
		this.maxConcurrent = 3;
		this.activeUploads = 0;
		// 配置
		this.timeout = 30 * 60 * 1000; // 30分钟超时（大文件上传）
		this.maxFileSize = 1024 * 1024 * 1024; // 1GB

		this.init();
	}

	init() {
		if (!this.uploadForm) return;

		// Drag and drop handlers
		this.dropZone.addEventListener('dragover', (e) => {
			e.preventDefault();
			this.dropZone.classList.add('drag-over');
		});

		this.dropZone.addEventListener('dragleave', () => {
			this.dropZone.classList.remove('drag-over');
		});

		this.dropZone.addEventListener('drop', (e) => {
			e.preventDefault();
			this.dropZone.classList.remove('drag-over');
			const files = e.dataTransfer.files;
			this.handleFiles(files);
		});

		// File input change
		this.fileInput.addEventListener('change', (e) => {
			this.handleFiles(e.target.files);
		});

		// Form submit
		this.uploadForm.addEventListener('submit', (e) => {
			e.preventDefault();
		});
	}

	handleFiles(files) {
		for (let file of files) {
			this.addToQueue(file);
		}
		this.processQueue();
	}

	addToQueue(file) {
		const uploadId = Date.now() + '-' + Math.random().toString(36).substr(2, 9);

		const uploadItem = {
			id: uploadId,
			file: file,
			status: 'pending',
			progress: 0
		};

		this.uploadQueue.push(uploadItem);
		this.renderUploadItem(uploadItem);
	}

	renderUploadItem(uploadItem) {
		const container = document.getElementById('uploads-list');
		if (!container) return;

		const item = document.createElement('div');
		item.className = 'upload-item';
		item.id = `upload-${uploadItem.id}`;
		item.innerHTML = `
			<div class="upload-info">
				<div class="upload-name">${this.escapeHtml(uploadItem.file.name)}</div>
				<div class="upload-size">${this.formatSize(uploadItem.file.size)}</div>
			</div>
			<div class="upload-progress">
				<div class="progress-bar">
					<div class="progress-fill" style="width: 0%"></div>
				</div>
				<div class="progress-text">0%</div>
			</div>
			<div class="upload-status pending">准备上传</div>
		`;

		container.appendChild(item);
	}

	// HTML 转义防止 XSS
	escapeHtml(text) {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	}

	async processQueue() {
		while (this.uploadQueue.length > 0 && this.activeUploads < this.maxConcurrent) {
			const uploadItem = this.uploadQueue.shift();
			this.activeUploads++;
			await this.uploadFile(uploadItem);
			this.activeUploads--;
		}
	}

	uploadFile(uploadItem) {
		return new Promise((resolve) => {
			const file = uploadItem.file;
			const itemElement = document.getElementById(`upload-${uploadItem.id}`);
			const statusElement = itemElement?.querySelector('.upload-status');
			const progressFill = itemElement?.querySelector('.progress-fill');
			const progressText = itemElement?.querySelector('.progress-text');

			// 更新状态的辅助函数
			const setStatus = (text, className) => {
				if (statusElement) {
					statusElement.textContent = text;
					statusElement.className = 'upload-status ' + className;
				}
			};

			// 文件大小检查
			if (file.size > this.maxFileSize) {
				setStatus(`文件过大 (最大 ${this.formatSize(this.maxFileSize)})`, 'error');
				resolve();
				return;
			}

			setStatus('上传中...', 'uploading');

			const formData = new FormData();
			formData.append('files', file);

			const xhr = new XMLHttpRequest();

			// 设置超时
			xhr.timeout = this.timeout;

			// 进度事件
			xhr.upload.addEventListener('progress', (e) => {
				if (e.lengthComputable) {
					const percentComplete = Math.round((e.loaded / e.total) * 100);
					if (progressFill) progressFill.style.width = percentComplete + '%';
					if (progressText) progressText.textContent = percentComplete + '%';
				}
			});

			// 完成事件
			xhr.addEventListener('load', () => {
				if (xhr.status >= 200 && xhr.status < 300) {
					setStatus('上传成功', 'success');
					if (progressFill) progressFill.style.width = '100%';
					if (progressText) progressText.textContent = '100%';
				} else {
					// 尝试解析错误消息
					let errorMsg = '上传失败';
					try {
						const response = JSON.parse(xhr.responseText);
						if (response.error) {
							errorMsg = response.error;
						} else if (Array.isArray(response) && response[0]?.error) {
							errorMsg = response[0].error;
						}
					} catch (e) {
						errorMsg = `服务器错误 (${xhr.status})`;
					}
					setStatus(errorMsg, 'error');
				}
				resolve();
			});

			// 错误事件
			xhr.addEventListener('error', () => {
				setStatus('网络错误，请检查连接', 'error');
				resolve();
			});

			// 超时事件
			xhr.addEventListener('timeout', () => {
				setStatus('上传超时，请重试', 'error');
				resolve();
			});

			// 中止事件
			xhr.addEventListener('abort', () => {
				setStatus('上传已取消', 'error');
				resolve();
			});

			// 发送请求
			try {
				xhr.open('POST', '/api/upload');
				xhr.send(formData);
			} catch (error) {
				setStatus('上传失败: ' + error.message, 'error');
				resolve();
			}
		});
	}

	formatSize(bytes) {
		if (bytes === 0) return '0 Bytes';
		const k = 1024;
		const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
	}
}

// Initialize upload handler
if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', () => new UploadHandler());
} else {
	new UploadHandler();
}
