'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require routeflux.ui as routefluxUI';

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
				items: [],
				total_nodes: 0
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
		group.total_nodes += Array.isArray(sub.nodes) ? sub.nodes.length : 0;
		byId[trim(sub.id)] = item;
	}

	return {
		groups: groups,
		by_id: byId
	};
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

function badge(text, extraClass) {
	var className = 'label';

	if (extraClass)
		className += ' ' + extraClass;

	return E('span', { 'class': className }, [ text ]);
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
			var root = document.querySelector('#routeflux-subscriptions-root');
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

	handleAdd: function(ev) {
		var source = trim(document.querySelector('#routeflux-add-source').value);
		var name = trim(document.querySelector('#routeflux-add-name').value);
		var argv = [ 'add' ];

		if (source === '') {
			ui.addNotification(null, notificationParagraph(_('Paste a subscription URL, share link, or 3x-ui/Xray JSON first.')));
			return Promise.resolve();
		}

		if (name !== '')
			argv.push('--name', name);

		if (source.match(/^https?:\/\//i))
			argv.push('--url', source);
		else
			argv.push('--raw', source);

		return this.execAction(argv, _('Subscription added.'));
	},

	handleRefreshSubscription: function(subscriptionId, ev) {
		return this.execAction(
			[ 'refresh', '--subscription', subscriptionId ],
			_('Subscription refreshed.')
		);
	},

	handleRemoveSubscription: function(subscriptionId, displayName, ev) {
		if (!window.confirm(_('Remove subscription "%s"?').format(displayName || subscriptionId)))
			return Promise.resolve();

		return this.execAction(
			[ 'remove', subscriptionId ],
			_('Subscription removed.')
		);
	},

	handleRemoveAll: function(ev) {
		if (!window.confirm(_('Remove all imported subscriptions? This disconnects the active profile if needed.')))
			return Promise.resolve();

		return this.execAction(
			[ 'remove', '--all' ],
			_('All subscriptions removed.')
		);
	},

	handleConnectAuto: function(subscriptionId, ev) {
		return this.execAction(
			[ 'connect', '--auto', '--subscription', subscriptionId ],
			_('Auto mode enabled.')
		);
	},

	handleConnectNode: function(subscriptionId, nodeId, ev) {
		return this.execAction(
			[ 'connect', '--subscription', subscriptionId, '--node', nodeId ],
			_('Node connected.')
		);
	},

	renderNodeTable: function(subscription, activeSubscriptionId, activeNodeId) {
		var nodes = Array.isArray(subscription.nodes) ? subscription.nodes : [];

		if (nodes.length === 0)
			return E('p', {}, [ _('No nodes found in this subscription.') ]);

		var rows = nodes.map(L.bind(function(node) {
			var isActive = subscription.id === activeSubscriptionId && node.id === activeNodeId;
			var name = nodeDisplayName(node, node.id);
			var address = firstNonEmpty([
				node.address && node.port ? node.address + ':' + node.port : '',
				node.address
			], '-');

			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td' }, [
					name,
					isActive ? E('div', { 'style': 'margin-top:4px' }, [ badge(_('Active'), 'notice') ]) : ''
				]),
				E('td', { 'class': 'td' }, [ address ]),
				E('td', { 'class': 'td' }, [ firstNonEmpty([ node.protocol ], '-') ]),
				E('td', { 'class': 'td' }, [ firstNonEmpty([ node.transport ], '-') ]),
				E('td', { 'class': 'td' }, [ firstNonEmpty([ node.security ], '-') ]),
				E('td', { 'class': 'td right' }, [
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, 'handleConnectNode', subscription.id, node.id)
					}, [ _('Connect') ])
				])
			]);
		}, this));

		return E('table', { 'class': 'table cbi-section-table' }, [
			E('tr', { 'class': 'tr cbi-section-table-titles' }, [
				E('th', { 'class': 'th' }, [ _('Node') ]),
				E('th', { 'class': 'th' }, [ _('Address') ]),
				E('th', { 'class': 'th' }, [ _('Protocol') ]),
				E('th', { 'class': 'th' }, [ _('Transport') ]),
				E('th', { 'class': 'th' }, [ _('Security') ]),
				E('th', { 'class': 'th right' }, [ '\u00a0' ])
			])
		].concat(rows));
	},

	renderSubscriptionCard: function(entry, activeSubscriptionId, activeNodeId) {
		var subscription = entry.subscription;
		var displayName = entry.profile_label;
		var providerName = entry.provider_title;
		var isActive = subscription.id === activeSubscriptionId;
		var nodesCount = Array.isArray(subscription.nodes) ? subscription.nodes.length : 0;
		var metaRows = [
			[ _('ID'), subscription.id ],
			[ _('Provider'), providerName ],
			[ _('Profile'), displayName ],
			[ _('Source Type'), firstNonEmpty([ subscription.source_type ], '-') ],
			[ _('Updated'), routefluxUI.formatTimestamp(subscription.last_updated_at) || _('Never') ],
			[ _('Status'), firstNonEmpty([ subscription.parser_status ], _('unknown')) ],
			[ _('Nodes'), String(nodesCount) ]
		].map(function(item) {
			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td left', 'style': 'width:180px' }, [ item[0] ]),
				E('td', { 'class': 'td left' }, [ item[1] ])
			]);
		});

		var headerNodes = [
			E('div', { 'class': 'routeflux-subscription-title' }, [ displayName ])
		];

		headerNodes.push(E('div', { 'class': 'routeflux-subscription-provider' }, [ providerName ]));

		if (isActive)
			headerNodes.push(E('div', { 'style': 'margin-top:6px' }, [ badge(_('Active'), 'notice') ]));

		return E('div', { 'class': 'cbi-section routeflux-subscription-card' }, [
			E('div', { 'class': 'routeflux-subscription-header' }, [
				E('div', {}, headerNodes),
				E('div', { 'class': 'routeflux-subscription-actions' }, [
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, 'handleRefreshSubscription', subscription.id)
					}, [ _('Refresh') ]),
					E('button', {
						'class': 'cbi-button cbi-button-apply',
						'click': ui.createHandlerFn(this, 'handleConnectAuto', subscription.id)
					}, [ _('Connect Auto') ]),
					E('button', {
						'class': 'cbi-button cbi-button-negative',
						'click': ui.createHandlerFn(this, 'handleRemoveSubscription', subscription.id, displayName)
					}, [ _('Remove') ])
				])
			]),
			E('table', { 'class': 'table' }, metaRows),
			trim(subscription.last_error) !== '' ? E('div', { 'class': 'alert-message warning', 'style': 'margin-top:10px' }, [
				subscription.last_error
			]) : '',
			E('details', { 'class': 'routeflux-node-details', 'open': isActive ? 'open' : null }, [
				E('summary', {}, [
					_('Nodes (%d)').format(nodesCount)
				]),
				this.renderNodeTable(subscription, activeSubscriptionId, activeNodeId)
			])
		]);
	},

	renderProviderGroup: function(group, activeSubscriptionId, activeNodeId) {
		var description = _('%d profile(s), %d node(s)').format(group.items.length, group.total_nodes);
		var content = [
			E('div', { 'class': 'routeflux-provider-group-header' }, [
				E('div', { 'class': 'routeflux-provider-group-title' }, [ group.title ]),
				E('div', { 'class': 'routeflux-provider-group-meta' }, [ description ])
			])
		];

		for (var i = 0; i < group.items.length; i++)
			content.push(this.renderSubscriptionCard(group.items[i], activeSubscriptionId, activeNodeId));

		return E('div', { 'class': 'routeflux-provider-group' }, content);
	},

	renderPageContent: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var activeSubscriptionId = trim(status.active_subscription && status.active_subscription.id);
		var activeNodeId = trim(status.active_node && status.active_node.id);
		var content = [];

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-subscription-card { margin-bottom:16px; }',
			'.routeflux-subscription-header { display:flex; flex-wrap:wrap; justify-content:space-between; gap:12px; margin-bottom:12px; }',
			'.routeflux-subscription-title { font-size:18px; font-weight:600; }',
			'.routeflux-subscription-provider { color:var(--text-color-secondary, #666); margin-top:4px; }',
			'.routeflux-subscription-actions { display:flex; flex-wrap:wrap; gap:8px; align-items:flex-start; }',
			'.routeflux-add-grid { display:grid; grid-template-columns:minmax(220px, 320px) 1fr; gap:12px; margin-bottom:12px; }',
			'.routeflux-add-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-add-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-node-details { margin-top:12px; }',
			'.routeflux-node-details summary { cursor:pointer; margin-bottom:10px; }',
			'.routeflux-provider-group { margin-bottom:22px; }',
			'.routeflux-provider-group-header { display:flex; flex-wrap:wrap; justify-content:space-between; gap:12px; align-items:baseline; margin:12px 0 8px; }',
			'.routeflux-provider-group-title { font-size:22px; font-weight:600; }',
			'.routeflux-provider-group-meta { color:var(--text-color-secondary, #666); }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Subscriptions') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage imported subscriptions, add new providers, refresh existing profiles, remove one or all profiles, and connect a specific node or the best node automatically.')
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Add Subscription') ]),
			E('div', { 'class': 'routeflux-add-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-add-name' }, [ _('Display Name (optional)') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-add-name',
							'class': 'cbi-input-text',
							'type': 'text',
							'placeholder': _('Optional provider name')
						})
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-add-source' }, [ _('Subscription URL, share link, or 3x-ui/Xray JSON') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-add-source',
							'class': 'cbi-input-textarea',
							'placeholder': _('Paste a subscription URL, VLESS/VMess/Trojan link, or 3x-ui/Xray JSON here.')
						})
					])
				])
			]),
			E('div', { 'class': 'routeflux-add-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleAdd')
				}, [ _('Add Subscription') ]),
				E('button', {
					'class': 'cbi-button cbi-button-negative',
					'click': ui.createHandlerFn(this, 'handleRemoveAll'),
					'disabled': subscriptions.length === 0 ? 'disabled' : null
				}, [ _('Remove All') ])
			])
		]));

		if (subscriptions.length === 0) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('p', {}, [ _('No subscriptions imported yet.') ])
			]));
			return content;
		}

		for (var i = 0; i < presentation.groups.length; i++)
			content.push(this.renderProviderGroup(presentation.groups[i], activeSubscriptionId, activeNodeId));

		return content;
	},

	render: function(data) {
		return E('div', { 'id': 'routeflux-subscriptions-root' }, this.renderPageContent(data));
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
