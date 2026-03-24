'use strict';
'require view';
'require fs';
'require ui';

var routefluxBinary = '/usr/bin/routeflux';
var commonLogLevels = [ 'debug', 'info', 'warning', 'error' ];

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

function boolLabel(value) {
	return value === true ? _('Enabled') : _('Disabled');
}

function modeLabel(value) {
	switch (trim(value)) {
	case 'auto':
		return _('Auto');
	case 'manual':
		return _('Manual');
	default:
		return _('Disconnected');
	}
}

function renderLogLevelOptions(current) {
	var values = commonLogLevels.slice();
	var normalized = trim(current).toLowerCase() || 'info';
	var options = [];

	if (normalized !== '' && values.indexOf(normalized) === -1)
		values.push(normalized);

	for (var i = 0; i < values.length; i++) {
		var value = values[i];
		options.push(E('option', {
			'value': value,
			'selected': value === normalized ? 'selected' : null
		}, [ value ]));
	}

	return options;
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'settings', 'get' ]).catch(function(err) {
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
		var current = this.currentSettings || {};
		var refreshInterval = trim(document.querySelector('#routeflux-settings-refresh-interval').value);
		var healthCheckInterval = trim(document.querySelector('#routeflux-settings-health-check-interval').value);
		var switchCooldown = trim(document.querySelector('#routeflux-settings-switch-cooldown').value);
		var latencyThreshold = trim(document.querySelector('#routeflux-settings-latency-threshold').value);
		var autoMode = document.querySelector('#routeflux-settings-auto-mode').checked;
		var logLevel = trim(document.querySelector('#routeflux-settings-log-level').value).toLowerCase();
		var commands = [];

		if (refreshInterval === '' || healthCheckInterval === '' || switchCooldown === '' || latencyThreshold === '' || logLevel === '') {
			ui.addNotification(null, notificationParagraph(_('All settings fields must be filled in.')));
			return Promise.resolve();
		}

		if (trim(current.refresh_interval) !== refreshInterval)
			commands.push([ 'settings', 'set', 'refresh-interval', refreshInterval ]);

		if (trim(current.health_check_interval) !== healthCheckInterval)
			commands.push([ 'settings', 'set', 'health-check-interval', healthCheckInterval ]);

		if (trim(current.switch_cooldown) !== switchCooldown)
			commands.push([ 'settings', 'set', 'switch-cooldown', switchCooldown ]);

		if (trim(current.latency_threshold) !== latencyThreshold)
			commands.push([ 'settings', 'set', 'latency-threshold', latencyThreshold ]);

		if (!!current.auto_mode !== autoMode)
			commands.push([ 'settings', 'set', 'auto-mode', autoMode ? 'true' : 'false' ]);

		if (trim(current.log_level).toLowerCase() !== logLevel)
			commands.push([ 'settings', 'set', 'log-level', logLevel ]);

		if (commands.length === 0) {
			ui.addNotification(null, notificationParagraph(_('No settings changes to save.')), 'info');
			return Promise.resolve();
		}

		return this.runCommands(commands, _('Settings saved.'));
	},

	render: function(data) {
		var status = data[0] || {};
		var settings = data[1] && !data[1].__error__
			? data[1]
			: (status.settings || {});
		var subscriptions = Array.isArray(data[2]) ? data[2] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var connected = !!(status.state && status.state.connected === true);
		var state = status.state || {};
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

		this.currentSettings = {
			refresh_interval: trim(settings.refresh_interval),
			health_check_interval: trim(settings.health_check_interval),
			switch_cooldown: trim(settings.switch_cooldown),
			latency_threshold: trim(settings.latency_threshold),
			auto_mode: settings.auto_mode === true,
			log_level: trim(settings.log_level)
		};

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Settings error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[2].__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
			'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
			'.routeflux-settings-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; margin-bottom:12px; }',
			'.routeflux-settings-actions { display:flex; flex-wrap:wrap; gap:10px; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Settings') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage refresh timing, health checks, switching behavior, automatic selection, and log verbosity for RouteFlux.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected')),
			this.renderCard(_('Effective Mode'), modeLabel(state.mode)),
			this.renderCard(_('Auto Mode'), boolLabel(settings.auto_mode)),
			this.renderCard(_('Log Level'), firstNonEmpty([ settings.log_level ], 'info')),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]));

		if (connected) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message' }, [
					_('Changing auto mode or log level while connected can reapply the current runtime configuration immediately.')
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Configuration') ]),
			E('div', { 'class': 'routeflux-settings-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-refresh-interval' }, [ _('Refresh Interval') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-settings-refresh-interval',
							'class': 'cbi-input-text',
							'type': 'text',
							'value': trim(settings.refresh_interval) || '1h',
							'placeholder': _('Example: 1h')
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('How often RouteFlux should refresh subscriptions automatically. Go duration syntax such as 30m or 1h is supported.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-health-check-interval' }, [ _('Health Check Interval') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-settings-health-check-interval',
							'class': 'cbi-input-text',
							'type': 'text',
							'value': trim(settings.health_check_interval) || '30s',
							'placeholder': _('Example: 30s')
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('How often RouteFlux probes nodes while monitoring health and availability.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-switch-cooldown' }, [ _('Switch Cooldown') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-settings-switch-cooldown',
							'class': 'cbi-input-text',
							'type': 'text',
							'value': trim(settings.switch_cooldown) || '5m',
							'placeholder': _('Example: 5m')
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Minimum wait time before RouteFlux switches nodes again in automatic mode.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-latency-threshold' }, [ _('Latency Threshold') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-settings-latency-threshold',
							'class': 'cbi-input-text',
							'type': 'text',
							'value': trim(settings.latency_threshold) || '50ms',
							'placeholder': _('Example: 50ms')
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Maximum tolerated latency delta before automatic scoring considers one node meaningfully worse than another.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-auto-mode' }, [ _('Auto Mode') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('label', { 'style': 'display:flex; gap:8px; align-items:center;' }, [
							E('input', {
								'id': 'routeflux-settings-auto-mode',
								'type': 'checkbox',
								'checked': settings.auto_mode === true ? 'checked' : null
							}),
							_('Allow RouteFlux to select and switch to the best node automatically.')
						])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('When enabled while connected, RouteFlux can immediately re-enter automatic selection mode for the active subscription.')
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-settings-log-level' }, [ _('Log Level') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', { 'id': 'routeflux-settings-log-level' }, renderLogLevelOptions(settings.log_level))
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Controls the log verbosity written into the generated Xray configuration. Common values are debug, info, warning, and error.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-settings-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleSaveSettings')
				}, [ _('Save') ])
			])
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
