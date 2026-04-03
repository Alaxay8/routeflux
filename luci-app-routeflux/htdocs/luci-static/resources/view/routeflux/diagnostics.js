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

function isDomainLike(value) {
	var host = trim(value);

	if (host === '' || host.indexOf('://') >= 0 || host.indexOf(' ') >= 0)
		return false;

	return host.indexOf('.') >= 0;
}

function titleWords(value) {
	var parts = trim(value).toLowerCase().split(/\s+/).filter(Boolean);

	for (var i = 0; i < parts.length; i++)
		parts[i] = parts[i].charAt(0).toUpperCase() + parts[i].slice(1);

	return parts.join(' ');
}

function providerDomainStem(value) {
	var label = trim(value).toLowerCase().replace(/:\d+$/, '');
	var prefixes = [ 'conn', 'vpn', 'www', 'sub', 'api' ];
	var parts;

	if (label === '')
		return '';

	parts = label.split('.').filter(Boolean);
	if (parts.length >= 2)
		label = parts[parts.length - 2];
	else
		label = parts[0] || label;

	for (var i = 0; i < prefixes.length; i++) {
		if (label.indexOf(prefixes[i]) === 0 && label.length > prefixes[i].length + 2) {
			label = label.slice(prefixes[i].length);
			break;
		}
	}

	return trim(label);
}

function humanizeProviderName(value) {
	var label = trim(value);

	if (label === '')
		return _('Imported VPN');

	if (!isDomainLike(label))
		return label;

	label = providerDomainStem(label);
	label = titleWords(label.replace(/[-_]+/g, ' '));
	if (label.toLowerCase().indexOf('vpn') < 0)
		label += ' VPN';

	return trim(label);
}

function providerTitle(sub) {
	return humanizeProviderName(firstNonEmpty([
		sub && sub.provider_name,
		sub && sub.display_name,
		sub && sub.id
	], _('Imported VPN')));
}

function buildSubscriptionPresentation(subscriptions) {
	var groupsByKey = {};
	var byId = {};

	for (var i = 0; i < subscriptions.length; i++) {
		var sub = subscriptions[i];
		var title = providerTitle(sub);
		var key = title.toLowerCase();
		var group = groupsByKey[key];

		if (!group) {
			group = {
				title: title,
				count: 0
			};
			groupsByKey[key] = group;
		}

		group.count += 1;
		byId[trim(sub.id)] = {
			provider_title: title,
			profile_label: _('Profile %d').format(group.count)
		};
	}

	return byId;
}

function presentationForSubscription(sub, presentation) {
	var id = trim(sub && sub.id);

	if (id === '' || !presentation)
		return null;

	return presentation[id] || null;
}

function nodeDisplayName(node, fallback) {
	var name = trim(node && node.name);
	var remark = trim(node && node.remark);
	var explicit = '';

	if (name !== '' && !isPlaceholderNodeLabel(name))
		explicit = name;
	else if (remark !== '' && !isPlaceholderNodeLabel(remark))
		explicit = remark;

	if (explicit !== '' && !isDomainLike(explicit))
		return explicit;

	var code = firstNonEmpty([
		inferRegionCodeFromText(explicit),
		inferRegionCodeFromAddress(explicit),
		inferRegionCodeFromAddress(node && node.address)
	], '');

	if (code !== '') {
		var localizedRegion = regionName(code);
		if (localizedRegion !== '')
			return localizedRegion;
	}

	return firstNonEmpty([
		explicit,
		node && node.address,
		node && node.id
	], fallback || '');
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
		parts.push(_('Modified: %s').format(routefluxUI.formatTimestamp(file.modified_at)));
	if (trim(file.symlink_target) !== '')
		parts.push(_('Symlink: %s').format(file.symlink_target));
	if (trim(file.error) !== '')
		parts.push(_('Error: %s').format(file.error));

	if (parts.length === 0)
		return '-';

	return parts.join(' | ');
}

function ipv6StateLabel(ipv6) {
	if (ipv6 && ipv6.runtime_disabled === true)
		return _('Disabled');

	if (ipv6 && ipv6.available === true)
		return _('Enabled');

	return _('Unknown');
}

