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

function hasItems(values) {
	return Array.isArray(values) && values.length > 0;
}

function firewallMode(settings) {
	var hasSources = hasItems(settings.source_cidrs);
	var hasTargets = hasItems(settings.target_cidrs) || hasItems(settings.target_domains);

	if (settings.enabled !== true || (!hasSources && !hasTargets))
		return 'disabled';

	if (hasSources && !hasTargets)
		return 'hosts';

	if (hasTargets && !hasSources)
		return 'targets';

	return 'mixed';
}

function formMode(settings) {
	var mode = firewallMode(settings);

	if (mode === 'mixed')
		return 'hosts';

	return mode;
}

function selectorsForMode(settings, mode) {
	if (mode === 'hosts' && hasItems(settings.source_cidrs))
		return settings.source_cidrs.join('\n');

	if (mode === 'targets') {
		var values = [];

		if (hasItems(settings.target_domains))
			values = values.concat(settings.target_domains);
		if (hasItems(settings.target_cidrs))
			values = values.concat(settings.target_cidrs);

		return values.join('\n');
	}

	return '';
}

function modeSummary(mode) {
	switch (mode) {
	case 'hosts':
		return _('Hosts');
	case 'targets':
		return _('Targets');
	case 'mixed':
		return _('Mixed');
	default:
		return _('Disabled');
	}
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'firewall', 'get' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'list', 'subscriptions' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execText([ 'firewall', 'explain' ]).catch(function(err) {
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

	renderCard: function(label, value, options) {
		return routefluxUI.renderSummaryCard(label, value, options);
	},

	updateModeUI: function(mode) {
		var row = document.querySelector('#routeflux-firewall-selectors-row');
		var label = document.querySelector('#routeflux-firewall-selectors-label');
		var textarea = document.querySelector('#routeflux-firewall-selectors');
		var help = document.querySelector('#routeflux-firewall-selectors-help');
		var blockHelp = document.querySelector('#routeflux-firewall-block-quic-help');

		if (!row || !label || !textarea || !help || !blockHelp)
			return;

		if (mode === 'hosts') {
			label.textContent = _('Hosts');
			textarea.placeholder = _('Examples: 192.168.1.150 192.168.1.0/24 192.168.1.150-192.168.1.159 all');
			textarea.disabled = false;
			row.style.opacity = '1';
			help.textContent = _('Route selected LAN clients through RouteFlux. Separate values with spaces, commas, or new lines.');
			blockHelp.textContent = _('When enabled, RouteFlux drops UDP/443 in host mode so applications cannot bypass TCP routing through QUIC.');
			return;
		}

		if (mode === 'targets') {
			label.textContent = _('Targets');
			textarea.placeholder = _('Examples: youtube.com instagram.com 1.1.1.1 203.0.113.10-203.0.113.20');
			textarea.disabled = false;
			row.style.opacity = '1';
			help.textContent = _('Route only selected services, domains, or destination IPv4 targets through RouteFlux. Popular targets like youtube.com and instagram.com auto-expand to the domain families they need. Domains match subdomains and work best when clients use the router DNS.');
			blockHelp.textContent = _('When enabled, RouteFlux drops LAN UDP/443 in targets mode so selected services cannot bypass domain matching through QUIC.');
			return;
		}

		label.textContent = _('Selectors');
		textarea.placeholder = _('No selectors are required while the firewall is disabled.');
		textarea.disabled = true;
		row.style.opacity = '0.65';
		help.textContent = _('Save disabled mode to keep the saved settings but stop redirecting traffic.');
		blockHelp.textContent = _('This value is saved even while the firewall is disabled.');
	},

	handleModeChange: function(ev) {
		this.updateModeUI(trim(ev.currentTarget.value));
	},

	handleSaveSettings: function(ev) {
		var modeElement = document.querySelector('#routeflux-firewall-mode');
		var selectorsElement = document.querySelector('#routeflux-firewall-selectors');
		var portElement = document.querySelector('#routeflux-firewall-port');
		var blockQUICElement = document.querySelector('#routeflux-firewall-block-quic');
		var mode = trim(modeElement && modeElement.value);
		var selectors = parseList(selectorsElement && selectorsElement.value);
		var portRaw = trim(portElement && portElement.value);
		var port = parseInt(portRaw, 10);
		var blockQUIC = !!(blockQUICElement && blockQUICElement.checked);
		var commands = [];

		if (!/^\d+$/.test(portRaw) || isNaN(port) || port <= 0) {
			ui.addNotification(null, notificationParagraph(_('Transparent port must be a positive integer.')));
			return Promise.resolve();
		}

		if (mode === 'hosts' || mode === 'targets') {
			if (selectors.length === 0) {
				ui.addNotification(null, notificationParagraph(
					mode === 'hosts'
						? _('Enter at least one LAN host, CIDR, range, or all.')
						: _('Enter at least one domain, IPv4 address, CIDR, or range.')
				));
				return Promise.resolve();
			}

			commands.push([ 'firewall', 'set', 'block-quic', blockQUIC ? 'true' : 'false' ]);
			commands.push([ 'firewall', 'set', '--port', String(port), mode ].concat(selectors));

			return this.runCommands(commands, _('Firewall settings saved.'));
		}

		commands.push([ 'firewall', 'set', 'block-quic', blockQUIC ? 'true' : 'false' ]);
		commands.push([ 'firewall', 'set', 'port', String(port) ]);
		commands.push([ 'firewall', 'disable' ]);

		return this.runCommands(commands, _('Firewall settings saved and routing disabled.'));
	},

	handleDisable: function(ev) {
		if (!window.confirm(_('Disable transparent routing?')))
			return Promise.resolve();

		return this.runCommands([
			[ 'firewall', 'disable' ]
		], _('Firewall disabled.'));
	},

	render: function(data) {
		var status = data[0] || {};
		var firewall = data[1] && !data[1].__error__
			? data[1]
			: ((status.settings || {}).firewall || {});
		var subscriptions = Array.isArray(data[2]) ? data[2] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var explainText = data[3] && data[3].__error__ ? '' : trim(data[3]);
		var connected = !!(status.state && status.state.connected === true);
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var currentMode = firewallMode(firewall);
		var selectedMode = formMode(firewall);
		var selectorsText = selectorsForMode(firewall, selectedMode);
		var activeEntry = presentationForSubscription(activeSubscription, presentation);
		var activeProvider = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.provider_title : providerTitle(activeSubscription))
			: _('Not selected');
		var activeProfile = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.profile_label : _('Profile 1'))
			: _('Not selected');
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));
		var content = [];

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Firewall error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[2].__error__)));

		if (data[3] && data[3].__error__)
			ui.addNotification(null, notificationParagraph(_('Firewall help error: %s').format(data[3].__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-firewall-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; margin-bottom:12px; }',
			'.routeflux-firewall-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-firewall-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-firewall-help { white-space:pre-wrap; margin:0; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Firewall') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage transparent routing for selected LAN hosts or service/domain/IPv4 targets without leaving LuCI.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(connected),
				'primary': true
			}),
			this.renderCard(_('Mode'), modeSummary(currentMode)),
			this.renderCard(_('Transparent Port'), String(firewall.transparent_port || 12345)),
			this.renderCard(_('Block QUIC'), firewall.block_quic === true ? _('Enabled') : _('Disabled')),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]));

		if (currentMode === 'mixed') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('A mixed firewall state was detected with both hosts and targets configured. This page edits one mode at a time; saving will normalize the configuration.')
				])
			]));
		} else if (currentMode !== 'disabled' && !connected) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message' }, [
					_('Transparent routing settings are saved, but RouteFlux is currently disconnected. Connect a node to apply the nftables rules.')
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Configuration') ]),
			E('div', { 'class': 'routeflux-firewall-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-firewall-mode' }, [ _('Mode') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', {
							'id': 'routeflux-firewall-mode',
							'change': ui.createHandlerFn(this, 'handleModeChange')
						}, [
							E('option', { 'value': 'disabled', 'selected': selectedMode === 'disabled' ? 'selected' : null }, [ _('Disabled') ]),
							E('option', { 'value': 'hosts', 'selected': selectedMode === 'hosts' ? 'selected' : null }, [ _('Hosts') ]),
							E('option', { 'value': 'targets', 'selected': selectedMode === 'targets' ? 'selected' : null }, [ _('Targets') ])
						])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Hosts routes all TCP traffic from selected LAN clients. Targets routes only selected services, domains, or destination IPv4 targets.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-firewall-port' }, [ _('Transparent Port') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-firewall-port',
							'class': 'cbi-input-text',
							'type': 'number',
							'min': '1',
							'value': String(firewall.transparent_port || 12345)
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Transparent redirect port used by nftables and the generated Xray runtime config.')
					])
				]),
				E('div', {
					'id': 'routeflux-firewall-selectors-row',
					'class': 'cbi-value',
					'style': 'grid-column:1 / -1'
				}, [
					E('label', { 'id': 'routeflux-firewall-selectors-label', 'class': 'cbi-value-title', 'for': 'routeflux-firewall-selectors' }, [ _('Selectors') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-firewall-selectors',
							'class': 'cbi-input-textarea'
						}, [ selectorsText ])
					]),
					E('div', { 'id': 'routeflux-firewall-selectors-help', 'class': 'cbi-value-description' }, [
						'\u00a0'
					])
				]),
				E('div', {
					'class': 'cbi-value',
					'style': 'grid-column:1 / -1'
				}, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-firewall-block-quic' }, [ _('Block QUIC') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('label', { 'style': 'display:flex; gap:8px; align-items:center;' }, [
							E('input', {
								'id': 'routeflux-firewall-block-quic',
								'type': 'checkbox',
								'checked': firewall.block_quic === true ? 'checked' : null
							}),
							_('Drop UDP/443 traffic to stop QUIC from bypassing transparent TCP routing.')
						])
					]),
					E('div', { 'id': 'routeflux-firewall-block-quic-help', 'class': 'cbi-value-description' }, [
						'\u00a0'
					])
				])
			]),
			E('div', { 'class': 'routeflux-firewall-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleSaveSettings')
				}, [ _('Save') ]),
				E('button', {
					'class': 'cbi-button cbi-button-reset',
					'click': ui.createHandlerFn(this, 'handleDisable'),
					'disabled': currentMode === 'disabled' ? 'disabled' : null
				}, [ _('Disable Routing') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('details', { 'open': 'open' }, [
				E('summary', {}, [ _('Help') ]),
				E('pre', { 'class': 'routeflux-firewall-help' }, [
					explainText || _('No firewall help text is available.')
				])
			])
		]));

		window.setTimeout(L.bind(function() {
			this.updateModeUI(selectedMode);
		}, this), 0);

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
