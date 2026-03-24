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

function nodeDisplayName(node, fallback) {
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

function hasItems(values) {
	return Array.isArray(values) && values.length > 0;
}

function firewallMode(settings) {
	var hasSources = hasItems(settings.source_cidrs);
	var hasTargets = hasItems(settings.target_cidrs);

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

	if (mode === 'targets' && hasItems(settings.target_cidrs))
		return settings.target_cidrs.join('\n');

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

	renderCard: function(label, value) {
		return E('div', { 'class': 'routeflux-card' }, [
			E('div', { 'class': 'routeflux-card-label' }, [ label ]),
			E('div', { 'class': 'routeflux-card-value' }, [ value || '-' ])
		]);
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
			textarea.placeholder = _('Examples: 1.1.1.1 8.8.8.8/32 203.0.113.10-203.0.113.20');
			textarea.disabled = false;
			row.style.opacity = '1';
			help.textContent = _('Route only selected destination IPv4 addresses, CIDRs, or ranges through RouteFlux.');
			blockHelp.textContent = _('This value is stored now and becomes effective when you switch back to host mode.');
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
						: _('Enter at least one destination IPv4 address, CIDR, or range.')
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
		var explainText = data[2] && data[2].__error__ ? '' : trim(data[2]);
		var connected = !!(status.state && status.state.connected === true);
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var currentMode = firewallMode(firewall);
		var selectedMode = formMode(firewall);
		var selectorsText = selectorsForMode(firewall, selectedMode);
		var activeProfile = firstNonEmpty([
			activeSubscription.display_name,
			activeSubscription.provider_name
		], _('Not selected'));
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));
		var content = [];

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Firewall error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__)
			ui.addNotification(null, notificationParagraph(_('Firewall help error: %s').format(data[2].__error__)));

		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid var(--border-color-medium, #d9d9d9); border-radius:6px; padding:12px 14px; background:var(--background-color-primary, #fff); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #666); font-size:12px; margin-bottom:4px; text-transform:uppercase; letter-spacing:.04em; }',
			'.routeflux-card-value { font-size:16px; font-weight:600; word-break:break-word; }',
			'.routeflux-firewall-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; margin-bottom:12px; }',
			'.routeflux-firewall-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-firewall-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-firewall-help { white-space:pre-wrap; margin:0; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Firewall') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage transparent routing for selected LAN hosts or destination IPv4 targets without leaving LuCI.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected')),
			this.renderCard(_('Mode'), modeSummary(currentMode)),
			this.renderCard(_('Transparent Port'), String(firewall.transparent_port || 12345)),
			this.renderCard(_('Block QUIC'), firewall.block_quic === true ? _('Enabled') : _('Disabled')),
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
						_('Hosts routes all TCP traffic from selected LAN clients. Targets routes only selected destination IPv4 addresses or ranges.')
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
