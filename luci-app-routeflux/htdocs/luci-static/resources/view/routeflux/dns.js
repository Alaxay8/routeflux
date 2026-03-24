'use strict';
'require view';
'require fs';
'require ui';

var routefluxBinary = '/usr/bin/routeflux';
var defaultDNSProfile = {
	mode: 'split',
	transport: 'doh',
	servers: [ '1.1.1.1', '1.0.0.1' ],
	bootstrap: [],
	direct_domains: [ 'domain:lan', 'full:router.lan' ]
};

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

function isPlaceholderNodeLabel(value) {
	return trim(value).toLowerCase() === 'proxy';
}

var regionNameFallbacks = {
	'AT': 'Austria',
	'AU': 'Australia',
	'BE': 'Belgium',
	'BG': 'Bulgaria',
	'BR': 'Brazil',
	'CA': 'Canada',
	'CH': 'Switzerland',
	'CZ': 'Czechia',
	'DE': 'Germany',
	'EE': 'Estonia',
	'ES': 'Spain',
	'FI': 'Finland',
	'FR': 'France',
	'GB': 'United Kingdom',
	'HK': 'Hong Kong',
	'HU': 'Hungary',
	'IE': 'Ireland',
	'IN': 'India',
	'IT': 'Italy',
	'JP': 'Japan',
	'KR': 'South Korea',
	'KZ': 'Kazakhstan',
	'LT': 'Lithuania',
	'LV': 'Latvia',
	'MD': 'Moldova',
	'NL': 'Netherlands',
	'NO': 'Norway',
	'PL': 'Poland',
	'PT': 'Portugal',
	'RO': 'Romania',
	'RS': 'Serbia',
	'RU': 'Russia',
	'SE': 'Sweden',
	'SG': 'Singapore',
	'SK': 'Slovakia',
	'TR': 'Turkey',
	'UA': 'Ukraine',
	'US': 'United States'
};

function normalizeRegionCode(value) {
	var code = trim(value).toUpperCase();

	if (code === 'UK')
		return 'GB';

	return code;
}

function regionName(code) {
	var normalized = normalizeRegionCode(code);

	if (normalized === '')
		return '';

	try {
		if (typeof Intl !== 'undefined' && typeof Intl.DisplayNames === 'function') {
			var displayNames = new Intl.DisplayNames([ navigator.language || 'en' ], { 'type': 'region' });
			var localized = displayNames.of(normalized);

			if (localized && localized !== normalized)
				return localized;
		}
	}
	catch (err) {
	}

	return regionNameFallbacks[normalized] || '';
}

function inferRegionCodeFromText(value) {
	var tokens = trim(value).toLowerCase().split(/[^a-z]+/).filter(Boolean);

	for (var i = 0; i < tokens.length; i++) {
		if (!/^[a-z]{2}$/.test(tokens[i]))
			continue;

		if (regionName(tokens[i]) !== '')
			return normalizeRegionCode(tokens[i]);
	}

	return '';
}

function inferRegionCodeFromAddress(value) {
	var host = trim(value).toLowerCase();

	if (host === '')
		return '';

	var labels = host.split('.').filter(Boolean);

	if (labels.length === 0)
		return '';

	var firstTokens = labels[0].split(/[^a-z0-9]+/).filter(Boolean);
	for (var i = 0; i < firstTokens.length; i++) {
		if (!/^[a-z]{2}$/.test(firstTokens[i]))
			continue;

		if (regionName(firstTokens[i]) !== '')
			return normalizeRegionCode(firstTokens[i]);
	}

	var tld = labels[labels.length - 1];
	if (/^[a-z]{2}$/.test(tld) && regionName(tld) !== '')
		return normalizeRegionCode(tld);

	return '';
}

function nodeDisplayName(node, fallback) {
	var code = firstNonEmpty([
		inferRegionCodeFromText(node && node.name),
		inferRegionCodeFromText(node && node.remark),
		inferRegionCodeFromAddress(node && node.address)
	], '');

	if (code !== '') {
		var localizedRegion = regionName(code);
		if (localizedRegion !== '')
			return localizedRegion;
	}

	var name = trim(node && node.name);
	var remark = trim(node && node.remark);

	if (name !== '' && !isPlaceholderNodeLabel(name))
		return name;

	if (remark !== '' && !isPlaceholderNodeLabel(remark))
		return remark;

	return firstNonEmpty([
		node && node.address,
		node && node.id
	], fallback || '');
}

