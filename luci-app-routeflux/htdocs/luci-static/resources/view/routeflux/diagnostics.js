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
			[ _('Zapret Service'), files.zapret_service ],
			[ _('Zapret Hostlist'), files.zapret_hostlist ],
			[ _('Zapret Marker'), files.zapret_marker ],
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
		var zapret = diagnostics.zapret || {};
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
		var warningRows = [];
		var quickFacts = [
			_('Mode: %s').format(firstNonEmpty([ state.mode ], _('disconnected'))),
			_('Provider: %s').format(activeProvider),
			_('Profile: %s').format(activeProfile),
			_('Node: %s').format(activeNodeName),
			_('Last success: %s').format(routefluxUI.formatTimestamp(state.last_success_at) || _('Never')),
			_('Last switch: %s').format(routefluxUI.formatTimestamp(state.last_switch_at) || _('Never'))
		];

		if (diagnostics.__error__)
			ui.addNotification(null, notificationParagraph(_('Diagnostics error: %s').format(diagnostics.__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'#routeflux-diagnostics-root { --routeflux-diagnostics-ink:#14283f; --routeflux-diagnostics-ink-muted:#506273; --routeflux-diagnostics-panel-bg:linear-gradient(160deg, rgba(245, 248, 252, 0.98) 0%, rgba(236, 242, 248, 0.98) 55%, rgba(229, 237, 245, 0.98) 100%); --routeflux-diagnostics-surface-bg:linear-gradient(180deg, rgba(255, 255, 255, 0.97) 0%, rgba(247, 250, 253, 0.97) 100%); --routeflux-diagnostics-surface-strong:linear-gradient(180deg, #1d3a56 0%, #13293f 100%); }',
			'.routeflux-diagnostics-layout { display:grid; gap:14px; color:var(--routeflux-diagnostics-ink); }',
			'.routeflux-diagnostics-panel { position:relative; overflow:hidden; border:1px solid rgba(132, 149, 170, 0.34); border-radius:20px; padding:18px 20px; background:var(--routeflux-diagnostics-panel-bg); box-shadow:0 16px 34px rgba(15, 23, 42, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.82); }',
			'.routeflux-diagnostics-panel h3 { margin:0 0 10px; color:var(--routeflux-diagnostics-ink); font-size:24px; letter-spacing:-0.03em; }',
			'.routeflux-diagnostics-panel .cbi-section-descr { margin:0; color:var(--routeflux-diagnostics-ink-muted); line-height:1.58; max-width:72ch; }',
			'.routeflux-diagnostics-summary-shell { padding:16px 18px; border:1px solid rgba(125, 145, 168, 0.32); border-radius:16px; background:var(--routeflux-diagnostics-surface-bg); box-shadow:0 10px 24px rgba(15, 23, 42, 0.08); }',
			'.routeflux-diagnostics-summary-shell h4 { margin:0 0 10px; color:var(--routeflux-diagnostics-ink); font-size:19px; letter-spacing:-0.02em; }',
			'.routeflux-diagnostics-summary-list { margin:0; padding-left:18px; color:var(--routeflux-diagnostics-ink-muted); line-height:1.6; }',
			'.routeflux-diagnostics-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-diagnostics-actions .cbi-button { min-height:48px; padding:0 18px; border:1px solid rgba(56, 189, 248, 0.4); border-radius:15px; background:var(--routeflux-diagnostics-surface-strong); color:#eef8ff; font-weight:800; text-shadow:0 1px 0 rgba(0, 0, 0, 0.2); box-shadow:0 14px 28px rgba(3, 15, 32, 0.18); }',
			'.routeflux-diagnostics-actions .cbi-button:hover { border-color:rgba(96, 165, 250, 0.66); background:linear-gradient(180deg, #244665 0%, #17324a 100%); color:#ffffff; }',
			'.routeflux-diagnostics-warning-actions { display:flex; flex-wrap:wrap; gap:10px; margin-top:12px; }',
			'.routeflux-diagnostics-advanced summary { cursor:pointer; font-weight:800; color:var(--routeflux-diagnostics-ink); }',
			'.routeflux-diagnostics-advanced-shell { margin-top:12px; }',
			'.routeflux-diagnostics-advanced-grid { display:grid; gap:12px; margin-top:12px; }',
			'@media (max-width: 720px) { .routeflux-diagnostics-actions .cbi-button { width:100%; } }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Diagnostics') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('A calmer snapshot of the current RouteFlux runtime. Start here for the health of the connection, and open advanced details only when you need low-level system checks.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), state.connected === true ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(state.connected === true),
				'primary': true
			}),
			this.renderCard(_('Transport'), firstNonEmpty([ status.active_transport ], _('direct'))),
			this.renderCard(_('Backend'), backendLabel(runtime, diagnostics.runtime_error)),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Node'), activeNodeName),
			this.renderCard(_('Zapret'), zapret.test_active === true
				? _('Manual test active')
				: (zapret.active === true
					? _('Active in RouteFlux')
					: (zapret.service_active === true ? _('Service running outside RouteFlux') : _('Inactive')))),
			this.renderCard(_('IPv6'), ipv6.fail_state === true ? _('Needs attention') : ipv6StateLabel(ipv6), {
				'tone': ipv6.fail_state === true ? 'disconnected' : 'connected'
			})
		]));

		if (trim(diagnostics.runtime_error) !== '')
			warningRows.push(_('Backend status error: %s').format(diagnostics.runtime_error));

		if (trim(state.last_failure_reason) !== '')
			warningRows.push(_('Last failure: %s').format(state.last_failure_reason));

		if (trim(zapret.last_reason) !== '' && zapret.test_active !== true)
			warningRows.push(_('Zapret status: %s').format(zapret.last_reason));

		if (warningRows.length > 0) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, warningRows.map(function(message) {
					return E('div', {}, [ message ]);
				}))
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

		content.push(E('div', { 'class': 'cbi-section routeflux-diagnostics-layout' }, [
			E('div', { 'class': 'routeflux-diagnostics-panel' }, [
				E('h3', {}, [ _('Quick status') ]),
				E('p', { 'class': 'cbi-section-descr' }, [
					_('This is the shortest useful summary of the current runtime. If everything looks healthy here, you usually do not need the advanced sections below.')
				]),
				E('div', { 'class': 'routeflux-diagnostics-summary-shell', 'style': 'margin-top:16px;' }, [
					E('h4', {}, [ _('Current route snapshot') ]),
					E('ul', { 'class': 'routeflux-diagnostics-summary-list' }, quickFacts.map(function(line) {
						return E('li', {}, [ line ]);
					}))
				]),
				E('div', { 'class': 'routeflux-diagnostics-actions', 'style': 'margin-top:16px;' }, [
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleRefreshPage')
					}, [ _('Refresh') ])
				])
			]),
			E('div', { 'class': 'routeflux-diagnostics-panel' }, [
				E('h3', {}, [ _('Advanced details') ]),
				E('p', { 'class': 'cbi-section-descr' }, [
					_('Open these sections only when you are debugging a specific failure or checking how RouteFlux interacts with the router.')
				]),
				E('div', { 'class': 'routeflux-diagnostics-advanced-shell' }, [
					E('details', { 'class': 'routeflux-diagnostics-advanced' }, [
						E('summary', {}, [ _('Zapret and backend details') ]),
						E('div', { 'class': 'routeflux-diagnostics-advanced-grid' }, [
							E('div', { 'class': 'routeflux-overview-grid' }, [
								this.renderCard(_('Service State'), firstNonEmpty([ runtime.service_state ], _('unknown'))),
								this.renderCard(_('Backend Config'), firstNonEmpty([ runtime.config_path ], _('Not configured'))),
								this.renderCard(_('Zapret Service'), firstNonEmpty([ zapret.service_state ], _('not-installed'))),
								this.renderCard(_('Zapret Service Owner'), zapret.managed === true ? _('RouteFlux') : _('External or inactive')),
								this.renderCard(_('Zapret Installed'), zapret.installed === true ? _('Yes') : _('No')),
								this.renderCard(_('Zapret Test'), zapret.test_active === true ? _('Active') : _('Inactive'))
							]),
							E('div', { 'class': 'routeflux-diagnostics-summary-shell' }, [
								E('h4', {}, [ _('Low-level runtime notes') ]),
								E('ul', { 'class': 'routeflux-diagnostics-summary-list' }, [
									E('li', {}, [ _('Backend config path: %s').format(firstNonEmpty([ runtime.config_path ], '-')) ]),
									E('li', {}, [ _('RouteFlux service state: %s').format(firstNonEmpty([ runtime.service_state ], _('unknown'))) ]),
									E('li', {}, [ _('Zapret service state: %s').format(firstNonEmpty([ zapret.service_state ], '-')) ]),
									E('li', {}, [ _('Zapret service owner: %s').format(zapret.managed === true ? _('RouteFlux') : _('External or inactive')) ])
								])
							])
						])
					]),
					E('details', { 'class': 'routeflux-diagnostics-advanced' }, [
						E('summary', {}, [ _('IPv6 details') ]),
						E('div', { 'class': 'routeflux-diagnostics-advanced-grid' }, [
							E('div', { 'class': 'routeflux-overview-grid' }, [
								this.renderCard(_('Configured by RouteFlux'), ipv6.configured_disabled === true ? _('Disable IPv6') : _('Leave Enabled')),
								this.renderCard(_('Persistent State'), ipv6.persistent_disabled === true ? _('Disabled') : _('Enabled')),
								this.renderCard(_('Enabled Interfaces'), ipv6EnabledInterfacesLabel(ipv6)),
								this.renderCard(_('Config Path'), firstNonEmpty([ ipv6.config_path ], '-'))
							])
						])
					]),
					E('details', { 'class': 'routeflux-diagnostics-advanced' }, [
						E('summary', {}, [ _('File checks') ]),
						E('div', { 'class': 'routeflux-diagnostics-advanced-grid' }, [
							this.renderFileTable(files)
						])
					])
				])
			])
		]));

		return E('div', { 'id': 'routeflux-diagnostics-root' }, content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
