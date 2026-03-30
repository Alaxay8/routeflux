'use strict';
'require view';
'require fs';
'require ui';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var whatsNewBaseRelease = 'v0.1.5';
var whatsNewEntries = [
	{
		kind: _('New'),
		title: _('Update RouteFlux from LuCI'),
		summary: _('The About page can now download and install the latest published RouteFlux release directly on the router.')
	},
	{
		kind: _('Fix'),
		title: _('Anti-target routing is more reliable'),
		summary: _('Anti-target mode now handles more UDP traffic correctly and works more predictably for browser-facing service presets.')
	},
	{
		kind: _('New'),
		title: _('Anti-target mode is now available'),
		summary: _('You can keep selected services and destinations direct while sending the rest of your LAN traffic through RouteFlux.')
	}
];

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function renderWhatsNewCard(entry) {
	return E('div', { 'class': 'routeflux-card routeflux-card-primary routeflux-about-update-card' }, [
		E('div', { 'class': 'routeflux-card-accent' }, []),
		E('div', { 'class': 'routeflux-card-label' }, [ entry.kind ]),
		E('div', { 'class': 'routeflux-card-value routeflux-about-update-title' }, [ entry.title ]),
		E('p', { 'class': 'routeflux-about-update-summary' }, [ entry.summary ])
	]);
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'version' ]).catch(function(err) {
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

	execText: function(argv) {
		return fs.exec(routefluxBinary, argv).then(function(res) {
			var stderr = trim(res.stderr);
			var stdout = trim(res.stdout);

			if (res.code !== 0)
				throw new Error(stderr || stdout || _('RouteFlux command failed.'));

			return {
				stdout: stdout,
				stderr: stderr
			};
		});
	},

	handleUpgrade: function(ev) {
		if (ev)
			ev.preventDefault();

		if (!window.confirm(_('Download the latest RouteFlux release and install it over the current router version? Existing /etc/routeflux state is preserved by the installer.')))
			return Promise.resolve();

		return this.execText([ '--upgrade' ]).then(function(res) {
			ui.addNotification(null, notificationParagraph(res.stdout || _('Upgrade completed. Reloading the page...')), 'info');
			window.setTimeout(function() {
				window.location.reload();
			}, 1500);
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	showWhatsNewModal: function() {
		var body = [
			E('p', { 'class': 'routeflux-modal-help' }, [
				_('Changes included after the %s release, rewritten as practical user-facing updates.').format(whatsNewBaseRelease)
			]),
			E('div', { 'class': 'routeflux-overview-grid routeflux-about-update-grid' }, whatsNewEntries.map(renderWhatsNewCard))
		];
		var actions = [
			E('button', {
				'class': 'cbi-button',
				'type': 'button',
				'click': function(ev) {
					ui.hideModal();
					return false;
				}
			}, [ _('Close') ])
		];

		routefluxUI.showModal(_('What\'s New'), body, {
			'bodyClass': 'routeflux-modal-whats-new',
			'modalClass': 'routeflux-modal-whats-new',
			'actions': actions
		});
	},

	handleShowWhatsNew: function(ev) {
		if (ev)
			ev.preventDefault();

		this.showWhatsNewModal();
		return false;
	},

	render: function(data) {
		var info = data[0] || {};
		var content = [];
		var version = trim(info.version) || 'dev';
		var commit = trim(info.commit) || 'unknown';
		var buildDate = trim(info.build_date) || 'unknown';
		var versionText = 'RouteFlux ' + version + '\nCommit: ' + commit + '\nBuilt: ' + buildDate;

		if (info.__error__)
			ui.addNotification(null, notificationParagraph(_('Version error: %s').format(info.__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-about-pre { white-space:pre-wrap; margin:0; }',
			'.routeflux-about-update-grid { align-items:stretch; }',
			'.routeflux-about-update-card { min-height:168px; }',
			'.routeflux-about-update-title { margin-bottom:10px; }',
			'.routeflux-about-update-summary { margin:0; color:var(--text-color-secondary, #526175); line-height:1.6; }',
			'.routeflux-modal-help { margin:0 0 12px; color:var(--text-color-medium, #586677); max-width:100%; overflow-wrap:anywhere; word-break:break-word; line-height:1.45; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - About') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('RouteFlux build information, update actions, and recent user-facing changes.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			routefluxUI.renderSummaryCard(_('Version'), version),
			routefluxUI.renderSummaryCard(_('Commit'), commit),
			routefluxUI.renderSummaryCard(_('Build Date'), buildDate)
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('routeflux version') ]),
			E('pre', { 'class': 'routeflux-about-pre' }, [ versionText ])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Update') ]),
			E('p', { 'class': 'cbi-section-descr' }, [
				_('Download and install the latest published RouteFlux release on this router. The installer preserves existing /etc/routeflux state files.')
			]),
			E('div', { 'class': 'cbi-page-actions' }, [
				E('button', {
					'class': 'btn cbi-button',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleShowWhatsNew')
				}, [ _('What\'s New') ]),
				E('button', {
					'class': 'btn cbi-button cbi-button-action important',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleUpgrade')
				}, [ _('Update to latest version') ])
			])
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
