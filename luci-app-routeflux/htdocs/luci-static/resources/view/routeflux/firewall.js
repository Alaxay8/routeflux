'use strict';
'require view';
'require fs';
'require dom';
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

function cleanList(values) {
	var seen = {};
	var out = [];
	var list = Array.isArray(values) ? values : [];

	for (var i = 0; i < list.length; i++) {
		var value = trim(list[i]);

		if (value === '' || seen[value])
			continue;

		seen[value] = true;
		out.push(value);
	}

	return out;
}

function emptySelectorSet() {
	return {
		'services': [],
		'domains': [],
		'cidrs': []
	};
}

function cloneSelectorSet(value) {
	var selectors = value || {};

	return {
		'services': cleanList(selectors.services || []),
		'domains': cleanList(selectors.domains || []),
		'cidrs': cleanList(selectors.cidrs || [])
	};
}

function selectorValues(selectors) {
	var value = cloneSelectorSet(selectors);

	return value.services.concat(value.domains).concat(value.cidrs);
}

function selectorSetHasEntries(selectors) {
	var value = cloneSelectorSet(selectors);

	return value.services.length > 0 || value.domains.length > 0 || value.cidrs.length > 0;
}

function emptySelectorEditor() {
	return {
		'services': [],
		'domains': [],
		'cidrs': [],
		'serviceChoice': '',
		'selectorInput': ''
	};
}

function selectorEditorFromSet(value) {
	var selectors = cloneSelectorSet(value);

	return {
		'services': selectors.services,
		'domains': selectors.domains,
		'cidrs': selectors.cidrs,
		'serviceChoice': '',
		'selectorInput': ''
	};
}

function selectorSetFromEditor(editor) {
	return {
		'services': cleanList((editor || {}).services || []),
		'domains': cleanList((editor || {}).domains || []),
		'cidrs': cleanList((editor || {}).cidrs || [])
	};
}

function listEditorFromEntries(entries) {
	return {
		'entries': cleanList(entries || []),
		'input': ''
	};
}

function listValues(editor) {
	return cleanList((editor || {}).entries || []);
}

function isIPv4Selector(value) {
	var normalized = trim(value);

	return /^(\d{1,3}\.){3}\d{1,3}$/.test(normalized) ||
		/^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/.test(normalized) ||
		/^(\d{1,3}\.){3}\d{1,3}\s*-\s*(\d{1,3}\.){3}\d{1,3}$/.test(normalized);
}

function normalizeDomainSelector(value) {
	return trim(value).toLowerCase();
}

function normalizeSourceSelector(value) {
	var normalized = trim(value).toLowerCase();

	if (normalized === '*')
		return 'all';

	return normalized;
}

function legacyTargetMode(settings) {
	return trim(settings.target_mode) === 'bypass' ? 'bypass' : 'proxy';
}

function legacyFirewallMode(settings) {
	var hasSources = hasItems(settings.source_cidrs);
	var hasTargets = hasItems(settings.target_services) || hasItems(settings.target_domains) || hasItems(settings.target_cidrs);

	if (settings.enabled !== true || (!hasSources && !hasTargets))
		return 'disabled';

	if (hasSources)
		return 'hosts';

	if (legacyTargetMode(settings) === 'bypass')
		return 'split';

	return 'targets';
}

function normalizedSelectorSet(raw, legacy) {
	return {
		'services': cleanList((raw && raw.services) || (legacy && legacy.target_services) || []),
		'domains': cleanList((raw && raw.domains) || (legacy && legacy.target_domains) || []),
		'cidrs': cleanList((raw && raw.cidrs) || (legacy && legacy.target_cidrs) || [])
	};
}

function normalizedSplitSettings(firewall, mode) {
	var split = firewall.split || {};
	var legacyMode = legacyTargetMode(firewall);
	var useLegacyBypass = !selectorSetHasEntries(split.bypass) && !selectorSetHasEntries(split.proxy) && mode === 'split' && legacyMode === 'bypass';

	return {
		'proxy': normalizedSelectorSet(split.proxy || {}, null),
		'bypass': useLegacyBypass ? normalizedSelectorSet(null, firewall) : normalizedSelectorSet(split.bypass || {}, null),
		'excluded_sources': cleanList(split.excluded_sources || []),
		'default_action': trim(split.default_action) !== '' ? trim(split.default_action) : (useLegacyBypass ? 'proxy' : 'direct')
	};
}

function splitLooksLikeBypass(split) {
	return trim((split || {}).default_action) === 'proxy' && !selectorSetHasEntries((split || {}).proxy);
}

function splitLooksLikeTargets(split) {
	return trim((split || {}).default_action) === 'direct' &&
		selectorSetHasEntries((split || {}).proxy) &&
		!selectorSetHasEntries((split || {}).bypass) &&
		cleanList((split || {}).excluded_sources || []).length === 0;
}

