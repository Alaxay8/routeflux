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
			'provider_title': title,
			'profile_label': _('Profile %d').format(group.count)
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

function sameList(left, right) {
	var leftList = cleanList(left || []);
	var rightList = cleanList(right || []);

	if (leftList.length !== rightList.length)
		return false;

	for (var i = 0; i < leftList.length; i++) {
		if (leftList[i] !== rightList[i])
			return false;
	}

	return true;
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

function selectorSetHasEntries(value) {
	var selectors = cloneSelectorSet(value);

	return selectors.services.length > 0 || selectors.domains.length > 0 || selectors.cidrs.length > 0;
}

function selectorValues(value) {
	var selectors = cloneSelectorSet(value);

	return selectors.services.concat(selectors.domains).concat(selectors.cidrs);
}

function sameSelectorSet(left, right) {
	var leftValue = cloneSelectorSet(left);
	var rightValue = cloneSelectorSet(right);

	return sameList(leftValue.services, rightValue.services) &&
		sameList(leftValue.domains, rightValue.domains) &&
		sameList(leftValue.cidrs, rightValue.cidrs);
}

function selectorEditorFromSet(value) {
	var selectors = cloneSelectorSet(value);

	return {
		'cli_services': selectors.services,
		'domains': selectors.domains,
		'cidrs': selectors.cidrs,
		'selectorInput': ''
	};
}

function selectorSetFromEditor(editor) {
	return {
		'services': [],
		'domains': cleanList((editor || {}).domains || []),
		'cidrs': cleanList((editor || {}).cidrs || [])
	};
}

function editorCLIServiceValues(editor) {
	return cleanList((editor || {}).cli_services || []);
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

function appendStringSliceFlags(argv, flag, values) {
	var list = cleanList(values || []);

	for (var i = 0; i < list.length; i++) {
		argv.push(flag);
		argv.push(list[i]);
	}

	return argv;
}

function normalizedSelectorSet(raw, legacy) {
	return {
		'services': cleanList((raw && raw.services) || (legacy && legacy.target_services) || []),
		'domains': cleanList((raw && raw.domains) || (legacy && legacy.target_domains) || []),
		'cidrs': cleanList((raw && raw.cidrs) || (legacy && legacy.target_cidrs) || [])
	};
}

function inferFirewallMode(raw) {
	var mode = trim(raw && raw.mode);
	var split = raw && raw.split ? raw.split : {};
	var targets = raw && raw.targets ? raw.targets : {};
	var hasDeviceSelectors = cleanList((raw && raw.hosts) || (raw && raw.source_cidrs) || []).length > 0;
	var hasDestinationSelectors = selectorSetHasEntries(targets) ||
		cleanList(raw && raw.target_services).length > 0 ||
		cleanList(raw && raw.target_domains).length > 0 ||
		cleanList(raw && raw.target_cidrs).length > 0;
	var hasSplit = selectorSetHasEntries(split.proxy) || selectorSetHasEntries(split.bypass) ||
		cleanList(split.excluded_sources).length > 0;

	if (mode === 'hosts' || mode === 'targets' || mode === 'split' || mode === 'disabled')
		return mode;
	if (hasDeviceSelectors)
		return 'hosts';
	if (hasSplit)
		return 'split';
	if (hasDestinationSelectors)
		return 'targets';

	return 'disabled';
}

function normalizedSplitSettings(raw) {
	var split = raw && raw.split ? raw.split : {};
	var explicitBypass = selectorSetHasEntries(split.bypass);
	var explicitProxy = selectorSetHasEntries(split.proxy);
	var legacyBypass = trim(raw && raw.target_mode) === 'bypass';
	var useLegacyBypass = !explicitBypass && !explicitProxy && legacyBypass;

	return {
		'proxy': normalizedSelectorSet(split.proxy || {}, null),
		'bypass': useLegacyBypass ? normalizedSelectorSet(null, raw) : normalizedSelectorSet(split.bypass || {}, null),
		'excluded_sources': cleanList(split.excluded_sources || []),
		'default_action': trim(split.default_action) !== '' ? trim(split.default_action) : (useLegacyBypass ? 'proxy' : 'direct')
	};
}

function splitLooksLikeBypass(split) {
	return trim((split || {}).default_action) === 'proxy' && !selectorSetHasEntries((split || {}).proxy);
}

function describeActiveRoutingMode(value) {
	switch (trim(value)) {
	case 'bypass':
		return _('Bypass');
	case 'disabled':
		return _('Off');
	case 'targets':
		return _('Destination-only routing');
	case 'hosts':
		return _('Device-based routing');
	case 'split':
		return _('Advanced split routing');
	default:
		return _('Unknown');
	}
}

function summarizeSelectors(selectors) {
	var values = selectorValues(selectors);

	return values.length > 0 ? values.join(', ') : '-';
}

function canonicalFirewall(firewall) {
	var raw = firewall || {};
	var enabled = raw.enabled === true;
	var mode = inferFirewallMode(raw);
	var split = normalizedSplitSettings(raw);
	var drafts = raw.mode_drafts || {};
	var draftSplit = drafts.split || {};
	var bypassDraft = {
		'selectors': cloneSelectorSet(draftSplit.bypass || {}),
		'excluded_sources': cleanList(draftSplit.excluded_sources || [])
	};
	var currentMode = 'disabled';
	var supported = true;
	var warning = '';
	var activeBypass = {
		'selectors': emptySelectorSet(),
		'excluded_sources': []
	};
	var summaryLines = [];

	if (enabled === true) {
		if (mode === 'split' && splitLooksLikeBypass(split)) {
			currentMode = 'bypass';
			activeBypass = {
				'selectors': cloneSelectorSet(split.bypass),
				'excluded_sources': cleanList(split.excluded_sources || [])
			};
		}
		else if (mode === 'disabled') {
			currentMode = 'disabled';
		}
		else {
			supported = false;
			currentMode = mode;
			warning = _('The current routing config uses an advanced RouteFlux mode. This Routing page only edits Off and Bypass; advanced routing stays in the CLI.');
		}
	}

	summaryLines.push(_('Current mode: %s').format(describeActiveRoutingMode(currentMode)));
	if (currentMode === 'bypass') {
		summaryLines.push(_('Keep direct: %s').format(summarizeSelectors(activeBypass.selectors)));
		summaryLines.push(_('Excluded devices: %s').format(cleanList(activeBypass.excluded_sources).join(', ') || '-'));
	}
	else if (currentMode === 'targets') {
		summaryLines.push(_('Destination-only selectors: %s').format(summarizeSelectors(normalizedSelectorSet(raw.targets || {}, raw))));
	}
	else if (currentMode === 'hosts') {
		summaryLines.push(_('Device-based selectors: %s').format(cleanList(raw.hosts || raw.source_cidrs || []).join(', ') || '-'));
	}
	else if (currentMode === 'split') {
		summaryLines.push(_('Proxy selectors: %s').format(summarizeSelectors(split.proxy)));
		summaryLines.push(_('Direct selectors: %s').format(summarizeSelectors(split.bypass)));
		summaryLines.push(_('Excluded devices: %s').format(cleanList(split.excluded_sources).join(', ') || '-'));
	}

	return {
		'enabled': enabled,
		'current_mode': currentMode,
		'supported': supported,
		'warning': warning,
		'bypass': activeBypass,
		'bypass_draft': bypassDraft,
		'summary_lines': summaryLines
	};
}

var defaultDNSServers = [ '1.1.1.1', '1.0.0.1' ];
var defaultDNSDirectDomains = [ 'domain:lan', 'full:router.lan' ];

function canonicalDNS(dns) {
	var raw = dns || {};
	var normalized = {
		'mode': trim(raw.mode).toLowerCase(),
		'transport': trim(raw.transport).toLowerCase(),
		'servers': cleanList(raw.servers || []),
		'bootstrap': cleanList(raw.bootstrap || []),
		'direct_domains': cleanList(raw.direct_domains || [])
	};
	var isDefault = normalized.mode === 'split' &&
		normalized.transport === 'doh' &&
		sameList(normalized.servers, defaultDNSServers) &&
		sameList(normalized.bootstrap, []) &&
		sameList(normalized.direct_domains, defaultDNSDirectDomains);
	var supported = normalized.mode === 'system' || isDefault;
	var choice = '';

	if (normalized.mode === 'system')
		choice = 'system';
	else if (isDefault)
		choice = 'default';

	return {
		'supported': supported,
		'choice': supported ? choice : '',
		'warning': supported ? '' : _('The current DNS profile is custom. Routing only edits System DNS or the Recommended DNS preset here. Advanced DNS settings are available in the CLI.'),
		'summary_lines': [
			_('Mode: %s').format(normalized.mode || '-'),
			_('Transport: %s').format(normalized.transport || '-'),
			_('Servers: %s').format(normalized.servers.join(', ') || '-'),
			_('Bootstrap: %s').format(normalized.bootstrap.join(', ') || '-'),
			_('Direct domains: %s').format(normalized.direct_domains.join(', ') || '-')
		]
	};
}

function buildFormState(firewall, dns) {
	var routing = canonicalFirewall(firewall);
	var dnsState = canonicalDNS(dns);
	var selectorSet = routing.current_mode === 'bypass'
		? routing.bypass.selectors
		: routing.bypass_draft.selectors;
	var excludedSources = routing.current_mode === 'bypass'
		? routing.bypass.excluded_sources
		: routing.bypass_draft.excluded_sources;

	return {
		'mode': routing.supported ? routing.current_mode : '',
		'dns_choice': dnsState.supported ? dnsState.choice : '',
		'bypass': {
			'selectors': selectorEditorFromSet(selectorSet),
			'excluded': listEditorFromEntries(excludedSources)
		}
	};
}

function modeSummary(value) {
	switch (trim(value)) {
	case 'bypass':
		return _('Bypass');
	case 'disabled':
		return _('Off');
	case 'targets':
		return _('Destination-only routing');
	case 'hosts':
		return _('Device-based routing');
	case 'split':
		return _('Advanced split routing');
	default:
		return _('Off');
	}
}

function dnsChoiceSummary(value) {
	switch (trim(value)) {
	case 'system':
		return _('System DNS');
	case 'default':
		return _('Recommended DNS preset');
	default:
		return _('Custom DNS');
	}
}

function choiceClass(selected) {
	return selected === true
		? 'routeflux-routing-choice routeflux-routing-choice-selected'
		: 'routeflux-routing-choice';
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
			this.execJSON([ '--json', 'dns', 'get' ]).catch(function(err) {
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
		var dnsPayload = data[3] && !data[3].__error__
			? data[3]
			: ((status.settings || {}).dns || {});

		this.pageData = {
			'status': status,
			'firewall': canonicalFirewall(firewallPayload),
			'dns': canonicalDNS(dnsPayload),
			'subscriptions': Array.isArray(data[2]) ? data[2] : []
		};
		this.formState = buildFormState(firewallPayload, dnsPayload);
		this.rootErrors = {
			'status': trim(data[0] && data[0].__error__),
			'firewall': trim(data[1] && data[1].__error__),
			'subscriptions': trim(data[2] && data[2].__error__),
			'dns': trim(data[3] && data[3].__error__)
		};
	},

	renderIntoRoot: function() {
		var root = document.querySelector('#routeflux-routing-root');

		if (root)
			dom.content(root, this.renderPageContent());
	},

	handleModeChange: function(ev) {
		this.formState.mode = trim(ev.currentTarget.value);
		this.renderIntoRoot();
	},

	handleDNSChoiceChange: function(ev) {
		this.formState.dns_choice = trim(ev.currentTarget.value);
		this.renderIntoRoot();
	},

	handleSelectorInputChange: function(ev) {
		this.formState.bypass.selectors.selectorInput = ev.currentTarget.value;
	},

	handleExcludedInputChange: function(ev) {
		this.formState.bypass.excluded.input = ev.currentTarget.value;
	},

	handleAddSelector: function(ev) {
		var editor;
		var parts;
		var i;
		var value;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		editor = this.formState.bypass.selectors;
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

	handleAddExcluded: function(ev) {
		var list;
		var parts;
		var i;
		var value;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		list = this.formState.bypass.excluded;
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

	handleRemoveSelector: function(field, value, ev) {
		var editor;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		editor = this.formState.bypass.selectors;
		if (!Array.isArray(editor[field]))
			return;

		editor[field] = cleanList(editor[field].filter(function(entry) {
			return entry !== value;
		}));
		this.renderIntoRoot();
	},

	handleRemoveExcluded: function(value, ev) {
		var list;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		list = this.formState.bypass.excluded;
		list.entries = cleanList(list.entries.filter(function(entry) {
			return entry !== value;
		}));
		this.renderIntoRoot();
	},

	handleSaveSettings: function(ev) {
		var routing = this.pageData.firewall || canonicalFirewall({});
		var dnsState = this.pageData.dns || canonicalDNS({});
		var mode = trim(this.formState.mode);
		var dnsChoice = trim(this.formState.dns_choice);
		var desiredSelectors = selectorSetFromEditor(this.formState.bypass.selectors);
		var desiredExcluded = listValues(this.formState.bypass.excluded);
		var desiredSelectorValues = selectorValues(desiredSelectors);
		var commands = [];
		var draftCommand = [ 'firewall', 'draft', 'bypass' ];
		var bypassCommand = [ 'firewall', 'set', 'bypass' ];
		var draftChanged;
		var routingChanged;
		var dnsChanged;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!routing.supported && mode === '') {
			ui.addNotification(null, notificationParagraph(_('Choose Off or Bypass to replace the current advanced routing setup.')));
			return Promise.resolve();
		}

		if (mode === 'bypass' && desiredSelectorValues.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Bypass needs at least one Keep Direct selector.')));
			return Promise.resolve();
		}

		if (!dnsState.supported && dnsChoice === '') {
			ui.addNotification(null, notificationParagraph(_('Choose System DNS or the Recommended DNS preset here. Advanced DNS settings are available in the CLI.')));
			return Promise.resolve();
		}

		draftChanged = !sameSelectorSet(desiredSelectors, routing.bypass_draft.selectors) ||
			!sameList(desiredExcluded, routing.bypass_draft.excluded_sources);
		if (draftChanged) {
			if (selectorSetHasEntries(desiredSelectors) || desiredExcluded.length > 0) {
				appendStringSliceFlags(draftCommand, '--exclude-host', desiredExcluded);
				commands.push(draftCommand.concat(desiredSelectorValues));
			}
			else {
				commands.push([ 'firewall', 'draft', 'bypass' ]);
			}
		}

		if (mode === 'bypass') {
			routingChanged = !routing.supported ||
				routing.current_mode !== 'bypass' ||
				!sameSelectorSet(desiredSelectors, routing.bypass.selectors) ||
				!sameList(desiredExcluded, routing.bypass.excluded_sources);
			if (routingChanged) {
				appendStringSliceFlags(bypassCommand, '--exclude-host', desiredExcluded);
				commands.push(bypassCommand.concat(desiredSelectorValues));
			}
		}
		else if (mode === 'disabled') {
			routingChanged = !routing.supported || routing.current_mode !== 'disabled' || routing.enabled === true;
			if (routingChanged)
				commands.push([ 'firewall', 'disable' ]);
		}

		if (dnsChoice !== '') {
			dnsChanged = !dnsState.supported || dnsState.choice !== dnsChoice;
			if (dnsChanged) {
				if (dnsChoice === 'system')
					commands.push([ 'dns', 'set', 'mode', 'system' ]);
				else if (dnsChoice === 'default')
					commands.push([ 'dns', 'set', 'default' ]);
			}
		}

		if (commands.length === 0) {
			ui.addNotification(null, notificationParagraph(_('No routing changes to save.')), 'info');
			return Promise.resolve();
		}

		return this.runCommands(commands, _('Routing settings saved.'));
	},

	renderSelectorItems: function(editor) {
		var rows = [];
		var selectors = selectorSetFromEditor(editor);
		var total = 0;
		var i;

		for (i = 0; i < selectors.domains.length; i++) {
			total++;
			rows.push(E('div', { 'class': 'routeflux-routing-item routeflux-routing-item-domain' }, [
				E('span', { 'class': 'routeflux-routing-badge routeflux-routing-badge-domain' }, [ _('Domain') ]),
				E('span', { 'class': 'routeflux-routing-item-value routeflux-routing-item-value-code' }, [ selectors.domains[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveSelector', 'domains', selectors.domains[i])
				}, [ _('Remove') ])
			]));
		}

		for (i = 0; i < selectors.cidrs.length; i++) {
			total++;
			rows.push(E('div', { 'class': 'routeflux-routing-item routeflux-routing-item-ip' }, [
				E('span', { 'class': 'routeflux-routing-badge routeflux-routing-badge-ip' }, [ _('IPv4') ]),
				E('span', { 'class': 'routeflux-routing-item-value routeflux-routing-item-value-code' }, [ selectors.cidrs[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveSelector', 'cidrs', selectors.cidrs[i])
				}, [ _('Remove') ])
			]));
		}

		return E('div', { 'class': 'routeflux-routing-selector-shell' }, [
			E('div', { 'class': 'routeflux-routing-selector-head' }, [
				E('div', { 'class': 'routeflux-routing-selector-copy' }, [
					E('h4', {}, [ _('Direct selectors') ]),
					E('p', {}, [
						_('Domains and IPv4 selectors stay direct only while bypass mode is active.')
					])
				]),
				E('div', { 'class': 'routeflux-routing-selector-meta' }, [
					_('%d selector(s)').format(total)
				])
			]),
			rows.length === 0
				? E('div', { 'class': 'routeflux-routing-empty' }, [ _('Nothing added yet.') ])
				: E('div', { 'class': 'routeflux-routing-list' }, rows)
		]);
	},

	renderExcludedItems: function(list) {
		var values = listValues(list);
		var rows = [];

		for (var i = 0; i < values.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-routing-item' }, [
				E('span', { 'class': 'routeflux-routing-badge routeflux-routing-badge-host' }, [ _('Host') ]),
				E('span', { 'class': 'routeflux-routing-item-value' }, [ values[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-remove',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveExcluded', values[i])
				}, [ _('Remove') ])
			]));
		}

		if (rows.length === 0)
			return E('div', { 'class': 'routeflux-routing-empty' }, [ _('No excluded devices added.') ]);

		return E('div', { 'class': 'routeflux-routing-list' }, rows);
	},

	renderSummaryShell: function(title, lines) {
		var items = Array.isArray(lines) ? lines : [];

		return E('div', { 'class': 'cbi-section' }, [
			E('div', { 'class': 'routeflux-routing-summary-shell' }, [
				E('h3', {}, [ title ]),
				E('ul', { 'class': 'routeflux-routing-summary-list' }, items.map(function(line) {
					return E('li', {}, [ line ]);
				}))
			])
		]);
	},

	renderCLIOnlyAliasSummary: function(aliases) {
		var values = cleanList(aliases || []);

		if (values.length === 0)
			return '';

		return this.renderSummaryShell(_('CLI-only alias summary'), [
			_('Current bypass aliases: %s').format(values.join(', ')),
			_('Routing in LuCI edits only direct domains and IPv4 selectors. Saving here replaces these aliases with the direct selectors shown on this page.'),
			_('Use the CLI when you want to keep reusable preset or service aliases in routing rules.')
		]);
	},

	renderPageContent: function() {
		var status = this.pageData.status || {};
		var routing = this.pageData.firewall || canonicalFirewall({});
		var dnsState = this.pageData.dns || canonicalDNS({});
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
		var currentDNSLabel = dnsChoiceSummary(dnsState.choice);
		var content = [];

		if (this.rootErrors.status !== '')
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(this.rootErrors.status)));
		if (this.rootErrors.firewall !== '')
			ui.addNotification(null, notificationParagraph(_('Routing error: %s').format(this.rootErrors.firewall)));
		if (this.rootErrors.subscriptions !== '')
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(this.rootErrors.subscriptions)));
		if (this.rootErrors.dns !== '')
			ui.addNotification(null, notificationParagraph(_('DNS error: %s').format(this.rootErrors.dns)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'#routeflux-routing-root { --routeflux-routing-ink:#10263f; --routeflux-routing-ink-muted:#44566b; --routeflux-routing-ink-soft:#62758a; --routeflux-routing-panel-bg:linear-gradient(160deg, rgba(243, 248, 255, 0.98) 0%, rgba(230, 239, 249, 0.98) 56%, rgba(220, 232, 245, 0.98) 100%); --routeflux-routing-surface-bg:linear-gradient(180deg, rgba(255, 255, 255, 0.97) 0%, rgba(246, 250, 254, 0.97) 100%); --routeflux-routing-surface-strong:linear-gradient(180deg, #17324d 0%, #10243a 100%); }',
			'#routeflux-routing-root.routeflux-theme-dark { --routeflux-routing-ink:#eef4ff; --routeflux-routing-ink-muted:#a8b8ce; --routeflux-routing-ink-soft:#8ea0b8; --routeflux-routing-panel-bg:linear-gradient(160deg, rgba(15, 23, 37, 0.96) 0%, rgba(10, 17, 29, 0.98) 56%, rgba(8, 13, 24, 0.99) 100%); --routeflux-routing-surface-bg:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); --routeflux-routing-surface-strong:linear-gradient(180deg, rgba(52, 147, 235, 0.92) 0%, rgba(30, 116, 211, 0.94) 100%); }',
			'#routeflux-routing-root.routeflux-theme-dark::before, #routeflux-routing-root.routeflux-theme-dark::after { display:none; }',
			'#routeflux-routing-root .routeflux-routing-layout { display:grid; gap:14px; padding:0; border:0; background:transparent; box-shadow:none; color:var(--routeflux-routing-ink); overflow:visible; }',
			'#routeflux-routing-root .routeflux-routing-layout::before { display:none; }',
			'.routeflux-routing-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(240px, 1fr)); gap:14px; }',
			'.routeflux-routing-panel { position:relative; overflow:hidden; isolation:isolate; border:1px solid rgba(164, 184, 207, 0.56); border-radius:22px; padding:20px; background:var(--routeflux-routing-panel-bg); box-shadow:0 16px 30px rgba(69, 90, 118, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
			'.routeflux-routing-panel::before { content:""; position:absolute; inset:0; background:radial-gradient(circle at top left, rgba(125, 211, 252, 0.16), transparent 32%), radial-gradient(circle at bottom right, rgba(59, 130, 246, 0.08), transparent 38%); pointer-events:none; }',
			'.routeflux-routing-panel > * { position:relative; z-index:1; }',
			'.routeflux-routing-panel h3 { margin:0; color:var(--routeflux-routing-ink); font-size:clamp(24px, 1.1vw + 18px, 34px); line-height:1.12; letter-spacing:-0.03em; }',
			'.routeflux-routing-mode-grid, .routeflux-routing-dns-grid { display:grid; gap:14px; }',
			'.routeflux-routing-choice { position:relative; display:flex; gap:12px; align-items:flex-start; padding:16px 18px; border:1px solid rgba(120, 141, 167, 0.34); border-radius:18px; background:var(--routeflux-routing-surface-bg); box-shadow:0 14px 30px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.9); transition:transform .18s ease, border-color .18s ease, box-shadow .18s ease, background .18s ease; cursor:pointer; }',
			'.routeflux-routing-choice:hover { transform:translateY(-1px); border-color:rgba(34, 197, 94, 0.34); box-shadow:0 16px 32px rgba(15, 23, 42, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.94); }',
			'.routeflux-routing-choice-selected { border-color:rgba(34, 197, 94, 0.52); background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(220, 252, 231, 0.99) 100%); box-shadow:0 18px 34px rgba(21, 128, 61, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.96); }',
			'.routeflux-routing-choice-control { position:absolute; width:1px; height:1px; margin:-1px; padding:0; border:0; overflow:hidden; clip:rect(0, 0, 0, 0); clip-path:inset(50%); white-space:nowrap; }',
			'.routeflux-routing-choice-indicator { position:relative; flex:0 0 auto; width:26px; height:26px; margin-top:2px; border:1.5px solid rgba(71, 85, 105, 0.42); border-radius:999px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(241, 245, 249, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.95), 0 10px 18px rgba(15, 23, 42, 0.08); transition:border-color .18s ease, box-shadow .18s ease, background .18s ease; }',
			'.routeflux-routing-choice-indicator::after { content:""; position:absolute; inset:0; display:flex; align-items:center; justify-content:center; border-radius:999px; color:transparent; transform:scale(0.62); transition:transform .18s ease, background .18s ease, color .18s ease, box-shadow .18s ease; }',
			'.routeflux-routing-choice-control:focus-visible + .routeflux-routing-choice-indicator { outline:2px solid rgba(34, 197, 94, 0.28); outline-offset:3px; }',
			'.routeflux-routing-choice-selected .routeflux-routing-choice-indicator { border-color:rgba(22, 163, 74, 0.54); background:linear-gradient(180deg, rgba(240, 253, 244, 0.99) 0%, rgba(220, 252, 231, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.96), 0 12px 20px rgba(21, 128, 61, 0.12); }',
			'.routeflux-routing-choice-selected .routeflux-routing-choice-indicator::after { content:"\\2713"; background:linear-gradient(180deg, #22c55e 0%, #15803d 100%); color:#ffffff; font-size:15px; font-weight:900; transform:scale(1); box-shadow:0 10px 18px rgba(21, 128, 61, 0.28); }',
			'.routeflux-routing-choice-copy { flex:1 1 auto; min-width:0; }',
			'.routeflux-routing-choice-title { display:block; font-weight:800; font-size:clamp(18px, 0.55vw + 15px, 24px); color:var(--routeflux-routing-ink); letter-spacing:-0.02em; }',
			'.routeflux-routing-choice-description { display:block; margin-top:6px; color:var(--routeflux-routing-ink-muted); line-height:1.55; font-size:15px; }',
			'.routeflux-theme-dark .routeflux-routing-panel { border-color:rgba(145, 175, 220, 0.16); background:var(--routeflux-routing-panel-bg); box-shadow:0 24px 42px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.04); }',
			'.routeflux-theme-dark .routeflux-routing-panel::before { background:radial-gradient(circle at top left, rgba(88, 196, 255, 0.14), transparent 34%), radial-gradient(circle at bottom right, rgba(52, 147, 235, 0.08), transparent 40%); }',
			'.routeflux-theme-dark .routeflux-routing-choice { border-color:rgba(145, 175, 220, 0.16); background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); box-shadow:0 18px 32px rgba(0, 0, 0, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.04); }',
			'.routeflux-theme-dark .routeflux-routing-choice:hover { border-color:rgba(34, 197, 94, 0.28); box-shadow:0 20px 34px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-theme-dark .routeflux-routing-choice-selected { border-color:rgba(34, 197, 94, 0.42); background:linear-gradient(180deg, rgba(13, 35, 28, 0.96) 0%, rgba(10, 24, 21, 1) 100%); box-shadow:0 22px 38px rgba(8, 23, 19, 0.32), 0 0 0 1px rgba(34, 197, 94, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.06); }',
			'.routeflux-theme-dark .routeflux-routing-choice-indicator { border-color:rgba(145, 162, 189, 0.42); background:linear-gradient(180deg, rgba(22, 31, 45, 0.96) 0%, rgba(14, 22, 34, 1) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 10px 18px rgba(0, 0, 0, 0.22); }',
			'.routeflux-theme-dark .routeflux-routing-inline > .cbi-input-text, .routeflux-theme-dark .routeflux-routing-inline > .cbi-input-select { border-color:rgba(145, 175, 220, 0.16); background:rgba(6, 12, 22, 0.72); color:#eef4ff; box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-theme-dark .routeflux-routing-inline .cbi-input-text::placeholder { color:#91a2bd; opacity:0.84; }',
			'.routeflux-theme-dark .routeflux-routing-inline > .cbi-input-text:focus, .routeflux-theme-dark .routeflux-routing-inline > .cbi-input-select:focus { border-color:rgba(88, 196, 255, 0.54); box-shadow:0 0 0 1px rgba(88, 196, 255, 0.18), 0 0 0 8px rgba(88, 196, 255, 0.05); }',
			'.routeflux-theme-dark .routeflux-routing-inline > .cbi-button-action { border-color:rgba(120, 160, 214, 0.2); background:rgba(12, 20, 34, 0.82); color:#a8d7ff; box-shadow:0 12px 24px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-theme-dark .routeflux-routing-inline > .cbi-button-action:hover { border-color:rgba(145, 190, 246, 0.28); background:rgba(14, 24, 40, 0.9); color:#c6e6ff; box-shadow:0 16px 26px rgba(0, 0, 0, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.04); }',
			'.routeflux-routing-editor-head { display:grid; gap:10px; margin-bottom:16px; }',
			'.routeflux-routing-editor-head .cbi-section-descr { margin:0; color:var(--routeflux-routing-ink-muted); line-height:1.6; max-width:72ch; }',
			'.routeflux-routing-editor-kicker { display:inline-flex; align-items:center; width:max-content; max-width:100%; padding:5px 11px; border-radius:999px; background:rgba(14, 165, 233, 0.14); color:#075985; font-size:12px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
			'.routeflux-routing-panel .cbi-value-title { display:inline-block; margin-bottom:8px; color:var(--routeflux-routing-ink); font-weight:800; }',
			'.routeflux-routing-editor-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(280px, 1fr)); gap:14px; margin-bottom:14px; }',
			'.routeflux-routing-inline { display:flex; gap:10px; align-items:stretch; }',
			'.routeflux-routing-inline > .cbi-input-text, .routeflux-routing-inline > .cbi-input-select { flex:1 1 auto; min-width:0; min-height:52px; padding:0 14px; border:1px solid rgba(71, 85, 105, 0.38); border-radius:15px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(244, 248, 252, 0.99) 100%); color:var(--routeflux-routing-ink); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.92), 0 10px 24px rgba(15, 23, 42, 0.08); }',
			'.routeflux-routing-inline > .cbi-input-select { padding-right:44px; }',
			'.routeflux-routing-inline .cbi-input-text::placeholder { color:rgba(68, 86, 107, 0.88); opacity:1; }',
			'.routeflux-routing-inline > .cbi-input-text:focus, .routeflux-routing-inline > .cbi-input-select:focus { border-color:rgba(14, 165, 233, 0.72); box-shadow:0 0 0 1px rgba(14, 165, 233, 0.24), 0 16px 30px rgba(14, 165, 233, 0.16); }',
			'.routeflux-theme-light .routeflux-routing-inline > .cbi-input-text, .routeflux-theme-light .routeflux-routing-inline > .cbi-input-select { border-color:rgba(125, 146, 170, 0.2); background:linear-gradient(180deg, rgba(251, 252, 254, 0.99) 0%, rgba(244, 248, 252, 0.99) 100%); color:#162638; box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 8px 18px rgba(63, 87, 118, 0.06); }',
			'.routeflux-theme-light .routeflux-routing-inline .cbi-input-text::placeholder { color:#63768c; opacity:1; }',
			'.routeflux-theme-light .routeflux-routing-inline > .cbi-input-text:focus, .routeflux-theme-light .routeflux-routing-inline > .cbi-input-select:focus { border-color:rgba(37, 99, 235, 0.36); box-shadow:0 0 0 1px rgba(37, 99, 235, 0.12), 0 0 0 6px rgba(37, 99, 235, 0.04); }',
			'.routeflux-routing-inline > .cbi-button-action { min-width:132px; min-height:52px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b; font-weight:800; box-shadow:0 12px 22px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
			'.routeflux-routing-inline > .cbi-button-action:hover, .routeflux-routing-actions .cbi-button:hover { border-color:rgba(37, 99, 235, 0.28); background:linear-gradient(180deg, rgba(236, 244, 251, 0.99) 0%, rgba(225, 236, 247, 0.99) 100%); color:#102f4c; }',
			'.routeflux-routing-selector-shell { display:grid; gap:14px; padding:16px 18px; border:1px solid rgba(125, 145, 168, 0.3); border-radius:18px; background:rgba(255, 255, 255, 0.78); box-shadow:0 14px 28px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.78); }',
			'.routeflux-routing-selector-head { display:flex; flex-wrap:wrap; justify-content:space-between; align-items:flex-end; gap:10px 14px; }',
			'.routeflux-routing-selector-copy { display:grid; gap:6px; min-width:0; }',
			'.routeflux-routing-selector-copy h4 { margin:0; color:var(--routeflux-routing-ink); font-size:18px; font-weight:800; letter-spacing:-0.02em; }',
			'.routeflux-routing-selector-copy p { margin:0; color:var(--routeflux-routing-ink-muted); line-height:1.55; }',
			'.routeflux-routing-selector-meta { display:inline-flex; align-items:center; min-height:30px; padding:0 12px; border-radius:999px; background:rgba(16, 185, 129, 0.14); color:#047857; font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
			'.routeflux-routing-list { display:grid; gap:10px; }',
			'.routeflux-routing-item { display:flex; gap:12px; align-items:center; padding:14px 16px; border-radius:18px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(244, 248, 252, 0.96) 100%); border:1px solid rgba(125, 145, 168, 0.24); box-shadow:0 12px 24px rgba(15, 23, 42, 0.08); }',
			'.routeflux-routing-item-value { flex:1 1 auto; min-width:0; word-break:break-word; font-weight:700; color:var(--routeflux-routing-ink); }',
			'.routeflux-routing-item-value-code { font-family:ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size:13px; letter-spacing:-0.01em; }',
			'.routeflux-routing-item .cbi-button-remove { min-width:96px; min-height:40px; padding:0 14px; border:1px solid rgba(239, 68, 68, 0.22); border-radius:12px; background:rgba(255, 255, 255, 0.86); color:#b91c1c; box-shadow:0 10px 18px rgba(15, 23, 42, 0.08); }',
			'.routeflux-routing-badge { display:inline-flex; align-items:center; justify-content:center; min-width:58px; padding:4px 8px; border-radius:999px; background:rgba(37, 99, 235, 0.13); color:#1d4ed8; font-size:11px; font-weight:800; letter-spacing:.05em; text-transform:uppercase; }',
			'.routeflux-routing-badge-domain { background:rgba(16, 185, 129, 0.16); color:#047857; }',
			'.routeflux-routing-badge-ip { background:rgba(249, 115, 22, 0.14); color:#c2410c; }',
			'.routeflux-routing-badge-host { background:rgba(99, 102, 241, 0.14); color:#4338ca; }',
			'.routeflux-routing-empty { padding:14px; border-radius:14px; background:rgba(255, 255, 255, 0.82); border:1px dashed rgba(125, 145, 168, 0.42); color:var(--routeflux-routing-ink-muted); }',
			'.routeflux-routing-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-routing-actions .cbi-button { min-height:48px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b; font-weight:800; box-shadow:0 12px 22px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
			'.routeflux-routing-summary-shell { padding:16px 18px; border:1px solid rgba(125, 145, 168, 0.34); border-radius:16px; background:rgba(255, 255, 255, 0.84); box-shadow:0 10px 22px rgba(15, 23, 42, 0.08); }',
			'.routeflux-routing-summary-shell h3 { margin-top:0; margin-bottom:10px; color:var(--routeflux-routing-ink); font-size:20px; letter-spacing:-0.02em; }',
			'.routeflux-routing-summary-list { margin:0; padding-left:18px; color:var(--routeflux-routing-ink-muted); line-height:1.55; }',
			'.routeflux-routing-optional summary { cursor:pointer; font-weight:800; color:var(--routeflux-routing-ink); }',
			'.routeflux-routing-optional-shell { margin-top:12px; }',
			'.routeflux-theme-dark .routeflux-routing-selector-shell { display:grid; gap:14px; padding:16px 18px; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(8, 15, 26, 0.5); box-shadow:0 14px 28px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-theme-dark .routeflux-routing-selector-meta { background:rgba(46, 216, 170, 0.14); color:#9bf5d8; }',
			'.routeflux-theme-dark .routeflux-routing-item { background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); border-color:rgba(145, 175, 220, 0.14); box-shadow:0 12px 24px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-theme-dark .routeflux-routing-item .cbi-button-remove { border-color:rgba(255, 123, 140, 0.28); background:rgba(52, 16, 26, 0.82); color:#ffb7c0; box-shadow:0 16px 28px rgba(52, 16, 26, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-theme-dark .routeflux-routing-badge { background:rgba(37, 99, 235, 0.18); color:#a8d7ff; }',
			'.routeflux-theme-dark .routeflux-routing-badge-domain { background:rgba(46, 216, 170, 0.14); color:#9bf5d8; }',
			'.routeflux-theme-dark .routeflux-routing-badge-ip { background:rgba(249, 115, 22, 0.16); color:#fdba74; }',
			'.routeflux-theme-dark .routeflux-routing-badge-host { background:rgba(99, 102, 241, 0.18); color:#c7d2fe; }',
			'.routeflux-theme-dark .routeflux-routing-empty { background:rgba(8, 15, 26, 0.5); border-color:rgba(145, 175, 220, 0.24); color:#a8b8ce; }',
			'.routeflux-theme-dark .routeflux-routing-actions .cbi-button-apply { border-color:rgba(88, 196, 255, 0.34); background:linear-gradient(180deg, rgba(52, 147, 235, 0.92) 0%, rgba(30, 116, 211, 0.94) 100%); color:#f4fbff; box-shadow:0 18px 32px rgba(30, 116, 211, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.14); }',
			'.routeflux-theme-dark .routeflux-routing-actions .cbi-button-apply:hover { border-color:rgba(88, 196, 255, 0.42); background:linear-gradient(180deg, rgba(44, 132, 221, 0.96) 0%, rgba(26, 101, 192, 0.98) 100%); color:#ffffff; box-shadow:0 20px 34px rgba(30, 116, 211, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.16); }',
			'.routeflux-theme-dark .routeflux-routing-summary-shell { background:rgba(8, 15, 26, 0.58); border-color:rgba(145, 175, 220, 0.16); box-shadow:0 12px 24px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'@media (max-width: 720px) { .routeflux-routing-inline { flex-direction:column; } .routeflux-routing-inline > .cbi-button-action, .routeflux-routing-actions .cbi-button { width:100%; } .routeflux-routing-selector-head { align-items:stretch; } .routeflux-routing-selector-meta { width:max-content; } .routeflux-routing-item { align-items:flex-start; flex-direction:column; } .routeflux-routing-item .cbi-button-remove { width:100%; } }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Routing') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('RouteFlux status, the active connection, and the safe everyday routing actions. Advanced DNS settings and reusable aliases stay in the CLI.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(connected),
				'primary': true
			}),
			this.renderCard(_('Routing Mode'), modeSummary(routing.current_mode)),
			this.renderCard(_('DNS Profile'), currentDNSLabel),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]));

		if (routing.warning !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [ routing.warning ])
			]));
			content.push(this.renderSummaryShell(_('Current Routing Summary'), routing.summary_lines));
		}

		if (dnsState.warning !== '') {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [ dnsState.warning ])
			]));
			content.push(this.renderSummaryShell(_('Current DNS Summary'), dnsState.summary_lines));
		}

		if (editorCLIServiceValues(this.formState.bypass.selectors).length > 0) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('The current Keep Direct config includes CLI-only aliases. Routing in LuCI edits only direct domains and IPv4 selectors.')
				])
			]));
			content.push(this.renderCLIOnlyAliasSummary(editorCLIServiceValues(this.formState.bypass.selectors)));
		}

		content.push(E('div', { 'class': 'cbi-section routeflux-routing-layout' }, [
			E('div', { 'class': 'routeflux-routing-grid' }, [
				E('div', { 'class': 'routeflux-routing-panel' }, [
					E('h3', {}, [ _('Routing') ]),
					E('div', { 'class': 'routeflux-routing-mode-grid' }, [
						E('label', { 'class': choiceClass(this.formState.mode === 'disabled') }, [
							E('input', {
								'class': 'routeflux-routing-choice-control',
								'type': 'radio',
								'name': 'routeflux-routing-mode',
								'value': 'disabled',
								'checked': this.formState.mode === 'disabled' ? 'checked' : null,
								'change': L.bind(function(ev) {
									this.handleModeChange(ev);
								}, this)
							}),
							E('span', { 'class': 'routeflux-routing-choice-indicator', 'aria-hidden': 'true' }, []),
							E('span', { 'class': 'routeflux-routing-choice-copy' }, [
								E('span', { 'class': 'routeflux-routing-choice-title' }, [ _('Off') ]),
								E('span', { 'class': 'routeflux-routing-choice-description' }, [
									_('Keep RouteFlux connected if you want, but do not intercept router traffic from LuCI.')
								])
							])
						]),
						E('label', { 'class': choiceClass(this.formState.mode === 'bypass') }, [
							E('input', {
								'class': 'routeflux-routing-choice-control',
								'type': 'radio',
								'name': 'routeflux-routing-mode',
								'value': 'bypass',
								'checked': this.formState.mode === 'bypass' ? 'checked' : null,
								'change': L.bind(function(ev) {
									this.handleModeChange(ev);
								}, this)
							}),
							E('span', { 'class': 'routeflux-routing-choice-indicator', 'aria-hidden': 'true' }, []),
							E('span', { 'class': 'routeflux-routing-choice-copy' }, [
								E('span', { 'class': 'routeflux-routing-choice-title' }, [ _('Bypass') ]),
								E('span', { 'class': 'routeflux-routing-choice-description' }, [
									_('Proxy everything except the direct domains, IPv4 targets, and optional device exclusions listed below.')
								])
							])
						])
					])
				]),
				E('div', { 'class': 'routeflux-routing-panel' }, [
					E('h3', {}, [ _('DNS') ]),
					E('div', { 'class': 'routeflux-routing-dns-grid' }, [
						E('label', { 'class': choiceClass(this.formState.dns_choice === 'system') }, [
							E('input', {
								'class': 'routeflux-routing-choice-control',
								'type': 'radio',
								'name': 'routeflux-routing-dns',
								'value': 'system',
								'checked': this.formState.dns_choice === 'system' ? 'checked' : null,
								'change': L.bind(function(ev) {
									this.handleDNSChoiceChange(ev);
								}, this)
							}),
							E('span', { 'class': 'routeflux-routing-choice-indicator', 'aria-hidden': 'true' }, []),
							E('span', { 'class': 'routeflux-routing-choice-copy' }, [
								E('span', { 'class': 'routeflux-routing-choice-title' }, [ _('System DNS') ]),
								E('span', { 'class': 'routeflux-routing-choice-description' }, [
									_('Leave DNS exactly as the router already handles it.')
								])
							])
						]),
						E('label', { 'class': choiceClass(this.formState.dns_choice === 'default') }, [
							E('input', {
								'class': 'routeflux-routing-choice-control',
								'type': 'radio',
								'name': 'routeflux-routing-dns',
								'value': 'default',
								'checked': this.formState.dns_choice === 'default' ? 'checked' : null,
								'change': L.bind(function(ev) {
									this.handleDNSChoiceChange(ev);
								}, this)
							}),
							E('span', { 'class': 'routeflux-routing-choice-indicator', 'aria-hidden': 'true' }, []),
							E('span', { 'class': 'routeflux-routing-choice-copy' }, [
								E('span', { 'class': 'routeflux-routing-choice-title' }, [ _('Recommended DNS preset') ]),
								E('span', { 'class': 'routeflux-routing-choice-description' }, [
									_('Use the everyday preset: split mode plus DoH, with local names kept on the router. On OpenWrt while connected, router and LAN public DNS also follow this profile through the local Xray DNS runtime.')
								])
							])
						])
					]),
					E('p', { 'class': 'cbi-section-descr' }, [
						_('Routing keeps DNS choices intentionally small: System DNS or the Recommended DNS preset. Advanced settings are available in the CLI.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-routing-panel' }, [
					E('div', { 'class': 'routeflux-routing-editor-head' }, [
						E('span', { 'class': 'routeflux-routing-editor-kicker' }, [ _('Bypass') ]),
						E('h3', {}, [ _('Keep Direct') ]),
						E('p', { 'class': 'cbi-section-descr' }, [
							_('Add direct domains or IPv4 selectors that should stay direct while bypass mode is active. Reusable aliases stay in the CLI-only workflow.')
						])
					]),
					E('div', { 'class': 'routeflux-routing-editor-grid' }, [
						E('div', { 'class': 'cbi-value' }, [
							E('label', { 'class': 'cbi-value-title' }, [ _('Direct Domain or IPv4') ]),
							E('div', { 'class': 'routeflux-routing-inline' }, [
								E('input', {
									'class': 'cbi-input-text',
								'type': 'text',
								'placeholder': _('Examples: gosuslugi.ru 203.0.113.10 203.0.113.10-203.0.113.20'),
								'value': this.formState.bypass.selectors.selectorInput,
								'input': L.bind(function(ev) {
									this.handleSelectorInputChange(ev);
								}, this)
							}),
							E('button', {
								'class': 'cbi-button cbi-button-action',
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleAddSelector')
							}, [ _('Add Selector') ])
						])
					])
				]),
				this.renderSelectorItems(this.formState.bypass.selectors),
				E('div', { 'class': 'routeflux-routing-optional-shell' }, [
					E('details', { 'class': 'routeflux-routing-optional' }, [
						E('summary', {}, [ _('Excluded Devices') ]),
						E('div', { 'class': 'routeflux-routing-editor-grid', 'style': 'margin-top:12px;' }, [
							E('div', { 'class': 'cbi-value' }, [
								E('label', { 'class': 'cbi-value-title' }, [ _('Excluded Devices') ]),
								E('div', { 'class': 'routeflux-routing-inline' }, [
									E('input', {
										'class': 'cbi-input-text',
										'type': 'text',
										'placeholder': _('Examples: 192.168.1.50 192.168.1.0/24 192.168.1.10-192.168.1.20 all'),
										'value': this.formState.bypass.excluded.input,
										'input': L.bind(function(ev) {
											this.handleExcludedInputChange(ev);
										}, this)
									}),
									E('button', {
										'class': 'cbi-button cbi-button-action',
										'type': 'button',
										'click': ui.createHandlerFn(this, 'handleAddExcluded')
									}, [ _('Add') ])
								])
							])
						]),
						this.renderExcludedItems(this.formState.bypass.excluded)
					])
				]),
				E('div', { 'class': 'routeflux-routing-actions' }, [
					E('button', {
						'class': 'cbi-button cbi-button-apply',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleSaveSettings')
					}, [ _('Save') ])
				])
			])
		]));

		return content;
	},

	render: function(data) {
		this.initializePageState(data);
		return E('div', {
			'id': 'routeflux-routing-root',
			'class': routefluxUI.withThemeClass('routeflux-page-shell routeflux-page-shell-firewall')
		}, this.renderPageContent());
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
