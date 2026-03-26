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

function stringifyJSON(value) {
	try {
		return JSON.stringify(value, null, 2);
	}
	catch (err) {
		return String(value);
	}
}

function formatMbps(value) {
	var parsed = Number(value);

	if (!isFinite(parsed))
		return '-';

	return parsed.toFixed(2) + ' Mbps';
}

function formatMilliseconds(value) {
	var parsed = Number(value);

	if (!isFinite(parsed))
		return '-';

	return parsed.toFixed(2) + ' ms';
}

function formatBytes(value) {
	var parsed = Number(value);
	var units = [ 'B', 'KB', 'MB', 'GB' ];
	var unit = 0;

	if (!isFinite(parsed) || parsed < 0)
		return '-';

	while (parsed >= 1024 && unit < units.length - 1) {
		parsed /= 1024;
		unit++;
	}

	return parsed.toFixed(unit === 0 ? 0 : 2) + ' ' + units[unit];
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

	copyText: function(value) {
		var text = String(value == null ? '' : value);

		if (navigator.clipboard && navigator.clipboard.writeText)
			return navigator.clipboard.writeText(text);

		return new Promise(function(resolve, reject) {
			var input = document.createElement('textarea');
			input.value = text;
			input.setAttribute('readonly', 'readonly');
			input.style.position = 'fixed';
			input.style.left = '-9999px';
			document.body.appendChild(input);
			input.focus();
			input.select();

			try {
				document.execCommand('copy');
				resolve();
			}
			catch (err) {
				reject(err);
			}
			finally {
				document.body.removeChild(input);
			}
		});
	},

	downloadJSON: function(filename, value) {
		var blob = new Blob([ stringifyJSON(value) + '\n' ], { 'type': 'application/json;charset=utf-8' });
		var href = URL.createObjectURL(blob);
		var link = document.createElement('a');

		link.href = href;
		link.download = filename;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		setTimeout(function() { URL.revokeObjectURL(href); }, 0);
	},

	showModal: function(title, body, actions, bodyClass) {
		var buttons = Array.isArray(actions) ? actions.slice() : [];
		var className = 'routeflux-modal-body';
		var modalClass = trim(bodyClass);

		if (modalClass !== '')
			className += ' ' + modalClass;

		if (buttons.length === 0) {
			buttons.push(E('button', {
				'class': 'cbi-button',
				'click': function(ev) {
					ui.hideModal();
					return false;
				}
			}, [ _('Close') ]));
		}

		if (modalClass !== '') {
			ui.showModal(title, [
				E('div', { 'class': className }, Array.isArray(body) ? body : [ body ]),
				E('div', { 'class': 'routeflux-modal-actions' }, buttons)
			], modalClass);
			return;
		}

		ui.showModal(title, [
			E('div', { 'class': className }, Array.isArray(body) ? body : [ body ]),
			E('div', { 'class': 'routeflux-modal-actions' }, buttons)
		]);
	},

	showJSONModal: function(subscriptionId, node, payload) {
		var nodeName = nodeDisplayName(node, node && node.id);
		var content = stringifyJSON(payload);
		var filename = 'routeflux-' + subscriptionId + '-' + trim(node && node.id) + '.json';

		this.showModal(
			_('Generated Xray JSON: %s').format(nodeName),
			[
				E('p', { 'class': 'routeflux-modal-help' }, [
					_('This preview shows the full RouteFlux-generated Xray config for the selected node with the current app settings.')
				]),
				E('pre', { 'class': 'routeflux-inspect-pre' }, [ content ])
			],
			[
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, function() {
						return this.copyText(content).then(function() {
							ui.addNotification(null, notificationParagraph(_('JSON copied to clipboard.')), 'info');
						}).catch(function(err) {
							ui.addNotification(null, notificationParagraph(err.message || String(err)));
						});
					})
				}, [ _('Copy') ]),
				E('button', {
					'class': 'cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, function() {
						this.downloadJSON(filename, payload);
						ui.addNotification(null, notificationParagraph(_('JSON file downloaded.')), 'info');
					})
				}, [ _('Download .json') ]),
				E('button', {
					'class': 'cbi-button',
					'click': function(ev) {
						ui.hideModal();
						return false;
					}
				}, [ _('Close') ])
			],
			'routeflux-modal-json'
		);
	},

	showSpeedTestModal: function(node, state) {
		var nodeName = nodeDisplayName(node, node && node.id);
		var body = [
			E('p', { 'class': 'routeflux-modal-help' }, [
				_('This is a router-side diagnostic. It measures the selected node from the router through a temporary isolated Xray process and does not change the active RouteFlux connection.')
			])
		];

		if (state === 'loading') {
			body.push(E('p', { 'class': 'routeflux-speedtest-status' }, [
				_('Running speed test. This can take a few seconds.')
			]));
		}
		else if (state && state.error) {
			body.push(E('div', { 'class': 'alert-message warning' }, [ state.error ]));
		}
		else if (state && state.result) {
			body.push(E('div', { 'class': 'routeflux-speedtest-grid' }, [
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Ping') ]),
					E('div', { 'class': 'routeflux-speedtest-value' }, [ formatMilliseconds(state.result.latency_ms) ])
				]),
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Download') ]),
					E('div', { 'class': 'routeflux-speedtest-value' }, [ formatMbps(state.result.download_mbps) ])
				]),
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Upload') ]),
					E('div', { 'class': 'routeflux-speedtest-value' }, [ formatMbps(state.result.upload_mbps) ])
				]),
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Downloaded') ]),
					E('div', { 'class': 'routeflux-speedtest-value routeflux-speedtest-subtle' }, [ formatBytes(state.result.download_bytes) ])
				]),
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Uploaded') ]),
					E('div', { 'class': 'routeflux-speedtest-value routeflux-speedtest-subtle' }, [ formatBytes(state.result.upload_bytes) ])
				]),
				E('div', { 'class': 'routeflux-speedtest-metric' }, [
					E('div', { 'class': 'routeflux-speedtest-label' }, [ _('Finished') ]),
					E('div', { 'class': 'routeflux-speedtest-value routeflux-speedtest-subtle' }, [ routefluxUI.formatTimestamp(state.result.finished_at) || '-' ])
				])
			]));
		}

		this.showModal(_('Speed Test: %s').format(nodeName), body, null, 'routeflux-modal-speedtest');
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

	handleInspectJSON: function(subscriptionId, node, ev) {
		return this.execJSON([
			'--json', 'inspect', 'xray',
			'--subscription', subscriptionId,
			'--node', trim(node && node.id)
		]).then(L.bind(function(payload) {
			this.showJSONModal(subscriptionId, node, payload);
		}, this)).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleSpeedTest: function(subscriptionId, node, ev) {
		if (this.speedTestBusy === true) {
			ui.addNotification(null, notificationParagraph(_('Another speed test is already running.')));
			return Promise.resolve();
		}

		this.speedTestBusy = true;
		this.showSpeedTestModal(node, 'loading');
		this.refreshPageContent();

		return this.execJSON([
			'--json', 'inspect', 'speed',
			'--subscription', subscriptionId,
			'--node', trim(node && node.id)
		]).then(L.bind(function(result) {
			this.showSpeedTestModal(node, { 'result': result });
			return result;
		}, this)).catch(L.bind(function(err) {
			this.showSpeedTestModal(node, { 'error': err.message || String(err) });
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		}, this)).finally(L.bind(function() {
			this.speedTestBusy = false;
			this.refreshPageContent();
		}, this));
	},

	renderNodeTable: function(subscription, activeSubscriptionId, activeNodeId) {
		var nodes = Array.isArray(subscription.nodes) ? subscription.nodes : [];
		var speedTestBusy = this.speedTestBusy === true;

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
					E('div', { 'class': 'routeflux-node-actions' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(this, 'handleConnectNode', subscription.id, node.id)
						}, [ _('Connect') ]),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(this, 'handleInspectJSON', subscription.id, node)
						}, [ _('JSON') ]),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(this, 'handleSpeedTest', subscription.id, node),
							'disabled': speedTestBusy ? 'disabled' : null
						}, [ _('Speed Test') ])
					])
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
			'.routeflux-node-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; }',
			'.routeflux-add-grid { display:grid; grid-template-columns:minmax(220px, 320px) 1fr; gap:12px; margin-bottom:12px; }',
			'.routeflux-add-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-add-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-node-details { margin-top:12px; }',
			'.routeflux-node-details summary { cursor:pointer; margin-bottom:10px; }',
			'.routeflux-provider-group { margin-bottom:22px; }',
			'.routeflux-provider-group-header { display:flex; flex-wrap:wrap; justify-content:space-between; gap:12px; align-items:baseline; margin:12px 0 8px; }',
			'.routeflux-provider-group-title { font-size:22px; font-weight:600; }',
			'.routeflux-provider-group-meta { color:var(--text-color-secondary, #666); }',
			'.routeflux-modal-body { width:100%; max-width:100%; min-width:0; box-sizing:border-box; overflow:hidden; }',
			'.modal.routeflux-modal-json { width:min(92vw, 980px); max-width:92vw; }',
			'.modal.routeflux-modal-speedtest { width:min(92vw, 720px); max-width:92vw; overflow:hidden; }',
			'.routeflux-modal-help { margin:0 0 12px; color:var(--text-color-secondary, #586677); max-width:100%; overflow-wrap:anywhere; word-break:break-word; line-height:1.45; }',
			'.routeflux-modal-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; margin-top:14px; }',
			'.routeflux-inspect-pre { white-space:pre-wrap; margin:0; max-height:56vh; max-width:100%; overflow:auto; padding:14px 16px; border:1px solid rgba(71, 85, 105, 0.82); border-radius:12px; background:linear-gradient(180deg, #09111d 0%, #0d1623 48%, #101a29 100%); color:#eef4fb; font-family:SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size:13px; line-height:1.72; box-sizing:border-box; }',
			'.routeflux-speedtest-status { margin:0; font-weight:600; }',
			'.routeflux-speedtest-grid { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:10px; margin-top:12px; max-width:100%; min-width:0; align-items:stretch; }',
			'.routeflux-speedtest-metric { min-width:0; width:100%; border:1px solid rgba(98, 112, 129, 0.34); border-radius:12px; padding:10px 12px; background:linear-gradient(180deg, rgba(243, 247, 251, 0.98) 0%, rgba(233, 238, 244, 0.98) 100%); box-sizing:border-box; }',
			'.routeflux-speedtest-label { color:var(--text-color-secondary, #586677); font-size:10px; text-transform:uppercase; letter-spacing:.12em; font-weight:700; margin-bottom:4px; }',
			'.routeflux-speedtest-value { color:var(--text-color-primary, #17263a); font-size:15px; font-weight:700; line-height:1.3; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-speedtest-subtle { font-size:14px; }',
			'@media (max-width: 640px) { .routeflux-speedtest-grid { grid-template-columns:minmax(0, 1fr); } }'
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
