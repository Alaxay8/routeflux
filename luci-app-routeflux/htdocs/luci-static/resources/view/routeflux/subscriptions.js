'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var pingSubscriptionSessionKey = 'routeflux.subscriptions.ping.latest';
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

function formatSecurityLabel(value) {
	var normalized = trim(value).toLowerCase();

	if (normalized === '')
		return '-';

	if (normalized === 'tls' || normalized === 'xtls' || normalized === 'utls')
		return normalized.toUpperCase();

	if (normalized === 'reality')
		return 'Reality';

	return titleWords(normalized.replace(/[-_]+/g, ' '));
}

function normalizeCommandError(value, fallback) {
	var text = trim(value || '');
	var lines;
	var i;

	if (text === '')
		return fallback || _('RouteFlux command failed.');

	lines = text.split(/\r?\n/);
	for (i = 0; i < lines.length; i++) {
		lines[i] = trim(lines[i]);
		if (lines[i] === '' || lines[i].indexOf('Usage:') === 0 || lines[i].indexOf('Flags:') === 0 || lines[i].indexOf('Global Flags:') === 0)
			continue;
		if (lines[i].indexOf('-h, --help') >= 0)
			continue;
		return lines[i];
	}

	return fallback || _('RouteFlux command failed.');
}

function formatBytes(value) {
	var parsed = Number(value);
	var units = [ 'B', 'KB', 'MB', 'GB', 'TB' ];
	var unit = 0;

	if (!isFinite(parsed) || parsed < 0)
		return '-';

	while (parsed >= 1024 && unit < units.length - 1) {
		parsed /= 1024;
		unit++;
	}

	return parsed.toFixed(unit === 0 ? 0 : 2) + ' ' + units[unit];
}

function trafficPresentation(subscription) {
	var traffic = subscription && subscription.traffic;
	var total = Number(traffic && traffic.total_bytes);
	var remaining = Number(traffic && traffic.remaining_bytes);
	var used = Number(traffic && traffic.used_bytes);

	if (!traffic)
		return null;

	if (traffic.unlimited === true || total === 0) {
		return {
			'unlimited': true,
			'primary': _('Unlimited'),
			'secondary': ''
		};
	}

	if (!isFinite(total) || total <= 0)
		return null;

	if (!isFinite(remaining) || remaining < 0)
		remaining = 0;

	if (!isFinite(used) || used < 0)
		used = Math.max(0, total - remaining);

	return {
		'unlimited': false,
		'primary': formatBytes(remaining) + ' / ' + formatBytes(total),
		'secondary': _('Used: %s').format(formatBytes(used)),
		'percent': Math.max(0, Math.min(100, (remaining / total) * 100))
	};
}

function renderTrafficSummary(subscription) {
	var presentation = trafficPresentation(subscription);

	if (!presentation)
		return '-';

	if (presentation.unlimited) {
		return E('div', { 'class': 'routeflux-traffic-shell routeflux-traffic-shell-unlimited' }, [
			E('div', { 'class': 'routeflux-traffic-copy' }, [
				E('div', { 'class': 'routeflux-traffic-primary' }, [ presentation.primary ])
			])
		]);
	}

	return E('div', { 'class': 'routeflux-traffic-shell' }, [
		E('div', { 'class': 'routeflux-traffic-copy' }, [
			E('div', { 'class': 'routeflux-traffic-primary' }, [ presentation.primary ]),
			E('div', { 'class': 'routeflux-traffic-secondary' }, [ presentation.secondary ])
		]),
		E('div', { 'class': 'routeflux-traffic-meter', 'title': presentation.primary }, [
			E('div', {
				'class': 'routeflux-traffic-meter-fill',
				'style': 'width:' + presentation.percent.toFixed(2) + '%'
			}, [])
		])
	]);
}

function badge(text, extraClass) {
	var className = 'label';

	if (extraClass)
		className += ' ' + extraClass;

	return E('span', { 'class': className }, [ text ]);
}