function parseList(raw) {
	var value = trim(raw);

	if (value === '')
		return [];

	var parts = value.split(/[\s,]+/);
	var out = [];

	for (var i = 0; i < parts.length; i++) {
		var part = trim(parts[i]);
		if (part !== '')
			out.push(part);
	}

	return out;
}

function listString(values) {
	if (!Array.isArray(values) || values.length === 0)
		return '';

	return values.join('\n');
}

function listCSV(values) {
	if (!Array.isArray(values) || values.length === 0)
		return '';

	return values.join(',');
}

function listEquals(left, right) {
	var a = Array.isArray(left) ? left : [];
	var b = Array.isArray(right) ? right : [];

	if (a.length !== b.length)
		return false;

	for (var i = 0; i < a.length; i++) {
		if (a[i] !== b[i])
			return false;
	}

	return true;
}

function isDefaultProfile(dns) {
	if (!dns)
		return false;

	return trim(dns.mode) === defaultDNSProfile.mode &&
		trim(dns.transport) === defaultDNSProfile.transport &&
		listEquals(dns.servers, defaultDNSProfile.servers) &&
		listEquals(dns.bootstrap, defaultDNSProfile.bootstrap) &&
		listEquals(dns.direct_domains, defaultDNSProfile.direct_domains);
}

function modeSummary(mode) {
	switch (trim(mode)) {
	case 'remote':
		return _('Remote');
	case 'split':
		return _('Split');
	case 'disabled':
		return _('Disabled');
	default:
		return _('System');
	}
}

