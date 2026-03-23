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
			var name = firstNonEmpty([ node.name, node.remark, node.id ], node.id);
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

	renderSubscriptionCard: function(subscription, activeSubscriptionId, activeNodeId) {
		var displayName = firstNonEmpty([
			subscription.display_name,
			subscription.provider_name,
			subscription.id
		], subscription.id);
		var isActive = subscription.id === activeSubscriptionId;
		var nodesCount = Array.isArray(subscription.nodes) ? subscription.nodes.length : 0;
		var metaRows = [
			[ _('ID'), subscription.id ],
			[ _('Provider'), firstNonEmpty([ subscription.provider_name ], '-') ],
			[ _('Source Type'), firstNonEmpty([ subscription.source_type ], '-') ],
			[ _('Updated'), firstNonEmpty([ subscription.last_updated_at ], _('Never')) ],
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

	render: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
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
			'.routeflux-subscription-actions { display:flex; flex-wrap:wrap; gap:8px; align-items:flex-start; }',
			'.routeflux-add-grid { display:grid; grid-template-columns:minmax(220px, 320px) 1fr; gap:12px; margin-bottom:12px; }',
			'.routeflux-add-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-node-details { margin-top:12px; }',
			'.routeflux-node-details summary { cursor:pointer; margin-bottom:10px; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Subscriptions') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage imported subscriptions, add new providers, refresh existing profiles, and connect a specific node or the best node automatically.')
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
							'placeholder': _('Example: Liberty VPN')
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
			E('button', {
				'class': 'cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(this, 'handleAdd')
			}, [ _('Add Subscription') ])
		]));

		if (subscriptions.length === 0) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('p', {}, [ _('No subscriptions imported yet.') ])
			]));
			return E(content);
		}

		for (var i = 0; i < subscriptions.length; i++)
			content.push(this.renderSubscriptionCard(subscriptions[i], activeSubscriptionId, activeNodeId));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
