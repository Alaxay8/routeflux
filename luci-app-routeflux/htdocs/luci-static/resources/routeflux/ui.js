'use strict';
'require baseclass';
'require ui';

var themePreferenceKey = 'routeflux.ui.theme.preference';

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function hasContent(value) {
	if (Array.isArray(value))
		return value.length > 0;

	return trim(value) !== '';
}

function pad2(value) {
	value = Number(value) || 0;
	return value < 10 ? '0' + value : String(value);
}

function appendClass(base, extra) {
	var suffix = trim(extra);

	if (suffix === '')
		return base;

	return trim(base + ' ' + suffix);
}

function normalizeChildren(value) {
	return Array.isArray(value) ? value : [ value ];
}

function readLocalStorageValue(key) {
	var normalizedKey = trim(key);

	if (normalizedKey === '' || typeof window === 'undefined' || !window.localStorage)
		return null;

	try {
		return window.localStorage.getItem(normalizedKey);
	}
	catch (err) {
		return null;
	}
}

function writeLocalStorageValue(key, value) {
	var normalizedKey = trim(key);

	if (normalizedKey === '' || typeof window === 'undefined' || !window.localStorage)
		return;

	try {
		window.localStorage.setItem(normalizedKey, String(value));
	}
	catch (err) {
	}
}