function transportSummary(transport) {
	switch (trim(transport)) {
	case 'doh':
		return _('DoH');
	case 'dot':
		return _('DoT');
	default:
		return _('Plain');
	}
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'dns', 'get' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execText([ 'dns', 'explain' ]).catch(function(err) {
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

			return stdout;
		});
	},

	runCommands: function(commands, successMessage) {
		var self = this;
		var outputs = [];
		var chain = Promise.resolve();

		for (var i = 0; i < commands.length; i++) {
			(function(argv) {
				chain = chain.then(function() {
					return self.execText(argv).then(function(stdout) {
						outputs.push(stdout);
					});
				});
			})(commands[i]);
		}

		return chain.then(function() {
			var lastOutput = '';

			for (var i = outputs.length - 1; i >= 0; i--) {
				if (trim(outputs[i]) !== '') {
					lastOutput = outputs[i];
					break;
				}
			}

			ui.addNotification(null, notificationParagraph(lastOutput || successMessage), 'info');
			window.setTimeout(function() {
				window.location.reload();
			}, 350);
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	renderCard: function(label, value) {
		return E('div', { 'class': 'routeflux-card' }, [
			E('div', { 'class': 'routeflux-card-label' }, [ label ]),
			E('div', { 'class': 'routeflux-card-value' }, [ value || '-' ])
		]);
	},

	handleSaveSettings: function(ev) {
		var current = this.currentDNS || {};
		var mode = trim(document.querySelector('#routeflux-dns-mode').value);
		var transport = trim(document.querySelector('#routeflux-dns-transport').value);
		var servers = parseList(document.querySelector('#routeflux-dns-servers').value);
		var bootstrap = parseList(document.querySelector('#routeflux-dns-bootstrap').value);
		var directDomains = parseList(document.querySelector('#routeflux-dns-direct-domains').value);
		var commands = [];

		if (trim(current.mode) !== mode)
			commands.push([ 'dns', 'set', 'mode', mode ]);

		if (trim(current.transport) !== transport)
			commands.push([ 'dns', 'set', 'transport', transport ]);

		if (!listEquals(current.servers, servers))
			commands.push([ 'dns', 'set', 'servers', listCSV(servers) ]);

		if (!listEquals(current.bootstrap, bootstrap))
			commands.push([ 'dns', 'set', 'bootstrap', listCSV(bootstrap) ]);

		if (!listEquals(current.direct_domains, directDomains))
			commands.push([ 'dns', 'set', 'direct-domains', listCSV(directDomains) ]);

		if (commands.length === 0) {
			ui.addNotification(null, notificationParagraph(_('No DNS changes to save.')), 'info');
			return Promise.resolve();
		}

		return this.runCommands(commands, _('DNS settings saved.'));
	},

	handleRestoreDefault: function(ev) {
		return this.runCommands([
			[ 'dns', 'default' ]
		], _('RouteFlux default DNS profile applied.'));
	},

	render: function(data) {
		var status = data[0] || {};
		var dns = data[1] && !data[1].__error__
			? data[1]
			: ((status.settings || {}).dns || {});
		var explainText = data[2] && data[2].__error__ ? '' : trim(data[2]);
		var connected = !!(status.state && status.state.connected === true);
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var profile = isDefaultProfile(dns) ? _('RouteFlux Default') : _('Custom');
		var activeProfile = firstNonEmpty([
			activeSubscription.display_name,
			activeSubscription.provider_name
		], _('Not selected'));
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));
		var content = [];

		this.currentDNS = {
			mode: trim(dns.mode),
			transport: trim(dns.transport),
			servers: Array.isArray(dns.servers) ? dns.servers.slice() : [],
			bootstrap: Array.isArray(dns.bootstrap) ? dns.bootstrap.slice() : [],
			direct_domains: Array.isArray(dns.direct_domains) ? dns.direct_domains.slice() : []
		};

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('DNS error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__)
			ui.addNotification(null, notificationParagraph(_('DNS help error: %s').format(data[2].__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
			'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
			'.routeflux-dns-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; margin-bottom:12px; }',
			'.routeflux-dns-grid textarea { min-height:100px; width:100%; }',
			'.routeflux-dns-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-dns-help { white-space:pre-wrap; margin:0; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - DNS') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage DNS mode, transport, upstream servers, bootstrap resolvers, and split-DNS local domains from LuCI.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected')),
			this.renderCard(_('Mode'), modeSummary(dns.mode)),
			this.renderCard(_('Transport'), transportSummary(dns.transport)),
			this.renderCard(_('Profile'), profile),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]));

		if (trim(dns.transport) === 'dot') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('DoT is saved, but the current RouteFlux Xray backend does not apply DNS over TLS yet.')
				])
			]));
		}

		if (connected) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message' }, [
					_('Saving DNS settings while connected reapplies the current runtime configuration.')
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Configuration') ]),
			E('div', { 'class': 'routeflux-dns-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-dns-mode' }, [ _('Mode') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', { 'id': 'routeflux-dns-mode' }, [
							E('option', { 'value': 'system', 'selected': trim(dns.mode) === 'system' ? 'selected' : null }, [ _('System') ]),
							E('option', { 'value': 'remote', 'selected': trim(dns.mode) === 'remote' ? 'selected' : null }, [ _('Remote') ]),
							E('option', { 'value': 'split', 'selected': trim(dns.mode) === 'split' ? 'selected' : null }, [ _('Split') ]),
							E('option', { 'value': 'disabled', 'selected': trim(dns.mode) === 'disabled' ? 'selected' : null }, [ _('Disabled') ])
						])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('System leaves DNS alone, remote sends everything upstream, split keeps local names local, and disabled skips RouteFlux DNS config.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-dns-transport' }, [ _('Transport') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', { 'id': 'routeflux-dns-transport' }, [
							E('option', { 'value': 'plain', 'selected': trim(dns.transport) === 'plain' ? 'selected' : null }, [ _('Plain') ]),
							E('option', { 'value': 'doh', 'selected': trim(dns.transport) === 'doh' ? 'selected' : null }, [ _('DoH') ]),
							E('option', { 'value': 'dot', 'selected': trim(dns.transport) === 'dot' ? 'selected' : null }, [ _('DoT') ])
						])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('DoH is the currently working encrypted DNS transport in the RouteFlux backend.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-dns-servers' }, [ _('Servers') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-dns-servers',
							'class': 'cbi-input-textarea',
							'placeholder': _('Examples: 1.1.1.1 1.0.0.1 dns.google')
						}, [ listString(dns.servers) ])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Main upstream DNS servers. Separate values with spaces, commas, or new lines.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-dns-bootstrap' }, [ _('Bootstrap') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-dns-bootstrap',
							'class': 'cbi-input-textarea',
							'placeholder': _('Optional fallback resolvers such as 9.9.9.9')
						}, [ listString(dns.bootstrap) ])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Used when an upstream DNS server is written as a hostname instead of a raw IP address.')
					])
				]),
				E('div', {
					'class': 'cbi-value',
					'style': 'grid-column:1 / -1'
				}, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-dns-direct-domains' }, [ _('Direct Domains') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-dns-direct-domains',
							'class': 'cbi-input-textarea',
							'placeholder': _('Examples: domain:lan full:router.lan')
						}, [ listString(dns.direct_domains) ])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Domains that stay on local DNS in split mode. RouteFlux default uses domain:lan and full:router.lan.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-dns-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleSaveSettings')
				}, [ _('Save') ]),
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, 'handleRestoreDefault')
				}, [ _('Restore Default') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('details', { 'open': 'open' }, [
				E('summary', {}, [ _('Help') ]),
				E('pre', { 'class': 'routeflux-dns-help' }, [
					explainText || _('No DNS help text is available.')
				])
			])
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
