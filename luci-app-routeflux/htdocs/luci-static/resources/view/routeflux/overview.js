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

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
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

	execAction: function(argv, successMessage) {
		return fs.exec(routefluxBinary, argv).then(function(res) {
			var stderr = trim(res.stderr);
			var stdout = trim(res.stdout);

			if (res.code !== 0)
				throw new Error(stderr || stdout || _('RouteFlux command failed.'));

			ui.addNotification(null, notificationParagraph(stdout || successMessage), 'info');
			window.setTimeout(function() {
				window.location.reload();
			}, 350);

			return res;
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleConnectAuto: function(subscriptionId, ev) {
		var select = document.querySelector('#routeflux-subscription');
		var subscription = subscriptionId || (select ? trim(select.value) : '');

		if (subscription === '') {
			ui.addNotification(null, notificationParagraph(_('Choose a subscription first.')));
			return Promise.resolve();
		}

		return this.execAction(
			[ 'connect', '--auto', '--subscription', subscription ],
			_('Auto mode enabled.')
		);
	},

	handleDisconnect: function(ev) {
		return this.execAction([ 'disconnect' ], _('Disconnected.'));
	},

	handleRefreshActive: function(subscriptionId, ev) {
		if (trim(subscriptionId) === '') {
			ui.addNotification(null, notificationParagraph(_('There is no active subscription to refresh.')));
			return Promise.resolve();
		}

		return this.execAction(
			[ 'refresh', '--subscription', subscriptionId ],
			_('Active subscription refreshed.')
		);
	},

	renderCard: function(label, value) {
		return E('div', { 'class': 'routeflux-card' }, [
			E('div', { 'class': 'routeflux-card-label' }, [ label ]),
			E('div', { 'class': 'routeflux-card-value' }, [ value || _('Not selected') ])
		]);
	},

	renderSubscriptionsTable: function(subscriptions) {
		if (!Array.isArray(subscriptions) || subscriptions.length === 0)
			return E('p', {}, [ _('No subscriptions imported yet.') ]);

		var rows = subscriptions.map(function(sub) {
			var name = firstNonEmpty([
				sub.display_name,
				sub.provider_name,
				sub.id
			], sub.id);
			var nodes = Array.isArray(sub.nodes) ? sub.nodes.length : 0;

			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td' }, [ name ]),
				E('td', { 'class': 'td' }, [ String(nodes) ]),
				E('td', { 'class': 'td' }, [ trim(sub.last_updated_at) || _('Never') ]),
				E('td', { 'class': 'td' }, [ trim(sub.parser_status) || _('unknown') ])
			]);
		});

		return E('table', { 'class': 'table cbi-section-table' }, [
			E('tr', { 'class': 'tr cbi-section-table-titles' }, [
				E('th', { 'class': 'th' }, [ _('Name') ]),
				E('th', { 'class': 'th' }, [ _('Nodes') ]),
				E('th', { 'class': 'th' }, [ _('Updated') ]),
				E('th', { 'class': 'th' }, [ _('Status') ])
			])
		].concat(rows));
	},

	render: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var settings = status.settings || {};
		var firewall = settings.firewall || {};
		var dns = settings.dns || {};
		var firewallMode = 'disabled';

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		if (firewall.enabled === true) {
			if (Array.isArray(firewall.source_cidrs) && firewall.source_cidrs.length > 0)
				firewallMode = 'hosts';
			else if (Array.isArray(firewall.target_cidrs) && firewall.target_cidrs.length > 0)
				firewallMode = 'targets';
			else
				firewallMode = 'enabled';
		}

		var connected = status.state && status.state.connected === true;
		var provider = firstNonEmpty([
			activeSubscription.provider_name,
			activeSubscription.display_name
		], _('Not selected'));
		var profile = firstNonEmpty([
			activeSubscription.display_name,
			activeSubscription.provider_name
		], _('Not selected'));
		var nodeName = nodeDisplayName(activeNode, _('Not selected'));
		var activeSubscriptionId = trim(activeSubscription.id);
		var currentSubscriptionId = activeSubscriptionId;

		if (currentSubscriptionId === '' && subscriptions.length > 0)
			currentSubscriptionId = trim(subscriptions[0].id);

		var subscriptionOptions = subscriptions.map(function(sub) {
			var label = firstNonEmpty([
				sub.display_name,
				sub.provider_name,
				sub.id
			], sub.id);
			var attrs = { value: sub.id };

			if (trim(sub.id) === currentSubscriptionId)
				attrs.selected = 'selected';

			return E('option', attrs, [ label ]);
		});

		var content = [
			E('style', { 'type': 'text/css' }, [
				'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
				'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
				'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
				'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
				'.routeflux-actions { display:flex; flex-wrap:wrap; gap:10px; align-items:flex-end; margin-bottom:16px; }',
				'.routeflux-actions > * { margin:0; }',
				'.routeflux-actions select { min-width:260px; }'
			]),
			E('h2', {}, [ _('RouteFlux') ]),
			E('p', { 'class': 'cbi-section-descr' }, [
				_('RouteFlux overview for the current connection state, active profile, and the most common control actions.')
			]),
			E('div', { 'class': 'routeflux-overview-grid' }, [
				this.renderCard(_('State'), connected ? _('Connected') : _('Disconnected')),
				this.renderCard(_('Mode'), firstNonEmpty([ status.state && status.state.mode ], _('disconnected'))),
				this.renderCard(_('Provider'), provider),
				this.renderCard(_('Profile'), profile),
				this.renderCard(_('Node'), nodeName),
				this.renderCard(_('DNS'), firstNonEmpty([ dns.mode ], _('system'))),
				this.renderCard(_('Firewall'), firewallMode),
				this.renderCard(_('Last Refresh'), firstNonEmpty([ activeSubscription.last_updated_at ], _('Never')))
			]),
			E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, [ _('Actions') ]),
				E('div', { 'class': 'routeflux-actions' }, [
					E('div', { 'class': 'cbi-value' }, [
						E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-subscription' }, [ _('Subscription') ]),
						E('div', { 'class': 'cbi-value-field' }, [
							E('select', {
								'id': 'routeflux-subscription',
								'disabled': subscriptions.length === 0 ? 'disabled' : null
							}, subscriptionOptions)
						])
					]),
					E('button', {
						'class': 'cbi-button cbi-button-apply',
						'click': ui.createHandlerFn(this, 'handleConnectAuto', null),
						'disabled': subscriptions.length === 0 ? 'disabled' : null
					}, [ _('Connect Auto') ]),
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, 'handleRefreshActive', activeSubscriptionId),
						'disabled': activeSubscriptionId === '' ? 'disabled' : null
					}, [ _('Refresh Active') ]),
					E('button', {
						'class': 'cbi-button cbi-button-reset',
						'click': ui.createHandlerFn(this, 'handleDisconnect'),
						'disabled': connected ? null : 'disabled'
					}, [ _('Disconnect') ])
				])
			]),
			E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, [ _('Subscriptions') ]),
				this.renderSubscriptionsTable(subscriptions)
			])
		];

		if (trim(activeSubscription.last_error) !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, [ _('Last Error') ]),
				E('div', { 'class': 'alert-message warning' }, [ activeSubscription.last_error ])
			]));
		}

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
