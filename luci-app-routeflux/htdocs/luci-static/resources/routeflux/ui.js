'use strict';
'require baseclass';
'require ui';

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
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:10px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid rgba(98, 112, 129, 0.46); border-radius:12px; padding:11px 13px; min-height:88px; background:linear-gradient(180deg, rgba(235, 240, 246, 0.98) 0%, rgba(225, 231, 239, 0.98) 100%); box-shadow:0 14px 28px rgba(8, 15, 26, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.46); overflow:hidden; }',
			'.routeflux-card-primary { border-color:rgba(71, 85, 105, 0.58); box-shadow:0 18px 36px rgba(8, 15, 26, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.5); }',
			'.routeflux-card-accent { height:4px; border-radius:999px; margin-bottom:11px; background:linear-gradient(90deg, #38bdf8 0%, #60a5fa 100%); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #56657a); font-size:10px; margin-bottom:7px; text-transform:uppercase; letter-spacing:.14em; font-weight:800; }',
			'.routeflux-card-value { color:var(--text-color-primary, #17263a); font-size:15px; font-weight:700; line-height:1.42; word-break:break-word; }',
			'.routeflux-card-primary .routeflux-card-value { font-size:16px; }',
			'.routeflux-card-connected { background:linear-gradient(180deg, rgba(223, 237, 229, 0.98) 0%, rgba(210, 228, 219, 0.98) 100%); border-color:rgba(34, 112, 83, 0.46); }',
			'.routeflux-card-connected .routeflux-card-label { color:#355a49; }',
			'.routeflux-card-connected .routeflux-card-value { color:#153f30; }',
			'.routeflux-card-connected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, #22c55e 0%, #14b8a6 100%); }',
			'.routeflux-card-disconnected { background:linear-gradient(180deg, rgba(229, 235, 243, 0.98) 0%, rgba(220, 227, 236, 0.98) 100%); border-color:rgba(90, 103, 121, 0.52); }',
			'.routeflux-card-disconnected .routeflux-card-label { color:#526175; }',
			'.routeflux-card-disconnected .routeflux-card-value { color:#1f2d40; }',
			'.routeflux-card-disconnected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, #64748b 0%, #94a3b8 100%); }',
			'.routeflux-modal-body { width:100%; max-width:100%; min-width:0; box-sizing:border-box; overflow:hidden; }',
			'.routeflux-modal-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; margin-top:14px; }'
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