function parseDurationMilliseconds(value) {
	var normalized = trim(value);
	var pattern = /(-?\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g;
	var unitMap = {
		'ns': 0.000001,
		'us': 0.001,
		'µs': 0.001,
		'ms': 1,
		's': 1000,
		'm': 60000,
		'h': 3600000
	};
	var match;
	var matchedLength = 0;
	var total = 0;

	if (typeof value === 'number' && isFinite(value))
		return value;

	if (normalized === '')
		return null;

	if (/^-?\d+(?:\.\d+)?$/.test(normalized))
		return Number(normalized);

	while ((match = pattern.exec(normalized)) !== null) {
		matchedLength += match[0].length;
		total += Number(match[1]) * unitMap[match[2]];
	}

	if (matchedLength !== normalized.length)
		return null;

	return total;
}

return baseclass.extend({
	formatTimestamp: function(value) {
		var normalized = trim(value);
		var parsed;

		if (normalized === '')
			return '';

		parsed = new Date(normalized);
		if (isNaN(parsed.getTime()))
			return normalized;

		return parsed.getFullYear() + '-' +
			pad2(parsed.getMonth() + 1) + '-' +
			pad2(parsed.getDate()) + ' ' +
			pad2(parsed.getHours()) + ':' +
			pad2(parsed.getMinutes()) + ':' +
			pad2(parsed.getSeconds());
	},

	durationToMilliseconds: function(value) {
		return parseDurationMilliseconds(value);
	},

	formatLatencyMS: function(value) {
		var milliseconds = parseDurationMilliseconds(value);

		if (milliseconds == null || !isFinite(milliseconds))
			return '';

		return Math.round(milliseconds) + ' ms';
	},

	readSessionJSON: function(key) {
		var normalizedKey = trim(key);
		var raw;

		if (normalizedKey === '' || typeof window === 'undefined' || !window.sessionStorage)
			return null;

		try {
			raw = window.sessionStorage.getItem(normalizedKey);
			return raw ? JSON.parse(raw) : null;
		}
		catch (err) {
			return null;
		}
	},

	writeSessionJSON: function(key, value) {
		var normalizedKey = trim(key);

		if (normalizedKey === '' || typeof window === 'undefined' || !window.sessionStorage)
			return;

		try {
			window.sessionStorage.setItem(normalizedKey, JSON.stringify(value));
		}
		catch (err) {
		}
	},

	currentTheme: function() {
		var stored = trim(readLocalStorageValue(themePreferenceKey)).toLowerCase();

		return stored === 'light' ? 'light' : 'dark';
	},

	setThemePreference: function(value) {
		var normalized = trim(value).toLowerCase();

		writeLocalStorageValue(themePreferenceKey, normalized === 'light' ? 'light' : 'dark');
	},

	withThemeClass: function(className) {
		return appendClass(trim(className), 'routeflux-theme-' + this.currentTheme());
	},

	statusTone: function(connected) {
		return connected === true ? 'connected' : 'disconnected';
	},

	isPendingAction: function(view, key) {
		var normalizedKey = trim(key);
		var actions = view && view.pendingActions;

		if (normalizedKey === '' || !actions)
			return false;

		return actions[normalizedKey] != null;
	},

	pendingActionMessage: function(view, key) {
		var normalizedKey = trim(key);
		var actions = view && view.pendingActions;

		if (normalizedKey === '' || !actions || !actions[normalizedKey])
			return '';

		return trim(actions[normalizedKey].message);
	},

	runPendingAction: function(view, key, executor, options) {
		var normalizedKey = trim(key);
		var settings = options || {};
		var actions;

		if (normalizedKey === '')
			return Promise.reject(new Error('missing action key'));

		if (typeof executor !== 'function')
			return Promise.reject(new Error('missing action executor'));

		view.pendingActions = view.pendingActions || {};
		actions = view.pendingActions;
		if (actions[normalizedKey] != null)
			return Promise.resolve(false);

		actions[normalizedKey] = {
			'message': trim(settings.message)
		};

		if (view && typeof view.renderIntoRoot === 'function')
			view.renderIntoRoot();

		return Promise.resolve().then(executor).finally(function() {
			delete actions[normalizedKey];
			if (view && typeof view.renderIntoRoot === 'function')
				view.renderIntoRoot();
		});
	},

	showModal: function(title, body, options) {
		var settings = options || {};
		var buttons = Array.isArray(settings.actions) ? settings.actions.slice() : [];
		var modalClass = trim(settings.modalClass || settings.bodyClass);
		var bodyClass = appendClass('routeflux-modal-body', settings.bodyClass);

		if (buttons.length === 0) {
			buttons.push(E('button', {
				'class': 'cbi-button',
				'click': function(ev) {
					ui.hideModal();
					return false;
				}
			}, [ _('Close') ]));
		}

		if (modalClass !== '') {
			ui.showModal(title, [
				E('div', { 'class': bodyClass }, normalizeChildren(body)),
				E('div', { 'class': 'routeflux-modal-actions' }, buttons)
			], modalClass);
			return;
		}

		ui.showModal(title, [
			E('div', { 'class': bodyClass }, normalizeChildren(body)),
			E('div', { 'class': 'routeflux-modal-actions' }, buttons)
		]);
	},

	renderSharedStyles: function() {
		return E('style', { 'type': 'text/css' }, [
			'.routeflux-theme-dark { --routeflux-bg:#070b14; --routeflux-bg-soft:#0b1220; --routeflux-surface:#0f1725; --routeflux-surface-elevated:#141f31; --routeflux-surface-muted:rgba(116, 151, 196, 0.08); --routeflux-border:rgba(146, 178, 224, 0.16); --routeflux-border-strong:rgba(132, 191, 255, 0.34); --routeflux-text-primary:#eef4ff; --routeflux-text-secondary:#c7d4e8; --routeflux-text-muted:#91a2bd; --routeflux-accent:#58c4ff; --routeflux-accent-strong:#2ea7ff; --routeflux-accent-soft:rgba(88, 196, 255, 0.14); --routeflux-success:#2ed8aa; --routeflux-success-soft:rgba(46, 216, 170, 0.14); --routeflux-danger:#ff7b8c; --routeflux-danger-soft:rgba(255, 123, 140, 0.14); --routeflux-shadow:0 26px 60px rgba(0, 0, 0, 0.34); --routeflux-shadow-soft:0 18px 36px rgba(0, 0, 0, 0.24); }',
			'.routeflux-theme-light { --routeflux-bg:#f3f6fb; --routeflux-bg-soft:#e8eef5; --routeflux-surface:#f8fbfd; --routeflux-surface-elevated:#fcfdfe; --routeflux-surface-muted:rgba(116, 134, 156, 0.08); --routeflux-border:rgba(125, 146, 170, 0.22); --routeflux-border-strong:rgba(37, 99, 235, 0.24); --routeflux-text-primary:#162638; --routeflux-text-secondary:#41566d; --routeflux-text-muted:#6a7c91; --routeflux-accent:#2563eb; --routeflux-accent-strong:#1d4ed8; --routeflux-accent-soft:rgba(37, 99, 235, 0.1); --routeflux-success:#15803d; --routeflux-success-soft:rgba(22, 163, 74, 0.12); --routeflux-danger:#b91c1c; --routeflux-danger-soft:rgba(220, 38, 38, 0.1); --routeflux-shadow:0 22px 52px rgba(63, 87, 118, 0.12); --routeflux-shadow-soft:0 16px 30px rgba(63, 87, 118, 0.08); }',
			'.routeflux-page-shell { position:relative; width:100%; max-width:100%; min-width:0; padding:22px 0 34px; color:var(--routeflux-text-primary); }',
			'.routeflux-page-shell, .routeflux-page-shell * { box-sizing:border-box; }',
			'.routeflux-page-shell.routeflux-theme-dark::before { content:""; position:absolute; inset:-18px -12px auto; height:260px; border-radius:32px; background:radial-gradient(circle at 0% 0%, rgba(88, 196, 255, 0.18) 0%, rgba(88, 196, 255, 0) 52%), radial-gradient(circle at 100% 0%, rgba(130, 108, 255, 0.12) 0%, rgba(130, 108, 255, 0) 44%), linear-gradient(180deg, rgba(17, 25, 41, 0.94) 0%, rgba(7, 11, 20, 0.98) 100%); z-index:0; pointer-events:none; }',
			'.routeflux-page-shell.routeflux-theme-dark::after { content:""; position:absolute; inset:180px 12% auto auto; width:240px; height:240px; border-radius:999px; background:radial-gradient(circle, rgba(88, 196, 255, 0.09) 0%, rgba(88, 196, 255, 0) 68%); filter:blur(8px); z-index:0; pointer-events:none; }',
			'.routeflux-page-shell.routeflux-theme-light::before { content:""; position:absolute; inset:-18px -12px auto; height:260px; border-radius:32px; background:radial-gradient(circle at 0% 0%, rgba(147, 197, 253, 0.16) 0%, rgba(147, 197, 253, 0) 50%), radial-gradient(circle at 100% 0%, rgba(191, 219, 254, 0.12) 0%, rgba(191, 219, 254, 0) 42%), linear-gradient(180deg, rgba(248, 250, 253, 0.98) 0%, rgba(239, 244, 249, 0.99) 100%); z-index:0; pointer-events:none; }',
			'.routeflux-page-shell.routeflux-theme-light::after { content:""; position:absolute; inset:180px 12% auto auto; width:240px; height:240px; border-radius:999px; background:radial-gradient(circle, rgba(37, 99, 235, 0.05) 0%, rgba(37, 99, 235, 0) 68%); filter:blur(8px); z-index:0; pointer-events:none; }',
			'.routeflux-page-shell > * { position:relative; z-index:1; }',
			'.routeflux-page-shell h2 { margin:0 0 10px; color:var(--routeflux-text-primary); font-size:clamp(28px, 1.6vw + 22px, 42px); line-height:1.06; letter-spacing:-0.04em; }',
			'.routeflux-page-shell h3 { margin:0; color:var(--routeflux-text-primary); font-size:clamp(18px, 1vw + 14px, 26px); line-height:1.16; letter-spacing:-0.03em; }',
			'.routeflux-page-shell p, .routeflux-page-shell li, .routeflux-page-shell label, .routeflux-page-shell summary, .routeflux-page-shell pre, .routeflux-page-shell code { color:var(--routeflux-text-secondary); line-height:1.62; }',
			'.routeflux-page-shell .cbi-section-descr, .routeflux-page-shell .cbi-value-description { margin:0; color:var(--routeflux-text-muted); font-size:15px; line-height:1.7; }',
			'.routeflux-page-shell .cbi-value-title { display:block; margin-bottom:8px; color:var(--routeflux-text-secondary); font-size:12px; font-weight:800; letter-spacing:.1em; text-transform:uppercase; }',
			'.routeflux-page-shell .cbi-section, .routeflux-surface { position:relative; margin:0 0 18px; padding:20px; border:1px solid var(--routeflux-border); border-radius:24px; background:linear-gradient(180deg, rgba(20, 31, 49, 0.94) 0%, rgba(12, 20, 33, 0.98) 100%); box-shadow:var(--routeflux-shadow-soft), inset 0 1px 0 rgba(255, 255, 255, 0.04); overflow:hidden; }',
			'.routeflux-surface::before, .routeflux-page-shell .cbi-section::before { content:""; position:absolute; inset:0 0 auto; height:1px; background:linear-gradient(90deg, rgba(88, 196, 255, 0.38) 0%, rgba(88, 196, 255, 0.08) 42%, rgba(88, 196, 255, 0) 100%); pointer-events:none; }',
			'.routeflux-surface-elevated { background:linear-gradient(180deg, rgba(20, 31, 49, 0.98) 0%, rgba(13, 21, 35, 1) 100%); border-color:var(--routeflux-border-strong); box-shadow:var(--routeflux-shadow), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:14px; margin:0 0 18px; }',
			'.routeflux-card { border:1px solid rgba(125, 159, 204, 0.16); border-radius:20px; padding:16px 16px 17px; min-height:104px; background:linear-gradient(180deg, rgba(16, 25, 40, 0.96) 0%, rgba(11, 18, 31, 1) 100%); box-shadow:0 18px 34px rgba(0, 0, 0, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.03); overflow:hidden; }',
			'.routeflux-card-primary { border-color:rgba(88, 196, 255, 0.28); box-shadow:0 24px 42px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-card-accent { height:4px; width:96px; border-radius:999px; margin-bottom:14px; background:linear-gradient(90deg, var(--routeflux-accent) 0%, #89d8ff 100%); box-shadow:0 0 18px rgba(88, 196, 255, 0.28); }',
			'.routeflux-card-label { color:var(--routeflux-text-muted); font-size:11px; margin-bottom:10px; text-transform:uppercase; letter-spacing:.14em; font-weight:800; }',
			'.routeflux-card-value { color:var(--routeflux-text-primary); font-size:16px; font-weight:700; line-height:1.45; word-break:break-word; }',
			'.routeflux-card-primary .routeflux-card-value { font-size:18px; }',
			'.routeflux-card-connected { border-color:rgba(46, 216, 170, 0.26); background:linear-gradient(180deg, rgba(12, 34, 32, 0.96) 0%, rgba(10, 22, 25, 1) 100%); }',
			'.routeflux-card-connected .routeflux-card-label { color:#90d8c5; }',
			'.routeflux-card-connected .routeflux-card-value { color:#ecfff8; }',
			'.routeflux-card-connected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, var(--routeflux-success) 0%, #74ffd8 100%); box-shadow:0 0 18px rgba(46, 216, 170, 0.3); }',
			'.routeflux-card-disconnected { border-color:rgba(145, 162, 189, 0.18); background:linear-gradient(180deg, rgba(19, 26, 38, 0.96) 0%, rgba(11, 18, 30, 1) 100%); }',
			'.routeflux-card-disconnected .routeflux-card-label { color:#a7b7ce; }',
			'.routeflux-card-disconnected .routeflux-card-value { color:#e8eef7; }',
			'.routeflux-card-disconnected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, #7b91b5 0%, #adc1df 100%); box-shadow:0 0 18px rgba(123, 145, 181, 0.24); }',
			'.routeflux-page-hero { display:grid; grid-template-columns:minmax(0, 1.3fr) minmax(320px, .9fr); gap:20px; align-items:start; margin-bottom:18px; }',
			'.routeflux-page-hero-copy { min-width:0; }',
			'.routeflux-page-kicker { display:inline-flex; align-items:center; min-height:30px; padding:0 12px; border-radius:999px; margin-bottom:14px; background:var(--routeflux-accent-soft); color:#9ddfff; font-size:11px; font-weight:800; letter-spacing:.16em; text-transform:uppercase; }',
			'.routeflux-page-hero-title { margin:0 0 10px; color:var(--routeflux-text-primary); font-size:clamp(34px, 2.2vw + 22px, 56px); line-height:1; letter-spacing:-0.05em; }',
			'.routeflux-page-hero-description { margin:0; max-width:64ch; color:var(--routeflux-text-muted); line-height:1.72; }',
			'.routeflux-page-hero-meta { display:flex; flex-wrap:wrap; gap:10px; margin-top:16px; }',
			'.routeflux-page-hero-meta-item { display:grid; gap:4px; min-width:150px; padding:12px 14px; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(8, 15, 26, 0.36); }',
			'.routeflux-page-hero-meta-label { color:var(--routeflux-text-muted); font-size:10px; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }',
			'.routeflux-page-hero-meta-value { color:var(--routeflux-text-primary); font-size:15px; font-weight:700; word-break:break-word; }',
			'.routeflux-page-hero-actions { display:grid; gap:12px; align-content:start; }',
			'.routeflux-section-heading { display:flex; flex-wrap:wrap; justify-content:space-between; gap:12px 18px; align-items:end; margin-bottom:14px; }',
			'.routeflux-section-heading-copy { display:grid; gap:6px; min-width:0; }',
			'.routeflux-section-heading-copy p { margin:0; color:var(--routeflux-text-muted); }',
			'.routeflux-section-heading-actions { display:flex; flex-wrap:wrap; gap:10px; align-items:center; }',
			'.routeflux-page-shell .table, .routeflux-data-table { width:100%; margin:0; border-collapse:separate; border-spacing:0; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(7, 11, 20, 0.34); overflow:hidden; }',
			'.routeflux-page-shell .table .th, .routeflux-page-shell .table .td, .routeflux-data-table .th, .routeflux-data-table .td { padding:16px 14px; border-top:1px solid rgba(145, 175, 220, 0.1); color:var(--routeflux-text-secondary); background:transparent; vertical-align:top; }',
			'.routeflux-page-shell .table .tr:first-child .th, .routeflux-page-shell .table .tr:first-child .td, .routeflux-data-table .tr:first-child .th, .routeflux-data-table .tr:first-child .td { border-top:0; }',
			'.routeflux-page-shell .table .th, .routeflux-data-table .th { color:var(--routeflux-text-muted); font-size:11px; font-weight:800; letter-spacing:.12em; text-transform:uppercase; background:rgba(145, 175, 220, 0.04); }',
			'.routeflux-page-shell .table .td, .routeflux-data-table .td { color:var(--routeflux-text-primary); }',
			'.routeflux-page-shell .table .tr:hover .td, .routeflux-data-table .tr:hover .td { background:rgba(88, 196, 255, 0.03); }',
			'.routeflux-page-shell .label { display:inline-flex; align-items:center; min-height:28px; padding:0 11px; border-radius:999px; border:1px solid rgba(145, 175, 220, 0.16); background:rgba(145, 175, 220, 0.08); color:var(--routeflux-text-secondary); font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
			'.routeflux-page-shell .label.notice { border-color:rgba(88, 196, 255, 0.28); background:var(--routeflux-accent-soft); color:#9ddfff; }',
			'.routeflux-page-shell .label.warning { border-color:rgba(255, 123, 140, 0.28); background:var(--routeflux-danger-soft); color:#ffb7c0; }',
			'.routeflux-page-shell .cbi-input-text, .routeflux-page-shell .cbi-input-textarea, .routeflux-page-shell select, .routeflux-page-shell textarea, .routeflux-page-shell input[type="text"], .routeflux-page-shell input[type="number"] { width:100%; max-width:100%; min-height:46px; padding:0 14px; border:1px solid rgba(145, 175, 220, 0.16); border-radius:16px; background:rgba(6, 12, 22, 0.72); color:var(--routeflux-text-primary); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-page-shell textarea, .routeflux-page-shell .cbi-input-textarea { min-height:148px; padding:14px 16px; resize:vertical; }',
			'.routeflux-page-shell .cbi-input-text::placeholder, .routeflux-page-shell .cbi-input-textarea::placeholder, .routeflux-page-shell textarea::placeholder, .routeflux-page-shell input::placeholder { color:var(--routeflux-text-muted); opacity:.84; }',
			'.routeflux-page-shell .cbi-input-text:focus, .routeflux-page-shell .cbi-input-textarea:focus, .routeflux-page-shell select:focus, .routeflux-page-shell textarea:focus, .routeflux-page-shell input:focus { outline:none; border-color:rgba(88, 196, 255, 0.54); box-shadow:0 0 0 1px rgba(88, 196, 255, 0.18), 0 0 0 8px rgba(88, 196, 255, 0.05); }',
			'.routeflux-page-shell pre { margin:0; padding:14px 16px; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(6, 12, 22, 0.72); color:var(--routeflux-text-primary); overflow:auto; box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-page-shell code { display:inline-block; padding:1px 6px; border-radius:8px; background:rgba(145, 175, 220, 0.08); color:var(--routeflux-text-primary); }',
			'.routeflux-page-shell pre code { display:inline; padding:0; border-radius:0; background:transparent; }',
			'.routeflux-page-shell .cbi-page-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-page-shell .cbi-button, .routeflux-page-shell .btn, .routeflux-button-primary, .routeflux-button-secondary, .routeflux-button-danger { display:inline-flex; align-items:center; justify-content:center; min-height:46px; padding:0 18px; border:1px solid rgba(145, 175, 220, 0.18); border-radius:16px; background:rgba(15, 24, 38, 0.82); color:var(--routeflux-text-primary); box-shadow:0 12px 24px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03); transition:transform .16s ease, border-color .16s ease, box-shadow .16s ease, background .16s ease, color .16s ease; }',
			'.routeflux-page-shell .cbi-button:hover, .routeflux-page-shell .btn:hover, .routeflux-button-primary:hover, .routeflux-button-secondary:hover, .routeflux-button-danger:hover { transform:translateY(-1px); border-color:rgba(145, 190, 246, 0.28); box-shadow:0 16px 26px rgba(0, 0, 0, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.04); }',
			'.routeflux-page-shell .cbi-button:focus, .routeflux-page-shell .btn:focus, .routeflux-button-primary:focus, .routeflux-button-secondary:focus, .routeflux-button-danger:focus { outline:none; border-color:rgba(88, 196, 255, 0.56); box-shadow:0 0 0 1px rgba(88, 196, 255, 0.18), 0 0 0 8px rgba(88, 196, 255, 0.06); }',
			'.routeflux-page-shell .cbi-button[disabled], .routeflux-page-shell .cbi-button:disabled, .routeflux-page-shell .btn[disabled], .routeflux-page-shell .btn:disabled, .routeflux-button-primary[disabled], .routeflux-button-secondary[disabled], .routeflux-button-danger[disabled] { opacity:.46; cursor:not-allowed; transform:none; box-shadow:none; }',
			'.routeflux-page-shell .cbi-button-apply, .routeflux-page-shell .btn.cbi-button-apply, .routeflux-button-primary { border-color:rgba(88, 196, 255, 0.34); background:linear-gradient(180deg, rgba(52, 147, 235, 0.92) 0%, rgba(30, 116, 211, 0.94) 100%); color:#f4fbff; box-shadow:0 18px 32px rgba(30, 116, 211, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.14); }',
			'.routeflux-page-shell .cbi-button-action, .routeflux-page-shell .btn.cbi-button-action, .routeflux-button-secondary { border-color:rgba(120, 160, 214, 0.2); background:rgba(12, 20, 34, 0.82); color:#a8d7ff; }',
			'.routeflux-page-shell .cbi-button-negative, .routeflux-page-shell .cbi-button-reset, .routeflux-page-shell .btn.cbi-button-negative, .routeflux-page-shell .btn.cbi-button-reset, .routeflux-button-danger { border-color:rgba(255, 123, 140, 0.28); background:rgba(52, 16, 26, 0.82); color:#ffb7c0; box-shadow:0 16px 28px rgba(52, 16, 26, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-page-shell .alert-message, .routeflux-page-shell .routeflux-page-banner { padding:14px 16px; border:1px solid rgba(145, 175, 220, 0.16); border-radius:16px; background:rgba(7, 11, 20, 0.62); color:var(--routeflux-text-secondary); line-height:1.55; }',
			'.routeflux-page-shell .alert-message.notice, .routeflux-page-shell .routeflux-page-banner-info { border-color:rgba(88, 196, 255, 0.24); background:rgba(12, 37, 52, 0.72); color:#b8e8ff; }',
			'.routeflux-page-shell .alert-message.warning, .routeflux-page-shell .routeflux-page-banner-warning { border-color:rgba(255, 123, 140, 0.24); background:rgba(47, 16, 26, 0.72); color:#ffc8cf; }',
			'.routeflux-theme-light .routeflux-surface, .routeflux-page-shell.routeflux-theme-light .cbi-section { background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%); border-color:rgba(125, 146, 170, 0.18); box-shadow:0 16px 30px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
			'.routeflux-theme-light .routeflux-surface::before, .routeflux-page-shell.routeflux-theme-light .cbi-section::before { background:linear-gradient(90deg, rgba(14, 165, 233, 0.28) 0%, rgba(14, 165, 233, 0.08) 42%, rgba(14, 165, 233, 0) 100%); }',
			'.routeflux-theme-light .routeflux-surface-elevated { background:linear-gradient(180deg, rgba(252, 253, 254, 0.99) 0%, rgba(246, 249, 252, 1) 100%); border-color:rgba(37, 99, 235, 0.2); box-shadow:0 20px 36px rgba(63, 87, 118, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.88); }',
			'.routeflux-theme-light .routeflux-card { border-color:rgba(125, 146, 170, 0.15); background:linear-gradient(180deg, rgba(251, 252, 254, 0.98) 0%, rgba(245, 249, 252, 0.98) 100%); box-shadow:0 12px 24px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.88); }',
			'.routeflux-theme-light .routeflux-card-primary { border-color:rgba(37, 99, 235, 0.2); box-shadow:0 16px 28px rgba(63, 87, 118, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-theme-light .routeflux-card-connected { border-color:rgba(22, 163, 74, 0.2); background:linear-gradient(180deg, rgba(250, 252, 252, 0.99) 0%, rgba(238, 248, 242, 0.99) 100%); }',
			'.routeflux-theme-light .routeflux-card-connected .routeflux-card-label { color:#0f766e; }',
			'.routeflux-theme-light .routeflux-card-connected .routeflux-card-value { color:#14532d; }',
			'.routeflux-theme-light .routeflux-card-disconnected { border-color:rgba(148, 163, 184, 0.18); background:linear-gradient(180deg, rgba(251, 252, 254, 0.98) 0%, rgba(242, 246, 250, 0.98) 100%); }',
			'.routeflux-theme-light .routeflux-card-disconnected .routeflux-card-label { color:#64748b; }',
			'.routeflux-theme-light .routeflux-card-disconnected .routeflux-card-value { color:#1e293b; }',
			'.routeflux-theme-light .routeflux-page-kicker { background:rgba(37, 99, 235, 0.08); color:#1d4ed8; }',
			'.routeflux-theme-light .routeflux-page-hero-meta-item { background:rgba(249, 251, 253, 0.9); border-color:rgba(125, 146, 170, 0.18); box-shadow:0 8px 18px rgba(63, 87, 118, 0.06); }',
			'.routeflux-page-shell.routeflux-theme-light .table, .routeflux-theme-light .routeflux-data-table { border-color:rgba(125, 146, 170, 0.16); background:rgba(249, 251, 253, 0.9); }',
			'.routeflux-page-shell.routeflux-theme-light .table .th, .routeflux-theme-light .routeflux-data-table .th { background:rgba(125, 146, 170, 0.06); }',
			'.routeflux-page-shell.routeflux-theme-light .table .tr:hover .td, .routeflux-theme-light .routeflux-data-table .tr:hover .td { background:rgba(14, 165, 233, 0.04); }',
			'.routeflux-page-shell.routeflux-theme-light .label { border-color:rgba(106, 133, 164, 0.16); background:rgba(255, 255, 255, 0.9); color:var(--routeflux-text-secondary); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-input-text, .routeflux-page-shell.routeflux-theme-light .cbi-input-textarea, .routeflux-page-shell.routeflux-theme-light select, .routeflux-page-shell.routeflux-theme-light textarea, .routeflux-page-shell.routeflux-theme-light input[type="text"], .routeflux-page-shell.routeflux-theme-light input[type="number"] { border-color:rgba(125, 146, 170, 0.18); background:linear-gradient(180deg, rgba(249, 251, 253, 0.96) 0%, rgba(243, 247, 251, 0.96) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 6px 16px rgba(63, 87, 118, 0.05); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-input-text:focus, .routeflux-page-shell.routeflux-theme-light .cbi-input-textarea:focus, .routeflux-page-shell.routeflux-theme-light select:focus, .routeflux-page-shell.routeflux-theme-light textarea:focus, .routeflux-page-shell.routeflux-theme-light input:focus { border-color:rgba(37, 99, 235, 0.4); box-shadow:0 0 0 1px rgba(37, 99, 235, 0.12), 0 0 0 6px rgba(37, 99, 235, 0.04); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-section-descr, .routeflux-page-shell.routeflux-theme-light .cbi-value-description { color:var(--routeflux-text-secondary); }',
			'.routeflux-page-shell.routeflux-theme-light pre { border-color:rgba(125, 146, 170, 0.16); background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 8px 18px rgba(63, 87, 118, 0.06); }',
			'.routeflux-page-shell.routeflux-theme-light code { background:rgba(37, 99, 235, 0.08); color:#1e3a8a; }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button, .routeflux-page-shell.routeflux-theme-light .btn, .routeflux-theme-light .routeflux-button-primary, .routeflux-theme-light .routeflux-button-secondary, .routeflux-theme-light .routeflux-button-danger { border-color:rgba(125, 146, 170, 0.16); background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(241, 246, 251, 0.98) 100%); color:var(--routeflux-text-primary); box-shadow:0 10px 20px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.88); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button:hover, .routeflux-page-shell.routeflux-theme-light .btn:hover, .routeflux-theme-light .routeflux-button-primary:hover, .routeflux-theme-light .routeflux-button-secondary:hover, .routeflux-theme-light .routeflux-button-danger:hover { border-color:rgba(37, 99, 235, 0.22); box-shadow:0 12px 22px rgba(63, 87, 118, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button-apply, .routeflux-page-shell.routeflux-theme-light .btn.cbi-button-apply, .routeflux-theme-light .routeflux-button-primary { border-color:rgba(37, 99, 235, 0.34); background:linear-gradient(180deg, #2563eb 0%, #1d4ed8 100%); color:#f8fbff; box-shadow:0 14px 28px rgba(37, 99, 235, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.16); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button-apply:hover, .routeflux-page-shell.routeflux-theme-light .btn.cbi-button-apply:hover, .routeflux-theme-light .routeflux-button-primary:hover { border-color:rgba(29, 78, 216, 0.42); background:linear-gradient(180deg, #1d4ed8 0%, #1e40af 100%); color:#ffffff; box-shadow:0 16px 30px rgba(29, 78, 216, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.18); }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button-action, .routeflux-page-shell.routeflux-theme-light .btn.cbi-button-action, .routeflux-theme-light .routeflux-button-secondary { border-color:rgba(37, 99, 235, 0.18); background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#1d4ed8; }',
			'.routeflux-page-shell.routeflux-theme-light .cbi-button-negative, .routeflux-page-shell.routeflux-theme-light .cbi-button-reset, .routeflux-page-shell.routeflux-theme-light .btn.cbi-button-negative, .routeflux-page-shell.routeflux-theme-light .btn.cbi-button-reset, .routeflux-theme-light .routeflux-button-danger { border-color:rgba(220, 38, 38, 0.18); background:linear-gradient(180deg, rgba(253, 246, 246, 0.98) 0%, rgba(249, 237, 237, 0.98) 100%); color:#b91c1c; box-shadow:0 12px 22px rgba(127, 29, 29, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-page-shell.routeflux-theme-light .alert-message, .routeflux-page-shell.routeflux-theme-light .routeflux-page-banner { background:rgba(255, 255, 255, 0.88); border-color:rgba(106, 133, 164, 0.16); color:var(--routeflux-text-secondary); }',
			'.routeflux-page-shell.routeflux-theme-light .alert-message.notice, .routeflux-page-shell.routeflux-theme-light .routeflux-page-banner-info { background:rgba(239, 248, 255, 0.94); color:#075985; border-color:rgba(56, 189, 248, 0.2); }',
			'.routeflux-page-shell.routeflux-theme-light .alert-message.warning, .routeflux-page-shell.routeflux-theme-light .routeflux-page-banner-warning { background:rgba(254, 242, 242, 0.96); color:#b91c1c; border-color:rgba(239, 68, 68, 0.2); }',
			'.routeflux-modal-body { width:100%; max-width:100%; min-width:0; box-sizing:border-box; overflow:hidden; }',
			'.routeflux-modal-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; margin-top:14px; }',
			'@media (max-width: 980px) { .routeflux-page-hero { grid-template-columns:minmax(0, 1fr); } .routeflux-page-hero-actions { grid-template-columns:minmax(0, 1fr); } }',
			'@media (max-width: 700px) { .routeflux-page-shell { padding-top:14px; } .routeflux-page-shell .cbi-section, .routeflux-surface { padding:16px; border-radius:20px; } .routeflux-overview-grid { gap:12px; } .routeflux-section-heading, .routeflux-section-heading-actions, .routeflux-page-hero-meta { flex-direction:column; align-items:stretch; } .routeflux-page-hero-title { font-size:clamp(28px, 8vw, 42px); } .routeflux-page-shell .cbi-button { width:100%; } }',
			'@media (max-width: 560px) { .routeflux-page-shell .table, .routeflux-data-table { border-radius:16px; } .routeflux-card { min-height:0; } .routeflux-page-hero-meta-item { width:100%; } }'
		]);
	},

	renderSummaryCard: function(label, value, options) {
		var settings = options || {};
		var className = 'routeflux-card';
		var content = value;
		var attrs = {
			'class': className
		};

		if (trim(settings.id) !== '')
			attrs.id = settings.id;

		if (trim(settings.tone) !== '')
			attrs['class'] += ' routeflux-card-' + trim(settings.tone);

		if (settings.primary === true)
			attrs['class'] += ' routeflux-card-primary';

		if (!hasContent(content))
			content = settings.fallback != null ? settings.fallback : '-';

		if (!Array.isArray(content))
			content = [ content ];

		var valueAttrs = { 'class': 'routeflux-card-value' };
		if (trim(settings.valueId) !== '')
			valueAttrs.id = settings.valueId;

		var children = [];

		if (settings.primary === true)
			children.push(E('div', { 'class': 'routeflux-card-accent' }, []));

		children.push(E('div', { 'class': 'routeflux-card-label' }, [ label ]));
		children.push(E('div', valueAttrs, content));

		return E('div', attrs, children);
	}
});