function canonicalFirewall(firewall) {
	var raw = firewall || {};
	var mode = trim(raw.mode);
	var enabled = raw.enabled === true;
	var split;
	var targets;
	var compatibilityWarning = '';

	if (mode !== 'hosts' && mode !== 'targets' && mode !== 'split' && mode !== 'disabled')
		mode = legacyFirewallMode(raw);

	if (!enabled && mode !== 'hosts' && mode !== 'targets' && mode !== 'split')
		mode = 'disabled';

	split = normalizedSplitSettings(raw, mode);
	targets = normalizedSelectorSet(raw.targets || {}, mode === 'targets' ? raw : null);

	if (enabled && mode === 'split') {
		if (splitLooksLikeBypass(split))
			mode = 'bypass';
		else if (splitLooksLikeTargets(split)) {
			mode = 'targets';
			targets = cloneSelectorSet(split.proxy);
		}
		else
			compatibilityWarning = _('The current firewall config uses advanced split tunnelling created outside LuCI. Choose Targets, Bypass, Hosts, or Disabled to replace it.');
	}

	return {
		'enabled': enabled,
		'mode': compatibilityWarning !== '' ? 'advanced-split' : mode,
		'transparent_port': Number(raw.transparent_port) > 0 ? Number(raw.transparent_port) : 12345,
		'block_quic': raw.block_quic === true,
		'hosts': cleanList(raw.hosts || raw.source_cidrs || []),
		'targets': targets,
		'bypass': {
			'selectors': cloneSelectorSet(split.bypass),
			'excluded_sources': cleanList(split.excluded_sources || [])
		},
		'split': split,
		'mode_drafts': raw.mode_drafts || {},
		'compatibility_warning': compatibilityWarning
	};
}

function selectorSetFromDraft(rawDraft) {
	return normalizedSelectorSet(null, rawDraft || {});
}

function buildFormState(firewall) {
	var current = canonicalFirewall(firewall);
	var drafts = current.mode_drafts || {};
	var hosts = listEditorFromEntries((drafts.hosts || {}).source_cidrs || []);
	var targets = selectorEditorFromSet(selectorSetFromDraft(drafts.targets || {}));
	var bypass = {
		'selectors': selectorEditorFromSet(((drafts.split || {}).bypass) || emptySelectorSet()),
		'excluded': listEditorFromEntries(((drafts.split || {}).excluded_sources) || [])
	};

	if (current.mode === 'hosts')
		hosts = listEditorFromEntries(current.hosts);
	else if (current.mode === 'targets')
		targets = selectorEditorFromSet(current.targets);
	else if (current.mode === 'bypass') {
		bypass.selectors = selectorEditorFromSet(current.bypass.selectors);
		bypass.excluded = listEditorFromEntries(current.bypass.excluded_sources);
	}
	else if (current.mode === 'advanced-split') {
		targets = selectorEditorFromSet(current.split.proxy);
		bypass.selectors = selectorEditorFromSet(current.bypass.selectors);
		bypass.excluded = listEditorFromEntries(current.bypass.excluded_sources);
	}

	return {
		'mode': current.mode || 'disabled',
		'port': String(current.transparent_port),
		'block_quic': current.block_quic === true,
		'hosts': hosts,
		'targets': targets,
		'bypass': bypass,
		'compatibility_warning': current.compatibility_warning || ''
	};
}

function modeSummary(mode) {
	switch (mode) {
	case 'hosts':
		return _('Hosts');
	case 'targets':
		return _('Targets');
	case 'bypass':
		return _('Bypass');
	case 'advanced-split':
		return _('Advanced Split');
	default:
		return _('Disabled');
	}
}

function editorByKey(view, key) {
	if (!view.formState)
		return null;

	switch (key) {
	case 'targets':
		return view.formState.targets;
	case 'bypass':
		return view.formState.bypass.selectors;
	default:
		return null;
	}
}

function listByKey(view, key) {
	if (!view.formState)
		return null;

	switch (key) {
	case 'hosts':
		return view.formState.hosts;
	case 'bypass-excluded':
		return view.formState.bypass.excluded;
	default:
		return null;
	}
}

