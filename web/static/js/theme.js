// Theme Manager
class ThemeManager {
	constructor() {
		this.theme = localStorage.getItem('theme') || 'light';
		this.applyTheme();
		this.initToggle();
	}

	applyTheme() {
		document.documentElement.setAttribute('data-theme', this.theme);
		document.body.classList.add(`theme-${this.theme}`);
		document.body.classList.remove('theme-switching');
	}

	toggle() {
		// Add transitioning class to disable transitions during switch
		document.body.classList.add('theme-switching');

		// Toggle theme
		this.theme = this.theme === 'light' ? 'dark' : 'light';
		localStorage.setItem('theme', this.theme);

		// Apply new theme
		setTimeout(() => {
			this.applyTheme();
		}, 10);
	}

	initToggle() {
		const toggle = document.getElementById('theme-toggle');
		if (toggle) {
			toggle.addEventListener('click', () => this.toggle());

			// Update icon based on theme
			this.updateIcon(toggle);
		}
	}

	updateIcon(toggle) {
		const icon = toggle.querySelector('.icon');
		if (icon) {
			icon.textContent = this.theme === 'light' ? '🌙' : '☀️';
		}
	}
}

// Initialize theme manager when DOM is ready
if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', () => new ThemeManager());
} else {
	new ThemeManager();
}
