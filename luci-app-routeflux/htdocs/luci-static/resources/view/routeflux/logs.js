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
			this.execJSON([ '--json', 'list', 'subscriptions' ]).catch(function(err) {
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
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var logs = data[2] || {};
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

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__)
			ui.addNotification(null, notificationParagraph(_('Logs error: %s').format(data[2].__error__)));

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
			this.renderCard(_('Active Provider'), activeProvider),
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
