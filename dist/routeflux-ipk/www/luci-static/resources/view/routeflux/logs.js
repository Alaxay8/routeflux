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

function joinLines(lines, emptyText) {
	if (!Array.isArray(lines) || lines.length === 0)
		return emptyText;

	return lines.join('\n');
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'logs' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			})
		]);
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

	handleRefreshPage: function(ev) {
		window.location.reload();
	},

	renderLogSection: function(title, lines, emptyText) {
		return E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ title ]),
			E('pre', { 'class': 'routeflux-logs-pre' }, [
				joinLines(lines, emptyText)
			])
		]);
	},

	render: function(data) {
		var status = data[0] || {};
		var logs = data[1] || {};
		var state = status.state || {};
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

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Logs error: %s').format(data[1].__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
			'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
			'.routeflux-logs-actions { display:flex; flex-wrap:wrap; gap:10px; margin-bottom:16px; }',
			'.routeflux-logs-pre { white-space:pre-wrap; margin:0; max-height:420px; overflow:auto; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Logs') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Inspect recent RouteFlux-related logs, Xray runtime logs, and the tail of the system log without leaving LuCI.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), state.connected === true ? _('Connected') : _('Disconnected')),
			this.renderCard(_('Mode'), firstNonEmpty([ state.mode ], _('disconnected'))),
			this.renderCard(_('Log Source'), firstNonEmpty([ logs.source ], _('/sbin/logread'))),
			this.renderCard(_('logread'), logs.available === true ? _('Available') : _('Unavailable')),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]));

		if (trim(logs.error) !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('Log source error: %s').format(logs.error)
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Actions') ]),
			E('div', { 'class': 'routeflux-logs-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, 'handleRefreshPage')
				}, [ _('Refresh') ])
			])
		]));

		content.push(this.renderLogSection(
			_('RouteFlux'),
			logs.routeflux,
			_('No recent RouteFlux log lines matched in logread.')
		));
		content.push(this.renderLogSection(
			_('Xray'),
			logs.xray,
			_('No recent Xray log lines matched in logread.')
		));
		content.push(this.renderLogSection(
			_('System Tail'),
			logs.system,
			_('No recent system log lines are available.')
		));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