function appendStringSliceFlags(argv, flag, values) {
	var list = cleanList(values || []);

	for (var i = 0; i < list.length; i++) {
		argv.push(flag);
		argv.push(list[i]);
	}

	return argv;
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execJSON([ '--json', 'firewall', 'get' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execJSON([ '--json', 'list', 'subscriptions' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execText([ 'firewall', 'explain' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execJSON([ '--json', 'services', 'list' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
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

	initializePageState: function(data) {
		var status = data[0] || {};
		var firewallPayload = data[1] && !data[1].__error__
			? data[1]
			: ((status.settings || {}).firewall || {});
		var servicesPayload = Array.isArray(data[4]) ? data[4] : [];

		this.pageData = {
			'status': status,
			'firewall': canonicalFirewall(firewallPayload),
			'subscriptions': Array.isArray(data[2]) ? data[2] : [],
			'explainText': data[3] && data[3].__error__ ? '' : trim(data[3]),
			'services': servicesPayload
		};
		this.formState = buildFormState(firewallPayload);
		this.rootErrors = {
			'status': trim(data[0] && data[0].__error__),
			'firewall': trim(data[1] && data[1].__error__),
			'subscriptions': trim(data[2] && data[2].__error__),
			'explain': trim(data[3] && data[3].__error__),
			'services': trim(data[4] && data[4].__error__)
		};
	},

	renderIntoRoot: function() {
		var root = document.querySelector('#routeflux-firewall-root');

		if (root)
			dom.content(root, this.renderPageContent());
	},

	handleModeChange: function(ev) {
		this.formState.mode = trim(ev.currentTarget.value) || 'disabled';
		this.renderIntoRoot();
	},

	handlePortInput: function(ev) {
		this.formState.port = trim(ev.currentTarget.value);
	},

	handleBlockQUICChange: function(ev) {
		this.formState.block_quic = ev.currentTarget.checked === true;
	},

	handleServiceChoiceChange: function(key, ev) {
		var editor = editorByKey(this, key);

		if (!editor)
			return;

		editor.serviceChoice = trim(ev.currentTarget.value);
	},

	handleSelectorInputChange: function(key, ev) {
		var editor = editorByKey(this, key);

		if (!editor)
			return;

		editor.selectorInput = ev.currentTarget.value;
	},

	handleListInputChange: function(key, ev) {
		var list = listByKey(this, key);

		if (!list)
			return;

		list.input = ev.currentTarget.value;
	},

	handleAddService: function(key, ev) {
		var editor = editorByKey(this, key);
		var value;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!editor)
			return;

		value = trim(editor.serviceChoice).toLowerCase();
		if (value === '')
			return;

		editor.services = cleanList(editor.services.concat([ value ]));
		editor.serviceChoice = '';
		this.renderIntoRoot();
	},

	handleAddSelector: function(key, ev) {
		var editor = editorByKey(this, key);
		var parts;
		var i;
		var value;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!editor)
			return;

		parts = parseList(editor.selectorInput);
		for (i = 0; i < parts.length; i++) {
			value = trim(parts[i]);
			if (value === '')
				continue;

			if (isIPv4Selector(value))
				editor.cidrs = cleanList(editor.cidrs.concat([ value ]));
			else
				editor.domains = cleanList(editor.domains.concat([ normalizeDomainSelector(value) ]));
		}

		editor.selectorInput = '';
		this.renderIntoRoot();
	},

	handleAddListEntry: function(key, ev) {
		var list = listByKey(this, key);
		var parts;
		var i;
		var value;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!list)
			return;

		parts = parseList(list.input);
		for (i = 0; i < parts.length; i++) {
			value = normalizeSourceSelector(parts[i]);
			if (value === '')
				continue;

			list.entries = cleanList(list.entries.concat([ value ]));
		}

		list.input = '';
		this.renderIntoRoot();
	},

	handleRemoveSelector: function(key, field, value, ev) {
		var editor = editorByKey(this, key);

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!editor || !Array.isArray(editor[field]))
			return;

		editor[field] = cleanList(editor[field].filter(function(entry) {
			return entry !== value;
		}));
		this.renderIntoRoot();
	},

	handleRemoveListEntry: function(key, value, ev) {
		var list = listByKey(this, key);

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!list)
			return;

		list.entries = cleanList(list.entries.filter(function(entry) {
			return entry !== value;
		}));
		this.renderIntoRoot();
	},

	draftCommands: function() {
		var commands = [];
		var bypassDraft = [ 'firewall', 'draft', 'bypass' ];

		commands.push([ 'firewall', 'draft', 'hosts' ].concat(listValues(this.formState.hosts)));
		commands.push([ 'firewall', 'draft', 'targets' ].concat(selectorValues(selectorSetFromEditor(this.formState.targets))));
		appendStringSliceFlags(bypassDraft, '--exclude-host', listValues(this.formState.bypass.excluded));
		commands.push(bypassDraft.concat(selectorValues(selectorSetFromEditor(this.formState.bypass.selectors))));

		return commands;
	},

	handleSaveSettings: function(ev) {
		var portRaw = trim(this.formState.port);
		var port = parseInt(portRaw, 10);
		var mode = trim(this.formState.mode);
		var commands = this.draftCommands();
		var targetSelectors = selectorValues(selectorSetFromEditor(this.formState.targets));
		var bypassSelectors = selectorValues(selectorSetFromEditor(this.formState.bypass.selectors));
		var bypassExcluded = listValues(this.formState.bypass.excluded);
		var bypassCommand = [ 'firewall', 'set', '--port', String(port), 'bypass' ];

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!/^\d+$/.test(portRaw) || isNaN(port) || port <= 0) {
			ui.addNotification(null, notificationParagraph(_('Transparent port must be a positive integer.')));
			return Promise.resolve();
		}

		if (mode === 'hosts' && listValues(this.formState.hosts).length === 0) {
			ui.addNotification(null, notificationParagraph(_('Enter at least one LAN device, CIDR, range, or all.')));
			return Promise.resolve();
		}

		if (mode === 'targets' && targetSelectors.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Add at least one service preset, domain, IPv4 address, CIDR, or range for Route Through RouteFlux.')));
			return Promise.resolve();
		}

		if (mode === 'bypass' && bypassSelectors.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Bypass needs at least one Keep Direct entry.')));
			return Promise.resolve();
		}

		if (mode === 'advanced-split') {
			ui.addNotification(null, notificationParagraph(_('Choose Targets, Bypass, Hosts, or Disabled to replace this advanced split tunnelling config.')));
			return Promise.resolve();
		}

		if (mode === 'hosts') {
			commands.push([ 'firewall', 'set', '--port', String(port), 'hosts' ].concat(listValues(this.formState.hosts)));
			commands.push([ 'firewall', 'set', 'block-quic', this.formState.block_quic ? 'true' : 'false' ]);
			return this.runCommands(commands, _('Firewall settings saved.'));
		}

		if (mode === 'targets') {
			commands.push([ 'firewall', 'set', '--port', String(port), 'targets' ].concat(targetSelectors));
			commands.push([ 'firewall', 'set', 'block-quic', this.formState.block_quic ? 'true' : 'false' ]);
			return this.runCommands(commands, _('Firewall settings saved.'));
		}

		if (mode === 'bypass') {
			appendStringSliceFlags(bypassCommand, '--exclude-host', bypassExcluded);
			commands.push(bypassCommand.concat(bypassSelectors));
			commands.push([ 'firewall', 'set', 'block-quic', this.formState.block_quic ? 'true' : 'false' ]);
			return this.runCommands(commands, _('Firewall settings saved.'));
		}

		commands.push([ 'firewall', 'disable' ]);
		commands.push([ 'firewall', 'set', 'port', String(port) ]);
		commands.push([ 'firewall', 'set', 'block-quic', this.formState.block_quic ? 'true' : 'false' ]);
		return this.runCommands(commands, _('Firewall settings saved and routing disabled.'));
	},

	handleDisable: function(ev) {
		var commands;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!window.confirm(_('Disable transparent routing?')))
			return Promise.resolve();

		commands = this.draftCommands();
		commands.push([ 'firewall', 'disable' ]);

		return this.runCommands(commands, _('Firewall disabled.'));
	},

	renderServiceOptions: function(selectedValue) {
		var services = Array.isArray(this.pageData.services) ? this.pageData.services : [];
		var options = [
			E('option', { 'value': '', 'selected': trim(selectedValue) === '' ? 'selected' : null }, [ _('Choose a service preset') ])
		];

		for (var i = 0; i < services.length; i++) {
			var entry = services[i];
			var suffix = entry.readonly === true ? _('Built-in') : _('Custom');
			var label = trim(entry.name) + ' \u00b7 ' + suffix;

			options.push(E('option', {
				'value': trim(entry.name),
				'selected': trim(selectedValue) === trim(entry.name) ? 'selected' : null
			}, [ label ]));
		}

		return options;
	},

	renderSelectorItems: function(key, editor) {
		var rows = [];
		var selectors = selectorSetFromEditor(editor);
		var i;

		for (i = 0; i < selectors.services.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-firewall-item' }, [
				E('span', { 'class': 'routeflux-firewall-badge' }, [ _('Preset') ]),
				E('span', { 'class': 'routeflux-firewall-item-value' }, [ selectors.services[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveSelector', key, 'services', selectors.services[i])
				}, [ _('Remove') ])
			]));
		}

		for (i = 0; i < selectors.domains.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-firewall-item' }, [
				E('span', { 'class': 'routeflux-firewall-badge routeflux-firewall-badge-domain' }, [ _('Domain') ]),
				E('span', { 'class': 'routeflux-firewall-item-value' }, [ selectors.domains[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveSelector', key, 'domains', selectors.domains[i])
				}, [ _('Remove') ])
			]));
		}

		for (i = 0; i < selectors.cidrs.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-firewall-item' }, [
				E('span', { 'class': 'routeflux-firewall-badge routeflux-firewall-badge-ip' }, [ _('IPv4') ]),
				E('span', { 'class': 'routeflux-firewall-item-value' }, [ selectors.cidrs[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveSelector', key, 'cidrs', selectors.cidrs[i])
				}, [ _('Remove') ])
			]));
		}

		if (rows.length === 0)
			return E('div', { 'class': 'routeflux-firewall-empty' }, [ _('Nothing added yet.') ]);

		return E('div', { 'class': 'routeflux-firewall-list' }, rows);
	},

	renderListItems: function(key, list, emptyLabel) {
		var values = listValues(list);
		var rows = [];

		for (var i = 0; i < values.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-firewall-item' }, [
				E('span', { 'class': 'routeflux-firewall-badge routeflux-firewall-badge-host' }, [ _('Host') ]),
				E('span', { 'class': 'routeflux-firewall-item-value' }, [ values[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveListEntry', key, values[i])
				}, [ _('Remove') ])
			]));
		}

		if (rows.length === 0)
			return E('div', { 'class': 'routeflux-firewall-empty' }, [ emptyLabel ]);

		return E('div', { 'class': 'routeflux-firewall-list' }, rows);
	},

	renderSelectorEditor: function(title, description, key, editor, placeholder, options) {
		var settings = options || {};
		var className = 'routeflux-firewall-editor';
		var descriptionClassName = 'cbi-value-description';
		var kicker = trim(settings.kicker);

		if (trim(settings.className) !== '')
			className += ' ' + trim(settings.className);

		if (trim(settings.descriptionClassName) !== '')
			descriptionClassName += ' ' + trim(settings.descriptionClassName);

		return E('div', { 'class': className }, [
			E('div', { 'class': 'routeflux-firewall-editor-head' }, [
				kicker !== '' ? E('span', { 'class': 'routeflux-firewall-editor-kicker' }, [ kicker ]) : null,
				E('h4', {}, [ title ]),
				E('p', { 'class': descriptionClassName }, [ description ])
			].filter(Boolean)),
			E('div', { 'class': 'routeflux-firewall-editor-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title' }, [ _('Service Preset') ]),
					E('div', { 'class': 'routeflux-firewall-inline' }, [
						E('select', {
							'class': 'cbi-input-select',
							'change': function(ev) {
								this.handleServiceChoiceChange(key, ev);
							}.bind(this)
						}, this.renderServiceOptions(editor.serviceChoice)),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleAddService', key)
						}, [ _('Add Preset') ])
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title' }, [ _('Extra Domain or IPv4') ]),
					E('div', { 'class': 'routeflux-firewall-inline' }, [
						E('input', {
							'class': 'cbi-input-text',
							'type': 'text',
							'placeholder': placeholder,
							'value': editor.selectorInput,
							'input': function(ev) {
								this.handleSelectorInputChange(key, ev);
							}.bind(this)
						}),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleAddSelector', key)
						}, [ _('Add Selector') ])
					])
				])
			]),
			this.renderSelectorItems(key, editor)
		]);
	},

	renderListEditor: function(title, description, key, list, placeholder, emptyLabel, options) {
		var settings = options || {};
		var className = 'routeflux-firewall-editor';
		var kicker = trim(settings.kicker);

		if (trim(settings.className) !== '')
			className += ' ' + trim(settings.className);

		return E('div', { 'class': className }, [
			E('div', { 'class': 'routeflux-firewall-editor-head' }, [
				kicker !== '' ? E('span', { 'class': 'routeflux-firewall-editor-kicker' }, [ kicker ]) : null,
				E('h4', {}, [ title ]),
				E('p', { 'class': 'cbi-value-description' }, [ description ])
			].filter(Boolean)),
			E('div', { 'class': 'cbi-value' }, [
				E('div', { 'class': 'routeflux-firewall-inline' }, [
					E('input', {
						'class': 'cbi-input-text',
						'type': 'text',
						'placeholder': placeholder,
						'value': list.input,
						'input': function(ev) {
							this.handleListInputChange(key, ev);
						}.bind(this)
					}),
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleAddListEntry', key)
					}, [ _('Add') ])
				])
			]),
			this.renderListItems(key, list, emptyLabel)
		]);
	},

	renderConfigurationPanels: function() {
		var mode = this.formState.mode;
		var panels = [];

		if (mode === 'hosts') {
			panels.push(this.renderListEditor(
				_('Devices Through RouteFlux'),
				_('Every destination from these LAN devices will be routed through RouteFlux.'),
				'hosts',
				this.formState.hosts,
				_('Examples: 192.168.1.150 192.168.1.0/24 192.168.1.150-192.168.1.159 all'),
				_('Add one or more LAN devices to start host-based routing.'),
				{
					'className': 'routeflux-firewall-editor-muted routeflux-firewall-editor-hosts',
					'kicker': _('Device matching')
				}
			));
		}

		if (mode === 'targets') {
			panels.push(this.renderSelectorEditor(
				_('Route Through RouteFlux'),
				_('Choose service presets plus any extra domains or IPv4 targets that should go through RouteFlux.'),
				'targets',
				this.formState.targets,
				_('Examples: youtube.com 1.1.1.1 203.0.113.10-203.0.113.20'),
				{
					'className': 'routeflux-firewall-editor-emphasis routeflux-firewall-editor-targets',
					'kicker': _('Proxy targets')
				}
			));
		}

		if (mode === 'bypass') {
			panels.push(this.renderSelectorEditor(
				_('Keep Direct'),
				_('These service presets, domains, and IPv4 targets stay direct while all other traffic keeps using RouteFlux.'),
				'bypass',
				this.formState.bypass.selectors,
				_('Examples: gosuslugi.ru 203.0.113.10 203.0.113.10-203.0.113.20'),
				{
					'className': 'routeflux-firewall-editor-emphasis routeflux-firewall-editor-bypass',
					'descriptionClassName': 'routeflux-firewall-editor-description-strong',
					'kicker': _('Direct exceptions')
				}
			));
			panels.push(this.renderListEditor(
				_('Excluded Devices'),
				_('These LAN devices are never intercepted by bypass mode.'),
				'bypass-excluded',
				this.formState.bypass.excluded,
				_('Examples: 192.168.1.50 192.168.1.0/24 192.168.1.10-192.168.1.20 all'),
				_('Excluded devices are optional.'),
				{
					'className': 'routeflux-firewall-editor-muted routeflux-firewall-editor-excluded',
					'kicker': _('LAN exclusions')
				}
			));
		}

		if (mode === 'advanced-split') {
			panels.push(E('div', { 'class': 'alert-message warning' }, [
				this.formState.compatibility_warning || _('The current firewall config uses advanced split tunnelling created outside LuCI. Choose a supported mode to replace it.')
			]));
		}

		if (mode === 'disabled') {
			panels.push(E('div', { 'class': 'routeflux-firewall-empty routeflux-firewall-disabled-note' }, [
				_('Firewall routing is disabled. Drafts for Hosts, Targets, and Bypass will still be saved when you click Save.')
			]));
		}

		return panels;
	},

	renderPageContent: function() {
		var status = this.pageData.status || {};
		var firewall = this.pageData.firewall || canonicalFirewall({});
		var subscriptions = this.pageData.subscriptions || [];
		var presentation = buildSubscriptionPresentation(subscriptions);
		var connected = !!(status.state && status.state.connected === true);
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
		var currentMode = firewall.enabled ? firewall.mode : 'disabled';
		var content = [];

		if (this.rootErrors.status !== '')
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(this.rootErrors.status)));
		if (this.rootErrors.firewall !== '')
			ui.addNotification(null, notificationParagraph(_('Firewall error: %s').format(this.rootErrors.firewall)));
		if (this.rootErrors.subscriptions !== '')
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(this.rootErrors.subscriptions)));
		if (this.rootErrors.explain !== '')
			ui.addNotification(null, notificationParagraph(_('Firewall help error: %s').format(this.rootErrors.explain)));
		if (this.rootErrors.services !== '')
			ui.addNotification(null, notificationParagraph(_('Services error: %s').format(this.rootErrors.services)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-firewall-layout { display:grid; gap:14px; }',
			'.routeflux-firewall-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(240px, 1fr)); gap:12px; margin-bottom:14px; }',
			'.routeflux-firewall-grid > .cbi-value { padding:15px 16px; border:1px solid var(--border-color-medium, rgba(98, 112, 129, 0.34)); border-radius:16px; background:linear-gradient(180deg, var(--background-color-high, rgba(249, 250, 251, 0.98)) 0%, var(--background-color-low, rgba(237, 242, 247, 0.98)) 100%); box-shadow:0 12px 24px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.56); }',
			'.routeflux-firewall-grid > .cbi-value .cbi-value-title { color:var(--text-color-high, #17263a); font-weight:700; }',
			'.routeflux-firewall-grid > .cbi-value .cbi-value-description { margin-top:8px; color:var(--text-color-medium, #4f5f70); line-height:1.55; }',
			'.routeflux-firewall-editors { display:grid; gap:12px; }',
			'.routeflux-firewall-editor { position:relative; border:1px solid rgba(98, 112, 129, 0.32); border-radius:18px; padding:18px; background:linear-gradient(180deg, rgba(248, 250, 252, 0.98) 0%, rgba(238, 243, 249, 0.98) 100%); box-shadow:0 16px 32px rgba(15, 23, 42, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.66); overflow:hidden; }',
			'.routeflux-firewall-editor-emphasis { border-color:rgba(56, 189, 248, 0.34); background:linear-gradient(180deg, rgba(246, 250, 255, 0.99) 0%, rgba(233, 243, 252, 0.98) 100%); box-shadow:0 18px 36px rgba(14, 165, 233, 0.11), inset 0 1px 0 rgba(255, 255, 255, 0.72); }',
			'.routeflux-firewall-editor-muted { background:linear-gradient(180deg, rgba(248, 250, 252, 0.97) 0%, rgba(236, 241, 247, 0.97) 100%); }',
			'.routeflux-firewall-editor-bypass { border-color:rgba(37, 99, 128, 0.36); background:linear-gradient(180deg, rgba(228, 238, 244, 0.98) 0%, rgba(214, 226, 235, 0.98) 100%); box-shadow:0 16px 30px rgba(22, 50, 74, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.62); }',
			'.routeflux-firewall-editor-bypass::before { content:""; position:absolute; inset:0 0 auto 0; height:4px; background:linear-gradient(90deg, #0ea5e9 0%, #22c55e 100%); }',
			'.routeflux-firewall-editor-head { display:grid; gap:8px; margin-bottom:14px; }',
			'.routeflux-firewall-editor-kicker { display:inline-flex; align-items:center; width:max-content; max-width:100%; padding:4px 10px; border-radius:999px; background:rgba(15, 118, 110, 0.12); color:#0f766e; font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
			'.routeflux-firewall-editor-head h4 { margin:0; color:var(--text-color-high, #17263a); font-size:clamp(20px, 1vw + 16px, 28px); line-height:1.2; }',
			'.routeflux-firewall-editor-head .cbi-value-description { color:var(--text-color-medium, #4f5f70); margin:0; max-width:72ch; line-height:1.6; font-size:13px; }',
			'.routeflux-firewall-editor-bypass .routeflux-firewall-editor-head h4 { color:#16324a !important; }',
			'.routeflux-firewall-editor-description-strong { color:#16324a !important; font-weight:500; }',
			'.routeflux-firewall-editor-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(280px, 1fr)); gap:14px; margin-bottom:14px; }',
			'.routeflux-firewall-editor-grid .cbi-value-title { display:inline-block; margin-bottom:8px; color:var(--text-color-high, #17263a); font-weight:700; }',
			'.routeflux-firewall-editor-bypass .routeflux-firewall-editor-grid .cbi-value-title { color:#284357 !important; }',
			'.routeflux-firewall-inline { display:flex; gap:10px; align-items:stretch; }',
			'.routeflux-firewall-inline > .cbi-input-text, .routeflux-firewall-inline > .cbi-input-select { flex:1 1 auto; min-width:0; min-height:56px; padding:0 16px; border:1px solid rgba(71, 85, 105, 0.44); border-radius:14px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(244, 247, 251, 0.98) 100%); color:var(--text-color-high, #17263a); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 10px 24px rgba(15, 23, 42, 0.08); }',
			'.routeflux-firewall-inline > .cbi-input-select { padding-right:44px; }',
			'.routeflux-firewall-inline .cbi-input-text::placeholder { color:rgba(71, 85, 105, 0.72); opacity:1; }',
			'.routeflux-firewall-inline > .cbi-input-text:focus, .routeflux-firewall-inline > .cbi-input-select:focus { border-color:rgba(14, 165, 233, 0.72); box-shadow:0 0 0 1px rgba(14, 165, 233, 0.2), 0 14px 30px rgba(14, 165, 233, 0.14); }',
			'.routeflux-firewall-inline > .cbi-button-action { min-width:138px; min-height:56px; padding:0 18px; border:1px solid rgba(14, 165, 233, 0.5); border-radius:14px; background:linear-gradient(180deg, #14324b 0%, #10283b 100%); color:#eef6ff; font-weight:700; box-shadow:0 14px 30px rgba(15, 23, 42, 0.16); }',
			'.routeflux-firewall-list { display:grid; gap:8px; }',
			'.routeflux-firewall-item { display:flex; gap:12px; align-items:center; padding:12px 14px; border-radius:15px; background:rgba(255, 255, 255, 0.9); border:1px solid rgba(148, 163, 184, 0.26); box-shadow:0 8px 18px rgba(15, 23, 42, 0.06); }',
			'.routeflux-firewall-item-value { flex:1 1 auto; min-width:0; word-break:break-word; font-weight:600; color:#1f2d40; }',
			'.routeflux-firewall-badge { display:inline-flex; align-items:center; justify-content:center; min-width:58px; padding:4px 8px; border-radius:999px; background:rgba(59, 130, 246, 0.12); color:#1d4ed8; font-size:11px; font-weight:700; letter-spacing:.04em; text-transform:uppercase; }',
			'.routeflux-firewall-badge-domain { background:rgba(16, 185, 129, 0.12); color:#047857; }',
			'.routeflux-firewall-badge-ip { background:rgba(249, 115, 22, 0.14); color:#c2410c; }',
			'.routeflux-firewall-badge-host { background:rgba(99, 102, 241, 0.12); color:#4338ca; }',
			'.routeflux-firewall-item .cbi-button-remove { margin-left:auto; border-radius:12px; }',
			'.routeflux-firewall-empty { padding:14px; border-radius:14px; background:rgba(255, 255, 255, 0.78); border:1px dashed rgba(148, 163, 184, 0.42); color:#4b5563; }',
			'.routeflux-firewall-disabled-note { margin-top:4px; }',
			'.routeflux-firewall-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-firewall-actions .cbi-button { min-height:48px; padding:0 18px; border-radius:14px; font-weight:700; }',
			'.routeflux-firewall-page-description { color:var(--text-color-medium, #4f5f70); max-width:78ch; line-height:1.6; }',
			'.routeflux-firewall-toggle { display:flex; gap:10px; align-items:flex-start; font-weight:600; color:var(--text-color-high, #17263a); }',
			'.routeflux-firewall-toggle input { margin-top:4px; }',
			'.routeflux-firewall-help { white-space:pre-wrap; margin:0; padding:14px 16px; border:1px solid rgba(98, 112, 129, 0.28); border-radius:14px; background:linear-gradient(180deg, rgba(248, 250, 252, 0.96) 0%, rgba(238, 243, 249, 0.96) 100%); }',
			'@media (max-width: 720px) { .routeflux-firewall-inline { flex-direction:column; } .routeflux-firewall-inline > .cbi-button-action, .routeflux-firewall-actions .cbi-button { width:100%; } .routeflux-firewall-grid > .cbi-value { padding:14px; } .routeflux-firewall-editor { padding:16px; } }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Firewall') ]));
		content.push(E('p', { 'class': 'cbi-section-descr routeflux-firewall-page-description' }, [
			_('Manage transparent routing with clear structured editors for Hosts, Targets, and Bypass. Use the Services tab to create reusable custom service presets.')
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

		if (currentMode !== 'disabled' && !connected) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message' }, [
					_('Transparent routing settings are saved, but RouteFlux is currently disconnected. Connect a node to apply nftables and Xray runtime changes.')
				])
			]));
		}

		content.push(E('div', { 'class': 'cbi-section routeflux-firewall-layout' }, [
			E('h3', {}, [ _('Configuration') ]),
			E('div', { 'class': 'routeflux-firewall-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-firewall-mode' }, [ _('Mode') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', {
							'id': 'routeflux-firewall-mode',
							'class': 'cbi-input-select',
							'change': function(ev) {
								this.handleModeChange(ev);
							}.bind(this)
						}, [
							E('option', { 'value': 'disabled', 'selected': this.formState.mode === 'disabled' ? 'selected' : null }, [ _('Disabled') ]),
							E('option', { 'value': 'hosts', 'selected': this.formState.mode === 'hosts' ? 'selected' : null }, [ _('Hosts') ]),
							E('option', { 'value': 'targets', 'selected': this.formState.mode === 'targets' ? 'selected' : null }, [ _('Targets') ]),
							E('option', { 'value': 'bypass', 'selected': this.formState.mode === 'bypass' ? 'selected' : null }, [ _('Bypass') ]),
							this.formState.mode === 'advanced-split'
								? E('option', { 'value': 'advanced-split', 'selected': 'selected', 'disabled': 'disabled' }, [ _('Advanced Split (switch required)') ])
								: null
						].filter(function(option) { return option !== null; }))
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Hosts sends all traffic from selected LAN devices through RouteFlux. Targets proxies only selected resources. Bypass keeps everything on RouteFlux except selected direct resources and excluded devices.')
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
							'value': this.formState.port,
							'input': function(ev) {
								this.handlePortInput(ev);
							}.bind(this)
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Transparent redirect port used by nftables and the generated Xray runtime config.')
					])
				]),
				E('div', {
					'class': 'cbi-value',
					'style': 'grid-column:1 / -1'
				}, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-firewall-block-quic' }, [ _('Block QUIC') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('label', { 'class': 'routeflux-firewall-toggle' }, [
							E('input', {
								'id': 'routeflux-firewall-block-quic',
								'type': 'checkbox',
								'checked': this.formState.block_quic ? 'checked' : null,
								'change': function(ev) {
									this.handleBlockQUICChange(ev);
								}.bind(this)
							}),
							_('Keep the legacy QUIC compatibility switch for TCP-only transparent routing setups.')
						])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('When enabled, RouteFlux blocks proxied QUIC and UDP traffic so clients retry over TCP. Leave it off when you want QUIC to be proxied normally.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-firewall-editors' }, this.renderConfigurationPanels()),
			E('div', { 'class': 'routeflux-firewall-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleSaveSettings')
				}, [ _('Save') ]),
				E('button', {
					'class': 'cbi-button cbi-button-reset',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleDisable'),
					'disabled': currentMode === 'disabled' ? 'disabled' : null
				}, [ _('Disable Routing') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('details', { 'open': 'open' }, [
				E('summary', {}, [ _('Help') ]),
				E('pre', { 'class': 'routeflux-firewall-help' }, [
					this.pageData.explainText || _('No firewall help text is available.')
				])
			])
		]));

		return content;
	},

	render: function(data) {
		this.initializePageState(data);
		return E('div', { 'id': 'routeflux-firewall-root' }, this.renderPageContent());
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
