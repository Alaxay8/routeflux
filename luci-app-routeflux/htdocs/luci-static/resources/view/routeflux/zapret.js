'use strict';
'require view';
'require fs';
'require ui';
'require dom';
'require routeflux.ui as routefluxUI';

var routefluxBinary = '/usr/bin/routeflux';
var zapretPresetPrefix = 'zapret-';
var zapretPresetBrandNames = {
	'youtube': 'YouTube'
};

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

function wordsTitleCase(value) {
	return String(value || '').split(/\s+/).filter(function(part) {
		return trim(part) !== '';
	}).map(function(part) {
		var lowered = part.toLowerCase();
		return zapretPresetBrandNames[lowered] || (lowered.charAt(0).toUpperCase() + lowered.slice(1));
	}).join(' ');
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function cleanList(values) {
	var list = Array.isArray(values) ? values : [];
	var out = [];
	var seen = {};

	for (var i = 0; i < list.length; i++) {
		var value = trim(list[i]).toLowerCase();

		if (value === '' || seen[value])
			continue;

		seen[value] = true;
		out.push(value);
	}

	return out;
}

function cloneDomains(values) {
	return cleanList(values || []);
}

function selectorHasEntries(value) {
	return cloneDomains(value && value.domains).length > 0 || cleanList(value && value.cidrs).length > 0;
}

function domainLooksValid(value) {
	var candidate = trim(value).toLowerCase();

	return candidate !== '' &&
		candidate.indexOf('://') < 0 &&
		candidate.indexOf(' ') < 0 &&
		candidate.indexOf('.') > 0;
}

function ipv4SelectorLooksValid(value) {
	var candidate = trim(value).toLowerCase();
	var ipv4 = /^(?:25[0-5]|2[0-4]\d|1?\d?\d)(?:\.(?:25[0-5]|2[0-4]\d|1?\d?\d)){3}$/;

	if (candidate === '')
		return false;

	if (ipv4.test(candidate))
		return true;

	if (/\/(?:[0-9]|[12][0-9]|3[0-2])$/.test(candidate))
		return ipv4.test(candidate.split('/')[0]);

	if (candidate.indexOf('-') > 0) {
		var parts = candidate.split('-');
		return parts.length === 2 && ipv4.test(parts[0]) && ipv4.test(parts[1]);
	}

	return false;
}

function selectorLooksValid(value) {
	return domainLooksValid(value) || ipv4SelectorLooksValid(value);
}

function parseSelectorInput(raw) {
	var chunks = String(raw || '').split(/[\s,]+/);
	var values = [];
	var invalid = [];

	for (var i = 0; i < chunks.length; i++) {
		var value = trim(chunks[i]).toLowerCase();
		if (value === '')
			continue;
		if (!selectorLooksValid(value)) {
			invalid.push(value);
			continue;
		}
		values.push(value);
	}

	return {
		'values': cleanList(values),
		'invalid': invalid
	};
}

function serviceSelectors(service) {
	return cleanList([].concat(service && service.domains || [], service && service.cidrs || []));
}

function isZapretPresetName(name) {
	return trim(name).indexOf(zapretPresetPrefix) === 0;
}

function slugifyZapretPresetName(value) {
	var slug = trim(value).toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+/, '')
		.replace(/-+$/, '');

	if (slug !== '' && !/^[a-z]/.test(slug))
		slug = 'preset-' + slug;

	return slug;
}

function zapretPresetStorageName(value) {
	var slug = slugifyZapretPresetName(value);
	return slug === '' ? '' : zapretPresetPrefix + slug;
}

function zapretPresetDisplayName(value) {
	var normalized = trim(value);

	if (isZapretPresetName(normalized))
		normalized = normalized.slice(zapretPresetPrefix.length);

	return wordsTitleCase(normalized.replace(/-/g, ' '));
}

function isCustomService(service) {
	return trim(service && service.source) === 'custom' && service && service.readonly !== true;
}

function availableZapretServices(services) {
	return (Array.isArray(services) ? services.slice() : []).filter(function(service) {
		return isCustomService(service) &&
			isZapretPresetName(service && service.name) &&
			serviceSelectors(service).length > 0;
	}).sort(function(left, right) {
		return trim(left.name).localeCompare(trim(right.name));
	});
}

function buildFormState(settings, services) {
	var selectors = settings && settings.selectors ? settings.selectors : {};
	var activeSelectors = cleanList([].concat(selectors.domains || [], selectors.cidrs || []));
	var selectedPresets = {};
	var remaining = activeSelectors.slice();
	var persistedPresetNames = cleanList(selectors.services || []);
	var entries = availableZapretServices(services).slice().sort(function(left, right) {
		return serviceSelectors(right).length - serviceSelectors(left).length;
	});

	for (var i = 0; i < persistedPresetNames.length; i++) {
		if (!isZapretPresetName(persistedPresetNames[i]))
			continue;
		selectedPresets[persistedPresetNames[i]] = true;
	}

	for (var i = 0; i < entries.length; i++) {
		if (selectedPresets[trim(entries[i].name)] !== true)
			continue;
		remaining = remaining.filter(function(value) {
			return serviceSelectors(entries[i]).indexOf(value) < 0;
		});
	}

	if (Object.keys(selectedPresets).length > 0) {
		return {
			'enabled': settings && settings.enabled === true,
			'threshold': String((settings && settings.failback_success_threshold) || 3),
			'unmanagedSelectors': remaining,
			'selectedPresets': selectedPresets,
			'presetDraftName': '',
			'presetDraftSelectors': '',
			'editingPresetName': ''
		};
	}

	for (i = 0; i < entries.length; i++) {
		var presetSelectors = serviceSelectors(entries[i]);
		var matched = presetSelectors.length > 0;

		for (var j = 0; j < presetSelectors.length; j++) {
			if (remaining.indexOf(presetSelectors[j]) < 0) {
				matched = false;
				break;
			}
		}

		if (!matched)
			continue;

		selectedPresets[trim(entries[i].name)] = true;
		remaining = remaining.filter(function(value) {
			return presetSelectors.indexOf(value) < 0;
		});
	}

	return {
		'enabled': settings && settings.enabled === true,
		'threshold': String((settings && settings.failback_success_threshold) || 3),
		'unmanagedSelectors': remaining,
		'selectedPresets': selectedPresets,
		'presetDraftName': '',
		'presetDraftSelectors': '',
		'editingPresetName': ''
	};
}