function ipv6EnabledInterfacesLabel(ipv6) {
	var values = Array.isArray(ipv6 && ipv6.enabled_interfaces) ? ipv6.enabled_interfaces.filter(function(entry) {
		return trim(entry) !== '';
	}) : [];

	return values.length > 0 ? values.join(', ') : '-';
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'diagnostics' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'list', 'subscriptions' ]).catch(function(err) {
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

	runCommand: function(argv, successMessage) {
		return this.execText(argv).then(function(stdout) {
			ui.addNotification(null, notificationParagraph(stdout || successMessage), 'info');
			window.setTimeout(function() {
				window.location.reload();
			}, 350);
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	renderCard: function(label, value, options) {
		return routefluxUI.renderSummaryCard(label, value, options);
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
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		window.location.reload();
	},

	handleDisableIPv6: function(ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runCommand([ 'firewall', 'set', 'ipv6', 'disable' ], _('IPv6 disabled.'));
	},

	render: function(data) {
		var diagnostics = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var status = diagnostics.status || {};
		var state = status.state || {};
		var runtime = diagnostics.runtime || {};
		var ipv6 = diagnostics.ipv6 || {};
		var files = diagnostics.files || {};
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var activeEntry = presentationForSubscription(activeSubscription, presentation);
		var activeProvider = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.provider_title : providerTitle(activeSubscription))
			: _('Not selected');
		var activeProfile = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.profile_label : _('Profile 1'))
			: _('Not selected');
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));
		var content = [];

		if (diagnostics.__error__)
			ui.addNotification(null, notificationParagraph(_('Diagnostics error: %s').format(diagnostics.__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-diagnostics-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-diagnostics-warning-actions { display:flex; flex-wrap:wrap; gap:10px; margin-top:12px; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Diagnostics') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Inspect current runtime state, backend status, recent failure details, and the critical files RouteFlux depends on.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), state.connected === true ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(state.connected === true),
				'primary': true
			}),
			this.renderCard(_('Effective Mode'), firstNonEmpty([ state.mode ], _('disconnected'))),
			this.renderCard(_('Backend'), backendLabel(runtime, diagnostics.runtime_error)),
			this.renderCard(_('Service State'), firstNonEmpty([ runtime.service_state ], _('unknown'))),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName),
			this.renderCard(_('IPv6 Runtime'), ipv6StateLabel(ipv6), {
				'tone': ipv6.runtime_disabled === true ? 'connected' : 'disconnected'
			}),
			this.renderCard(_('IPv6 Fail-State'), ipv6.fail_state === true ? _('Detected') : _('Clear'), {
				'tone': ipv6.fail_state === true ? 'disconnected' : 'connected'
			}),
			this.renderCard(_('Last Success'), routefluxUI.formatTimestamp(state.last_success_at) || _('Never')),
			this.renderCard(_('Last Switch'), routefluxUI.formatTimestamp(state.last_switch_at) || _('Never')),
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

		if (ipv6.fail_state === true) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					E('strong', {}, [ _('IPv6 fail-state detected.') ]),
					E('div', {}, [
						firstNonEmpty([ ipv6.message ], _('Transparent routing does not intercept IPv6 traffic.'))
					]),
					E('div', { 'class': 'routeflux-diagnostics-warning-actions' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleDisableIPv6')
						}, [ _('Disable IPv6 in RouteFlux') ])
					])
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Actions') ]),
			E('div', { 'class': 'routeflux-diagnostics-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRefreshPage')
				}, [ _('Refresh') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('IPv6 State') ]),
			E('div', { 'class': 'routeflux-overview-grid' }, [
				this.renderCard(_('Configured by RouteFlux'), ipv6.configured_disabled === true ? _('Disable IPv6') : _('Leave Enabled')),
				this.renderCard(_('Persistent State'), ipv6.persistent_disabled === true ? _('Disabled') : _('Enabled')),
				this.renderCard(_('Enabled Interfaces'), ipv6EnabledInterfacesLabel(ipv6)),
				this.renderCard(_('Config Path'), firstNonEmpty([ ipv6.config_path ], '-'))
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