function responsiveTableCell(label, content, extraClass) {
	var className = trim(extraClass);

	if (className !== '')
		className = 'td ' + className;
	else
		className = 'td';

	return E('td', {
		'class': className,
		'data-title': trim(label)
	}, Array.isArray(content) ? content : [ content ]);
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

function compactTimestamp(value) {
	var formatted = routefluxUI.formatTimestamp(value);

	if (formatted === '')
		return '';

	return formatted.slice(0, 16);
}

function renderNodeStackCell(node) {
	var chips = [];
	var protocol = trim(node && node.protocol);
	var transport = trim(node && node.transport);
	var security = formatSecurityLabel(node && node.security);

	if (protocol !== '')
		chips.push(E('span', { 'class': 'routeflux-node-stack-chip routeflux-node-stack-chip-protocol' }, [ protocol.toUpperCase() ]));

	if (transport !== '')
		chips.push(E('span', { 'class': 'routeflux-node-stack-chip routeflux-node-stack-chip-transport' }, [ transport.toUpperCase() ]));

	if (security !== '' && security !== '-')
		chips.push(E('span', { 'class': 'routeflux-node-stack-chip routeflux-node-stack-chip-security' }, [ security ]));

	if (chips.length === 0)
		return '-';

	return E('div', { 'class': 'routeflux-node-stack routeflux-node-stack-vertical' }, chips);
}

function emptyAddDraft() {
	return {
		'source': ''
	};
}

return view.extend({
	load: function() {
		this.ensureState();
		return this.requestPageData().then(L.bind(function(data) {
			return this.applyRequestedPageData(data);
		}, this));
	},

	ensureState: function() {
		if (this.__routefluxStateInitialized === true)
			return;

		this.__routefluxStateInitialized = true;
		this.pendingActions = {};
		this.pageData = [ {}, [] ];
		this.lastGoodPageData = null;
		this.pageError = '';
		this.pageInfo = '';
		this.pageLoading = false;
		this.addDraft = emptyAddDraft();
		this.subscriptionOpen = {};
		this.livePingBySubscription = {};
	},

	setPageInfo: function(message) {
		this.pageInfo = trim(message);
	},

	setPageError: function(message) {
		this.pageError = trim(message);
	},

	clearPageMessages: function() {
		this.pageInfo = '';
		this.pageError = '';
	},

	renderIntoRoot: function() {
		var root;

		this.ensureState();
		root = document.querySelector('#routeflux-subscriptions-root');
		if (root)
			dom.content(root, this.renderPageContent(this.pageData));
	},

	normalizeRequestedPageData: function(data) {
		var statusPayload = data && data[0];
		var subscriptionsPayload = data && data[1];

		return {
			'status': statusPayload && !statusPayload.__error__ ? statusPayload : {},
			'subscriptions': Array.isArray(subscriptionsPayload) ? subscriptionsPayload : [],
			'status_error': trim(statusPayload && statusPayload.__error__),
			'subscriptions_error': trim(subscriptionsPayload && subscriptionsPayload.__error__)
		};
	},

	applyRequestedPageData: function(data) {
		var parsed;
		var fallback = { 'status': {}, 'subscriptions': [] };
		var next;
		var messages = [];

		this.ensureState();
		parsed = this.normalizeRequestedPageData(data);

		if (this.lastGoodPageData)
			fallback = this.normalizeRequestedPageData(this.lastGoodPageData);

		next = [
			parsed.status_error === '' ? parsed.status : fallback.status,
			parsed.subscriptions_error === '' ? parsed.subscriptions : fallback.subscriptions
		];

		if (!this.lastGoodPageData) {
			if (parsed.status_error !== '')
				next[0] = {};
			if (parsed.subscriptions_error !== '')
				next[1] = [];
		}

		this.pageData = next;

		if (parsed.status_error === '' && parsed.subscriptions_error === '') {
			this.lastGoodPageData = next;
			this.pageError = '';
			return this.pageData;
		}

		if (parsed.status_error !== '')
			messages.push(_('Status error: %s').format(parsed.status_error));
		if (parsed.subscriptions_error !== '')
			messages.push(_('Subscriptions error: %s').format(parsed.subscriptions_error));
		this.pageError = messages.join(' ');

		return this.pageData;
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
			var message = normalizeCommandError(stderr || stdout, _('RouteFlux command failed.'));

			if (res.code !== 0)
				throw new Error(message);

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

	execCommand: function(argv) {
		return fs.exec(routefluxBinary, argv).then(function(res) {
			var stderr = trim(res.stderr);
			var stdout = trim(res.stdout);
			var message = normalizeCommandError(stderr || stdout, _('RouteFlux command failed.'));

			if (res.code !== 0)
				throw new Error(message);

			return {
				'stdout': stdout,
				'stderr': stderr
			};
		});
	},

	autoExcludedNodeKey: function(subscriptionId, nodeId) {
		var normalizedSubscriptionId = trim(subscriptionId);
		var normalizedNodeId = trim(nodeId);

		if (normalizedSubscriptionId === '' || normalizedNodeId === '')
			return '';

		return normalizedSubscriptionId + '/' + normalizedNodeId;
	},

	normalizedAutoExcludedNodes: function(status) {
		var raw = status && status.settings && Array.isArray(status.settings.auto_excluded_nodes)
			? status.settings.auto_excluded_nodes
			: [];
		var out = [];
		var seen = {};
		var i;

		for (i = 0; i < raw.length; i++) {
			var value = trim(raw[i]);
			var cut = value.indexOf('/');
			var key;

			if (cut <= 0 || cut >= value.length - 1)
				continue;

			key = this.autoExcludedNodeKey(value.substring(0, cut), value.substring(cut + 1));
			if (key === '' || Object.prototype.hasOwnProperty.call(seen, key))
				continue;

			seen[key] = true;
			out.push(key);
		}

		return out.sort();
	},

	isNodeAutoExcluded: function(status, subscriptionId, nodeId) {
		return this.normalizedAutoExcludedNodes(status).indexOf(this.autoExcludedNodeKey(subscriptionId, nodeId)) >= 0;
	},

	autoExcludedNodesForSubscription: function(subscription, status) {
		var nodes = Array.isArray(subscription && subscription.nodes) ? subscription.nodes : [];
		var out = [];
		var i;

		for (i = 0; i < nodes.length; i++) {
			if (!this.isNodeAutoExcluded(status, subscription && subscription.id, nodes[i].id))
				continue;

			out.push(nodes[i]);
		}

		return out;
	},

	nextAutoExcludedNodes: function(status, subscriptionId, nodeId, shouldExclude) {
		var target = this.autoExcludedNodeKey(subscriptionId, nodeId);
		var current = this.normalizedAutoExcludedNodes(status);
		var filtered = current.filter(function(value) {
			return value !== target;
		});

		if (shouldExclude === true && target !== '')
			filtered.push(target);

		return filtered.sort();
	},

	handleToggleAutoExcluded: function(subscriptionId, nodeId, shouldExclude, ev) {
		var updated;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		updated = this.nextAutoExcludedNodes(this.pageData && this.pageData[0], subscriptionId, nodeId, shouldExclude);

		return this.runCLIAction(
			this.nodeAutoActionKey(subscriptionId, nodeId),
			[ 'settings', 'set', 'auto.excluded-nodes', updated.join(', ') ],
			_('Auto exclusions updated.'),
			shouldExclude === true ? _('Excluding node from Auto...') : _('Allowing node in Auto...'),
			{
				'loadingMessage': _('Reloading runtime status...')
			}
		);
	},

	actionKey: function(scope, subscriptionId, nodeId) {
		return [ trim(scope), trim(subscriptionId), trim(nodeId) ].filter(Boolean).join(':');
	},

	subscriptionActionKey: function(subscriptionId) {
		return this.actionKey('subscription', subscriptionId);
	},

	nodeActionKey: function(subscriptionId, nodeId) {
		return this.actionKey('node', subscriptionId, nodeId);
	},

	nodeAutoActionKey: function(subscriptionId, nodeId) {
		return this.actionKey('node-auto', subscriptionId, nodeId);
	},

	subscriptionPingActionKey: function(subscriptionId) {
		return this.actionKey('subscription-ping', subscriptionId);
	},

	nodePingActionKey: function(subscriptionId, nodeId) {
		return this.actionKey('node-ping', subscriptionId, nodeId);
	},

	hasPendingActionPrefix: function(prefix) {
		var actions;
		var keys;
		var i;

		this.ensureState();
		actions = this.pendingActions || {};
		keys = Object.keys(actions);
		for (i = 0; i < keys.length; i++) {
			if (keys[i].indexOf(prefix) === 0)
				return true;
		}

		return false;
	},

	pendingMessageByPrefix: function(prefix) {
		var actions;
		var keys;
		var i;

		this.ensureState();
		actions = this.pendingActions || {};
		keys = Object.keys(actions);
		for (i = 0; i < keys.length; i++) {
			if (keys[i].indexOf(prefix) === 0)
				return trim(actions[keys[i]].message);
		}

		return '';
	},

	isSubscriptionBusy: function(subscriptionId) {
		return routefluxUI.isPendingAction(this, this.subscriptionActionKey(subscriptionId)) ||
			this.hasPendingActionPrefix(this.actionKey('node', subscriptionId) + ':') ||
			this.hasPendingActionPrefix(this.actionKey('node-auto', subscriptionId) + ':');
	},

	subscriptionBusyMessage: function(subscriptionId) {
		var direct = routefluxUI.pendingActionMessage(this, this.subscriptionActionKey(subscriptionId));

		if (direct !== '')
			return direct;

		return this.pendingMessageByPrefix(this.actionKey('node', subscriptionId) + ':') ||
			this.pendingMessageByPrefix(this.actionKey('node-auto', subscriptionId) + ':');
	},

	isNodeBusy: function(subscriptionId, nodeId) {
		return routefluxUI.isPendingAction(this, this.nodeActionKey(subscriptionId, nodeId)) ||
			routefluxUI.isPendingAction(this, this.nodeAutoActionKey(subscriptionId, nodeId)) ||
			routefluxUI.isPendingAction(this, this.subscriptionActionKey(subscriptionId));
	},

	nodeBusyMessage: function(subscriptionId, nodeId) {
		var direct = routefluxUI.pendingActionMessage(this, this.nodeActionKey(subscriptionId, nodeId));

		if (direct !== '')
			return direct;

		direct = routefluxUI.pendingActionMessage(this, this.nodeAutoActionKey(subscriptionId, nodeId));
		if (direct !== '')
			return direct;

		return routefluxUI.pendingActionMessage(this, this.subscriptionActionKey(subscriptionId));
	},

	isSubscriptionPingBusy: function(subscriptionId) {
		return routefluxUI.isPendingAction(this, this.subscriptionPingActionKey(subscriptionId)) ||
			this.hasPendingActionPrefix(this.actionKey('node-ping', subscriptionId) + ':');
	},

	subscriptionPingBusyMessage: function(subscriptionId) {
		var direct = routefluxUI.pendingActionMessage(this, this.subscriptionPingActionKey(subscriptionId));

		if (direct !== '')
			return direct;

		return this.pendingMessageByPrefix(this.actionKey('node-ping', subscriptionId) + ':');
	},

	isNodePingBusy: function(subscriptionId, nodeId) {
		return routefluxUI.isPendingAction(this, this.nodePingActionKey(subscriptionId, nodeId)) ||
			routefluxUI.isPendingAction(this, this.subscriptionPingActionKey(subscriptionId));
	},

	nodePingBusyMessage: function(subscriptionId, nodeId) {
		var direct = routefluxUI.pendingActionMessage(this, this.nodePingActionKey(subscriptionId, nodeId));

		if (direct !== '')
			return direct;

		return routefluxUI.pendingActionMessage(this, this.subscriptionPingActionKey(subscriptionId));
	},

	isSubscriptionOpen: function(subscriptionId, fallback) {
		var key = trim(subscriptionId);

		if (Object.prototype.hasOwnProperty.call(this.subscriptionOpen, key))
			return this.subscriptionOpen[key] === true;

		return fallback === true;
	},

	handleSubscriptionToggle: function(subscriptionId, ev) {
		this.ensureState();
		this.subscriptionOpen[trim(subscriptionId)] = !!(ev && ev.target && ev.target.open);
	},

	handleDraftInput: function(field, ev) {
		var key = trim(field);

		this.ensureState();
		if (key === '')
			return;

		this.addDraft[key] = ev && ev.target ? ev.target.value : '';
	},

	refreshPageContent: function(options) {
		var settings = options || {};

		this.ensureState();
		this.pageLoading = settings.showLoading !== false;
		if (this.pageLoading)
			this.pageInfo = trim(settings.loadingMessage) || _('Refreshing page data...');
		this.renderIntoRoot();

		return this.requestPageData().then(L.bind(function(data) {
			this.pageLoading = false;
			this.pageInfo = '';
			this.applyRequestedPageData(data);
			this.renderIntoRoot();
			return this.pageData;
		}, this));
	},

	runAction: function(key, message, executor) {
		this.ensureState();
		this.clearPageMessages();
		return routefluxUI.runPendingAction(this, key, executor, {
			'message': message
		});
	},

	runCLIAction: function(key, argv, successMessage, pendingMessage, options) {
		var settings = options || {};

		return this.runAction(key, pendingMessage, L.bind(function() {
			return this.execCommand(argv).then(L.bind(function(res) {
				var message = trim(res.stdout) || successMessage;

				if (message !== '')
					ui.addNotification(null, notificationParagraph(message), 'info');

				if (settings.clearDraft === true)
					this.addDraft = emptyAddDraft();

				if (settings.refreshPage === false)
					return res;

				return this.refreshPageContent({
					'showLoading': true,
					'loadingMessage': settings.loadingMessage
				}).then(function() {
					return res;
				});
			}, this)).catch(L.bind(function(err) {
				var message = err.message || String(err);

				this.setPageError(message);
				ui.addNotification(null, notificationParagraph(message));
				this.renderIntoRoot();
				return null;
			}, this));
		}, this));
	},

	resolveSelectedSubscriptionId: function() {
		var select = document.querySelector('#routeflux-subscription');
		var subscriptions = Array.isArray(this.pageData && this.pageData[1]) ? this.pageData[1] : [];
		var selected = trim(select && select.value);
		var active = trim(this.pageData && this.pageData[0] && this.pageData[0].active_subscription && this.pageData[0].active_subscription.id);

		if (selected !== '')
			return selected;
		if (active !== '')
			return active;
		if (subscriptions.length > 0)
			return trim(subscriptions[0].id);

		return '';
	},

	handleAdd: function(ev) {
		var source;
		var argv;
		var message;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		source = trim(this.addDraft && this.addDraft.source);
		argv = [ 'add' ];

		if (source === '') {
			message = _('Paste a subscription URL or raw import data first.');
			this.setPageError(message);
			ui.addNotification(null, notificationParagraph(message));
			this.renderIntoRoot();
			return Promise.resolve();
		}

		if (source.match(/^https?:\/\//i))
			argv.push('--url', source);
		else
			argv.push('--raw', source);

		return this.runCLIAction(
			'add',
			argv,
			_('Subscription added.'),
			_('Adding subscription...'),
			{
				'clearDraft': true,
				'loadingMessage': _('Reloading subscriptions...')
			}
		);
	},

	handleRefreshSubscription: function(subscriptionId, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runCLIAction(
			this.subscriptionActionKey(subscriptionId),
			[ 'refresh', '--subscription', subscriptionId ],
			_('Subscription refreshed.'),
			_('Refreshing subscription...'),
			{
				'loadingMessage': _('Reloading subscriptions...')
			}
		);
	},

	handleRemoveSubscription: function(subscriptionId, displayName, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!window.confirm(_('Remove subscription "%s"?').format(displayName || subscriptionId)))
			return Promise.resolve();

		return this.runCLIAction(
			this.subscriptionActionKey(subscriptionId),
			[ 'remove', subscriptionId ],
			_('Subscription removed.'),
			_('Removing subscription...'),
			{
				'loadingMessage': _('Reloading subscriptions...')
			}
		);
	},

	handleCopySubscriptionID: function(subscriptionId, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return routefluxUI.copyValueToClipboard(subscriptionId).then(function() {
			ui.addNotification(null, notificationParagraph(_('Subscription ID copied to clipboard.')), 'info');
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err && err.message ? err.message : _('Could not copy subscription ID.')));
		});
	},

	handleRemoveAll: function(ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!window.confirm(_('Remove all imported subscriptions? This disconnects the active profile if needed.')))
			return Promise.resolve();

		return this.runCLIAction(
			'remove-all',
			[ 'remove', '--all' ],
			_('All subscriptions removed.'),
			_('Removing subscriptions...'),
			{
				'loadingMessage': _('Reloading subscriptions...')
			}
		);
	},

	handleConnectAuto: function(subscriptionId, ev) {
		var targetSubscriptionID;
		var message;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		targetSubscriptionID = trim(subscriptionId) || this.resolveSelectedSubscriptionId();
		if (targetSubscriptionID === '') {
			message = _('Choose a subscription first.');
			this.setPageError(message);
			ui.addNotification(null, notificationParagraph(message));
			this.renderIntoRoot();
			return Promise.resolve();
		}

		return this.runCLIAction(
			this.subscriptionActionKey(targetSubscriptionID),
			[ 'connect', '--auto', '--subscription', targetSubscriptionID ],
			_('Auto mode enabled.'),
			_('Connecting automatic selection...'),
			{
				'loadingMessage': _('Reloading runtime status...')
			}
		);
	},

	handleConnectNode: function(subscriptionId, nodeId, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runCLIAction(
			this.nodeActionKey(subscriptionId, nodeId),
			[ 'connect', '--subscription', subscriptionId, '--node', nodeId ],
			_('Node connected.'),
			_('Connecting node...'),
			{
				'loadingMessage': _('Reloading runtime status...')
			}
		);
	},

	handleDisconnect: function(ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runCLIAction(
			'disconnect',
			[ 'disconnect' ],
			_('Disconnected.'),
			_('Disconnecting RouteFlux...'),
			{
				'loadingMessage': _('Reloading runtime status...')
			}
		);
	},

	handleRefreshActive: function(ev) {
		var activeSubscriptionID;
		var message;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		activeSubscriptionID = trim(this.pageData && this.pageData[0] && this.pageData[0].active_subscription && this.pageData[0].active_subscription.id);
		if (activeSubscriptionID === '') {
			message = _('There is no active subscription to refresh.');
			this.setPageError(message);
			ui.addNotification(null, notificationParagraph(message));
			this.renderIntoRoot();
			return Promise.resolve();
		}

		return this.runCLIAction(
			'refresh-active',
			[ 'refresh', '--subscription', activeSubscriptionID ],
			_('Active subscription refreshed.'),
			_('Refreshing active subscription...'),
			{
				'loadingMessage': _('Reloading subscriptions...')
			}
		);
	},

	normalizePingResult: function(result) {
		var latencyMS = Number(result && result.latency_ms);

		if (!isFinite(latencyMS))
			latencyMS = null;

		return {
			'node_id': trim(result && result.node_id),
			'healthy': result && result.healthy === true,
			'latency_ms': latencyMS,
			'checked_at': trim(result && result.checked_at),
			'error': summarizePingError(result && result.error)
		};
	},

	storeLatestPingSession: function(cache) {
		if (!cache)
			return;

		routefluxUI.writeSessionJSON(pingSubscriptionSessionKey, cache);
		routefluxUI.writeSessionJSON(pingOverviewSessionKey, cache);
	},

	applyPingPayload: function(subscriptionId, payload) {
		var key = trim(subscriptionId);
		var existing = this.livePingBySubscription[key] || {};
		var resultsById = existing.results_by_id || {};
		var results = Array.isArray(payload && payload.results) ? payload.results : [];
		var i;
		var normalized;
		var cache;

		for (i = 0; i < results.length; i++) {
			normalized = this.normalizePingResult(results[i]);
			if (normalized.node_id === '')
				continue;

			resultsById[normalized.node_id] = normalized;
		}

		cache = {
			'subscription_id': key,
			'timeout_ms': Number(payload && payload.timeout_ms) || 0,
			'updated_at': (new Date()).toISOString(),
			'results_by_id': resultsById
		};

		this.livePingBySubscription[key] = cache;
		this.storeLatestPingSession(cache);
	},

	runPingAction: function(subscriptionId, nodeId, nodeCount) {
		var key = trim(nodeId) !== ''
			? this.nodePingActionKey(subscriptionId, nodeId)
			: this.subscriptionPingActionKey(subscriptionId);
		var argv = [ '--json', 'inspect', 'ping', '--subscription', subscriptionId ];
		var pendingMessage = trim(nodeId) !== ''
			? _('Checking ping...')
			: _('Checking %d nodes...').format(nodeCount);

		if (trim(nodeId) !== '')
			argv.push('--node', nodeId);

		return this.runAction(key, pendingMessage, L.bind(function() {
			return this.execJSON(argv).then(L.bind(function(payload) {
				this.applyPingPayload(subscriptionId, payload);
				this.renderIntoRoot();
				ui.addNotification(null, notificationParagraph(trim(nodeId) !== '' ? _('Ping updated.') : _('Ping check completed.')), 'info');
				return payload;
			}, this)).catch(L.bind(function(err) {
				var message = err.message || String(err);

				this.setPageError(message);
				ui.addNotification(null, notificationParagraph(message));
				this.renderIntoRoot();
				return null;
			}, this));
		}, this));
	},

	handleCheckPing: function(subscriptionId, nodeCount, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runPingAction(subscriptionId, '', Math.max(1, Number(nodeCount) || 0));
	},

	handleRecheckPing: function(subscriptionId, nodeId, ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.runPingAction(subscriptionId, nodeId, 1);
	},

	seededPingForNode: function(status, nodeId) {
		var healthMap = status && status.state && status.state.health;
		var health = healthMap && healthMap[nodeId];
		var rawLatency;
		var latencyMS;

		if (!health)
			return null;

		rawLatency = firstNonEmpty([ health.last_latency, health.average_latency ], '');
		latencyMS = routefluxUI.durationToMilliseconds(rawLatency);

		return {
			'source': 'seed',
			'healthy': health.healthy === true,
			'latency_ms': latencyMS,
			'checked_at': trim(health.last_checked_at),
			'error': summarizePingError(health.last_failure_reason)
		};
	},

	livePingForNode: function(subscriptionId, nodeId) {
		var cache = this.livePingBySubscription[trim(subscriptionId)];
		var results = cache && cache.results_by_id;

		if (!results)
			return null;

		return results[trim(nodeId)] || null;
	},

	resolvePingForNode: function(subscriptionId, nodeId, status) {
		return this.livePingForNode(subscriptionId, nodeId) || this.seededPingForNode(status, nodeId);
	},

	nodePingSortMeta: function(subscriptionId, nodeId, status) {
		var ping = this.resolvePingForNode(subscriptionId, nodeId, status);
		var latencyMS = Number(ping && ping.latency_ms);

		if (ping && ping.healthy === false) {
			return {
				'bucket': 2,
				'latency_ms': Number.POSITIVE_INFINITY
			};
		}

		if (isFinite(latencyMS) && ping && ping.healthy === true) {
			return {
				'bucket': 0,
				'latency_ms': latencyMS
			};
		}

		return {
			'bucket': 1,
			'latency_ms': Number.POSITIVE_INFINITY
		};
	},

	compareNodeTableEntries: function(left, right) {
		if (!!left.is_active !== !!right.is_active)
			return left.is_active ? -1 : 1;

		if (left.ping_sort_bucket !== right.ping_sort_bucket)
			return left.ping_sort_bucket - right.ping_sort_bucket;

		if (left.ping_sort_bucket === 0 && left.ping_latency_ms !== right.ping_latency_ms)
			return left.ping_latency_ms - right.ping_latency_ms;

		return left.original_index - right.original_index;
	},

	pingPrimaryLabel: function(result) {
		var latencyLabel = routefluxUI.formatLatencyMS(result && result.latency_ms);

		if (!result)
			return _('Not checked');

		if (result.healthy === false)
			return _('Unavailable');

		if (latencyLabel !== '')
			return latencyLabel;

		return _('Not checked');
	},

	pingStatusLabel: function(result) {
		if (!result)
			return '';

		if (result.source === 'seed')
			return _('Last known');

		return _('Current');
	},

	pingTimestampLabel: function(result) {
		var checkedAt = compactTimestamp(result && result.checked_at);

		if (checkedAt === '')
			return '';

		return checkedAt;
	},

	renderPingCell: function(subscription, node, status) {
		var ping = this.resolvePingForNode(subscription.id, node.id, status);
		var primaryClass = 'routeflux-ping-primary';
		var content;

		if (ping && ping.source === 'seed')
			primaryClass += ' routeflux-ping-primary-seed';
		else if (ping && ping.healthy === false)
			primaryClass += ' routeflux-ping-primary-down';
		else if (ping && ping.healthy === true)
			primaryClass += ' routeflux-ping-primary-live';

		content = [
			E('div', { 'class': 'routeflux-ping-cell' }, [
				E('div', { 'class': primaryClass }, [ this.pingPrimaryLabel(ping) ]),
				this.pingStatusLabel(ping) !== '' ? E('div', { 'class': 'routeflux-ping-meta routeflux-ping-meta-status' }, [ this.pingStatusLabel(ping) ]) : '',
				this.pingTimestampLabel(ping) !== '' ? E('div', { 'class': 'routeflux-ping-meta' }, [ this.pingTimestampLabel(ping) ]) : '',
				ping && ping.error ? E('div', { 'class': 'routeflux-ping-detail', 'title': ping.error }, [ ping.error ]) : ''
			])
		];

		return content;
	},

	renderAutoExclusionSummary: function(subscription, status) {
		var excludedNodes = this.autoExcludedNodesForSubscription(subscription, status);

		if (excludedNodes.length === 0)
			return '';

		return E('div', { 'class': 'routeflux-auto-exclusions' }, [
			E('div', { 'class': 'routeflux-auto-exclusions-title' }, [ _('Auto exclusions') ]),
			E('p', { 'class': 'routeflux-auto-exclusions-copy' }, [
				_('Auto mode skips these nodes when selecting the best route.')
			]),
			E('div', { 'class': 'routeflux-auto-exclusions-list' }, excludedNodes.map(function(node) {
				return E('span', { 'class': 'routeflux-auto-exclusions-pill' }, [
					nodeDisplayName(node, node.id)
				]);
			}))
		]);
	},

	renderCard: function(label, value, options) {
		var settings = options || {};

		settings.fallback = settings.fallback != null ? settings.fallback : _('Not selected');

		return routefluxUI.renderSummaryCard(label, value, settings);
	},

	renderSummarySection: function(status, presentation) {
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

		return E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), connected ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(connected),
				'primary': true
			}),
			this.renderCard(_('Effective Mode'), firstNonEmpty([ status.state && status.state.mode ], _('disconnected'))),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName),
			this.renderCard(_('Last Refresh'), routefluxUI.formatTimestamp(activeSubscription.last_updated_at) || _('Never'))
		]);
	},

	renderPageActions: function(status, subscriptions, presentation) {
		var connected = !!(status.state && status.state.connected === true);
		var activeSubscriptionID = trim(status.active_subscription && status.active_subscription.id);
		var currentSubscriptionID = activeSubscriptionID;
		var options = [];

		if (currentSubscriptionID === '' && subscriptions.length > 0)
			currentSubscriptionID = trim(subscriptions[0].id);

		for (var i = 0; i < subscriptions.length; i++) {
			var sub = subscriptions[i];
			var entry = presentationForSubscription(sub, presentation);
			var label = entry
				? entry.provider_title + ' / ' + entry.profile_label
				: providerTitle(sub) + ' / ' + _('Profile 1');
			var attrs = { 'value': trim(sub.id) };

			if (trim(sub.id) === currentSubscriptionID)
				attrs.selected = 'selected';

			options.push(E('option', attrs, [ label ]));
		}

		return E('div', { 'class': 'routeflux-surface routeflux-subscriptions-hero-controls' }, [
			E('div', { 'class': 'routeflux-section-heading' }, [
				E('div', { 'class': 'routeflux-section-heading-copy' }, [
					E('h3', {}, [ _('Quick Actions') ]),
					E('p', {}, [ _('Choose any imported profile, then connect, refresh, or disconnect from one control surface.') ])
				])
			]),
			E('div', { 'class': 'routeflux-subscriptions-hero-grid' }, [
				E('div', { 'class': 'cbi-value routeflux-subscriptions-hero-select' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-subscription' }, [ _('Subscription') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('select', {
							'id': 'routeflux-subscription',
							'disabled': subscriptions.length === 0 ? 'disabled' : null
						}, options)
					])
				]),
				E('div', { 'class': 'routeflux-page-hero-actions routeflux-subscriptions-hero-actions' }, [
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleRefreshActive'),
						'disabled': activeSubscriptionID === '' ? 'disabled' : null
					}, [ _('Refresh Active') ]),
					E('button', {
						'class': 'cbi-button cbi-button-apply',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleConnectAuto', null),
						'disabled': subscriptions.length === 0 ? 'disabled' : null
					}, [ _('Connect Auto') ]),
					E('button', {
						'class': 'cbi-button cbi-button-reset',
						'type': 'button',
						'click': ui.createHandlerFn(this, 'handleDisconnect'),
						'disabled': connected ? null : 'disabled'
					}, [ _('Disconnect') ])
				])
			])
		]);
	},

	renderNodeTable: function(subscription, activeSubscriptionId, activeNodeId, status) {
		var nodes = Array.isArray(subscription && subscription.nodes) ? subscription.nodes : [];
		var sortedEntries;

		if (nodes.length === 0)
			return E('p', {}, [ _('No nodes found in this subscription.') ]);

		sortedEntries = nodes.map(L.bind(function(node, index) {
			var isActive = subscription.id === activeSubscriptionId && node.id === activeNodeId;
			var pingSort = this.nodePingSortMeta(subscription.id, node.id, status);

			return {
				'node': node,
				'original_index': index,
				'is_active': isActive,
				'ping_sort_bucket': pingSort.bucket,
				'ping_latency_ms': pingSort.latency_ms
			};
		}, this));
		sortedEntries.sort(L.bind(this.compareNodeTableEntries, this));

		var rows = sortedEntries.map(L.bind(function(entry) {
			var node = entry.node;
			var isActive = entry.is_active;
			var autoExcluded = this.isNodeAutoExcluded(status, subscription.id, node.id);
			var nodeBusy = this.isNodeBusy(subscription.id, node.id);
			var busyMessage = this.nodeBusyMessage(subscription.id, node.id);
			var pingBusy = this.isNodePingBusy(subscription.id, node.id);
			var pingBusyMessage = this.nodePingBusyMessage(subscription.id, node.id);
			var name = nodeDisplayName(node, node.id);
			var address = firstNonEmpty([
				node.address && node.port ? node.address + ':' + node.port : '',
				node.address
			], '-');

			return E('tr', { 'class': 'tr routeflux-node-row' }, [
				responsiveTableCell(_('Node'), [
					name,
					(isActive || autoExcluded) ? E('div', { 'class': 'routeflux-node-status-badges' }, [
						isActive ? E('div', { 'class': 'routeflux-node-active-badge' }, [ badge(_('Active'), 'notice') ]) : '',
						autoExcluded ? E('div', { 'class': 'routeflux-node-auto-badge' }, [ badge(_('Auto excluded')) ]) : ''
					]) : ''
				], 'routeflux-node-cell-primary'),
				responsiveTableCell(_('Address'), address, 'routeflux-node-cell-address'),
				responsiveTableCell(_('Stack'), renderNodeStackCell(node), 'routeflux-node-cell-stack'),
				responsiveTableCell(_('Ping'), this.renderPingCell(subscription, node, status), 'routeflux-node-cell-ping'),
				responsiveTableCell(_('Actions'), [
					E('div', { 'class': 'routeflux-node-action-stack' }, [
						E('div', { 'class': 'routeflux-node-actions' }, [
							E('button', {
								'class': 'cbi-button cbi-button-action routeflux-node-button-compact',
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleConnectNode', subscription.id, node.id),
								'disabled': nodeBusy ? 'disabled' : null
							}, [ _('Connect') ])
						]),
						E('div', { 'class': 'routeflux-node-actions routeflux-node-actions-secondary' }, [
							E('button', {
								'class': 'cbi-button cbi-button-action routeflux-node-button-compact',
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleRecheckPing', subscription.id, node.id),
								'disabled': nodeBusy || pingBusy ? 'disabled' : null
							}, [ _('Recheck') ]),
							E('button', {
								'class': 'cbi-button cbi-button-action routeflux-node-button-compact ' + (autoExcluded ? 'routeflux-node-button-allow' : 'routeflux-node-button-exclude'),
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleToggleAutoExcluded', subscription.id, node.id, !autoExcluded),
								'disabled': nodeBusy ? 'disabled' : null
							}, [ autoExcluded ? _('Allow in Auto') : _('Exclude') ])
						])
					]),
					busyMessage !== '' ? E('div', { 'class': 'routeflux-action-status' }, [ busyMessage ]) : '',
					pingBusyMessage !== '' ? E('div', { 'class': 'routeflux-action-status' }, [ pingBusyMessage ]) : ''
				], 'right routeflux-node-cell-actions')
			]);
		}, this));

		return E('div', { 'class': 'routeflux-node-table-wrap' }, [
			E('table', { 'class': 'table cbi-section-table routeflux-node-table routeflux-data-table' }, [
				E('tr', { 'class': 'tr cbi-section-table-titles' }, [
					E('th', { 'class': 'th' }, [ _('Node') ]),
					E('th', { 'class': 'th' }, [ _('Address') ]),
					E('th', { 'class': 'th' }, [ _('Stack') ]),
					E('th', { 'class': 'th' }, [ _('Ping') ]),
					E('th', { 'class': 'th right routeflux-node-heading-actions' }, [
						E('span', { 'class': 'routeflux-node-heading-actions-label' }, [ _('Actions') ])
					])
				])
			].concat(rows))
		]);
	},

	renderSubscriptionCard: function(entry, activeSubscriptionId, activeNodeId, status) {
		var subscription = entry.subscription;
		var displayName = entry.profile_label;
		var providerName = entry.provider_title;
		var isActive = subscription.id === activeSubscriptionId;
		var subscriptionBusy = this.isSubscriptionBusy(subscription.id);
		var busyMessage = this.subscriptionBusyMessage(subscription.id);
		var pingBusy = this.isSubscriptionPingBusy(subscription.id);
		var pingBusyMessage = this.subscriptionPingBusyMessage(subscription.id);
		var nodesCount = Array.isArray(subscription.nodes) ? subscription.nodes.length : 0;
		var metaRows = [
			[ _('ID'), E('div', { 'class': 'routeflux-meta-copy-shell' }, [
				E('span', { 'class': 'routeflux-meta-copy-value' }, [ subscription.id ]),
				E('button', {
					'class': 'cbi-button cbi-button-action routeflux-meta-copy-button',
					'type': 'button',
					'title': _('Copy ID'),
					'aria-label': _('Copy ID'),
					'click': ui.createHandlerFn(this, 'handleCopySubscriptionID', subscription.id)
				}, [ '\u29c9' ])
			]) ],
			[ _('Provider'), providerName ],
			[ _('Profile'), displayName ],
			[ _('Updated'), routefluxUI.formatTimestamp(subscription.last_updated_at) || _('Never') ],
			[ _('Remaining traffic'), renderTrafficSummary(subscription) ],
			[ _('Expiration date'), routefluxUI.formatTimestamp(subscription.expires_at) || '-' ],
			[ _('Status'), firstNonEmpty([ subscription.parser_status ], _('unknown')) ],
			[ _('Nodes'), String(nodesCount) ]
		].map(function(item) {
			return E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td left routeflux-meta-label' }, [ item[0] ]),
				E('td', { 'class': 'td left routeflux-meta-value' }, [ item[1] ])
			]);
		});

		var heading = [
			E('div', { 'class': 'routeflux-subscription-title' }, [ displayName ]),
			E('div', { 'class': 'routeflux-subscription-provider' }, [ providerName ])
		];

		if (isActive)
			heading.push(E('div', { 'class': 'routeflux-subscription-badges' }, [ badge(_('Active'), 'notice') ]));

		return E('section', { 'class': 'cbi-section routeflux-surface routeflux-subscription-card' + (isActive ? ' routeflux-subscription-card-active' : '') }, [
			E('div', { 'class': 'routeflux-subscription-header' }, [
				E('div', { 'class': 'routeflux-subscription-heading' }, heading),
				E('div', { 'class': 'routeflux-subscription-controls' }, [
					E('div', { 'class': 'routeflux-subscription-actions' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleRefreshSubscription', subscription.id),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Refresh') ]),
						E('button', {
							'class': 'cbi-button cbi-button-apply',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleConnectAuto', subscription.id),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Connect Auto') ]),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleCheckPing', subscription.id, nodesCount),
							'disabled': subscriptionBusy || pingBusy ? 'disabled' : null
						}, [ _('Check Ping') ]),
						E('button', {
							'class': 'cbi-button cbi-button-negative',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleRemoveSubscription', subscription.id, displayName),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Remove') ])
					]),
					busyMessage !== '' ? E('div', { 'class': 'routeflux-action-status routeflux-action-status-group' }, [ busyMessage ]) : '',
					pingBusyMessage !== '' ? E('div', { 'class': 'routeflux-action-status routeflux-action-status-group routeflux-ping-status-group' }, [ pingBusyMessage ]) : ''
				])
			]),
			E('table', { 'class': 'table routeflux-meta-table routeflux-data-table' }, metaRows),
			this.renderAutoExclusionSummary(subscription, status),
			trim(subscription.last_error) !== '' ? E('div', { 'class': 'alert-message warning', 'style': 'margin-top:10px' }, [
				subscription.last_error
			]) : '',
			E('details', {
				'class': 'routeflux-node-details',
				'open': this.isSubscriptionOpen(subscription.id, isActive) ? 'open' : null,
				'toggle': L.bind(function(ev) {
					this.handleSubscriptionToggle(subscription.id, ev);
				}, this)
			}, [
				E('summary', { 'class': 'routeflux-section-heading' }, [
					E('span', { 'class': 'routeflux-section-heading-copy' }, [
						E('h3', {}, [ _('Nodes (%d)').format(nodesCount) ])
					])
				]),
				this.renderNodeTable(subscription, activeSubscriptionId, activeNodeId, status)
			])
		]);
	},

	renderProviderGroup: function(group, activeSubscriptionId, activeNodeId, status) {
		var description = _('%d profile(s), %d node(s)').format(group.items.length, group.total_nodes);
		var content = [
			E('div', { 'class': 'routeflux-provider-group-header' }, [
				E('div', { 'class': 'routeflux-provider-group-title' }, [ group.title ]),
				E('div', { 'class': 'routeflux-provider-group-meta' }, [ description ])
			])
		];

		for (var i = 0; i < group.items.length; i++)
			content.push(this.renderSubscriptionCard(group.items[i], activeSubscriptionId, activeNodeId, status));

		return E('div', { 'class': 'routeflux-provider-group' }, content);
	},

	renderPageContent: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var presentation = buildSubscriptionPresentation(subscriptions);
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
		var activeSubscriptionId = trim(status.active_subscription && status.active_subscription.id);
		var activeNodeId = trim(status.active_node && status.active_node.id);
		var addBusy = routefluxUI.isPendingAction(this, 'add');
		var addBusyMessage = routefluxUI.pendingActionMessage(this, 'add');
		var removeAllBusy = routefluxUI.isPendingAction(this, 'remove-all');
		var removeAllMessage = routefluxUI.pendingActionMessage(this, 'remove-all');
		var addActionMessage = addBusyMessage || removeAllMessage;
		var content = [];
		var totalNodes = 0;

		for (var groupIndex = 0; groupIndex < presentation.groups.length; groupIndex++)
			totalNodes += Number(presentation.groups[groupIndex].total_nodes) || 0;

		this.ensureState();
		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-subscriptions-shell { width:100%; max-width:100%; min-width:0; }',
			'.routeflux-subscriptions-hero { margin-bottom:18px; }',
			'.routeflux-subscriptions-hero-controls { margin:0; padding:18px; }',
			'.routeflux-subscriptions-hero-grid { display:grid; gap:12px; }',
			'.routeflux-subscriptions-hero-select { margin:0; }',
			'.routeflux-subscriptions-hero-actions { grid-template-columns:repeat(3, minmax(0, 1fr)); }',
			'.routeflux-subscription-card { margin-bottom:16px; padding:22px; }',
			'.routeflux-subscription-card-active { border-color:rgba(88, 196, 255, 0.3); box-shadow:0 22px 40px rgba(0, 0, 0, 0.26), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
			'.routeflux-subscription-header { display:grid; grid-template-columns:minmax(0, 1fr) auto; gap:16px 18px; align-items:start; margin-bottom:16px; }',
			'.routeflux-subscription-heading { min-width:0; }',
			'.routeflux-subscription-title { color:var(--routeflux-text-primary); font-size:clamp(24px, 1vw + 20px, 34px); font-weight:700; line-height:1.08; letter-spacing:-0.04em; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-subscription-provider { color:var(--routeflux-text-muted); margin-top:6px; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-subscription-badges { display:flex; flex-wrap:wrap; gap:8px; margin-top:10px; }',
			'.routeflux-auto-exclusions { margin:14px 0 0; padding:14px 16px; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(8, 15, 26, 0.38); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-auto-exclusions-title { color:var(--routeflux-text-primary); font-size:12px; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }',
			'.routeflux-auto-exclusions-copy { margin:8px 0 0; color:var(--routeflux-text-muted); line-height:1.55; }',
			'.routeflux-auto-exclusions-list { display:flex; flex-wrap:wrap; gap:8px; margin-top:12px; }',
			'.routeflux-auto-exclusions-pill { display:inline-flex; align-items:center; min-height:30px; padding:0 12px; border:1px solid rgba(245, 158, 11, 0.18); border-radius:999px; background:rgba(245, 158, 11, 0.1); color:#fde68a; font-size:12px; font-weight:700; letter-spacing:.01em; }',
			'.routeflux-subscription-controls { display:grid; gap:10px; justify-items:end; min-width:0; max-width:100%; }',
			'.routeflux-subscription-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:10px; align-items:flex-start; max-width:100%; }',
			'.routeflux-subscription-actions .cbi-button, .routeflux-node-actions .cbi-button { white-space:nowrap; }',
			'.routeflux-meta-table { width:100%; table-layout:fixed; margin-bottom:0; }',
			'.routeflux-meta-label { width:180px; color:var(--routeflux-text-muted); font-weight:700; }',
			'.routeflux-meta-value { overflow-wrap:anywhere; word-break:break-word; color:var(--routeflux-text-primary); }',
			'.routeflux-meta-copy-shell { display:flex; align-items:center; gap:8px; min-width:0; }',
			'.routeflux-meta-copy-value { min-width:0; flex:1 1 auto; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-meta-copy-button { min-width:34px; width:34px; height:34px; padding:0; border-radius:10px; font-size:16px; line-height:1; display:inline-flex; align-items:center; justify-content:center; flex:0 0 auto; }',
			'.routeflux-traffic-shell { display:grid; gap:8px; min-width:0; }',
			'.routeflux-traffic-copy { display:flex; flex-wrap:wrap; gap:6px 10px; align-items:baseline; min-width:0; }',
			'.routeflux-traffic-primary { color:var(--routeflux-text-primary); font-weight:700; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-traffic-secondary { color:var(--routeflux-text-muted); font-size:12px; line-height:1.45; }',
			'.routeflux-traffic-meter { position:relative; width:min(100%, 260px); max-width:100%; height:10px; border-radius:999px; background:rgba(145, 175, 220, 0.12); overflow:hidden; box-shadow:inset 0 1px 1px rgba(0, 0, 0, 0.28); }',
			'.routeflux-traffic-meter-fill { height:100%; border-radius:inherit; background:linear-gradient(90deg, var(--routeflux-success) 0%, #6ef3cc 100%); box-shadow:0 0 16px rgba(46, 216, 170, 0.3); }',
			'.routeflux-traffic-shell-unlimited .routeflux-traffic-primary { color:#bdffe7; }',
			'.routeflux-node-details { margin-top:16px; }',
			'.routeflux-node-details summary { cursor:pointer; list-style:none; margin-bottom:12px; }',
			'.routeflux-node-details summary::-webkit-details-marker { display:none; }',
			'.routeflux-node-table-wrap { width:100%; max-width:100%; overflow-x:visible; }',
			'.routeflux-node-table { width:100%; min-width:0; table-layout:fixed; }',
			'.routeflux-node-table .th, .routeflux-node-table .td { vertical-align:top; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-node-table .th:nth-child(1), .routeflux-node-table .td:nth-child(1) { width:24%; }',
			'.routeflux-node-table .th:nth-child(2), .routeflux-node-table .td:nth-child(2) { width:24%; }',
			'.routeflux-node-table .th:nth-child(3), .routeflux-node-table .td:nth-child(3) { width:14%; }',
			'.routeflux-node-table .th:nth-child(4), .routeflux-node-table .td:nth-child(4) { width:22%; }',
			'.routeflux-node-table .th:nth-child(5), .routeflux-node-table .td:nth-child(5) { width:16%; }',
			'.routeflux-node-heading-actions, .routeflux-node-cell-actions { text-align:right; padding-right:14px; box-sizing:border-box; }',
			'.routeflux-node-heading-actions-label { display:grid; justify-items:center; width:132px; max-width:100%; margin-left:auto; text-align:center; transform:translateX(-6px); }',
			'.routeflux-node-action-stack { display:grid; gap:10px; width:132px; max-width:100%; margin-left:auto; }',
			'.routeflux-node-actions { display:grid; grid-template-columns:minmax(0, 1fr); width:100%; gap:8px; }',
			'.routeflux-node-actions-secondary { margin-top:0; }',
			'.routeflux-node-cell-address { color:var(--routeflux-text-primary); font-weight:600; line-height:1.5; }',
			'.routeflux-node-cell-stack { min-width:0; }',
			'.routeflux-node-status-badges { display:flex; flex-wrap:wrap; gap:8px; margin-top:10px; }',
			'.routeflux-node-stack { display:grid; gap:6px; justify-items:start; }',
			'.routeflux-node-stack-vertical { grid-auto-flow:row; }',
			'.routeflux-node-stack-chip { display:flex; align-items:center; justify-content:center; min-height:28px; padding:0 11px; border:1px solid transparent; border-radius:999px; font-size:11px; font-weight:700; letter-spacing:.03em; box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
			'.routeflux-node-stack-chip-protocol { background:rgba(70, 170, 235, 0.18); border-color:rgba(111, 202, 255, 0.16); color:#c9ecff; }',
			'.routeflux-node-stack-chip-transport { background:rgba(117, 137, 176, 0.18); border-color:rgba(154, 182, 228, 0.14); color:#dae6f8; }',
			'.routeflux-node-stack-chip-security { background:rgba(44, 173, 133, 0.18); border-color:rgba(103, 233, 197, 0.14); color:#c8fff0; }',
			'.routeflux-node-auto-badge .label { border-color:rgba(245, 158, 11, 0.2); background:rgba(245, 158, 11, 0.12); color:#fde68a; }',
			'.routeflux-theme-light .routeflux-subscription-card-active { border-color:rgba(37, 99, 235, 0.18); box-shadow:0 16px 30px rgba(63, 87, 118, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-theme-light .routeflux-subscription-provider { color:#52667c; }',
			'.routeflux-theme-light .routeflux-subscription-badges .label.notice, .routeflux-theme-light .routeflux-node-active-badge .label.notice { border-color:rgba(22, 163, 74, 0.22); background:rgba(22, 163, 74, 0.1); color:#166534; }',
			'.routeflux-theme-light .routeflux-auto-exclusions { border-color:rgba(245, 158, 11, 0.16); background:linear-gradient(180deg, rgba(255, 251, 235, 0.98) 0%, rgba(255, 247, 237, 0.98) 100%); box-shadow:0 10px 22px rgba(148, 163, 184, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.9); }',
			'.routeflux-theme-light .routeflux-auto-exclusions-title { color:#9a3412; }',
			'.routeflux-theme-light .routeflux-auto-exclusions-copy { color:#7c5a37; }',
			'.routeflux-theme-light .routeflux-auto-exclusions-pill { border-color:rgba(245, 158, 11, 0.18); background:rgba(245, 158, 11, 0.12); color:#9a3412; }',
			'.routeflux-theme-light .routeflux-provider-group-header { padding:12px 14px; border:1px solid rgba(125, 146, 170, 0.14); border-radius:16px; background:linear-gradient(180deg, rgba(250, 252, 254, 0.96) 0%, rgba(243, 247, 251, 0.96) 100%); box-shadow:0 10px 20px rgba(63, 87, 118, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.88); }',
			'.routeflux-theme-light .routeflux-provider-group-title { color:#162638; }',
			'.routeflux-theme-light .routeflux-provider-group-meta { color:#52667c; }',
			'.routeflux-theme-light .routeflux-node-table { background:rgba(249, 251, 253, 0.92); border-color:rgba(125, 146, 170, 0.18); }',
			'.routeflux-theme-light .routeflux-node-table .th { background:rgba(125, 146, 170, 0.08); color:#5c7085; }',
			'.routeflux-theme-light .routeflux-node-table .td { color:#162638; }',
			'.routeflux-theme-light .routeflux-traffic-meter { background:rgba(125, 146, 170, 0.16); box-shadow:inset 0 1px 1px rgba(125, 146, 170, 0.1); }',
			'.routeflux-theme-light .routeflux-traffic-shell-unlimited .routeflux-traffic-primary { color:#166534; }',
			'.routeflux-theme-light .routeflux-node-stack-chip-protocol { background:rgba(14, 165, 233, 0.12); border-color:rgba(14, 165, 233, 0.18); color:#075985; }',
			'.routeflux-theme-light .routeflux-node-stack-chip-transport { background:rgba(100, 116, 139, 0.12); border-color:rgba(100, 116, 139, 0.18); color:#334155; }',
			'.routeflux-theme-light .routeflux-node-stack-chip-security { background:rgba(16, 185, 129, 0.12); border-color:rgba(16, 185, 129, 0.18); color:#047857; }',
			'.routeflux-theme-light .routeflux-node-auto-badge .label { border-color:rgba(245, 158, 11, 0.18); background:rgba(245, 158, 11, 0.12); color:#9a3412; }',
			'.routeflux-theme-light .routeflux-subscription-actions .cbi-button-action, .routeflux-theme-light .routeflux-node-actions .cbi-button-action { border-color:rgba(37, 99, 235, 0.18); background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b; box-shadow:0 12px 22px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
			'.routeflux-theme-light .routeflux-subscription-actions .cbi-button-action:hover, .routeflux-theme-light .routeflux-node-actions .cbi-button-action:hover { border-color:rgba(37, 99, 235, 0.28); background:linear-gradient(180deg, rgba(236, 244, 251, 0.99) 0%, rgba(225, 236, 247, 0.99) 100%); color:#102f4c; }',
			'.routeflux-theme-light .routeflux-subscription-actions .cbi-button-apply { border-color:rgba(37, 99, 235, 0.34); background:linear-gradient(180deg, #2563eb 0%, #1d4ed8 100%); color:#f8fbff; box-shadow:0 14px 28px rgba(37, 99, 235, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.16); }',
			'.routeflux-theme-light .routeflux-subscription-actions .cbi-button-apply:hover { border-color:rgba(29, 78, 216, 0.42); background:linear-gradient(180deg, #1d4ed8 0%, #1e40af 100%); color:#ffffff; box-shadow:0 16px 30px rgba(29, 78, 216, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.18); }',
			'.routeflux-node-button-exclude { border-color:rgba(245, 158, 11, 0.22); background:linear-gradient(180deg, rgba(68, 45, 12, 0.9) 0%, rgba(54, 36, 11, 0.94) 100%); color:#fcd34d; }',
			'.routeflux-node-button-exclude:hover { border-color:rgba(245, 158, 11, 0.3); background:linear-gradient(180deg, rgba(92, 58, 15, 0.96) 0%, rgba(68, 45, 12, 0.98) 100%); color:#fde68a; }',
			'.routeflux-node-button-allow { border-color:rgba(34, 197, 94, 0.2); background:linear-gradient(180deg, rgba(18, 62, 39, 0.9) 0%, rgba(14, 46, 29, 0.94) 100%); color:#bbf7d0; }',
			'.routeflux-node-button-allow:hover { border-color:rgba(34, 197, 94, 0.3); background:linear-gradient(180deg, rgba(26, 86, 52, 0.96) 0%, rgba(18, 62, 39, 0.98) 100%); color:#dcfce7; }',
			'.routeflux-theme-light .routeflux-node-button-exclude { border-color:rgba(245, 158, 11, 0.18); background:linear-gradient(180deg, rgba(255, 251, 235, 0.98) 0%, rgba(255, 247, 237, 0.98) 100%); color:#9a3412; }',
			'.routeflux-theme-light .routeflux-node-button-exclude:hover { border-color:rgba(245, 158, 11, 0.28); background:linear-gradient(180deg, rgba(255, 247, 237, 1) 0%, rgba(255, 237, 213, 0.98) 100%); color:#7c2d12; }',
			'.routeflux-theme-light .routeflux-node-button-allow { border-color:rgba(22, 163, 74, 0.18); background:linear-gradient(180deg, rgba(240, 253, 244, 0.98) 0%, rgba(220, 252, 231, 0.98) 100%); color:#166534; }',
			'.routeflux-theme-light .routeflux-node-button-allow:hover { border-color:rgba(22, 163, 74, 0.28); background:linear-gradient(180deg, rgba(220, 252, 231, 0.98) 0%, rgba(187, 247, 208, 0.98) 100%); color:#14532d; }',
			'.routeflux-theme-light .routeflux-add-field-shell { border-color:rgba(125, 146, 170, 0.18); background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.86), 0 10px 20px rgba(63, 87, 118, 0.06); }',
			'.routeflux-theme-light .routeflux-add-format-badge { border-color:rgba(125, 146, 170, 0.16); background:rgba(37, 99, 235, 0.08); color:#294861; }',
			'.routeflux-theme-light .routeflux-add-kicker { background:rgba(37, 99, 235, 0.08); color:#1d4ed8; }',
			'.routeflux-node-cell-ping { width:23%; }',
			'.routeflux-ping-cell { display:grid; gap:6px; }',
			'.routeflux-ping-primary { color:var(--routeflux-text-primary); font-size:13px; font-weight:700; }',
			'.routeflux-ping-primary-live { color:#c6fff0; }',
			'.routeflux-ping-primary-down { color:#ffc7ce; }',
			'.routeflux-ping-primary-seed { color:#d6e2f3; }',
			'.routeflux-theme-light .routeflux-ping-primary-live { color:#0f766e; }',
			'.routeflux-theme-light .routeflux-ping-primary-down { color:#b91c1c; }',
			'.routeflux-theme-light .routeflux-ping-primary-seed { color:#475569; }',
			'.routeflux-ping-meta, .routeflux-ping-detail { color:var(--routeflux-text-muted); font-size:11px; line-height:1.42; }',
			'.routeflux-ping-meta-status { text-transform:uppercase; letter-spacing:.08em; font-weight:700; }',
			'.routeflux-ping-detail { overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-ping-actions { display:flex; justify-content:flex-start; width:100%; }',
			'.routeflux-node-button-compact { width:100%; min-height:32px; padding:0 10px; font-size:12px; }',
			'.routeflux-action-status { margin-top:8px; color:var(--routeflux-text-muted); font-size:12px; line-height:1.45; }',
			'.routeflux-action-status-group { width:100%; text-align:right; }',
			'.routeflux-ping-status-group { color:#bfe8ff; }',
			'.routeflux-theme-light .routeflux-ping-status-group { color:#1d4ed8; }',
			'.routeflux-page-status { margin-bottom:18px; }',
			'.routeflux-page-status-actions { display:flex; justify-content:flex-end; margin-top:10px; }',
			'.routeflux-add-panel { position:relative; overflow:hidden; padding:20px; }',
			'.routeflux-add-panel > * { position:relative; z-index:1; }',
			'.routeflux-add-panel-head { display:grid; gap:8px; margin-bottom:14px; }',
			'.routeflux-add-kicker { display:inline-flex; align-items:center; width:max-content; max-width:100%; padding:5px 11px; border-radius:999px; background:rgba(88, 196, 255, 0.12); color:#9ddfff; font-size:11px; font-weight:800; letter-spacing:.12em; text-transform:uppercase; }',
			'.routeflux-add-panel-head h3 { margin:0; font-size:clamp(22px, 0.85vw + 18px, 30px); letter-spacing:-0.03em; }',
			'.routeflux-add-panel-copy { margin:0; color:var(--routeflux-text-muted); line-height:1.68; max-width:72ch; }',
			'.routeflux-add-grid { display:grid; grid-template-columns:minmax(0, 1fr); gap:14px; margin-bottom:12px; }',
			'.routeflux-add-field { min-width:0; }',
			'.routeflux-add-field-label { display:block; margin-bottom:8px; color:var(--routeflux-text-secondary); font-size:13px; font-weight:800; letter-spacing:.01em; }',
			'.routeflux-add-field-shell { position:relative; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(6, 12, 22, 0.72); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.03), 0 12px 24px rgba(0, 0, 0, 0.14); transition:border-color .18s ease, box-shadow .18s ease, transform .18s ease; }',
			'.routeflux-add-field-shell:focus-within { border-color:rgba(88, 196, 255, 0.52); box-shadow:0 0 0 1px rgba(88, 196, 255, 0.18), 0 18px 34px rgba(10, 18, 34, 0.24); transform:translateY(-1px); }',
			'.routeflux-add-grid .cbi-value-field, .routeflux-add-grid .cbi-input-text, .routeflux-add-grid .cbi-input-textarea { width:100%; max-width:100%; box-sizing:border-box; }',
			'.routeflux-add-grid .cbi-input-textarea { display:block; min-height:168px; padding:16px 18px; border:0; border-radius:18px; background:transparent; color:var(--routeflux-text-primary); line-height:1.6; resize:vertical; box-shadow:none; }',
			'.routeflux-add-grid .cbi-input-textarea::placeholder { color:var(--routeflux-text-muted); opacity:0.9; }',
			'.routeflux-add-grid .cbi-input-textarea:focus { outline:none; box-shadow:none; }',
			'.routeflux-add-format-list { display:flex; flex-wrap:wrap; gap:8px; margin:12px 0 10px; }',
			'.routeflux-add-format-badge { display:inline-flex; align-items:center; min-height:30px; padding:0 12px; border-radius:999px; border:1px solid rgba(145, 175, 220, 0.14); background:rgba(145, 175, 220, 0.08); color:var(--routeflux-text-secondary); font-size:12px; font-weight:700; letter-spacing:.01em; }',
			'.routeflux-add-hint { margin:0; color:var(--routeflux-text-muted); line-height:1.65; }',
			'.routeflux-add-actions { display:flex; flex-wrap:wrap; gap:10px; margin-top:16px; }',
			'.routeflux-provider-group { margin-bottom:22px; }',
			'.routeflux-provider-group-header { display:grid; grid-template-columns:minmax(0, 1fr) auto; gap:8px 12px; align-items:end; margin:12px 0 10px; }',
			'.routeflux-provider-group-title { color:var(--routeflux-text-primary); font-size:clamp(26px, 1.4vw + 18px, 38px); font-weight:700; line-height:1.02; letter-spacing:-0.05em; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-provider-group-meta { color:var(--routeflux-text-muted); }',
			'@media (max-width: 980px) { .routeflux-subscriptions-hero-actions, .routeflux-subscription-header, .routeflux-provider-group-header, .routeflux-add-grid { grid-template-columns:minmax(0, 1fr); } .routeflux-subscription-controls { justify-items:stretch; min-width:0; } .routeflux-subscription-actions, .routeflux-ping-actions { justify-content:flex-start; } .routeflux-action-status-group { text-align:left; } .routeflux-node-table .th, .routeflux-node-table .td { padding-left:6px; padding-right:6px; } .routeflux-node-heading-actions, .routeflux-node-cell-actions { padding-right:6px; } .routeflux-node-heading-actions-label, .routeflux-node-action-stack { width:126px; } .routeflux-node-button-compact { min-height:30px; padding:0 8px; font-size:11px; } }',
			'@media (max-width: 700px) { .routeflux-page-status-actions, .routeflux-add-actions { flex-direction:column; } .routeflux-page-status-actions .cbi-button, .routeflux-add-actions .cbi-button { width:100%; } .routeflux-meta-table, .routeflux-meta-table .tr, .routeflux-meta-table .td { display:block; width:100%; box-sizing:border-box; } .routeflux-subscription-card .routeflux-meta-table, .routeflux-subscription-card .routeflux-meta-table .tr, .routeflux-subscription-card .routeflux-meta-table .td, .routeflux-subscription-card .routeflux-meta-table .td.left { text-align:center !important; } .routeflux-meta-table .tr { padding:10px 0; border-top:1px solid rgba(145, 175, 220, 0.1); text-align:center; } .routeflux-meta-table .tr:first-child { padding-top:0; border-top:0; } .routeflux-meta-table .td.routeflux-meta-label, .routeflux-subscription-card .routeflux-meta-table .td.routeflux-meta-label.left { width:100%; padding-bottom:4px; text-align:center !important; } .routeflux-meta-table .td.routeflux-meta-value, .routeflux-subscription-card .routeflux-meta-table .td.routeflux-meta-value.left { padding-top:0; text-align:center !important; } .routeflux-meta-copy-shell { display:grid; justify-items:center; gap:10px; width:100%; } .routeflux-meta-copy-value { width:100%; text-align:center; } .routeflux-meta-copy-button { margin:0 auto; } .routeflux-add-panel { padding:16px; border-radius:18px; } .routeflux-add-grid .cbi-input-textarea { min-height:152px; padding:14px 15px; } }',
			'@media (max-width: 560px) { .routeflux-subscriptions-hero, .routeflux-subscription-card, .routeflux-provider-group-header, .routeflux-auto-exclusions, .routeflux-node-details summary { text-align:center; } .routeflux-overview-grid { justify-items:center; } .routeflux-overview-grid .routeflux-card { width:100%; text-align:center; } .routeflux-overview-grid .routeflux-card-accent, .routeflux-overview-grid .routeflux-card-label, .routeflux-overview-grid .routeflux-card-value { text-align:center; justify-self:center; margin-left:auto; margin-right:auto; } .routeflux-page-hero-copy, .routeflux-subscription-heading, .routeflux-provider-group-meta, .routeflux-traffic-shell, .routeflux-traffic-copy { text-align:center; justify-items:center; justify-content:center; } .routeflux-page-hero-meta, .routeflux-subscription-controls { justify-items:center; } .routeflux-page-hero-meta-item, .routeflux-page-hero-meta-label, .routeflux-page-hero-meta-value, .routeflux-action-status-group { text-align:center; justify-self:center; } .routeflux-subscription-badges, .routeflux-node-status-badges, .routeflux-auto-exclusions-list, .routeflux-subscription-actions, .routeflux-ping-actions, .routeflux-node-actions { justify-content:center; } .routeflux-subscription-actions, .routeflux-ping-actions, .routeflux-node-actions { flex-direction:column; align-items:stretch; width:100%; } .routeflux-subscription-actions .cbi-button, .routeflux-ping-actions .cbi-button, .routeflux-node-actions .cbi-button { width:100%; } .routeflux-traffic-meter, .routeflux-node-action-stack, .routeflux-node-heading-actions-label { margin-left:auto; margin-right:auto; } .routeflux-node-heading-actions, .routeflux-node-cell-actions { text-align:center; padding-right:0; } .routeflux-node-table, .routeflux-node-table .tr, .routeflux-node-table .td { display:block; width:100%; box-sizing:border-box; } .routeflux-node-table { min-width:0; } .routeflux-node-table .cbi-section-table-titles { display:none; } .routeflux-node-table .routeflux-node-row { margin-bottom:12px; padding:12px 14px; border:1px solid rgba(145, 175, 220, 0.12); border-radius:16px; background:rgba(8, 15, 26, 0.5); box-shadow:0 10px 18px rgba(0, 0, 0, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.03); text-align:center; } .routeflux-theme-light .routeflux-node-table .routeflux-node-row { background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%); box-shadow:0 10px 20px rgba(63, 87, 118, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.88); } .routeflux-node-table .routeflux-node-row:last-child { margin-bottom:0; } .routeflux-node-table .routeflux-node-row > .td { width:100%; min-width:0; padding:8px 0; border-top:1px solid rgba(145, 175, 220, 0.08); text-align:center; } .routeflux-node-table .routeflux-node-row > .td:first-child { padding-top:0; border-top:0; } .routeflux-node-table .routeflux-node-row > .td:last-child { padding-bottom:0; } .routeflux-node-table .routeflux-node-row > .td::before { content:attr(data-title); display:block; margin-bottom:4px; color:var(--routeflux-text-muted); font-size:10px; text-transform:uppercase; letter-spacing:.12em; font-weight:700; white-space:nowrap; text-align:center; } .routeflux-node-stack, .routeflux-node-stack-vertical { justify-items:center; } .routeflux-ping-cell, .routeflux-node-action-stack { justify-items:center; } .routeflux-node-action-stack { margin:0 auto; } .routeflux-node-stack-chip { width:max-content; min-width:78px; max-width:100%; justify-content:center; } }'
		]));

		content.push(E('section', { 'class': 'routeflux-page-hero routeflux-surface routeflux-surface-elevated routeflux-subscriptions-hero' }, [
			E('div', { 'class': 'routeflux-page-hero-copy' }, [
				E('span', { 'class': 'routeflux-page-kicker' }, [ _('Subscriptions') ]),
				E('h2', { 'class': 'routeflux-page-hero-title' }, [ _('RouteFlux - Subscriptions') ]),
				E('p', { 'class': 'routeflux-page-hero-description' }, [
					_('RouteFlux status, the active connection, and the basic subscription actions you need every day.')
				]),
				E('div', { 'class': 'routeflux-page-hero-meta' }, [
					E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
						E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Active Provider') ]),
						E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ activeProvider ])
					]),
					E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
						E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Active Profile') ]),
						E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ activeProfile ])
					]),
					E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
						E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Active Node') ]),
						E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ activeNodeName ])
					]),
					E('div', { 'class': 'routeflux-page-hero-meta-item' }, [
						E('div', { 'class': 'routeflux-page-hero-meta-label' }, [ _('Inventory') ]),
						E('div', { 'class': 'routeflux-page-hero-meta-value' }, [ _('%d profile(s), %d node(s)').format(subscriptions.length, totalNodes) ])
					])
				])
			]),
			E('div', { 'class': 'routeflux-page-hero-actions' }, [
				this.renderPageActions(status, subscriptions, presentation)
			])
		]));

		if (this.pageInfo !== '' || this.pageError !== '') {
			content.push(E('div', { 'class': 'cbi-section routeflux-surface routeflux-page-status' }, [
				this.pageInfo !== '' ? E('div', { 'class': 'routeflux-page-banner routeflux-page-banner-info' }, [ this.pageInfo ]) : '',
				this.pageError !== '' ? E('div', { 'class': 'routeflux-page-banner routeflux-page-banner-warning' }, [ this.pageError ]) : '',
				this.pageError !== '' ? E('div', { 'class': 'routeflux-page-status-actions' }, [
					E('button', {
						'class': 'cbi-button',
						'type': 'button',
						'click': ui.createHandlerFn(this, function() {
							return this.refreshPageContent({
								'showLoading': true,
								'loadingMessage': _('Retrying page load...')
							});
						})
					}, [ _('Retry') ])
				]) : ''
			]));
		}

		content.push(this.renderSummarySection(status, presentation));

		content.push(E('div', { 'class': 'cbi-section routeflux-surface routeflux-add-panel' }, [
			E('div', { 'class': 'routeflux-add-panel-head' }, [
				E('span', { 'class': 'routeflux-add-kicker' }, [ _('Import') ]),
				E('h3', {}, [ _('Add Subscription') ]),
				E('p', { 'class': 'routeflux-add-panel-copy' }, [
					_('Drop in the source exactly as you received it. RouteFlux will detect the format and normalize it into router-ready profiles.')
				])
			]),
			E('div', { 'class': 'routeflux-add-grid' }, [
				E('div', { 'class': 'routeflux-add-field' }, [
					E('label', { 'class': 'routeflux-add-field-label', 'for': 'routeflux-add-source' }, [ _('Subscription URL or raw import data') ]),
					E('div', { 'class': 'routeflux-add-field-shell' }, [
						E('textarea', {
							'id': 'routeflux-add-source',
							'class': 'cbi-input-textarea',
							'placeholder': _('Paste an http(s) subscription URL, VLESS/VMess/Trojan/SS links, base64 payload, or Xray/3x-ui JSON.'),
							'input': L.bind(function(ev) {
								this.handleDraftInput('source', ev);
							}, this)
						}, [ this.addDraft.source ])
					]),
					E('div', { 'class': 'routeflux-add-format-list' }, [
						E('span', { 'class': 'routeflux-add-format-badge' }, [ _('http(s) URL') ]),
						E('span', { 'class': 'routeflux-add-format-badge' }, [ _('VLESS / VMess / Trojan / SS') ]),
						E('span', { 'class': 'routeflux-add-format-badge' }, [ _('base64 payload') ]),
						E('span', { 'class': 'routeflux-add-format-badge' }, [ _('Xray / 3x-ui JSON') ])
					]),
					E('p', { 'class': 'routeflux-add-hint' }, [
						_('Accepted input: an http(s) subscription URL; one or more VLESS, VMess, Trojan, or Shadowsocks links; a base64-encoded subscription payload; or an Xray/3x-ui JSON object or array with outbounds, protocol, config, or link.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-add-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleAdd'),
					'disabled': addBusy ? 'disabled' : null
				}, [ _('Add Subscription') ]),
				E('button', {
					'class': 'cbi-button cbi-button-negative',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleRemoveAll'),
					'disabled': subscriptions.length === 0 || removeAllBusy ? 'disabled' : null
				}, [ _('Remove All') ])
			]),
			addActionMessage !== '' ? E('div', { 'class': 'routeflux-action-status' }, [ addActionMessage ]) : ''
		]));

		if (subscriptions.length === 0) {
			content.push(E('div', { 'class': 'cbi-section routeflux-surface' }, [
				E('p', {}, [ _('No subscriptions imported yet.') ]),
				this.pageLoading ? E('p', { 'class': 'routeflux-action-status' }, [ _('Waiting for RouteFlux data...') ]) : ''
			]));
			return content;
		}

		for (var i = 0; i < presentation.groups.length; i++)
			content.push(this.renderProviderGroup(presentation.groups[i], activeSubscriptionId, activeNodeId, status));

		return content;
	},

	render: function(data) {
		this.ensureState();
		if (Array.isArray(data))
			this.pageData = data;
		return E('div', {
			'id': 'routeflux-subscriptions-root',
			'class': routefluxUI.withThemeClass('routeflux-subscriptions-shell routeflux-page-shell routeflux-page-shell-subscriptions')
		}, this.renderPageContent(this.pageData));
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
