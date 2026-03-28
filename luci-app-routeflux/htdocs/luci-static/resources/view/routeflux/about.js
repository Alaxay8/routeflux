'use strict';
'require view';
'require fs';
'require ui';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
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
			'.routeflux-about-pre { white-space:pre-wrap; margin:0; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - About') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('RouteFlux build information from the installed router binary.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			routefluxUI.renderSummaryCard(_('Version'), version, {
				'tone': 'connected',
				'primary': true
			}),
			routefluxUI.renderSummaryCard(_('Commit'), commit),
			routefluxUI.renderSummaryCard(_('Build Date'), buildDate)
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('routeflux version') ]),
			E('pre', { 'class': 'routeflux-about-pre' }, [ versionText ])
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
