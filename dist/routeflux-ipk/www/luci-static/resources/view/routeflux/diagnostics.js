'use strict';
'require view';
'require fs';
'require ui';

var routefluxBinary = '/usr/bin/routeflux';

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function firstNonEmpty(values, fallback) {
	for (var i = 0; i < values.length; i++) {
		var value = trim(values[i]);
		if (value !== '')
			return value;
	}

	return fallback || '';
}

function backendLabel(runtime, runtimeError) {
	if (trim(runtimeError) !== '')
		return _('Error');

	return runtime && runtime.running === true ? _('Running') : _('Stopped');
}

function fileState(file) {
	if (!file || file.exists !== true)
		return _('Missing');

	if (trim(file.error) !== '')
		return _('Broken');

	if (file.directory === true)
		return _('Directory');

	if (file.executable === true)
		return _('Executable');

	return _('Present');
}

function fileDetails(file) {
	if (!file)
		return '-';

	var parts = [];

	if (trim(file.mode) !== '')
		parts.push(_('Mode: %s').format(file.mode));
	if (trim(file.modified_at) !== '')
		parts.push(_('Modified: %s').format(file.modified_at));
	if (trim(file.symlink_target) !== '')
		parts.push(_('Symlink: %s').format(file.symlink_target));
	if (trim(file.error) !== '')
		parts.push(_('Error: %s').format(file.error));

	if (parts.length === 0)
		return '-';

	return parts.join(' | ');
}

return view.extend({
	load: function() {
		return this.execJSON([ '--json', 'diagnostics' ]).catch(function(err) {
			return { __error__: err.message || String(err) };
		});
	},

	execJSON: function(argv) {
		return fs.exec(routefluxBinary, argv).then(function(res) {
			var stderr = trim(res.stderr);
			var stdout = trim(res.stdout);

			if (res.code !== 0)
				throw new Error(stderr || stdout || _('RouteFlux command failed.'));

			if (stdout === '')
				throw new Error(_('RouteFlux returned empty JSON output.'));

			try {
				return JSON.parse(stdout);
			}
			catch (err) {
				throw new Error(_('RouteFlux returned invalid JSON output.'));
			}
		});
	},

	renderCard: function(label, value) {
		return E('div', { 'class': 'routeflux-card' }, [
			E('div', { 'class': 'routeflux-card-label' }, [ label ]),
			E('div', { 'class': 'routeflux-card-value' }, [ value || '-' ])
		]);
	},

	renderFileTable: function(files) {
		var rows = [
			[ _('RouteFlux Binary'), files.routeflux_binary ],
			[ _('RouteFlux Root'), files.routeflux_root ],
			[ _('Subscriptions File'), files.subscriptions_file ],
			[ _('Settings File'), files.settings_file ],
			[ _('State File'), files.state_file ],
			[ _('Xray Config'), files.xray_config ],
			[ _('Xray Service'), files.xray_service ],
			[ _('nft Binary'), files.nft_binary ],
			[ _('Firewall Rules'), files.firewall_rules ]
		].map(function(item) {
			var label = item[0];
			var file = item[1] || {};

			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td' }, [ label ]),
				E('td', { 'class': 'td' }, [ firstNonEmpty([ file.path ], '-') ]),
				E('td', { 'class': 'td' }, [ fileState(file) ]),
				E('td', { 'class': 'td' }, [ fileDetails(file) ])
			]);
		});

		return E('table', { 'class': 'table cbi-section-table' }, [
			E('tr', { 'class': 'tr cbi-section-table-titles' }, [
				E('th', { 'class': 'th' }, [ _('Item') ]),
				E('th', { 'class': 'th' }, [ _('Path') ]),
				E('th', { 'class': 'th' }, [ _('State') ]),
				E('th', { 'class': 'th' }, [ _('Details') ])
			])
		].concat(rows));
	},

	handleRefreshPage: function(ev) {
		window.location.reload();
	},

	render: function(data) {
		var diagnostics = data || {};
		var status = diagnostics.status || {};
		var state = status.state || {};
		var runtime = diagnostics.runtime || {};
		var files = diagnostics.files || {};
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var activeProfile = firstNonEmpty([
			activeSubscription.display_name,
			activeSubscription.provider_name
		], _('Not selected'));
		var activeNodeName = firstNonEmpty([
			activeNode.name,
			activeNode.remark
		], _('Not selected'));
		var content = [];

		if (diagnostics.__error__)
			ui.addNotification(null, notificationParagraph(_('Diagnostics error: %s').format(diagnostics.__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
			'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
			'.routeflux-diagnostics-actions { display:flex; flex-wrap:wrap; gap:10px; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Diagnostics') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Inspect current runtime state, backend status, recent failure details, and the critical files RouteFlux depends on.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), state.connected === true ? _('Connected') : _('Disconnected')),
			this.renderCard(_('Effective Mode'), firstNonEmpty([ state.mode ], _('disconnected'))),
			this.renderCard(_('Backend'), backendLabel(runtime, diagnostics.runtime_error)),
			this.renderCard(_('Service State'), firstNonEmpty([ runtime.service_state ], _('unknown'))),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName),
			this.renderCard(_('Last Success'), firstNonEmpty([ state.last_success_at ], _('Never'))),
			this.renderCard(_('Last Switch'), firstNonEmpty([ state.last_switch_at ], _('Never'))),
			this.renderCard(_('Backend Config'), firstNonEmpty([ runtime.config_path ], _('Not configured')))
		]));

		if (trim(diagnostics.runtime_error) !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('Backend status error: %s').format(diagnostics.runtime_error)
				])
			]));
		}

		if (trim(state.last_failure_reason) !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('Last failure: %s').format(state.last_failure_reason)
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Actions') ]),
			E('div', { 'class': 'routeflux-diagnostics-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, 'handleRefreshPage')
				}, [ _('Refresh') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('File Checks') ]),
			this.renderFileTable(files)
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
