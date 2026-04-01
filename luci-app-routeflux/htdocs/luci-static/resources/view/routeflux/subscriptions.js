'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var speedTestRPCTimeoutSeconds = 90;

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

function emptyAddDraft() {
	return {
		'name': '',
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
		this.speedTestBusy = false;
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

	copyTextToClipboard: function(text) {
		var value = String(text || '');

		if (value === '')
			return Promise.reject(new Error(_('Nothing to export.')));

		if (typeof navigator !== 'undefined' && navigator.clipboard && typeof navigator.clipboard.writeText === 'function')
			return navigator.clipboard.writeText(value);

		return new Promise(function(resolve, reject) {
			var area;

			if (!document || !document.body) {
				reject(new Error(_('Clipboard export is unavailable.')));
				return;
			}

			area = E('textarea', {
				'style': 'position:fixed; left:-9999px; top:-9999px; opacity:0'
			}, [ value ]);

			document.body.appendChild(area);
			area.focus();
			area.select();

			try {
				if (!document.execCommand('copy'))
					throw new Error(_('Clipboard export is unavailable.'));
				resolve();
			}
			catch (err) {
				reject(err);
			}
			finally {
				document.body.removeChild(area);
			}
		});
	},

	withRPCTimeout: function(timeoutSeconds, executor) {
		var nextTimeout = Number(timeoutSeconds);
		var previousRaw;
		var hasPrevious;

		if (!isFinite(nextTimeout) || nextTimeout <= 0 || typeof executor !== 'function')
			return Promise.resolve().then(executor);

		previousRaw = L.env.rpctimeout;
		hasPrevious = previousRaw != null && previousRaw !== '';
		L.env.rpctimeout = String(Math.ceil(nextTimeout));

		return Promise.resolve().then(executor).finally(function() {
			if (hasPrevious)
				L.env.rpctimeout = previousRaw;
			else
				delete L.env.rpctimeout;
		});
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
			this.hasPendingActionPrefix(this.actionKey('node', subscriptionId) + ':');
	},

	subscriptionBusyMessage: function(subscriptionId) {
		var direct = routefluxUI.pendingActionMessage(this, this.subscriptionActionKey(subscriptionId));

		if (direct !== '')
			return direct;

		return this.pendingMessageByPrefix(this.actionKey('node', subscriptionId) + ':');
	},

	isNodeBusy: function(subscriptionId, nodeId) {
		return routefluxUI.isPendingAction(this, this.nodeActionKey(subscriptionId, nodeId)) ||
			routefluxUI.isPendingAction(this, this.subscriptionActionKey(subscriptionId));
	},

	nodeBusyMessage: function(subscriptionId, nodeId) {
		var direct = routefluxUI.pendingActionMessage(this, this.nodeActionKey(subscriptionId, nodeId));

		if (direct !== '')
			return direct;

		return routefluxUI.pendingActionMessage(this, this.subscriptionActionKey(subscriptionId));
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

		routefluxUI.showModal(_('Speed Test: %s').format(nodeName), body, {
			'bodyClass': 'routeflux-modal-speedtest',
			'modalClass': 'routeflux-modal-speedtest'
		});
	},

	showInspectPreviewModal: function(node, state) {
		var nodeName = nodeDisplayName(node, node && node.id);
		var actions = [];
		var previewText = '';
		var body = [
			E('p', { 'class': 'routeflux-modal-help' }, [
				_('Sensitive values are redacted. DNS and DoH settings remain visible in this preview.')
			])
		];

		if (state === 'loading') {
			body.push(E('p', { 'class': 'routeflux-speedtest-status' }, [
				_('Loading generated Xray JSON preview...')
			]));
		}
		else if (state && state.error) {
			body.push(E('div', { 'class': 'alert-message warning' }, [ state.error ]));
		}
		else if (state && state.preview) {
			previewText = JSON.stringify(state.preview, null, 2);
			body.push(E('pre', { 'class': 'routeflux-inspect-pre' }, [
				previewText
			]));
			actions.push(E('button', {
				'class': 'cbi-button cbi-button-action',
				'click': L.bind(function(ev) {
					return this.handleExportPreviewJSON(previewText, ev);
				}, this)
			}, [ _('Export JSON') ]));
		}

		actions.push(E('button', {
			'class': 'cbi-button',
			'click': function(ev) {
				ui.hideModal();
				return false;
			}
		}, [ _('Close') ]));

		routefluxUI.showModal(_('Xray JSON Preview: %s').format(nodeName), body, {
			'bodyClass': 'routeflux-modal-json',
			'modalClass': 'routeflux-modal-json',
			'actions': actions
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

	handleAdd: function(ev) {
		var source = trim(this.addDraft && this.addDraft.source);
		var name = trim(this.addDraft && this.addDraft.name);
		var argv = [ 'add' ];
		var message;

		if (source === '') {
			message = _('Paste a subscription URL, share link, or 3x-ui/Xray JSON first.');
			this.setPageError(message);
			ui.addNotification(null, notificationParagraph(message));
			this.renderIntoRoot();
			return Promise.resolve();
		}

		if (name !== '')
			argv.push('--name', name);

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

	handleRemoveAll: function(ev) {
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
		return this.runCLIAction(
			this.subscriptionActionKey(subscriptionId),
			[ 'connect', '--auto', '--subscription', subscriptionId ],
			_('Auto mode enabled.'),
			_('Connecting automatic selection...'),
			{
				'loadingMessage': _('Reloading runtime status...')
			}
		);
	},

	handleConnectNode: function(subscriptionId, nodeId, ev) {
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

	handleSpeedTest: function(subscriptionId, node, ev) {
		var nodeID = trim(node && node.id);
		var message;

		if (this.speedTestBusy === true) {
			message = _('Another speed test is already running.');
			this.setPageError(message);
			ui.addNotification(null, notificationParagraph(message));
			this.renderIntoRoot();
			return Promise.resolve();
		}

		this.speedTestBusy = true;
		this.showSpeedTestModal(node, 'loading');
		this.renderIntoRoot();

		return this.runAction(
			this.nodeActionKey(subscriptionId, nodeID),
			_('Running speed test...'),
			L.bind(function() {
				return this.withRPCTimeout(speedTestRPCTimeoutSeconds, L.bind(function() {
					return this.execJSON([
						'--json', 'inspect', 'speed',
						'--subscription', subscriptionId,
						'--node', nodeID
					]);
				}, this)).then(L.bind(function(result) {
					this.showSpeedTestModal(node, { 'result': result });
					return result;
				}, this)).catch(L.bind(function(err) {
					var errorMessage = err.message || String(err);

					this.showSpeedTestModal(node, { 'error': errorMessage });
					this.setPageError(errorMessage);
					ui.addNotification(null, notificationParagraph(errorMessage));
					this.renderIntoRoot();
					return null;
				}, this));
			}, this)
		).finally(L.bind(function() {
			this.speedTestBusy = false;
			this.renderIntoRoot();
		}, this));
	},

	handleInspectPreview: function(subscriptionId, node, ev) {
		var nodeID = trim(node && node.id);

		this.showInspectPreviewModal(node, 'loading');
		this.renderIntoRoot();

		return this.runAction(
			this.nodeActionKey(subscriptionId, nodeID),
			_('Loading generated Xray JSON preview...'),
			L.bind(function() {
				return this.execJSON([
					'inspect', 'xray-safe',
					'--subscription', subscriptionId,
					'--node', nodeID
				]).then(L.bind(function(preview) {
					this.showInspectPreviewModal(node, { 'preview': preview });
					return preview;
				}, this)).catch(L.bind(function(err) {
					var errorMessage = err.message || String(err);

					this.showInspectPreviewModal(node, { 'error': errorMessage });
					this.setPageError(errorMessage);
					ui.addNotification(null, notificationParagraph(errorMessage));
					this.renderIntoRoot();
					return null;
				}, this));
			}, this)
		);
	},

	handleExportPreviewJSON: function(previewText, ev) {
		return this.copyTextToClipboard(previewText).then(L.bind(function() {
			ui.addNotification(null, notificationParagraph(_('JSON copied to clipboard.')), 'info');
		}, this)).catch(L.bind(function(err) {
			var errorMessage = err && err.message ? err.message : String(err);

			this.setPageError(errorMessage);
			ui.addNotification(null, notificationParagraph(errorMessage));
			this.renderIntoRoot();
			return null;
		}, this));
	},

	renderNodeTable: function(subscription, activeSubscriptionId, activeNodeId) {
		var nodes = Array.isArray(subscription.nodes) ? subscription.nodes : [];
		var speedTestBusy = this.speedTestBusy === true;

		if (nodes.length === 0)
			return E('p', {}, [ _('No nodes found in this subscription.') ]);

		var rows = nodes.map(L.bind(function(node) {
			var isActive = subscription.id === activeSubscriptionId && node.id === activeNodeId;
			var nodeBusy = this.isNodeBusy(subscription.id, node.id);
			var busyMessage = this.nodeBusyMessage(subscription.id, node.id);
			var name = nodeDisplayName(node, node.id);
			var address = firstNonEmpty([
				node.address && node.port ? node.address + ':' + node.port : '',
				node.address
			], '-');

			return E('tr', { 'class': 'tr routeflux-node-row' }, [
				responsiveTableCell(_('Node'), [
					name,
					isActive ? E('div', { 'class': 'routeflux-node-active-badge' }, [ badge(_('Active'), 'notice') ]) : ''
				], 'routeflux-node-cell-primary'),
				responsiveTableCell(_('Address'), address, 'routeflux-node-cell-address'),
				responsiveTableCell(_('Protocol'), firstNonEmpty([ node.protocol ], '-')),
				responsiveTableCell(_('Transport'), firstNonEmpty([ node.transport ], '-')),
				responsiveTableCell(_('Security'), firstNonEmpty([ node.security ], '-')),
				responsiveTableCell(_('Actions'), [
					E('div', { 'class': 'routeflux-node-actions-shell' }, [
						E('div', { 'class': 'routeflux-node-actions' }, [
							E('button', {
								'class': 'cbi-button cbi-button-action',
								'click': ui.createHandlerFn(this, 'handleConnectNode', subscription.id, node.id),
								'disabled': nodeBusy ? 'disabled' : null
							}, [ _('Connect') ]),
							E('button', {
								'class': 'cbi-button cbi-button-action',
								'click': ui.createHandlerFn(this, 'handleInspectPreview', subscription.id, node),
								'disabled': nodeBusy ? 'disabled' : null
							}, [ _('JSON') ]),
							E('button', {
								'class': 'cbi-button cbi-button-action',
								'click': ui.createHandlerFn(this, 'handleSpeedTest', subscription.id, node),
								'disabled': nodeBusy || speedTestBusy ? 'disabled' : null
							}, [ _('Speed Test') ])
						])
					]),
					busyMessage !== '' ? E('div', { 'class': 'routeflux-action-status' }, [ busyMessage ]) : ''
				], 'right routeflux-node-cell-actions')
			]);
		}, this));

		return E('div', { 'class': 'routeflux-node-table-wrap' }, [
			E('table', { 'class': 'table cbi-section-table routeflux-node-table' }, [
				E('colgroup', {}, [
					E('col', { 'class': 'routeflux-node-col-name' }),
					E('col', { 'class': 'routeflux-node-col-address' }),
					E('col', { 'class': 'routeflux-node-col-protocol' }),
					E('col', { 'class': 'routeflux-node-col-transport' }),
					E('col', { 'class': 'routeflux-node-col-security' }),
					E('col', { 'class': 'routeflux-node-col-actions' })
				]),
				E('tr', { 'class': 'tr cbi-section-table-titles' }, [
					E('th', { 'class': 'th' }, [ _('Node') ]),
					E('th', { 'class': 'th' }, [ _('Address') ]),
					E('th', { 'class': 'th' }, [ _('Protocol') ]),
					E('th', { 'class': 'th' }, [ _('Transport') ]),
					E('th', { 'class': 'th' }, [ _('Security') ]),
					E('th', { 'class': 'th right routeflux-node-heading-actions' }, [ _('Actions') ])
				])
			].concat(rows))
		]);
	},

	renderSubscriptionCard: function(entry, activeSubscriptionId, activeNodeId) {
		var subscription = entry.subscription;
		var displayName = entry.profile_label;
		var providerName = entry.provider_title;
		var isActive = subscription.id === activeSubscriptionId;
		var subscriptionBusy = this.isSubscriptionBusy(subscription.id);
		var busyMessage = this.subscriptionBusyMessage(subscription.id);
		var nodesCount = Array.isArray(subscription.nodes) ? subscription.nodes.length : 0;
		var metaRows = [
			[ _('ID'), subscription.id ],
			[ _('Provider'), providerName ],
			[ _('Profile'), displayName ],
			[ _('Source Type'), firstNonEmpty([ subscription.source_type ], '-') ],
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

		var headerNodes = [
			E('div', { 'class': 'routeflux-subscription-title' }, [ displayName ])
		];

		headerNodes.push(E('div', { 'class': 'routeflux-subscription-provider' }, [ providerName ]));

		if (isActive)
			headerNodes.push(E('div', { 'class': 'routeflux-subscription-badges' }, [ badge(_('Active'), 'notice') ]));

		return E('div', { 'class': 'cbi-section routeflux-subscription-card' }, [
			E('div', { 'class': 'routeflux-subscription-header' }, [
				E('div', { 'class': 'routeflux-subscription-heading' }, headerNodes),
				E('div', { 'class': 'routeflux-subscription-controls' }, [
					E('div', { 'class': 'routeflux-subscription-actions' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(this, 'handleRefreshSubscription', subscription.id),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Refresh') ]),
						E('button', {
							'class': 'cbi-button cbi-button-apply',
							'click': ui.createHandlerFn(this, 'handleConnectAuto', subscription.id),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Connect Auto') ]),
						E('button', {
							'class': 'cbi-button cbi-button-negative',
							'click': ui.createHandlerFn(this, 'handleRemoveSubscription', subscription.id, displayName),
							'disabled': subscriptionBusy ? 'disabled' : null
						}, [ _('Remove') ])
					]),
					busyMessage !== '' ? E('div', { 'class': 'routeflux-action-status routeflux-action-status-group' }, [ busyMessage ]) : ''
				])
			]),
			E('table', { 'class': 'table routeflux-meta-table' }, metaRows),
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
		var addBusy = routefluxUI.isPendingAction(this, 'add');
		var addBusyMessage = routefluxUI.pendingActionMessage(this, 'add');
		var removeAllBusy = routefluxUI.isPendingAction(this, 'remove-all');
		var removeAllMessage = routefluxUI.pendingActionMessage(this, 'remove-all');
		var addActionMessage = addBusyMessage || removeAllMessage;
		var content = [];

		this.ensureState();
		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-subscriptions-shell { width:100%; max-width:100%; min-width:0; box-sizing:border-box; }',
			'.routeflux-subscription-card { margin-bottom:16px; }',
			'.routeflux-subscription-header { display:grid; grid-template-columns:minmax(0, 1fr) auto; gap:14px 18px; align-items:start; margin-bottom:12px; }',
			'.routeflux-subscription-heading { min-width:0; }',
			'.routeflux-subscription-title { font-size:clamp(17px, 1.1vw + 14px, 22px); font-weight:600; line-height:1.25; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-subscription-provider { color:var(--text-color-medium, #666); margin-top:4px; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-subscription-badges { display:flex; flex-wrap:wrap; gap:8px; margin-top:8px; }',
			'.routeflux-subscription-controls { display:grid; gap:8px; justify-items:end; min-width:0; max-width:100%; }',
			'.routeflux-subscription-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; align-items:flex-start; max-width:100%; }',
			'.routeflux-subscription-actions .cbi-button, .routeflux-node-actions .cbi-button { white-space:nowrap; }',
			'.routeflux-meta-table { width:100%; table-layout:fixed; margin-bottom:0; }',
			'.routeflux-meta-label { width:180px; color:var(--text-color-medium, #586677); font-weight:600; }',
			'.routeflux-meta-value { overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-traffic-shell { display:grid; gap:8px; min-width:0; }',
			'.routeflux-traffic-copy { display:flex; flex-wrap:wrap; gap:6px 10px; align-items:baseline; min-width:0; }',
			'.routeflux-traffic-primary { color:var(--text-color-high, #17263a); font-weight:700; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-traffic-secondary { color:var(--text-color-medium, #586677); font-size:12px; line-height:1.4; }',
			'.routeflux-traffic-meter { position:relative; width:min(100%, 260px); max-width:100%; height:10px; border-radius:999px; background:linear-gradient(180deg, rgba(148, 163, 184, 0.3) 0%, rgba(148, 163, 184, 0.18) 100%); overflow:hidden; box-shadow:inset 0 1px 1px rgba(15, 23, 42, 0.14); }',
			'.routeflux-traffic-meter-fill { height:100%; border-radius:inherit; background:linear-gradient(90deg, #22c55e 0%, #14b8a6 100%); }',
			'.routeflux-traffic-shell-unlimited .routeflux-traffic-primary { color:#17603d; }',
			'.routeflux-node-table-wrap { width:100%; max-width:100%; overflow-x:auto; -webkit-overflow-scrolling:touch; }',
			'.routeflux-node-table { width:100%; min-width:920px; table-layout:fixed; }',
			'.routeflux-node-col-name { width:18%; }',
			'.routeflux-node-col-address { width:20%; }',
			'.routeflux-node-col-protocol { width:9%; }',
			'.routeflux-node-col-transport { width:9%; }',
			'.routeflux-node-col-security { width:12%; }',
			'.routeflux-node-col-actions { width:32%; }',
			'.routeflux-node-table .th, .routeflux-node-table .td { vertical-align:middle; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-node-heading-actions { text-align:right; }',
			'.routeflux-node-cell-actions { min-width:0; }',
			'.routeflux-node-active-badge { margin-top:6px; }',
			'.routeflux-node-actions-shell { width:100%; }',
			'.routeflux-node-actions { display:flex; flex-wrap:nowrap; justify-content:flex-end; align-items:stretch; gap:8px; width:100%; }',
			'.routeflux-node-actions .cbi-button { flex:1 1 0; min-width:0; text-align:center; }',
			'.routeflux-action-status { margin-top:8px; color:var(--text-color-medium, #586677); font-size:12px; line-height:1.4; }',
			'.routeflux-action-status-group { width:100%; text-align:right; }',
			'.routeflux-page-status { margin-bottom:14px; }',
			'.routeflux-page-banner { padding:10px 12px; border:1px solid rgba(98, 112, 129, 0.34); border-radius:12px; margin-bottom:10px; line-height:1.45; }',
			'.routeflux-page-banner-info { background:rgba(224, 242, 254, 0.6); border-color:rgba(56, 189, 248, 0.32); color:#0f3f57; }',
			'.routeflux-page-banner-warning { background:rgba(254, 242, 242, 0.7); border-color:rgba(239, 68, 68, 0.28); color:#6e1f1f; }',
			'.routeflux-page-status-actions { display:flex; justify-content:flex-end; margin-top:10px; }',
			'.routeflux-add-grid { display:grid; grid-template-columns:minmax(220px, 320px) minmax(0, 1fr); gap:12px; margin-bottom:12px; }',
			'.routeflux-add-actions { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-add-grid .cbi-value-field, .routeflux-add-grid .cbi-input-text, .routeflux-add-grid .cbi-input-textarea { width:100%; max-width:100%; box-sizing:border-box; }',
			'.routeflux-add-grid textarea { min-height:140px; width:100%; max-width:100%; }',
			'.routeflux-node-details { margin-top:12px; }',
			'.routeflux-node-details summary { cursor:pointer; margin-bottom:10px; }',
			'.routeflux-provider-group { margin-bottom:22px; }',
			'.routeflux-provider-group-header { display:grid; grid-template-columns:minmax(0, 1fr) auto; gap:8px 12px; align-items:end; margin:12px 0 8px; }',
			'.routeflux-provider-group-title { font-size:clamp(20px, 1.3vw + 15px, 26px); font-weight:600; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-provider-group-meta { color:var(--text-color-medium, #666); }',
			'.modal.routeflux-modal-json { width:min(92vw, 980px); max-width:92vw; }',
			'.modal.routeflux-modal-speedtest { width:min(92vw, 720px); max-width:92vw; overflow:hidden; }',
			'.routeflux-modal-help { margin:0 0 12px; color:var(--text-color-medium, #586677); max-width:100%; overflow-wrap:anywhere; word-break:break-word; line-height:1.45; }',
			'.routeflux-inspect-pre { white-space:pre-wrap; margin:0; max-height:56vh; max-width:100%; overflow:auto; padding:14px 16px; border:1px solid rgba(71, 85, 105, 0.82); border-radius:12px; background:linear-gradient(180deg, #09111d 0%, #0d1623 48%, #101a29 100%); color:#eef4fb; font-family:SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size:13px; line-height:1.72; box-sizing:border-box; }',
			'.routeflux-speedtest-status { margin:0; font-weight:600; }',
			'.routeflux-speedtest-grid { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:10px; margin-top:12px; max-width:100%; min-width:0; align-items:stretch; }',
			'.routeflux-speedtest-metric { min-width:0; width:100%; border:1px solid var(--border-color-medium); border-radius:12px; padding:10px 12px; background:linear-gradient(180deg, var(--background-color-high) 0%, var(--background-color-low) 100%); box-shadow:inset 0 1px 0 hsla(var(--background-color-high-hsl), 0.28); box-sizing:border-box; }',
			'.routeflux-speedtest-label { color:var(--text-color-medium, #586677); font-size:10px; text-transform:uppercase; letter-spacing:.12em; font-weight:700; margin-bottom:4px; }',
			'.routeflux-speedtest-value { color:var(--text-color-high, #17263a); font-size:15px; font-weight:700; line-height:1.3; overflow-wrap:anywhere; word-break:break-word; }',
			'.routeflux-speedtest-subtle { font-size:14px; }',
			'@media (min-width: 981px) { .routeflux-node-heading-actions, .routeflux-node-cell-actions { text-align:center !important; } .routeflux-node-actions-shell { display:block; width:min(100%, 320px); margin:0 auto; padding:8px 10px; border:1px solid var(--border-color-medium); border-radius:14px; background:linear-gradient(180deg, var(--background-color-high) 0%, var(--background-color-low) 100%); box-shadow:inset 0 1px 0 hsla(var(--background-color-high-hsl), 0.24), 0 0 0 1px hsla(var(--border-color-low-hsl), 0.25); } .routeflux-node-actions { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); justify-content:center; gap:8px; width:100%; } .routeflux-node-actions .cbi-button { width:100%; min-width:0; } .routeflux-node-actions .cbi-button:last-child { grid-column:1 / -1; } }',
			'@media (max-width: 980px) { .routeflux-subscription-header, .routeflux-provider-group-header, .routeflux-add-grid { grid-template-columns:minmax(0, 1fr); } .routeflux-subscription-controls { justify-items:stretch; min-width:0; } .routeflux-subscription-actions, .routeflux-node-actions { justify-content:flex-start; } .routeflux-action-status-group { text-align:left; } .routeflux-node-table { min-width:860px; } }',
			'@media (max-width: 700px) { .routeflux-meta-table, .routeflux-meta-table .tr, .routeflux-meta-table .td { display:block; width:100%; box-sizing:border-box; } .routeflux-meta-table .tr { padding:10px 0; border-top:1px solid rgba(98, 112, 129, 0.22); } .routeflux-meta-table .tr:first-child { padding-top:0; border-top:0; } .routeflux-meta-label { width:100%; padding-bottom:4px; } .routeflux-meta-value { padding-top:0; } .routeflux-add-actions, .routeflux-page-status-actions { flex-direction:column; } .routeflux-add-actions .cbi-button, .routeflux-page-status-actions .cbi-button { width:100%; } .routeflux-speedtest-grid { grid-template-columns:minmax(0, 1fr); } }',
			'@media (max-width: 560px) { .routeflux-subscription-actions, .routeflux-node-actions { flex-direction:column; align-items:stretch; } .routeflux-subscription-actions .cbi-button, .routeflux-node-actions .cbi-button { flex:1 1 auto; width:100%; } .routeflux-node-table, .routeflux-node-table .tr, .routeflux-node-table .td { display:block; width:100%; box-sizing:border-box; } .routeflux-node-table { min-width:0; } .routeflux-node-table .cbi-section-table-titles { display:none; } .routeflux-node-table .routeflux-node-row { margin-bottom:12px; padding:12px 14px; border:1px solid var(--border-color-medium); border-radius:12px; background:linear-gradient(180deg, var(--background-color-high) 0%, var(--background-color-low) 100%); box-shadow:0 8px 18px hsla(var(--border-color-low-hsl), 0.35), inset 0 1px 0 hsla(var(--background-color-high-hsl), 0.28); } .routeflux-node-table .routeflux-node-row:last-child { margin-bottom:0; } .routeflux-node-table .td { padding:8px 0; border-top:1px solid var(--border-color-low); text-align:left; } .routeflux-node-table .td:first-child { padding-top:0; border-top:0; } .routeflux-node-table .td:last-child { padding-bottom:0; } .routeflux-node-table .td::before { content:attr(data-title); display:block; margin-bottom:4px; color:var(--text-color-medium, #586677); font-size:10px; text-transform:uppercase; letter-spacing:.12em; font-weight:700; } .routeflux-node-cell-actions { min-width:0; } .routeflux-node-actions { width:100%; } .routeflux-action-status { margin-top:6px; } }',
			'@media (max-width: 420px) { .modal.routeflux-modal-json, .modal.routeflux-modal-speedtest { width:min(96vw, 96vw); max-width:96vw; } .routeflux-inspect-pre { font-size:12px; padding:12px; } }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Subscriptions') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Manage imported subscriptions, add new providers, refresh existing profiles, remove one or all profiles, and connect a specific node or the best node automatically.')
		]));

		if (this.pageInfo !== '' || this.pageError !== '') {
			content.push(E('div', { 'class': 'cbi-section routeflux-page-status' }, [
				this.pageInfo !== '' ? E('div', { 'class': 'routeflux-page-banner routeflux-page-banner-info' }, [ this.pageInfo ]) : '',
				this.pageError !== '' ? E('div', { 'class': 'routeflux-page-banner routeflux-page-banner-warning' }, [ this.pageError ]) : '',
				this.pageError !== '' ? E('div', { 'class': 'routeflux-page-status-actions' }, [
					E('button', {
						'class': 'cbi-button',
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
							'placeholder': _('Optional provider name'),
							'value': this.addDraft.name,
							'input': L.bind(function(ev) {
								this.handleDraftInput('name', ev);
							}, this)
						})
					])
				]),
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-add-source' }, [ _('Subscription URL, share link, or 3x-ui/Xray JSON') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-add-source',
							'class': 'cbi-input-textarea',
							'placeholder': _('Paste a subscription URL, VLESS/VMess/Trojan link, or 3x-ui/Xray JSON here.'),
							'input': L.bind(function(ev) {
								this.handleDraftInput('source', ev);
							}, this)
						}, [ this.addDraft.source ])
					])
				])
			]),
			E('div', { 'class': 'routeflux-add-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'handleAdd'),
					'disabled': addBusy ? 'disabled' : null
				}, [ _('Add Subscription') ]),
				E('button', {
					'class': 'cbi-button cbi-button-negative',
					'click': ui.createHandlerFn(this, 'handleRemoveAll'),
					'disabled': subscriptions.length === 0 || removeAllBusy ? 'disabled' : null
				}, [ _('Remove All') ])
			]),
			addActionMessage !== '' ? E('div', { 'class': 'routeflux-action-status' }, [ addActionMessage ]) : ''
		]));

		if (subscriptions.length === 0) {
			content.push(E('div', { 'class': 'cbi-section' }, [
				E('p', {}, [ _('No subscriptions imported yet.') ]),
				this.pageLoading ? E('p', { 'class': 'routeflux-action-status' }, [ _('Waiting for RouteFlux data...') ]) : ''
			]));
			return content;
		}

		for (var i = 0; i < presentation.groups.length; i++)
			content.push(this.renderProviderGroup(presentation.groups[i], activeSubscriptionId, activeNodeId));

		return content;
	},

	render: function(data) {
		this.ensureState();
		if (Array.isArray(data))
			this.pageData = data;
		return E('div', {
			'id': 'routeflux-subscriptions-root',
			'class': 'routeflux-subscriptions-shell'
		}, this.renderPageContent(this.pageData));
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
