'use strict';
'require view';
'require dom';
'require ui';
'require routeflux.ui as routefluxUI';

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function choiceClass(selected) {
	return selected
		? 'routeflux-settings-choice routeflux-settings-choice-selected'
		: 'routeflux-settings-choice';
}

return view.extend({
	load: function() {
		return Promise.resolve([]);
	},

	handleAppearanceChange: function(ev) {
		this.appearanceDraft = trim(ev && ev.currentTarget && ev.currentTarget.value).toLowerCase() === 'light' ? 'light' : 'dark';
		this.renderIntoRoot();
	},

	handleSaveSettings: function(ev) {
		var nextTheme = this.appearanceDraft || routefluxUI.currentTheme();

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (routefluxUI.currentTheme() === nextTheme) {
			ui.addNotification(null, notificationParagraph(_('No appearance changes to save.')), 'info');
			return Promise.resolve();
		}

		routefluxUI.setThemePreference(nextTheme);
		ui.addNotification(null, notificationParagraph(_('Appearance saved. Reloading the page...')), 'info');
		window.setTimeout(function() {
			window.location.reload();
		}, 150);

		return Promise.resolve();
	},

	renderIntoRoot: function() {
		var root = document.querySelector('#routeflux-settings-root');

		if (root && root.parentNode)
			dom.content(root.parentNode, [ this.render(this.loadedData || []) ]);
	},

	render: function(data) {
		var currentTheme = this.appearanceDraft || routefluxUI.currentTheme();
		var content = [];

		this.loadedData = data;

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-settings-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-settings-choice-grid { display:grid; gap:12px; }',
			'.routeflux-settings-choice { position:relative; display:flex; gap:12px; align-items:flex-start; min-height:96px; padding:16px 18px; border:1px solid rgba(145, 175, 220, 0.16); border-radius:18px; background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); box-shadow:0 18px 32px rgba(0, 0, 0, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.04); transition:transform .18s ease, border-color .18s ease, box-shadow .18s ease, background .18s ease; cursor:pointer; }',
			'.routeflux-settings-choice:hover { transform:translateY(-1px); border-color:rgba(34, 197, 94, 0.28); box-shadow:0 20px 34px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-settings-choice-selected { border-color:rgba(34, 197, 94, 0.42); background:linear-gradient(180deg, rgba(13, 35, 28, 0.96) 0%, rgba(10, 24, 21, 1) 100%); box-shadow:0 22px 38px rgba(8, 23, 19, 0.32), 0 0 0 1px rgba(34, 197, 94, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.06); }',
			'.routeflux-settings-choice-control { position:absolute; width:1px; height:1px; margin:-1px; padding:0; border:0; overflow:hidden; clip:rect(0, 0, 0, 0); clip-path:inset(50%); white-space:nowrap; }',
			'.routeflux-settings-choice-indicator { position:relative; flex:0 0 auto; width:26px; height:26px; margin-top:2px; border:1.5px solid rgba(145, 162, 189, 0.42); border-radius:999px; background:linear-gradient(180deg, rgba(22, 31, 45, 0.96) 0%, rgba(14, 22, 34, 1) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 10px 18px rgba(0, 0, 0, 0.22); transition:border-color .18s ease, box-shadow .18s ease, background .18s ease; }',
			'.routeflux-settings-choice-indicator::after { content:""; position:absolute; inset:0; display:flex; align-items:center; justify-content:center; border-radius:999px; color:transparent; transform:scale(0.62); transition:transform .18s ease, background .18s ease, color .18s ease, box-shadow .18s ease; }',
			'.routeflux-settings-choice-control:focus-visible + .routeflux-settings-choice-indicator { outline:2px solid rgba(34, 197, 94, 0.28); outline-offset:3px; }',
			'.routeflux-settings-choice-selected .routeflux-settings-choice-indicator { border-color:rgba(22, 163, 74, 0.54); background:linear-gradient(180deg, rgba(240, 253, 244, 0.99) 0%, rgba(220, 252, 231, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.96), 0 12px 20px rgba(21, 128, 61, 0.12); }',
			'.routeflux-settings-choice-selected .routeflux-settings-choice-indicator::after { content:"\\2713"; background:linear-gradient(180deg, #22c55e 0%, #15803d 100%); color:#ffffff; font-size:15px; font-weight:900; transform:scale(1); box-shadow:0 10px 18px rgba(21, 128, 61, 0.28); }',
			'.routeflux-settings-choice-copy { display:grid; gap:6px; flex:1 1 auto; min-width:0; }',
			'.routeflux-settings-choice-title { display:block; color:var(--routeflux-text-primary); font-size:19px; font-weight:800; letter-spacing:-0.02em; }',
			'.routeflux-settings-choice-description { display:block; color:var(--routeflux-text-secondary); line-height:1.62; }',
			'.routeflux-settings-choice-note { display:inline-flex; align-items:center; width:max-content; max-width:100%; min-height:26px; padding:0 10px; border-radius:999px; background:rgba(88, 196, 255, 0.12); color:#a7dcff; font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
			'.routeflux-theme-light .routeflux-settings-choice { border-color:rgba(125, 146, 170, 0.18); background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%); box-shadow:0 12px 24px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.86); }',
			'.routeflux-theme-light .routeflux-settings-choice:hover { border-color:rgba(37, 99, 235, 0.24); box-shadow:0 14px 26px rgba(63, 87, 118, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.88); }',
			'.routeflux-theme-light .routeflux-settings-choice-selected { border-color:rgba(22, 163, 74, 0.28); background:linear-gradient(180deg, rgba(248, 252, 249, 0.99) 0%, rgba(238, 247, 241, 0.96) 100%); box-shadow:0 16px 28px rgba(21, 128, 61, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-theme-light .routeflux-settings-choice-indicator { border-color:rgba(125, 146, 170, 0.28); background:linear-gradient(180deg, rgba(250, 252, 254, 0.99) 0%, rgba(241, 245, 249, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 8px 16px rgba(63, 87, 118, 0.06); }',
			'.routeflux-theme-light .routeflux-settings-choice-note { background:rgba(37, 99, 235, 0.08); color:#1d4ed8; }',
			'@media (max-width: 700px) { .routeflux-settings-actions { flex-direction:column; } .routeflux-settings-actions .cbi-button { width:100%; } }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Settings') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Choose the RouteFlux theme used inside LuCI.')
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Appearance') ]),
			E('p', { 'class': 'cbi-section-descr' }, [
				_('Choose how RouteFlux pages should look inside LuCI.')
			]),
			E('div', { 'class': 'routeflux-settings-choice-grid' }, [
				E('label', { 'class': choiceClass(currentTheme === 'dark') }, [
					E('input', {
						'id': 'routeflux-settings-appearance-dark',
						'class': 'routeflux-settings-choice-control',
						'type': 'radio',
						'name': 'routeflux-settings-appearance',
						'value': 'dark',
						'checked': currentTheme === 'dark' ? 'checked' : null,
						'change': ui.createHandlerFn(this, 'handleAppearanceChange')
					}),
					E('span', { 'class': 'routeflux-settings-choice-indicator', 'aria-hidden': 'true' }, []),
					E('span', { 'class': 'routeflux-settings-choice-copy' }, [
						E('span', { 'class': 'routeflux-settings-choice-title' }, [ _('Dark') ]),
						E('span', { 'class': 'routeflux-settings-choice-description' }, [
							_('Use the current premium dark interface across all RouteFlux pages.')
						]),
						E('span', { 'class': 'routeflux-settings-choice-note' }, [
							currentTheme === 'dark' ? _('Selected') : _('Available')
						])
					])
				]),
				E('label', { 'class': choiceClass(currentTheme === 'light') }, [
					E('input', {
						'id': 'routeflux-settings-appearance-light',
						'class': 'routeflux-settings-choice-control',
						'type': 'radio',
						'name': 'routeflux-settings-appearance',
						'value': 'light',
						'checked': currentTheme === 'light' ? 'checked' : null,
						'change': ui.createHandlerFn(this, 'handleAppearanceChange')
					}),
					E('span', { 'class': 'routeflux-settings-choice-indicator', 'aria-hidden': 'true' }, []),
					E('span', { 'class': 'routeflux-settings-choice-copy' }, [
						E('span', { 'class': 'routeflux-settings-choice-title' }, [ _('Light') ]),
						E('span', { 'class': 'routeflux-settings-choice-description' }, [
							_('Use a brighter RouteFlux interface with the same structure and actions.')
						]),
						E('span', { 'class': 'routeflux-settings-choice-note' }, [
							currentTheme === 'light' ? _('Selected') : _('Available')
						])
					])
				])
			]),
			E('div', { 'class': 'routeflux-settings-actions', 'style': 'margin-top:16px;' }, [
				E('button', {
					'id': 'routeflux-settings-save',
					'type': 'button',
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleSaveSettings')
				}, [ _('Save') ])
			])
		]));

		return E('div', {
			'id': 'routeflux-settings-root',
			'class': routefluxUI.withThemeClass('routeflux-page-shell routeflux-page-shell-settings')
		}, content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
