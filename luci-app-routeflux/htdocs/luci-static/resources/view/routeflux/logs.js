'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require poll';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var maxVisibleLogLines = 400;
var logsPollInterval = 2;

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

function joinLines(lines, emptyText) {
	if (!Array.isArray(lines) || lines.length === 0)
		return emptyText;

	return lines.join('\n');
}

function cloneLines(lines) {
	return Array.isArray(lines) ? lines.slice() : [];
}

function linesEqual(left, right) {
	var a = Array.isArray(left) ? left : [];
	var b = Array.isArray(right) ? right : [];

	if (a.length !== b.length)
		return false;

	for (var i = 0; i < a.length; i++) {
		if (a[i] !== b[i])
			return false;
	}

	return true;
}

function padNumber(value) {
	var number = parseInt(value, 10);

	if (isNaN(number) || number < 0)
		number = 0;

	return number < 10 ? '0' + number : String(number);
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'list', 'subscriptions' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'logs' ]).catch(function(err) {
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

	renderCard: function(label, value, options) {
		return routefluxUI.renderSummaryCard(label, value, options);
	},

	formatRefreshTime: function(date) {
		return [
			padNumber(date.getHours()),
			padNumber(date.getMinutes()),
			padNumber(date.getSeconds())
		].join(':');
	},

	updateLastUpdatedLabel: function(date) {
		this.lastUpdatedLabel = this.formatRefreshTime(date || new Date());

		var element = document.querySelector('#routeflux-logs-last-updated');
		if (element)
			element.textContent = _('Last updated: %s').format(this.lastUpdatedLabel);
	},

	setRefreshButtonState: function(isBusy) {
		this.isManualRefreshing = isBusy === true;

		var button = document.querySelector('#routeflux-logs-refresh-button');
		if (!button)
			return;

		button.disabled = this.isManualRefreshing;
		button.textContent = this.isManualRefreshing ? _('Refreshing...') : _('Refresh');
	},

	createLogBuffer: function(lines) {
		var snapshot = cloneLines(lines);

		return {
			snapshot: snapshot,
			visible: snapshot.slice(-maxVisibleLogLines),
			captureBaseline: false
		};
	},

	initializeLogBuffers: function(logs) {
		this.logBuffers = {
			routeflux: this.createLogBuffer(logs && logs.routeflux),
			xray: this.createLogBuffer(logs && logs.xray),
			system: this.createLogBuffer(logs && logs.system)
		};
	},

	resetLogBuffers: function() {
		if (!this.logBuffers)
			this.initializeLogBuffers({});

		var keys = [ 'routeflux', 'xray', 'system' ];
		for (var i = 0; i < keys.length; i++) {
			var existing = this.logBuffers[keys[i]] || {};

			this.logBuffers[keys[i]] = {
				snapshot: cloneLines(existing.snapshot),
				visible: [],
				captureBaseline: true
			};
		}
	},

	findLargestOverlap: function(previousLines, nextLines) {
		var prev = Array.isArray(previousLines) ? previousLines : [];
		var next = Array.isArray(nextLines) ? nextLines : [];
		var maxOverlap = Math.min(prev.length, next.length);

		for (var size = maxOverlap; size > 0; size--) {
			var matches = true;

			for (var i = 0; i < size; i++) {
				if (prev[prev.length - size + i] !== next[i]) {
					matches = false;
					break;
				}
			}

			if (matches)
				return size;
		}

		return 0;
	},

	mergeLogSnapshot: function(key, lines, options) {
		var buffer = this.logBuffers[key];
		var snapshot = cloneLines(lines);
		var settings = options || {};
		var previousSnapshot;
		var previousVisible;
		var nextVisible;

		if (!buffer) {
			this.logBuffers[key] = this.createLogBuffer(snapshot);
			return snapshot.length > 0;
		}

		previousSnapshot = cloneLines(buffer.snapshot);
		previousVisible = buffer.visible.slice();

		if (buffer.captureBaseline) {
			var initialOverlap = this.findLargestOverlap(previousSnapshot, snapshot);

			if (settings.revealBaseline === true)
				nextVisible = snapshot.slice(-maxVisibleLogLines);
			else if (linesEqual(previousSnapshot, snapshot))
				nextVisible = [];
			else if (initialOverlap > 0)
				nextVisible = snapshot.slice(initialOverlap).slice(-maxVisibleLogLines);
			else
				nextVisible = snapshot.slice(-maxVisibleLogLines);

			buffer.snapshot = snapshot;
			buffer.visible = nextVisible;
			buffer.captureBaseline = false;
			return !linesEqual(previousVisible, nextVisible);
		}

		if (linesEqual(buffer.snapshot, snapshot))
			return false;

		var overlap = this.findLargestOverlap(buffer.snapshot, snapshot);

		if (buffer.snapshot.length === 0 && previousVisible.length === 0) {
			nextVisible = snapshot.slice(-maxVisibleLogLines);
		} else if (overlap > 0) {
			nextVisible = previousVisible.concat(snapshot.slice(overlap)).slice(-maxVisibleLogLines);
		} else {
			nextVisible = snapshot.slice(-maxVisibleLogLines);
		}

		buffer.visible = nextVisible;
		buffer.snapshot = snapshot;

		return !linesEqual(previousVisible, nextVisible);
	},

	applyLogsSnapshot: function(logs, options) {
		var changed = false;

		if (!this.logBuffers)
			this.initializeLogBuffers(logs);

		changed = this.mergeLogSnapshot('routeflux', logs && logs.routeflux, options) || changed;
		changed = this.mergeLogSnapshot('xray', logs && logs.xray, options) || changed;
		changed = this.mergeLogSnapshot('system', logs && logs.system, options) || changed;

		return changed;
	},

	summarySignature: function(status, logs) {
		var currentStatus = status || {};
		var currentLogs = logs || {};
		var state = currentStatus.state || {};
		var activeSubscription = currentStatus.active_subscription || {};
		var activeNode = currentStatus.active_node || {};
		var activeEntry = presentationForSubscription(activeSubscription, this.subscriptionPresentation);
		var activeProvider = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.provider_title : providerTitle(activeSubscription))
			: _('Not selected');
		var activeProfile = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.profile_label : _('Profile 1'))
			: _('Not selected');
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));

		return JSON.stringify([
			state.connected === true,
			firstNonEmpty([ state.mode ], _('disconnected')),
			firstNonEmpty([ currentLogs.source ], _('/sbin/logread')),
			currentLogs.available === true,
			activeProvider,
			activeProfile,
			activeNodeName
		]);
	},

	errorSignature: function(logs, pollingError) {
		return JSON.stringify([
			trim(logs && logs.error),
			trim(pollingError)
		]);
	},

	buildSummaryGrid: function() {
		var status = this.currentStatus || {};
		var logs = this.currentLogsMeta || {};
		var state = status.state || {};
		var activeSubscription = status.active_subscription || {};
		var activeNode = status.active_node || {};
		var activeEntry = presentationForSubscription(activeSubscription, this.subscriptionPresentation);
		var activeProvider = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.provider_title : providerTitle(activeSubscription))
			: _('Not selected');
		var activeProfile = trim(activeSubscription.id) !== ''
			? (activeEntry ? activeEntry.profile_label : _('Profile 1'))
			: _('Not selected');
		var activeNodeName = nodeDisplayName(activeNode, _('Not selected'));

		return E('div', { 'class': 'routeflux-overview-grid' }, [
			this.renderCard(_('Connection'), state.connected === true ? _('Connected') : _('Disconnected'), {
				'tone': routefluxUI.statusTone(state.connected === true),
				'primary': true
			}),
			this.renderCard(_('Mode'), firstNonEmpty([ state.mode ], _('disconnected'))),
			this.renderCard(_('Log Source'), firstNonEmpty([ logs.source ], _('/sbin/logread'))),
			this.renderCard(_('logread'), logs.available === true ? _('Available') : _('Unavailable')),
			this.renderCard(_('Active Provider'), activeProvider),
			this.renderCard(_('Active Profile'), activeProfile),
			this.renderCard(_('Active Node'), activeNodeName)
		]);
	},

	buildErrorBanner: function() {
		var logSourceError = trim(this.currentLogsMeta && this.currentLogsMeta.error);
		var pollingError = trim(this.currentPollingError);

		if (pollingError !== '') {
			return E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('Logs error: %s').format(pollingError)
				])
			]);
		}

		if (logSourceError !== '') {
			return E('div', { 'class': 'cbi-section' }, [
				E('div', { 'class': 'alert-message warning' }, [
					_('Log source error: %s').format(logSourceError)
				])
			]);
		}

		return E([]);
	},

	logText: function(key, emptyText) {
		var buffer = this.logBuffers && this.logBuffers[key];
		return joinLines(buffer ? buffer.visible : [], emptyText);
	},

	renderLogSection: function(title, key, emptyText) {
		return E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ title ]),
			E('pre', {
				'class': 'routeflux-logs-pre',
				'id': 'routeflux-logs-' + key + '-pre'
			}, [
				this.logText(key, emptyText)
			])
		]);
	},

	isNearBottom: function(element) {
		return (element.scrollHeight - element.scrollTop - element.clientHeight) <= 24;
	},

	updatePreformattedBlock: function(selector, text, forceScroll) {
		var element = document.querySelector(selector);
		if (!element)
			return;

		var keepPinned = forceScroll === true || this.isNearBottom(element);
		element.textContent = text;

		if (keepPinned)
			element.scrollTop = element.scrollHeight;
	},

	scrollLogBlocksToBottom: function() {
		var selectors = [
			'#routeflux-logs-routeflux-pre',
			'#routeflux-logs-xray-pre',
			'#routeflux-logs-system-pre'
		];

		for (var i = 0; i < selectors.length; i++) {
			var element = document.querySelector(selectors[i]);
			if (element)
				element.scrollTop = element.scrollHeight;
		}
	},

	renderLiveSummary: function() {
		var target = document.querySelector('#routeflux-logs-summary');
		if (target)
			dom.content(target, this.buildSummaryGrid());
	},

	renderLiveError: function() {
		var target = document.querySelector('#routeflux-logs-error');
		if (target)
			dom.content(target, this.buildErrorBanner());
	},

	renderLiveLogs: function(forceScroll) {
		this.updatePreformattedBlock(
			'#routeflux-logs-routeflux-pre',
			this.logText('routeflux', _('No recent RouteFlux log lines matched in logread.')),
			forceScroll
		);
		this.updatePreformattedBlock(
			'#routeflux-logs-xray-pre',
			this.logText('xray', _('No recent Xray log lines matched in logread.')),
			forceScroll
		);
		this.updatePreformattedBlock(
			'#routeflux-logs-system-pre',
			this.logText('system', _('No recent system log lines are available.')),
			forceScroll
		);
	},

	requestRefreshSnapshot: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			}),
			this.execJSON([ '--json', 'logs' ]).catch(function(err) {
				return { __error__: err.message || String(err) };
			})
		]);
	},

	refreshLiveData: function(showNotification, force) {
		var forced = force === true;
		var requestId;
		var promise;

		if (!forced && this.backgroundRefreshPromise)
			return this.backgroundRefreshPromise;

		requestId = (this.requestSequence || 0) + 1;
		this.requestSequence = requestId;
		this.latestIssuedRequestId = requestId;

		if (forced)
			this.setRefreshButtonState(true);

		promise = this.requestRefreshSnapshot().then(L.bind(function(results) {
			var nextStatus = results[0] || {};
			var nextLogs = results[1] || {};
			var notifications = [];
			var previousSummarySignature = this.summarySignature(this.currentStatus, this.currentLogsMeta);
			var previousErrorSignature = this.errorSignature(this.currentLogsMeta, this.currentPollingError);
			var summaryChanged = false;
			var errorChanged = false;
			var logsChanged = false;

			if (requestId !== this.latestIssuedRequestId)
				return;

			if (nextStatus.__error__) {
				if (showNotification === true)
					notifications.push(_('Status error: %s').format(nextStatus.__error__));
			} else {
				this.currentStatus = nextStatus;
			}

			if (nextLogs.__error__) {
				this.currentPollingError = nextLogs.__error__;
				if (showNotification === true)
					notifications.push(_('Logs error: %s').format(nextLogs.__error__));
			} else {
				this.currentPollingError = '';
				this.currentLogsMeta = nextLogs;
				logsChanged = this.applyLogsSnapshot(nextLogs, {
					'revealBaseline': forced
				});
			}

			summaryChanged = previousSummarySignature !== this.summarySignature(this.currentStatus, this.currentLogsMeta);
			errorChanged = previousErrorSignature !== this.errorSignature(this.currentLogsMeta, this.currentPollingError);

			if (forced || summaryChanged)
				this.renderLiveSummary();
			if (forced || errorChanged)
				this.renderLiveError();
			if (forced || logsChanged)
				this.renderLiveLogs(forced);

			if (!nextStatus.__error__ && !nextLogs.__error__)
				this.updateLastUpdatedLabel(new Date());

			if (showNotification === true && notifications.length > 0)
				ui.addNotification(null, notificationParagraph(notifications.join(' ')));
		}, this)).finally(L.bind(function() {
			if (!forced && this.backgroundRefreshPromise === promise)
				this.backgroundRefreshPromise = null;

			if (forced)
				this.setRefreshButtonState(false);
		}, this));

		if (!forced)
			this.backgroundRefreshPromise = promise;

		return promise;
	},

	startPolling: function() {
		if (this.pollFn)
			return;

		this.pollFn = L.bind(function() {
			if (this.isManualRefreshing === true)
				return Promise.resolve();

			return this.refreshLiveData(false, false);
		}, this);

		poll.add(this.pollFn, logsPollInterval);
		poll.start();

		if (!this.beforeUnloadHandler) {
			this.beforeUnloadHandler = L.bind(function() {
				if (this.pollFn) {
					poll.remove(this.pollFn);
					this.pollFn = null;
				}
			}, this);

			window.addEventListener('beforeunload', this.beforeUnloadHandler);
		}
	},

	handleRefreshPage: function(ev) {
		return this.refreshLiveData(true, true).then(L.bind(function() {
			this.scrollLogBlocksToBottom();
		}, this));
	},

	handleCleanLogs: function(ev) {
		this.currentPollingError = '';
		this.resetLogBuffers();
		this.renderLiveError();
		this.renderLiveLogs(false);
	},

	render: function(data) {
		var status = data[0] || {};
		var subscriptions = Array.isArray(data[1]) ? data[1] : [];
		var logs = data[2] || {};
		var content = [];

		this.subscriptionPresentation = buildSubscriptionPresentation(subscriptions);
		this.currentStatus = status && !status.__error__ ? status : {};
		this.currentLogsMeta = logs && !logs.__error__ ? logs : {};
		this.currentPollingError = '';
		this.isManualRefreshing = false;
		this.initializeLogBuffers(this.currentLogsMeta);
		this.lastUpdatedLabel = (data[0] && data[0].__error__) || (data[2] && data[2].__error__)
			? _('Never')
			: this.formatRefreshTime(new Date());

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Status error: %s').format(data[0].__error__)));

		if (data[1] && data[1].__error__)
			ui.addNotification(null, notificationParagraph(_('Subscriptions error: %s').format(data[1].__error__)));

		if (data[2] && data[2].__error__) {
			this.currentPollingError = data[2].__error__;
			ui.addNotification(null, notificationParagraph(_('Logs error: %s').format(data[2].__error__)));
		}

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-logs-actions { display:flex; flex-wrap:wrap; justify-content:space-between; gap:12px; margin-bottom:16px; align-items:center; }',
			'.routeflux-logs-buttons { display:flex; flex-wrap:wrap; gap:10px; }',
			'.routeflux-logs-meta { color:var(--text-color-secondary, #94a3b8); font-size:12px; letter-spacing:.03em; }',
			'.routeflux-logs-pre { white-space:pre-wrap; margin:0; max-height:420px; overflow:auto; padding:14px 16px; border:1px solid rgba(71, 85, 105, 0.82); border-radius:12px; background:linear-gradient(180deg, #09111d 0%, #0d1623 48%, #101a29 100%); color:#eef4fb; box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.04), 0 18px 32px rgba(0, 0, 0, 0.24); font-family:SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size:13px; font-weight:500; line-height:1.72; letter-spacing:.01em; tab-size:4; word-break:break-word; overflow-wrap:anywhere; font-variant-ligatures:none; }',
			'.routeflux-logs-pre::selection { background:rgba(56, 189, 248, 0.25); }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Logs') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Inspect recent RouteFlux-related logs, Xray runtime logs, and the tail of the system log without leaving LuCI.')
		]));

		content.push(E('div', { 'id': 'routeflux-logs-summary' }, [
			this.buildSummaryGrid()
		]));

		content.push(E('div', { 'id': 'routeflux-logs-error' }, [
			this.buildErrorBanner()
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Actions') ]),
			E('div', { 'class': 'routeflux-logs-actions' }, [
				E('div', { 'class': 'routeflux-logs-buttons' }, [
					E('button', {
						'id': 'routeflux-logs-refresh-button',
						'class': 'cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, 'handleRefreshPage')
					}, [ _('Refresh') ]),
					E('button', {
						'class': 'cbi-button cbi-button-negative',
						'click': ui.createHandlerFn(this, 'handleCleanLogs')
					}, [ _('Clean') ])
				]),
				E('div', {
					'id': 'routeflux-logs-last-updated',
					'class': 'routeflux-logs-meta'
				}, [
					_('Last updated: %s').format(this.lastUpdatedLabel)
				])
			])
		]));

		content.push(this.renderLogSection(
			_('RouteFlux'),
			'routeflux',
			_('No recent RouteFlux log lines matched in logread.')
		));
		content.push(this.renderLogSection(
			_('Xray'),
			'xray',
			_('No recent Xray log lines matched in logread.')
		));
		content.push(this.renderLogSection(
			_('System Tail'),
			'system',
			_('No recent system log lines are available.')
		));

		window.setTimeout(L.bind(function() {
			this.startPolling();
			this.scrollLogBlocksToBottom();
		}, this), 0);

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
