'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var pingOverviewSessionKey = 'routeflux.overview.ping.latest';

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
	var groups = [];
	var groupsByKey = {};
	var byId = {};

	for (var i = 0; i < subscriptions.length; i++) {
		var sub = subscriptions[i];
		var title = providerTitle(sub);
		var key = title.toLowerCase();
		var group = groupsByKey[key];

		if (!group) {
			group = {
				key: key,
				title: title,
				items: []
			};
			groupsByKey[key] = group;
			groups.push(group);
		}

		var item = {
			subscription: sub,
			provider_title: title,
			profile_label: _('Profile %d').format(group.items.length + 1)
		};

		group.items.push(item);
		byId[trim(sub.id)] = item;
	}

	return {
		groups: groups,
		by_id: byId
	};
}

function presentationForSubscription(sub, presentation) {
	var id = trim(sub && sub.id);

	if (id === '' || !presentation || !presentation.by_id)
		return null;

	return presentation.by_id[id] || null;
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

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function summarizePingError(value) {
	var text = trim(value);

	if (text === '')
		return '';

	text = text.split(/\r?\n/)[0];
	if (text.length > 96)
		return text.slice(0, 93) + '...';

	return text;
}

function seededPingForNode(status, nodeId) {
	var healthMap = status && status.state && status.state.health;
	var health = healthMap && healthMap[nodeId];
	var rawLatency;

	if (!health)
		return null;

	rawLatency = firstNonEmpty([ health.last_latency, health.average_latency ], '');

	return {
		'source': 'seed',
		'healthy': health.healthy === true,
		'latency_ms': routefluxUI.durationToMilliseconds(rawLatency),
		'checked_at': trim(health.last_checked_at),
		'error': summarizePingError(health.last_failure_reason)
	};
}

function livePingForNode(subscriptionId, nodeId) {
	var cache = routefluxUI.readSessionJSON(pingOverviewSessionKey);
	var results;

	if (!cache || trim(cache.subscription_id) !== trim(subscriptionId))
		return null;

	results = cache.results_by_id || {};
	return results[trim(nodeId)] || null;
}

function resolveActivePing(status) {
	var activeSubscription = status && status.active_subscription;
	var activeNode = status && status.active_node;
	var subscriptionId = trim(activeSubscription && activeSubscription.id);
	var nodeId = trim(activeNode && activeNode.id);

	if (subscriptionId === '' || nodeId === '')
		return null;

	return livePingForNode(subscriptionId, nodeId) || seededPingForNode(status, nodeId);
}

function activePingPrimaryLabel(result) {
	var latencyLabel = routefluxUI.formatLatencyMS(result && result.latency_ms);

	if (!result)
		return _('Not checked');

	if (result.source === 'seed') {
		if (result.healthy === false)
			return _('Last known: Unavailable');
		if (latencyLabel !== '')
			return _('Last known: %s').format(latencyLabel);
		return _('Not checked');
	}

	if (result.healthy === false)
		return _('Unavailable');

	if (latencyLabel !== '')
		return latencyLabel;

	return _('Not checked');
}

function activePingSecondaryLabel(result) {
	var checkedAt = trim(result && result.checked_at);

	if (checkedAt === '')
		return '';

	if (result && result.source === 'seed')
		return _('Last check: %s').format(routefluxUI.formatTimestamp(checkedAt));

	return _('Checked: %s').format(routefluxUI.formatTimestamp(checkedAt));
}

return view.extend({
	load: function() {
		return this.requestPageData();
	},

	requestPageData: function() {
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

	refreshPageContent: function() {
		return this.requestPageData().then(L.bind(function(data) {
			var root = document.querySelector('#routeflux-overview-root');
			if (root)
				dom.content(root, this.renderPageContent(data));
		}, this));
	},

	execAction: function(argv, successMessage) {
		return fs.exec(routefluxBinary, argv).then(L.bind(function(res) {
			var stderr = trim(res.stderr);
			var stdout = trim(res.stdout);

			if (res.code !== 0)
				throw new Error(stderr || stdout || _('RouteFlux command failed.'));

			ui.addNotification(null, notificationParagraph(stdout || successMessage), 'info');
			return this.refreshPageContent().then(function() {
				return res;
			});
		}, this)).catch(function(err) {
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

	renderCard: function(label, value, options) {
		var settings = options || {};

		settings.fallback = settings.fallback != null ? settings.fallback : _('Not selected');

		return routefluxUI.renderSummaryCard(label, value, settings);
	},

	renderSubscriptionsTable: function(subscriptions, presentation) {
		if (!Array.isArray(subscriptions) || subscriptions.length === 0)
			return E('p', {}, [ _('No subscriptions imported yet.') ]);

		var rows = subscriptions.map(function(sub) {
			var entry = presentationForSubscription(sub, presentation);
			var name = entry
				? entry.provider_title + ' / ' + entry.profile_label
				: providerTitle(sub) + ' / ' + _('Profile 1');
			var nodes = Array.isArray(sub.nodes) ? sub.nodes.length : 0;

			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td' }, [ name ]),
				E('td', { 'class': 'td' }, [ String(nodes) ]),
				E('td', { 'class': 'td' }, [ routefluxUI.formatTimestamp(sub.last_updated_at) || _('Never') ]),
				E('td', { 'class': 'td' }, [ trim(sub.parser_status) || _('unknown') ])
			]);
		});

		return E('table', { 'class': 'table cbi-section-table routeflux-data-table' }, [
			E('tr', { 'class': 'tr cbi-section-table-titles' }, [
				E('th', { 'class': 'th' }, [ _('Name') ]),
				E('th', { 'class': 'th' }, [ _('Nodes') ]),
				E('th', { 'class': 'th' }, [ _('Updated') ]),
				E('th', { 'class': 'th' }, [ _('Status') ])
			])
		].concat(rows));
	},

	renderPageContent: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var zapret = status.zapret || {};
		var state = status.state || {};
		var settings = status.settings || {};
		var firewall = settings.firewall || {};
		var dns = settings.dns || {};
		var activeTransport = firstNonEmpty([
			status.active_transport,
			status.state && status.state.active_transport
		], 'direct');
		var firewallMode = 'disabled';
		var explicitFirewallMode = trim(firewall.mode);
		var hasTargets = (firewall.targets && Array.isArray(firewall.targets.services) && firewall.targets.services.length > 0) ||
			(firewall.targets && Array.isArray(firewall.targets.cidrs) && firewall.targets.cidrs.length > 0) ||
			(firewall.targets && Array.isArray(firewall.targets.domains) && firewall.targets.domains.length > 0) ||
			(Array.isArray(firewall.target_services) && firewall.target_services.length > 0) ||
			(Array.isArray(firewall.target_cidrs) && firewall.target_cidrs.length > 0) ||
			(Array.isArray(firewall.target_domains) && firewall.target_domains.length > 0);
		var hasSplit = (firewall.split && firewall.split.proxy && Array.isArray(firewall.split.proxy.services) && firewall.split.proxy.services.length > 0) ||
			(firewall.split && firewall.split.proxy && Array.isArray(firewall.split.proxy.cidrs) && firewall.split.proxy.cidrs.length > 0) ||
			(firewall.split && firewall.split.proxy && Array.isArray(firewall.split.proxy.domains) && firewall.split.proxy.domains.length > 0) ||
			(firewall.split && firewall.split.bypass && Array.isArray(firewall.split.bypass.services) && firewall.split.bypass.services.length > 0) ||
			(firewall.split && firewall.split.bypass && Array.isArray(firewall.split.bypass.cidrs) && firewall.split.bypass.cidrs.length > 0) ||
			(firewall.split && firewall.split.bypass && Array.isArray(firewall.split.bypass.domains) && firewall.split.bypass.domains.length > 0);
		var hasHosts = (Array.isArray(firewall.hosts) && firewall.hosts.length > 0) ||
			(Array.isArray(firewall.source_cidrs) && firewall.source_cidrs.length > 0);

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		if (firewall.enabled === true) {
			if (explicitFirewallMode === 'hosts' || hasHosts)
				firewallMode = 'hosts';
			else if (explicitFirewallMode === 'split' || hasSplit)
				firewallMode = 'split';
			else if (explicitFirewallMode === 'targets' || hasTargets)
				firewallMode = 'targets';
			else
				firewallMode = 'enabled';
		}

		var connected = state.connected === true;
		var activeEntry = presentationForSubscription(activeSubscription, presentation);
		var activePing = resolveActivePing(status);
		var provider = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.provider_title : providerTitle(activeSubscription))
			: _('Not selected');
		var profile = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.profile_label : _('Profile 1'))
			: _('Not selected');
		var nodeName = nodeDisplayName(activeNode, _('Not selected'));
		var activeSubscriptionId = trim(activeSubscription.id);
		var currentSubscriptionId = activeSubscriptionId;

		if (currentSubscriptionId === '' && subscriptions.length > 0)
			currentSubscriptionId = trim(subscriptions[0].id);

		var subscriptionOptions = subscriptions.map(function(sub) {
			var entry = presentationForSubscription(sub, presentation);
			var label = entry
				? entry.provider_title + ' / ' + entry.profile_label
				: providerTitle(sub) + ' / ' + _('Profile 1');
			var attrs = { value: sub.id };

			if (trim(sub.id) === currentSubscriptionId)
				attrs.selected = 'selected';

			return E('option', attrs, [ label ]);
		});

		var content = [
			routefluxUI.renderSharedStyles(),
			E('style', { 'type': 'text/css' }, [
				'.routeflux-overview-hero { margin-bottom:18px; }',
				'.routeflux-overview-hero-actions { grid-template-columns:minmax(0, 1fr); }',
				'.routeflux-overview-control { display:grid; gap:8px; }',
				'.routeflux-overview-control .cbi-value { margin:0; }',
				'.routeflux-overview-action-grid { display:grid; gap:10px; }',
				'.routeflux-overview-action-grid .cbi-button { width:100%; }',
				'.routeflux-overview-active-ping { display:grid; gap:6px; }',
				'.routeflux-overview-active-ping .routeflux-active-ping-primary { font-weight:700; }',
				'.routeflux-overview-active-ping .routeflux-active-ping-meta { color:var(--routeflux-text-muted); font-size:12px; line-height:1.45; overflow-wrap:anywhere; word-break:break-word; }'
			]),
			E('section', { 'class': 'routeflux-page-hero routeflux-surface routeflux-surface-elevated routeflux-overview-hero' }, [
				E('div', { 'class': 'routeflux-page-hero-copy' }, [
					E('span', { 'class': 'routeflux-page-kicker' }, [ _('Overview') ]),
					E('h2', { 'class': 'routeflux-page-hero-title' }, [ _('RouteFlux') ]),
					E('p', { 'class': 'routeflux-page-hero-description' }, [
						_('RouteFlux overview for the current connection state, active profile, and the most common control actions.')
					]),
					E('div', { 'class': 'routeflux-page-hero-meta' }, [
						E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
							E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Provider') ]),
							E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ provider ])
						]),
						E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
							E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Profile') ]),
							E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ profile ])
						]),
						E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
							E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Node') ]),
							E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ nodeName ])
						])
					])
				]),
				E('div', { 'class': 'routeflux-page-hero-actions routeflux-overview-hero-actions' }, [
					E('div', { 'class': 'routeflux-surface routeflux-overview-control' }, [
						E('div', { 'class': 'routeflux-section-heading' }, [
							E('div', { 'class': 'routeflux-section-heading-copy' }, [
								E('h3', {}, [ _('Quick Actions') ]),
								E('p', {}, [ _('Pick a subscription, then connect or refresh without leaving the dashboard.') ])
							])
						]),
						E('div', { 'class': 'cbi-value' }, [
							E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-subscription' }, [ _('Subscription') ]),
							E('div', { 'class': 'cbi-value-field' }, [
								E('select', {
									'id': 'routeflux-subscription',
									'disabled': subscriptions.length === 0 ? 'disabled' : null
								}, subscriptionOptions)
							])
						]),
						E('div', { 'class': 'routeflux-overview-action-grid routeflux-page-hero-actions' }, [
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
					])
				])
			]),
			E('div', { 'class': 'routeflux-overview-grid' }, [
				this.renderCard(_('State'), connected ? _('Connected') : _('Disconnected'), {
					'tone': routefluxUI.statusTone(connected),
					'primary': true
				}),
				this.renderCard(_('Mode'), firstNonEmpty([ state.mode ], _('disconnected'))),
				this.renderCard(_('Transport'), activeTransport),
				this.renderCard(_('Provider'), provider),
				this.renderCard(_('Profile'), profile),
				this.renderCard(_('Node'), nodeName),
				this.renderCard(_('Active Ping'), [
					E('div', { 'class': 'routeflux-overview-active-ping' }, [
						E('div', { 'class': 'routeflux-active-ping-primary' }, [ activePingPrimaryLabel(activePing) ]),
						activePingSecondaryLabel(activePing) !== '' ? E('div', { 'class': 'routeflux-active-ping-meta' }, [ activePingSecondaryLabel(activePing) ]) : '',
						activePing && activePing.error ? E('div', { 'class': 'routeflux-active-ping-meta', 'title': activePing.error }, [ activePing.error ]) : ''
					])
				], {
					'tone': activePing ? routefluxUI.statusTone(activePing.healthy === true) : ''
				}),
				this.renderCard(_('DNS'), firstNonEmpty([ dns.mode ], _('system'))),
				this.renderCard(_('Firewall'), firewallMode),
				this.renderCard(_('Last Refresh'), routefluxUI.formatTimestamp(activeSubscription.last_updated_at) || _('Never'))
			]),
			E('section', { 'class': 'cbi-section routeflux-surface' }, [
				E('div', { 'class': 'routeflux-section-heading' }, [
					E('div', { 'class': 'routeflux-section-heading-copy' }, [
						E('h3', {}, [ _('Subscriptions') ]),
						E('p', {}, [ _('Review imported profiles, their node counts, freshness, and parser status at a glance.') ])
					])
				]),
				this.renderSubscriptionsTable(subscriptions, presentation)
			])
		];

		if (trim(activeSubscription.last_error) !== '') {
				content.push(E('div', { 'class': 'cbi-section routeflux-surface' }, [
					E('h3', {}, [ _('Last Error') ]),
					E('div', { 'class': 'alert-message warning' }, [ activeSubscription.last_error ])
				]));
		}

		if (activeTransport === 'zapret') {
				content.push(E('div', { 'class': 'cbi-section routeflux-surface' }, [
					E('div', { 'class': 'alert-message notice' }, [
						E('strong', {}, [ _('Zapret fallback is active.') ]),
						E('div', {}, [
						_('Monitored subscription: %s').format(provider + ' / ' + profile)
					]),
					E('div', {}, [
						_('Reason: %s').format(firstNonEmpty([ state.last_failure_reason, zapret.last_reason ], _('No failure reason recorded.')))
					]),
					E('div', {}, [
						_('Zapret service state: %s').format(firstNonEmpty([ zapret.service_state ], _('unknown')))
					])
				])
			]));
		}

		return content;
	},

	render: function(data) {
		return E('div', {
			'id': 'routeflux-overview-root',
			'class': routefluxUI.withThemeClass('routeflux-page-shell routeflux-page-shell-overview')
		}, this.renderPageContent(data));
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
