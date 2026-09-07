'use strict';
'require view';
'require fs';
'require ui';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var routefluxSelfUpdateHelper = '/usr/libexec/routeflux-self-update';
var whatsNewEntries = [
	{
		kind: _('Fix'),
		title: _('Fast Node Switching & Timeout Prevention'),
		summary: _('Optimized node switching by eliminating redundant firewall and dnsmasq reloads, and increased LuCI RPC timeout to 60s')
	},
	{
		kind: _('New'),
		title: _('Local & LAN Proxy Mode'),
		summary: _('Added dedicated SOCKS5 and HTTP proxy endpoints with configurable LAN access in Settings and CLI')
	},
	{
		kind: _('Docs'),
		title: _('Documentation & Architecture Flow'),
		summary: _('Updated guides and added high-resolution system architecture flowcharts in English and Russian')
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

function extractSelfUpdateStatus(output) {
	var match = String(output || '').match(/ROUTEFLUX_SELF_UPDATE_STATUS=([^\n]+)/);
	return match ? trim(match[1]) : '';
}

function stripSelfUpdateStatus(output) {
	return trim(String(output || '').replace(/ROUTEFLUX_SELF_UPDATE_STATUS=[^\n]*\n?/, ''));
}

function padNumber(value) {
	return String(value).padStart(2, '0');
}

function formatBuildDate(value) {
	var raw = trim(value);
	var parsed;

	if (raw === '')
		return 'unknown';

	parsed = new Date(raw);
	if (isNaN(parsed.getTime()))
		return raw;

	return String(parsed.getFullYear()) + '-' +
		padNumber(parsed.getMonth() + 1) + '-' +
		padNumber(parsed.getDate()) + ' ' +
		padNumber(parsed.getHours()) + ':' +
		padNumber(parsed.getMinutes()) + ':' +
		padNumber(parsed.getSeconds());
}

function renderWhatsNewCard(entry) {
	var className = 'routeflux-card routeflux-card-primary routeflux-about-update-card';
	if (entry.kind === _('New') || entry.kind === 'New')
		className += ' routeflux-about-update-card-new';
	else if (entry.kind === _('Fix') || entry.kind === 'Fix')
		className += ' routeflux-about-update-card-fix';
	else if (entry.kind === _('Docs') || entry.kind === 'Docs')
		className += ' routeflux-about-update-card-docs';

	return E('div', { 'class': className }, [
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

	execHelper: function(command, argv) {
		return fs.exec(command, argv || []).then(function(res) {
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

		return this.execHelper(routefluxSelfUpdateHelper).then(function(res) {
			var status = extractSelfUpdateStatus(res.stdout);
			var message = stripSelfUpdateStatus(res.stdout);

			ui.addNotification(null, notificationParagraph(message || _('Upgrade completed. Reloading the page...')), 'info');
			if (status !== 'up-to-date') {
				window.setTimeout(function() {
					window.location.reload();
				}, 1500);
			}
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	showWhatsNewModal: function() {
		var body = [
			E('p', { 'class': 'routeflux-modal-help' }, [
				_('Recent user-facing changes in the simplified LuCI experience.')
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

	handleRestart: function(ev) {
		if (ev)
			ev.preventDefault();

		if (!window.confirm(_('Restart the RouteFlux service and clear all LuCI caches? This can help resolve temporary connection or display issues.')))
			return Promise.resolve();

		ui.showIndicator();

		return fs.exec(routefluxBinary, [ 'restart' ]).then(L.bind(function(res) {
			ui.hideIndicator();
			ui.addNotification(null, notificationParagraph(_('RouteFlux service restarted and LuCI cache cleared successfully. Reloading...')), 'info');
			window.setTimeout(function() {
				window.location.reload();
			}, 2000);
		}, this)).catch(L.bind(function(err) {
			ui.hideIndicator();
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		}, this));
	},

	render: function(data) {
		var info = data[0] || {};
		var content = [];
		var version = trim(info.version) || 'dev';
		var commit = trim(info.commit) || 'unknown';
		var formattedBuildDate = formatBuildDate(info.build_date);
		var versionText = 'RouteFlux ' + version + '\nCommit: ' + commit + '\nBuilt: ' + formattedBuildDate;

		if (info.__error__)
			ui.addNotification(null, notificationParagraph(_('Version error: %s').format(info.__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-about-pre { white-space:pre-wrap; margin:0; }',
			'.routeflux-about-update-grid { display:grid !important; grid-template-columns:repeat(2, 1fr) !important; align-items:stretch; }',
			'@media (max-width: 560px) { .routeflux-about-update-grid { grid-template-columns:1fr !important; } }',
			'.routeflux-about-update-card { min-height:168px; }',
			'.routeflux-about-update-card-new .routeflux-card-accent { background:linear-gradient(90deg, #22c55e 0%, #16a34a 100%); }',
			'.routeflux-about-update-card-fix .routeflux-card-accent { background:linear-gradient(90deg, #f59e0b 0%, #d97706 100%); }',
			'.routeflux-about-update-card-docs .routeflux-card-accent { background:linear-gradient(90deg, #38bdf8 0%, #0284c7 100%); }',
			'.routeflux-theme-light .routeflux-about-update-card-new { border-color:rgba(34, 197, 94, 0.2); background:linear-gradient(180deg, rgba(250, 252, 250, 0.99) 0%, rgba(240, 253, 244, 0.99) 100%); }',
			'.routeflux-theme-light .routeflux-about-update-card-new .routeflux-card-label { color:#15803d; }',
			'.routeflux-theme-light .routeflux-about-update-card-new .routeflux-about-update-title { color:#14532d; }',
			'.routeflux-theme-light .routeflux-about-update-card-fix { border-color:rgba(245, 158, 11, 0.2); background:linear-gradient(180deg, rgba(253, 250, 245, 0.99) 0%, rgba(254, 243, 199, 0.38) 100%); }',
			'.routeflux-theme-light .routeflux-about-update-card-fix .routeflux-card-label { color:#b45309; }',
			'.routeflux-theme-light .routeflux-about-update-card-fix .routeflux-about-update-title { color:#78350f; }',
			'.routeflux-theme-light .routeflux-about-update-card-docs { border-color:rgba(56, 189, 248, 0.2); background:linear-gradient(180deg, rgba(250, 252, 254, 0.99) 0%, rgba(240, 249, 255, 0.99) 100%); }',
			'.routeflux-theme-light .routeflux-about-update-card-docs .routeflux-card-label { color:#0284c7; }',
			'.routeflux-theme-light .routeflux-about-update-card-docs .routeflux-about-update-title { color:#0369a1; }',
			'.routeflux-about-update-title { margin-bottom:10px; }',
			'.routeflux-about-update-summary { margin:0; color:var(--routeflux-text-secondary); line-height:1.6; }',
			'.routeflux-modal-help { margin:0 0 12px; color:var(--routeflux-text-secondary); max-width:100%; overflow-wrap:anywhere; word-break:break-word; line-height:1.45; }',
			'.routeflux-modal-whats-new.modal { border-radius:24px !important; padding:24px 28px !important; max-width:800px !important; width:92% !important; border:1px solid var(--routeflux-border) !important; box-shadow:var(--routeflux-shadow) !important; transition:background-color .22s ease, border-color .22s ease, color .22s ease; }',
			'.routeflux-theme-dark.routeflux-modal-whats-new.modal { background:linear-gradient(180deg, rgba(20, 31, 49, 0.98) 0%, rgba(13, 21, 35, 1) 100%) !important; border-color:var(--routeflux-border-strong) !important; color:var(--routeflux-text-primary) !important; }',
			'.routeflux-theme-light.routeflux-modal-whats-new.modal { background:linear-gradient(180deg, rgba(252, 253, 254, 0.99) 0%, rgba(246, 249, 252, 1) 100%) !important; border-color:rgba(37, 99, 235, 0.2) !important; color:var(--routeflux-text-primary) !important; }',
			'.routeflux-modal-whats-new h4 { margin:0 0 16px !important; font-size:clamp(20px, 1.2vw + 15px, 26px) !important; font-weight:800 !important; letter-spacing:-0.03em !important; line-height:1.2 !important; color:var(--routeflux-text-primary) !important; }',
			'.routeflux-modal-whats-new .routeflux-modal-actions { display:flex !important; justify-content:flex-end !important; gap:10px !important; margin-top:20px !important; padding-top:16px !important; border-top:1px solid var(--routeflux-border) !important; }',
			'.routeflux-modal-whats-new .routeflux-modal-actions .cbi-button { min-height:42px !important; padding:0 22px !important; border-radius:14px !important; font-size:14px !important; font-weight:700 !important; cursor:pointer !important; display:inline-flex !important; align-items:center !important; justify-content:center !important; border:1px solid rgba(145, 175, 220, 0.18) !important; background:rgba(15, 24, 38, 0.82) !important; color:var(--routeflux-text-primary) !important; box-shadow:0 12px 24px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03) !important; transition:transform .16s ease, border-color .16s ease, box-shadow .16s ease, background .16s ease !important; }',
			'.routeflux-modal-whats-new .routeflux-modal-actions .cbi-button:hover { transform:translateY(-1px) !important; border-color:rgba(145, 190, 246, 0.28) !important; box-shadow:0 16px 26px rgba(0, 0, 0, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.04) !important; }',
			'.routeflux-theme-light.routeflux-modal-whats-new .routeflux-modal-actions .cbi-button { border:1px solid rgba(125, 146, 170, 0.16) !important; background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(241, 246, 251, 0.98) 100%) !important; color:var(--routeflux-text-primary) !important; box-shadow:0 10px 20px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.88) !important; }',
			'.routeflux-theme-light.routeflux-modal-whats-new .routeflux-modal-actions .cbi-button:hover { border-color:rgba(37, 99, 235, 0.22) !important; box-shadow:0 12px 22px rgba(63, 87, 118, 0.1) !important; inset 0 1px 0 rgba(255, 255, 255, 0.9) !important; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - About') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('RouteFlux build information, update actions, and recent user-facing changes.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			routefluxUI.renderSummaryCard(_('Version'), version),
			routefluxUI.renderSummaryCard(_('Commit'), commit),
			routefluxUI.renderSummaryCard(_('Build Date'), formattedBuildDate)
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
					'class': 'cbi-button cbi-button-action',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleShowWhatsNew')
				}, [ _('What\'s New') ]),
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleUpgrade')
				}, [ _('Update to new version') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Maintenance') ]),
			E('p', { 'class': 'cbi-section-descr' }, [
				_('Restart the RouteFlux service and clear LuCI caches. This is useful for troubleshooting or resolving display glitches.')
			]),
			E('div', { 'class': 'cbi-page-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRestart')
				}, [ _('Restart RouteFlux') ])
			]),
			E('p', { 'class': 'cbi-section-descr', 'style': 'margin-top:20px;' }, [
				_('About intentionally keeps destructive maintenance actions out of LuCI. For full removal over SSH, use the documented uninstall.sh command from README.')
			])
		]));

		return E('div', {
			'class': routefluxUI.withThemeClass('routeflux-page-shell routeflux-page-shell-about')
		}, content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