function choiceClass(selected) {
	return selected === true
		? 'routeflux-zapret-choice routeflux-zapret-choice-selected'
		: 'routeflux-zapret-choice';
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'status' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execJSON([ '--json', 'zapret', 'get' ]).catch(function(err) {
				return { '__error__': err.message || String(err) };
			}),
			this.execJSON([ '--json', 'zapret', 'status' ]).catch(function(err) {
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

	initializePageState: function(data) {
		this.pageData = {
			'runtime': data[0] || {},
			'settings': data[1] || {},
			'status': data[2] || {},
			'services': Array.isArray(data[3]) ? data[3] : []
		};
		this.formState = buildFormState(this.pageData.settings, this.pageData.services);
	},

	renderIntoRoot: function() {
		var root = document.querySelector('#routeflux-zapret-root');
		if (root)
			dom.content(root, this.renderPageContent());
	},

	refreshPageContent: function() {
		return this.load().then(L.bind(function(data) {
			this.initializePageState(data);
			this.renderIntoRoot();
		}, this));
	},

	findServiceByName: function(name) {
		var services = this.pageData.services || [];
		var target = trim(name);

		for (var i = 0; i < services.length; i++) {
			if (trim(services[i].name) === target)
				return services[i];
		}

		return null;
	},

	availableServices: function() {
		return availableZapretServices(this.pageData.services || []);
	},

	upsertLocalService: function(service) {
		var services = Array.isArray(this.pageData.services) ? this.pageData.services.slice() : [];
		var target = trim(service && service.name);
		var updated = false;

		for (var i = 0; i < services.length; i++) {
			if (trim(services[i].name) !== target)
				continue;
			services[i] = service;
			updated = true;
			break;
		}

		if (!updated)
			services.push(service);

		this.pageData.services = services;
	},

	removeLocalService: function(name) {
		var target = trim(name);
		this.pageData.services = (Array.isArray(this.pageData.services) ? this.pageData.services : []).filter(function(service) {
			return trim(service && service.name) !== target;
		});
	},

	setLocalZapretSelectors: function(selectors) {
		var selectedPresetNames = this.selectedZapretPresetNames();
		if (!this.pageData.settings || this.pageData.settings.__error__)
			this.pageData.settings = {};

		this.pageData.settings.enabled = this.formState.enabled === true;
		this.pageData.settings.failback_success_threshold = Number(firstNonEmpty([ this.formState.threshold ], '3')) || 3;
		this.pageData.settings.selectors = {
			'services': selectedPresetNames.length > 0 ? selectedPresetNames : null,
			'domains': cleanList(selectors),
			'cidrs': null
		};
	},

	selectedZapretPresetNames: function() {
		var selected = this.formState.selectedPresets || {};
		var services = this.availableServices();
		var names = [];

		for (var i = 0; i < services.length; i++) {
			if (selected[trim(services[i].name)] !== true)
				continue;
			names.push(trim(services[i].name));
		}

		return cleanList(names);
	},

	runCommands: function(commands, successMessage, options) {
		var self = this;
		var outputs = [];
		var chain = Promise.resolve();
		var shouldRefresh = !(options && options.refresh === false);

		for (var i = 0; i < commands.length; i++) {
			(function(argv) {
				chain = chain.then(function() {
					return self.execText(argv).then(function(stdout) {
						outputs.push(stdout);
					});
				});
			})(commands[i]);
		}

		return chain.then(L.bind(function() {
			var message = '';

			for (var i = outputs.length - 1; i >= 0; i--) {
				if (trim(outputs[i]) !== '') {
					message = outputs[i];
					break;
				}
			}

			ui.addNotification(null, notificationParagraph(message || successMessage), 'info');
			if (shouldRefresh)
				return this.refreshPageContent();
			this.renderIntoRoot();
			return Promise.resolve();
		}, this)).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleSave: function() {
		var selectors = this.expandedSelectors();
		var selectorArgs = this.selectedZapretPresetNames();

		if (this.formState.enabled === true && selectorArgs.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Zapret fallback needs at least one allowed preset.')));
			return Promise.resolve();
		}

		this.setLocalZapretSelectors(selectors);

		return this.runCommands([
			[ 'zapret', 'set', 'enabled', this.formState.enabled === true ? 'true' : 'false' ],
			[ 'zapret', 'set', 'selectors' ].concat(selectorArgs),
			[ 'zapret', 'set', 'failback-success-threshold', firstNonEmpty([ this.formState.threshold ], '3') ]
		], _('Zapret fallback settings saved.'), { 'refresh': false });
	},

	handleDisableFallback: function() {
		this.formState.enabled = false;
		this.setLocalZapretSelectors(this.expandedSelectors());
		return this.runCommands([
			[ 'zapret', 'set', 'enabled', 'false' ]
		], _('Zapret fallback disabled.'), { 'refresh': false });
	},

	handleRefreshPage: function() {
		return this.refreshPageContent();
	},

	handleStartTest: function() {
		return this.runCommands([
			[ 'zapret', 'test', 'start' ]
		], _('Zapret test mode started.'));
	},

	handleStopTest: function() {
		return this.runCommands([
			[ 'zapret', 'test', 'stop' ]
		], _('Zapret test mode stopped.'));
	},

	handleFallbackChoice: function(ev) {
		this.formState.enabled = trim(ev && ev.currentTarget && ev.currentTarget.value) === 'enabled';
		this.renderIntoRoot();
	},

	handleThresholdInput: function(ev) {
		this.formState.threshold = trim(ev && ev.currentTarget && ev.currentTarget.value);
	},

	handleStartNewPreset: function(ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		this.formState.editingPresetName = '';
		this.formState.presetDraftName = '';
		this.formState.presetDraftSelectors = '';
		this.renderIntoRoot();
	},

	handleEditPreset: function(name, ev) {
		var service = this.findServiceByName(name);

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!service || !isCustomService(service))
			return;

		this.formState.editingPresetName = trim(service.name);
		this.formState.presetDraftName = zapretPresetDisplayName(service.name);
		this.formState.presetDraftSelectors = serviceSelectors(service).join('\n');
		this.renderIntoRoot();
	},

	handleCancelPresetEditor: function(ev) {
		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		this.formState.editingPresetName = '';
		this.formState.presetDraftName = '';
		this.formState.presetDraftSelectors = '';
		this.renderIntoRoot();
	},

	readPresetDraftName: function() {
		var field = document.getElementById('routeflux-zapret-preset-name');
		return field ? String(field.value || '') : String(this.formState.presetDraftName || '');
	},

	readPresetDraftSelectors: function() {
		var field = document.getElementById('routeflux-zapret-preset-selectors');
		return field ? String(field.value || '') : String(this.formState.presetDraftSelectors || '');
	},

	persistPresetSelectors: function(successMessage) {
		var self = this;
		var selectors = this.expandedSelectors();
		var selectorArgs = this.selectedZapretPresetNames();
		this.setLocalZapretSelectors(selectors);

		return this.execText([ 'zapret', 'set', 'selectors' ].concat(selectorArgs)).then(function(stdout) {
			ui.addNotification(null, notificationParagraph(trim(stdout) || successMessage), 'info');
			self.renderIntoRoot();
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleSavePreset: function(ev) {
		var editingName = trim(this.formState.editingPresetName);
		var draftName = trim(this.readPresetDraftName());
		var name = zapretPresetStorageName(draftName);
		var parsed = parseSelectorInput(this.readPresetDraftSelectors());
		var self = this;

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (draftName === '') {
			ui.addNotification(null, notificationParagraph(_('Preset name is required.')));
			return Promise.resolve();
		}

		if (name === '') {
			ui.addNotification(null, notificationParagraph(_('Preset name must include Latin letters or digits.')));
			return Promise.resolve();
		}

		if (parsed.invalid.length > 0) {
			ui.addNotification(null, notificationParagraph(_('Use fully qualified domains or IPv4 selectors only. Invalid value: %s').format(parsed.invalid[0])));
			return Promise.resolve();
		}

		if (parsed.values.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Add at least one domain or IPv4 selector to the preset.')));
			return Promise.resolve();
		}

		return this.execText([ 'services', 'set', name ].concat(parsed.values)).then(function(stdout) {
			if (editingName !== '' && editingName !== name)
				return self.execText([ 'services', 'delete', editingName ]).then(function() {
					return stdout;
				});
			return stdout;
		}).then(function(stdout) {
			return self.execJSON([ '--json', 'services', 'get', name ]).then(function(service) {
				var selected = Object.assign({}, self.formState.selectedPresets || {});
				if (editingName !== '' && editingName !== name)
					self.removeLocalService(editingName);
				self.upsertLocalService(service);
				if (editingName !== '' && editingName !== name)
					delete selected[editingName];
				selected[name] = true;
				self.formState.selectedPresets = selected;
				self.formState.unmanagedSelectors = [];
				self.formState.editingPresetName = '';
				self.formState.presetDraftName = '';
				self.formState.presetDraftSelectors = '';
				return self.persistPresetSelectors(trim(stdout) || _('Zapret preset saved.'));
			});
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleDeletePreset: function(name, ev) {
		var self = this;
		var selected = Object.assign({}, this.formState.selectedPresets || {});

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		return this.execText([ 'services', 'delete', trim(name) ]).then(function(stdout) {
				delete selected[trim(name)];
				self.removeLocalService(name);
				self.formState.selectedPresets = selected;
				self.formState.unmanagedSelectors = [];
				if (Object.keys(selected).length === 0)
					self.formState.enabled = false;
				if (trim(self.formState.editingPresetName) === trim(name)) {
					self.formState.editingPresetName = '';
					self.formState.presetDraftName = '';
					self.formState.presetDraftSelectors = '';
				}
				return self.persistPresetSelectors(trim(stdout) || _('Zapret preset deleted.'));
		}).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	expandedSelectors: function() {
		var values = [];
		var selected = this.formState.selectedPresets || {};
		var services = this.availableServices();

		for (var i = 0; i < services.length; i++) {
			if (selected[trim(services[i].name)] !== true)
				continue;
			values = cleanList(values.concat(serviceSelectors(services[i])));
		}

		return values;
	},

	renderCard: function(label, value, options) {
		return routefluxUI.renderSummaryCard(label, value, options);
	},

	renderWarnings: function(settings, status) {
		var warnings = [];
		var selectors = settings && settings.selectors ? settings.selectors : {};

		if (status.installed !== true)
			warnings.push(_('The zapret-openwrt package is not installed, so fallback cannot activate.'));

		if (status.service_active === true && status.managed !== true && status.test_active !== true)
			warnings.push(_('A Zapret service is already running outside RouteFlux. RouteFlux can take over that service the next time fallback or test mode starts.'));

		if (!selectorHasEntries(selectors))
			warnings.push(_('Zapret selectors are empty. Add at least one allowed preset before enabling fallback.'));

		if ((this.formState.unmanagedSelectors || []).length > 0)
			warnings.push(_('Some existing Zapret selectors do not match a saved custom preset. Save from LuCI keeps only the custom presets listed on this page.'));

		if (warnings.length === 0)
			return '';

		return E('div', { 'class': 'cbi-section' }, [
			E('div', { 'class': 'alert-message warning' }, warnings.map(function(message) {
				return E('div', {}, [ message ]);
			}))
		]);
	},

	renderPresetRows: function() {
		var services = this.availableServices();
		var rows = [];
		var self = this;

		for (var i = 0; i < services.length; i++) {
			var service = services[i];
			var details = [];
			var selected = this.formState.selectedPresets[trim(service.name)] === true;

			if (!selected)
				continue;

			if (cleanList(service.domains || []).length > 0)
				details.push(_('Domains: %d').format(cleanList(service.domains || []).length));
			if (cleanList(service.cidrs || []).length > 0)
				details.push(_('IPv4: %d').format(cleanList(service.cidrs || []).length));
			details.unshift(_('Added to Zapret'));

			rows.push(E('div', { 'class': 'routeflux-zapret-item routeflux-zapret-item-preset' }, [
				E('span', { 'class': 'routeflux-zapret-badge routeflux-zapret-badge-preset' }, [ _('Preset') ]),
				E('div', { 'class': 'routeflux-zapret-item-value' }, [
					E('strong', {}, [ zapretPresetDisplayName(service.name) ]),
					E('div', { 'class': 'routeflux-zapret-item-meta' }, [ details.join(' · ') || _('Ready to use in Zapret.') ])
				]),
				isCustomService(service) ? E('button', {
					'class': 'cbi-button cbi-button-action',
					'type': 'button',
					'click': ui.createHandlerFn(self, 'handleEditPreset', service.name)
				}, [ _('Change') ]) : '',
				isCustomService(service) ? E('button', {
					'class': 'cbi-button cbi-button-negative',
					'type': 'button',
					'click': ui.createHandlerFn(self, 'handleDeletePreset', service.name)
				}, [ _('Delete') ]) : ''
			]));
		}

		if (rows.length === 0)
			return E('div', { 'class': 'routeflux-zapret-empty' }, [ _('No custom presets added to Zapret yet.') ]);

		return E('div', { 'class': 'routeflux-zapret-list' }, rows);
	},

	renderPageContent: function() {
		var runtime = this.pageData.runtime || {};
		var settings = this.pageData.settings || {};
		var status = this.pageData.status || {};
		var services = this.availableServices();
		var selectors = this.expandedSelectors();
		var selectedPresetCount = Object.keys(this.formState.selectedPresets || {}).length;
		var transport = firstNonEmpty([
			runtime.active_transport,
			runtime.state && runtime.state.active_transport
		], 'direct');
		var serviceLabel = status.service_active === true
			? (status.managed === true ? _('running (RouteFlux)') : _('running (external)'))
			: firstNonEmpty([ status.service_state ], _('not-installed'));
		var lastReason = status.test_active === true
			? _('Zapret test mode active')
			: firstNonEmpty([ status.last_reason ], _('Clear'));
		var canStartTest = status.test_active !== true && transport !== 'zapret';
		var cards = [
			this.renderCard(_('Installed'), status.installed === true ? _('Yes') : _('No'), {
				'tone': status.installed === true ? 'connected' : 'disconnected',
				'primary': true
			}),
			this.renderCard(_('Current Transport'), transport),
			this.renderCard(_('Zapret Service'), serviceLabel),
			this.renderCard(_('Last Reason'), lastReason)
		];

		[this.pageData.runtime, this.pageData.settings, this.pageData.status, this.pageData.services].forEach(function(entry, index) {
			if (entry && entry.__error__)
				ui.addNotification(null, notificationParagraph(_('RouteFlux data error %s: %s').format(String(index + 1), entry.__error__)));
		});

		return [
			routefluxUI.renderSharedStyles(),
			E('style', { 'type': 'text/css' }, [
				'#routeflux-zapret-root { --routeflux-zapret-ink:#102234; --routeflux-zapret-ink-muted:#3e5368; --routeflux-zapret-ink-soft:#5c7085; --routeflux-zapret-panel-bg:linear-gradient(160deg, rgba(248, 250, 253, 0.98) 0%, rgba(239, 244, 249, 0.98) 56%, rgba(231, 239, 246, 0.98) 100%); --routeflux-zapret-surface-bg:linear-gradient(180deg, rgba(251, 252, 254, 0.97) 0%, rgba(244, 248, 252, 0.97) 100%); }',
				'#routeflux-zapret-root.routeflux-theme-dark { --routeflux-zapret-ink:#eef4ff; --routeflux-zapret-ink-muted:#a8b8ce; --routeflux-zapret-ink-soft:#8ea0b8; --routeflux-zapret-panel-bg:linear-gradient(160deg, rgba(15, 23, 37, 0.96) 0%, rgba(10, 17, 29, 0.98) 56%, rgba(8, 13, 24, 0.99) 100%); --routeflux-zapret-surface-bg:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); }',
				'#routeflux-zapret-root.routeflux-theme-dark::before, #routeflux-zapret-root.routeflux-theme-dark::after { display:none; }',
				'#routeflux-zapret-root .routeflux-zapret-layout { display:grid; gap:14px; padding:0; border:0; background:transparent; box-shadow:none; color:var(--routeflux-zapret-ink); overflow:visible; }',
				'#routeflux-zapret-root .routeflux-zapret-layout::before { display:none; }',
				'.routeflux-zapret-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(240px, 1fr)); gap:14px; }',
				'.routeflux-zapret-panel { position:relative; overflow:hidden; isolation:isolate; border:1px solid rgba(120, 141, 167, 0.3); border-radius:18px; padding:18px; background:var(--routeflux-zapret-surface-bg); box-shadow:none; }',
				'.routeflux-zapret-panel::before { display:none; }',
				'.routeflux-zapret-panel > * { position:relative; z-index:1; }',
				'.routeflux-zapret-panel h3 { margin:0; color:var(--routeflux-zapret-ink); font-size:clamp(24px, 1.1vw + 18px, 34px); line-height:1.12; letter-spacing:-0.03em; }',
				'.routeflux-zapret-choice-grid, .routeflux-zapret-editor-grid { display:grid; gap:14px; }',
				'.routeflux-zapret-choice { position:relative; display:flex; gap:12px; align-items:flex-start; padding:16px 18px; border:1px solid rgba(120, 141, 167, 0.34); border-radius:18px; background:var(--routeflux-zapret-surface-bg); box-shadow:0 14px 30px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.9); transition:transform .18s ease, border-color .18s ease, box-shadow .18s ease, background .18s ease; cursor:pointer; }',
				'.routeflux-zapret-choice:hover { transform:translateY(-1px); border-color:rgba(34, 197, 94, 0.34); box-shadow:0 16px 32px rgba(15, 23, 42, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.94); }',
				'.routeflux-zapret-choice-selected { border-color:rgba(34, 197, 94, 0.52); background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(220, 252, 231, 0.99) 100%); box-shadow:0 18px 34px rgba(21, 128, 61, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.96); }',
				'.routeflux-zapret-choice-control { position:absolute; width:1px; height:1px; margin:-1px; padding:0; border:0; overflow:hidden; clip:rect(0, 0, 0, 0); clip-path:inset(50%); white-space:nowrap; }',
				'.routeflux-zapret-choice-indicator { position:relative; flex:0 0 auto; width:26px; height:26px; margin-top:2px; border:1.5px solid rgba(71, 85, 105, 0.42); border-radius:999px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(241, 245, 249, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.95), 0 10px 18px rgba(15, 23, 42, 0.08); transition:border-color .18s ease, box-shadow .18s ease, background .18s ease; }',
				'.routeflux-zapret-choice-indicator::after { content:""; position:absolute; inset:0; display:flex; align-items:center; justify-content:center; border-radius:999px; color:transparent; transform:scale(0.62); transition:transform .18s ease, background .18s ease, color .18s ease, box-shadow .18s ease; }',
				'.routeflux-zapret-choice-control:focus-visible + .routeflux-zapret-choice-indicator { outline:2px solid rgba(34, 197, 94, 0.28); outline-offset:3px; }',
				'.routeflux-zapret-choice-selected .routeflux-zapret-choice-indicator { border-color:rgba(22, 163, 74, 0.54); background:linear-gradient(180deg, rgba(240, 253, 244, 0.99) 0%, rgba(220, 252, 231, 0.99) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.96), 0 12px 20px rgba(21, 128, 61, 0.12); }',
				'.routeflux-zapret-choice-selected .routeflux-zapret-choice-indicator::after { content:"\\2713"; background:linear-gradient(180deg, #22c55e 0%, #15803d 100%); color:#ffffff; font-size:15px; font-weight:900; transform:scale(1); box-shadow:0 10px 18px rgba(21, 128, 61, 0.28); }',
				'.routeflux-zapret-choice-copy { flex:1 1 auto; min-width:0; }',
				'.routeflux-zapret-choice-title { display:block; font-weight:800; font-size:clamp(18px, 0.55vw + 15px, 24px); color:var(--routeflux-zapret-ink); letter-spacing:-0.02em; }',
				'.routeflux-zapret-choice-description { display:block; margin-top:6px; color:var(--routeflux-zapret-ink-muted); line-height:1.62; font-size:15px; font-weight:500; }',
				'.routeflux-zapret-editor-head { display:grid; gap:10px; margin-bottom:16px; }',
				'.routeflux-zapret-panel .cbi-section-descr { color:var(--routeflux-zapret-ink-muted); font-size:15px; font-weight:500; line-height:1.66; max-width:72ch; }',
				'.routeflux-zapret-editor-head .cbi-section-descr { margin:0; }',
				'.routeflux-zapret-editor-kicker { display:inline-flex; align-items:center; width:max-content; max-width:100%; padding:5px 11px; border-radius:999px; background:rgba(14, 165, 233, 0.14); color:#075985; font-size:12px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
				'.routeflux-zapret-inline { display:flex; gap:10px; align-items:stretch; }',
				'.routeflux-zapret-inline > .cbi-input-text { flex:1 1 auto; min-width:0; min-height:48px; padding:0 14px; border:1px solid rgba(71, 85, 105, 0.2); border-radius:14px; background:rgba(255, 255, 255, 0.98); color:var(--routeflux-zapret-ink); box-shadow:none; }',
				'.routeflux-zapret-inline > .cbi-input-textarea { flex:1 1 auto; min-width:0; min-height:112px; padding:14px; border:1px solid rgba(71, 85, 105, 0.2); border-radius:14px; background:rgba(255, 255, 255, 0.98); color:var(--routeflux-zapret-ink); box-shadow:none; resize:vertical; }',
				'.routeflux-zapret-inline > .cbi-input-text:focus { border-color:rgba(14, 165, 233, 0.72); box-shadow:0 0 0 1px rgba(14, 165, 233, 0.24), 0 16px 30px rgba(14, 165, 233, 0.16); }',
				'.routeflux-zapret-inline > .cbi-input-textarea:focus { border-color:rgba(14, 165, 233, 0.72); box-shadow:0 0 0 1px rgba(14, 165, 233, 0.24), 0 16px 30px rgba(14, 165, 233, 0.16); }',
				'.routeflux-theme-light .routeflux-zapret-inline > .cbi-input-text { border-color:rgba(125, 146, 170, 0.18); background:#fcfdff; color:#162638; box-shadow:none; }',
				'.routeflux-theme-light .routeflux-zapret-inline > .cbi-input-textarea { border-color:rgba(125, 146, 170, 0.18); background:#fcfdff; color:#162638; box-shadow:none; }',
				'.routeflux-theme-light .routeflux-zapret-inline > .cbi-input-text::placeholder { color:#63768c; opacity:1; }',
				'.routeflux-theme-light .routeflux-zapret-inline > .cbi-input-textarea::placeholder { color:#63768c; opacity:1; }',
				'.routeflux-theme-dark .routeflux-zapret-inline > .cbi-input-text { border-color:rgba(145, 175, 220, 0.16); background:rgba(6, 12, 22, 0.88); color:#eef4ff; box-shadow:none; }',
				'.routeflux-theme-dark .routeflux-zapret-inline > .cbi-input-textarea { border-color:rgba(145, 175, 220, 0.16); background:rgba(6, 12, 22, 0.88); color:#eef4ff; box-shadow:none; }',
				'.routeflux-theme-dark .routeflux-zapret-inline > .cbi-input-text::placeholder { color:#91a2bd; opacity:0.84; }',
				'.routeflux-theme-dark .routeflux-zapret-inline > .cbi-input-textarea::placeholder { color:#91a2bd; opacity:0.84; }',
				'.routeflux-zapret-inline > .cbi-button-action, .routeflux-zapret-actions .cbi-button { min-height:52px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b; font-weight:800; box-shadow:0 12px 22px rgba(63, 87, 118, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.84); }',
				'.routeflux-zapret-inline > .cbi-button-action:hover, .routeflux-zapret-actions .cbi-button:hover { border-color:rgba(37, 99, 235, 0.28); background:linear-gradient(180deg, rgba(236, 244, 251, 0.99) 0%, rgba(225, 236, 247, 0.99) 100%); color:#102f4c; }',
				'.routeflux-zapret-list { display:grid; gap:8px; }',
				'.routeflux-zapret-item { display:flex; gap:12px; align-items:center; padding:12px 14px; border-radius:15px; background:rgba(255, 255, 255, 0.93); border:1px solid rgba(125, 145, 168, 0.22); box-shadow:none; }',
				'.routeflux-zapret-item-meta { margin-top:4px; color:var(--routeflux-zapret-ink-muted); font-size:13px; font-weight:600; }',
				'.routeflux-zapret-item .cbi-button-action { min-width:92px; border-radius:12px; }',
				'.routeflux-zapret-item .cbi-button-negative { min-width:92px; border-radius:12px; }',
				'.routeflux-zapret-item-value { flex:1 1 auto; min-width:0; word-break:break-word; font-weight:700; color:var(--routeflux-zapret-ink); }',
				'.routeflux-zapret-badge { display:inline-flex; align-items:center; justify-content:center; min-width:58px; padding:4px 8px; border-radius:999px; background:rgba(16, 185, 129, 0.14); color:#047857; font-size:11px; font-weight:800; letter-spacing:.05em; text-transform:uppercase; }',
				'.routeflux-zapret-badge-preset { background:rgba(37, 99, 235, 0.14); color:#1d4ed8; }',
				'.routeflux-zapret-empty { padding:14px; border-radius:14px; background:rgba(255, 255, 255, 0.82); border:1px dashed rgba(125, 145, 168, 0.42); color:var(--routeflux-zapret-ink-muted); font-size:15px; font-weight:500; }',
				'.routeflux-zapret-summary-shell { padding:14px 16px; border:1px solid rgba(125, 145, 168, 0.22); border-radius:14px; background:rgba(255, 255, 255, 0.72); box-shadow:none; }',
				'.routeflux-zapret-summary-shell h3 { margin-top:0; margin-bottom:8px; color:var(--routeflux-zapret-ink); font-size:18px; letter-spacing:-0.02em; }',
				'.routeflux-zapret-summary-list { margin:0; padding-left:18px; color:var(--routeflux-zapret-ink-soft); line-height:1.65; font-size:15px; }',
				'.routeflux-zapret-summary-list li { color:var(--routeflux-zapret-ink); font-weight:500; }',
				'.routeflux-zapret-summary-list li + li { margin-top:6px; }',
				'.routeflux-zapret-actions { display:flex; flex-wrap:wrap; gap:10px; }',
				'.routeflux-theme-dark .routeflux-zapret-panel { border-color:rgba(145, 175, 220, 0.14); background:rgba(8, 14, 24, 0.94); box-shadow:none; }',
				'.routeflux-theme-dark .routeflux-zapret-choice { border-color:rgba(145, 175, 220, 0.16); background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); box-shadow:0 18px 32px rgba(0, 0, 0, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.04); }',
				'.routeflux-theme-dark .routeflux-zapret-choice-selected { border-color:rgba(34, 197, 94, 0.42); background:linear-gradient(180deg, rgba(13, 35, 28, 0.96) 0%, rgba(10, 24, 21, 1) 100%); box-shadow:0 22px 38px rgba(8, 23, 19, 0.32), 0 0 0 1px rgba(34, 197, 94, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.06); }',
				'.routeflux-theme-dark .routeflux-zapret-choice-indicator { border-color:rgba(145, 162, 189, 0.42); background:linear-gradient(180deg, rgba(22, 31, 45, 0.96) 0%, rgba(14, 22, 34, 1) 100%); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 10px 18px rgba(0, 0, 0, 0.22); }',
				'.routeflux-theme-dark .routeflux-zapret-inline > .cbi-button-action, .routeflux-theme-dark .routeflux-zapret-actions .cbi-button-action { border-color:rgba(120, 160, 214, 0.2); background:rgba(12, 20, 34, 0.82); color:#a8d7ff; box-shadow:0 12px 24px rgba(0, 0, 0, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.03); }',
				'.routeflux-theme-dark .routeflux-zapret-actions .cbi-button-apply { border-color:rgba(88, 196, 255, 0.34); background:linear-gradient(180deg, rgba(52, 147, 235, 0.92) 0%, rgba(30, 116, 211, 0.94) 100%); color:#f4fbff; box-shadow:0 18px 32px rgba(30, 116, 211, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.14); }',
				'.routeflux-theme-dark .routeflux-zapret-actions .cbi-button-reset, .routeflux-theme-dark .routeflux-zapret-item .cbi-button-negative { border-color:rgba(255, 123, 140, 0.28); background:rgba(52, 16, 26, 0.82); color:#ffb7c0; box-shadow:0 16px 28px rgba(52, 16, 26, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.05); }',
				'.routeflux-theme-dark .routeflux-zapret-item { background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); border-color:rgba(145, 175, 220, 0.14); box-shadow:none; }',
				'.routeflux-theme-dark .routeflux-zapret-empty { background:rgba(8, 15, 26, 0.5); border-color:rgba(145, 175, 220, 0.24); color:#a8b8ce; }',
				'.routeflux-theme-dark .routeflux-zapret-summary-shell { background:rgba(8, 15, 26, 0.58); border-color:rgba(145, 175, 220, 0.16); box-shadow:none; }',
				'@media (max-width: 720px) { .routeflux-zapret-inline { flex-direction:column; } .routeflux-zapret-inline > .cbi-button-action, .routeflux-zapret-actions .cbi-button { width:100%; } .routeflux-zapret-item { align-items:flex-start; flex-direction:column; } .routeflux-zapret-item .cbi-button-negative, .routeflux-zapret-item .cbi-button-action { width:100%; } }'
			]),
			E('h2', {}, [ _('RouteFlux - Zapret') ]),
			E('p', { 'class': 'cbi-section-descr' }, [
				_('Configure Zapret as an automatic fallback transport when no healthy proxy node remains in auto mode, and use a dedicated test mode when you want to verify Zapret before any proxy failure happens.')
			]),
			E('div', { 'class': 'routeflux-overview-grid' }, cards),
			this.renderWarnings(settings, status),
			E('div', { 'class': 'cbi-section routeflux-zapret-layout' }, [
				E('div', { 'class': 'routeflux-zapret-grid' }, [
					E('div', { 'class': 'routeflux-zapret-panel' }, [
						E('h3', {}, [ _('Automatic fallback') ]),
						E('div', { 'class': 'routeflux-zapret-choice-grid', 'style': 'margin-top:16px;' }, [
							E('label', { 'class': choiceClass(this.formState.enabled === true) }, [
								E('input', {
									'class': 'routeflux-zapret-choice-control',
									'type': 'radio',
									'name': 'routeflux-zapret-fallback',
									'value': 'enabled',
									'checked': this.formState.enabled === true ? 'checked' : null,
									'change': ui.createHandlerFn(this, 'handleFallbackChoice')
								}),
								E('span', { 'class': 'routeflux-zapret-choice-indicator', 'aria-hidden': 'true' }, []),
								E('div', { 'class': 'routeflux-zapret-choice-copy' }, [
									E('span', { 'class': 'routeflux-zapret-choice-title' }, [ _('Enabled') ]),
									E('span', { 'class': 'routeflux-zapret-choice-description' }, [
										_('Allow RouteFlux to switch into Zapret after proxy failure and return to proxy only after stable health checks pass.')
									])
								])
							]),
							E('label', { 'class': choiceClass(this.formState.enabled !== true) }, [
								E('input', {
									'class': 'routeflux-zapret-choice-control',
									'type': 'radio',
									'name': 'routeflux-zapret-fallback',
									'value': 'disabled',
									'checked': this.formState.enabled !== true ? 'checked' : null,
									'change': ui.createHandlerFn(this, 'handleFallbackChoice')
								}),
								E('span', { 'class': 'routeflux-zapret-choice-indicator', 'aria-hidden': 'true' }, []),
								E('div', { 'class': 'routeflux-zapret-choice-copy' }, [
									E('span', { 'class': 'routeflux-zapret-choice-title' }, [ _('Disabled') ]),
									E('span', { 'class': 'routeflux-zapret-choice-description' }, [
										_('Keep Zapret available for manual testing only and let RouteFlux fail open if proxy routing disappears.')
									])
								])
							])
						]),
						E('div', { 'class': 'routeflux-zapret-editor-grid', 'style': 'margin-top:16px;' }, [
							E('div', {}, [
								E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-zapret-threshold' }, [ _('Failback Success Threshold') ]),
								E('input', {
									'id': 'routeflux-zapret-threshold',
									'class': 'cbi-input-text',
									'type': 'number',
									'min': '1',
									'value': this.formState.threshold,
									'input': ui.createHandlerFn(this, 'handleThresholdInput')
								}),
								E('div', { 'class': 'cbi-value-description' }, [
									_('How many consecutive healthy checks a proxy candidate must pass before RouteFlux leaves Zapret and returns to proxy.')
								])
							])
						])
					]),
					E('div', { 'class': 'routeflux-zapret-panel' }, [
						E('h3', {}, [ _('Zapret Test') ]),
						E('div', { 'class': 'routeflux-zapret-summary-shell', 'style': 'margin-top:16px;' }, [
							E('h3', {}, [ status.test_active === true ? _('Test mode is active') : _('Manual verification mode') ]),
							E('ul', { 'class': 'routeflux-zapret-summary-list' }, [
								E('li', {}, [ _('Use this test mode to switch this router into Zapret even while proxy nodes stay healthy.') ]),
								E('li', {}, [ _('While the test is active, RouteFlux pauses automatic failback decisions and keeps traffic on Zapret until you stop the test.') ]),
								E('li', {}, [ _('When you leave the test, RouteFlux restores the previous route if one existed.') ])
							])
						]),
						E('div', { 'class': 'routeflux-zapret-actions', 'style': 'margin-top:16px;' }, [
							canStartTest ? E('button', {
								'class': 'cbi-button cbi-button-apply',
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleStartTest')
							}, [ _('Start ZapretTest') ]) : '',
							status.test_active === true ? E('button', {
								'class': 'cbi-button cbi-button-action',
								'type': 'button',
								'click': ui.createHandlerFn(this, 'handleStopTest')
							}, [ _('Return to Previous Route') ]) : ''
						])
					])
				]),
				E('div', { 'class': 'routeflux-zapret-panel' }, [
					E('div', { 'class': 'routeflux-zapret-editor-head' }, [
						E('span', { 'class': 'routeflux-zapret-editor-kicker' }, [ _('Selectors') ]),
						E('h3', {}, [ _('Choose what Zapret should cover') ]),
						E('p', { 'class': 'cbi-section-descr' }, [
							_('Create custom presets from domains and IPv4 selectors. Zapret only covers the presets listed below.')
						])
					]),
					E('div', { 'class': 'routeflux-zapret-summary-shell' }, [
						E('h3', {}, [ trim(this.formState.editingPresetName) !== '' ? _('Edit custom preset') : _('Create custom preset') ]),
						E('p', { 'class': 'cbi-section-descr', 'style': 'margin:10px 0 0 0;' }, [
							_('Saved presets are added to Zapret immediately. If no presets are listed below, fallback stays disabled.')
						]),
						E('div', { 'class': 'routeflux-zapret-editor-grid', 'style': 'margin-top:12px;' }, [
							E('div', {}, [
								E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-zapret-preset-name' }, [ _('Preset name') ]),
								E('input', {
									'id': 'routeflux-zapret-preset-name',
									'class': 'cbi-input-text',
									'type': 'text',
									'placeholder': 'YouTube',
									'value': this.formState.presetDraftName,
									'spellcheck': 'false',
									'autocapitalize': 'off',
									'autocorrect': 'off',
									'autocomplete': 'off',
									'inputmode': 'text'
								})
							]),
							E('div', {}, [
								E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-zapret-preset-selectors' }, [ _('Preset domains and IPv4') ]),
								E('textarea', {
									'id': 'routeflux-zapret-preset-selectors',
									'class': 'cbi-input-textarea',
									'placeholder': _('Examples: youtube.com\\ngooglevideo.com\\n203.0.113.0/24'),
									'spellcheck': 'false',
									'autocapitalize': 'off',
									'autocorrect': 'off',
									'autocomplete': 'off',
									'wrap': 'off'
								}, [ this.formState.presetDraftSelectors ])
							]),
							E('div', { 'class': 'routeflux-zapret-actions' }, [
								E('button', {
									'class': 'cbi-button cbi-button-apply',
									'type': 'button',
									'click': ui.createHandlerFn(this, 'handleSavePreset')
								}, [ _('Save preset') ]),
								E('button', {
									'class': 'cbi-button cbi-button-action',
									'type': 'button',
									'click': ui.createHandlerFn(this, 'handleCancelPresetEditor')
								}, [ _('Cancel') ])
							])
						])
					]),
					E('div', { 'style': 'margin-top:16px;' }, [
						E('div', { 'class': 'cbi-section-descr', 'style': 'margin:0 0 10px 0;' }, [
							_('Active presets in Zapret: %d. Expanded selectors: %d.').format(selectedPresetCount, selectors.length)
						]),
						this.renderPresetRows()
					]),
					E('div', { 'class': 'routeflux-zapret-actions', 'style': 'margin-top:16px;' }, [
						E('button', {
							'class': 'cbi-button cbi-button-apply',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleSave')
						}, [ _('Save') ]),
						E('button', {
							'class': 'cbi-button cbi-button-reset',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleDisableFallback')
						}, [ _('Disable Fallback') ]),
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'type': 'button',
							'click': ui.createHandlerFn(this, 'handleRefreshPage')
						}, [ _('Refresh page') ])
					])
				])
			])
		];
	},

	render: function(data) {
		this.initializePageState(data);
		return E('div', {
			'id': 'routeflux-zapret-root',
			'class': routefluxUI.withThemeClass('routeflux-page-shell routeflux-page-shell-zapret')
		}, this.renderPageContent());
	},

	handleSaveApply: null,
	handleReset: null
});
