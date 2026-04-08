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

function cloneSelectors(value) {
	var selectors = value || {};

	return {
		'services': cleanList(selectors.services || []),
		'domains': cleanList(selectors.domains || [])
	};
}

function selectorHasEntries(value) {
	var selectors = cloneSelectors(value);

	return selectors.services.length > 0 || selectors.domains.length > 0;
}

function selectorCount(value) {
	var selectors = cloneSelectors(value);

	return selectors.services.length + selectors.domains.length;
}

function normalizedDomain(value) {
	return trim(value).toLowerCase();
}

function domainLooksValid(value) {
	var candidate = normalizedDomain(value);

	return candidate !== '' &&
		candidate.indexOf('://') < 0 &&
		candidate.indexOf(' ') < 0 &&
		candidate.indexOf('.') > 0;
}

function choiceClass(selected) {
	return selected === true
		? 'routeflux-zapret-choice routeflux-zapret-choice-selected'
		: 'routeflux-zapret-choice';
}

function serviceNames(services) {
	var list = Array.isArray(services) ? services : [];
	var out = [];

	for (var i = 0; i < list.length; i++) {
		var name = trim(list[i] && list[i].name);
		if (name !== '')
			out.push(name);
	}

	return cleanList(out);
}

function buildFormState(settings, services) {
	var names = serviceNames(services);

	return {
		'enabled': settings && settings.enabled === true,
		'threshold': String((settings && settings.failback_success_threshold) || 3),
		'selectors': cloneSelectors(settings && settings.selectors),
		'serviceChoice': names.length > 0 ? names[0] : '',
		'domainInput': ''
	};
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

		return chain.then(L.bind(function() {
			var message = '';

			for (var i = outputs.length - 1; i >= 0; i--) {
				if (trim(outputs[i]) !== '') {
					message = outputs[i];
					break;
				}
			}

			ui.addNotification(null, notificationParagraph(message || successMessage), 'info');
			return this.refreshPageContent();
		}, this)).catch(function(err) {
			ui.addNotification(null, notificationParagraph(err.message || String(err)));
			throw err;
		});
	},

	handleSave: function() {
		var selectors = this.formState && this.formState.selectors ? this.formState.selectors : {};
		var values = cleanList((selectors.services || []).concat(selectors.domains || []));

		return this.runCommands([
			[ 'zapret', 'set', 'enabled', this.formState.enabled === true ? 'true' : 'false' ],
			[ 'zapret', 'set', 'selectors' ].concat(values),
			[ 'zapret', 'set', 'failback-success-threshold', firstNonEmpty([ this.formState.threshold ], '3') ]
		], _('Zapret fallback settings saved.'));
	},

	handleDisableFallback: function() {
		return this.runCommands([
			[ 'zapret', 'set', 'enabled', 'false' ]
		], _('Zapret fallback disabled.'));
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

	handleServiceChoice: function(ev) {
		this.formState.serviceChoice = trim(ev && ev.currentTarget && ev.currentTarget.value);
	},

	handleDomainInput: function(ev) {
		this.formState.domainInput = trim(ev && ev.currentTarget && ev.currentTarget.value);
	},

	handleAddService: function(ev) {
		var value = trim(this.formState.serviceChoice);
		var entries = cloneSelectors(this.formState.selectors);

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (value === '')
			return;

		entries.services = cleanList(entries.services.concat([ value ]));
		this.formState.selectors = entries;
		this.renderIntoRoot();
	},

	handleAddDomain: function(ev) {
		var value = trim(this.formState.domainInput);
		var entries = cloneSelectors(this.formState.selectors);

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		if (!domainLooksValid(value)) {
			ui.addNotification(null, notificationParagraph(_('Enter a fully qualified domain like youtube.com.')));
			return;
		}

		entries.domains = cleanList(entries.domains.concat([ value ]));
		this.formState.selectors = entries;
		this.formState.domainInput = '';
		this.renderIntoRoot();
	},

	handleRemoveSelector: function(kind, index, ev) {
		var entries = cloneSelectors(this.formState.selectors);
		var key = kind === 'services' ? 'services' : 'domains';

		if (ev && typeof ev.preventDefault === 'function')
			ev.preventDefault();

		entries[key].splice(index, 1);
		this.formState.selectors = entries;
		this.renderIntoRoot();
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
			warnings.push(_('Zapret selectors are empty. Add service aliases or fully qualified domains before enabling fallback.'));

		if (warnings.length === 0)
			return '';

		return E('div', { 'class': 'cbi-section' }, [
			E('div', { 'class': 'alert-message warning' }, warnings.map(function(message) {
				return E('div', {}, [ message ]);
			}))
		]);
	},

	renderSelectorRows: function(selectors) {
		var rows = [];
		var services = cleanList(selectors && selectors.services);
		var domains = cleanList(selectors && selectors.domains);
		var self = this;
		var i;

		for (i = 0; i < services.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-zapret-item' }, [
				E('span', { 'class': 'routeflux-zapret-badge' }, [ _('Preset') ]),
				E('span', { 'class': 'routeflux-zapret-item-value' }, [ services[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-negative',
					'type': 'button',
					'click': ui.createHandlerFn(self, 'handleRemoveSelector', 'services', i)
				}, [ _('Remove') ])
			]));
		}

		for (i = 0; i < domains.length; i++) {
			rows.push(E('div', { 'class': 'routeflux-zapret-item' }, [
				E('span', { 'class': 'routeflux-zapret-badge routeflux-zapret-badge-domain' }, [ _('Domain') ]),
				E('span', { 'class': 'routeflux-zapret-item-value' }, [ domains[i] ]),
				E('button', {
					'class': 'cbi-button cbi-button-negative',
					'type': 'button',
					'click': ui.createHandlerFn(self, 'handleRemoveSelector', 'domains', i)
				}, [ _('Remove') ])
			]));
		}

		if (rows.length === 0)
			return E('div', { 'class': 'routeflux-zapret-empty' }, [ _('Nothing added yet.') ]);

		return E('div', { 'class': 'routeflux-zapret-list' }, rows);
	},

	renderServiceOptions: function(services) {
		var names = serviceNames(services);
		var options = [ E('option', { 'value': '' }, [ _('Choose preset') ]) ];

		for (var i = 0; i < names.length; i++) {
			options.push(E('option', {
				'value': names[i],
				'selected': this.formState.serviceChoice === names[i] ? 'selected' : null
			}, [ names[i] ]));
		}

		return options;
	},

	renderPageContent: function() {
		var runtime = this.pageData.runtime || {};
		var settings = this.pageData.settings || {};
		var status = this.pageData.status || {};
		var services = this.pageData.services || [];
		var selectors = this.formState.selectors || {};
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
				'#routeflux-zapret-root { --routeflux-zapret-ink:#10263f; --routeflux-zapret-ink-muted:#44566b; --routeflux-zapret-panel-bg:linear-gradient(160deg, rgba(243, 248, 255, 0.98) 0%, rgba(230, 239, 249, 0.98) 56%, rgba(220, 232, 245, 0.98) 100%); --routeflux-zapret-surface-bg:linear-gradient(180deg, rgba(255, 255, 255, 0.97) 0%, rgba(246, 250, 254, 0.97) 100%); --routeflux-zapret-surface-strong:linear-gradient(180deg, #17324d 0%, #10243a 100%); }',
				'.routeflux-zapret-layout { display:grid; gap:14px; color:var(--routeflux-zapret-ink); }',
				'.routeflux-zapret-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(240px, 1fr)); gap:12px; }',
				'.routeflux-zapret-panel { position:relative; overflow:hidden; isolation:isolate; border:1px solid rgba(120, 141, 167, 0.42); border-radius:22px; padding:20px; background:var(--routeflux-zapret-panel-bg); box-shadow:0 20px 44px rgba(3, 15, 32, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.78); }',
				'.routeflux-zapret-panel::before { content:""; position:absolute; inset:0; background:radial-gradient(circle at top left, rgba(125, 211, 252, 0.22), transparent 34%), radial-gradient(circle at bottom right, rgba(59, 130, 246, 0.12), transparent 40%); pointer-events:none; }',
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
				'.routeflux-zapret-choice-description { display:block; margin-top:6px; color:var(--routeflux-zapret-ink-muted); line-height:1.55; font-size:15px; }',
				'.routeflux-zapret-editor-head { display:grid; gap:10px; margin-bottom:16px; }',
				'.routeflux-zapret-editor-head .cbi-section-descr { margin:0; color:var(--routeflux-zapret-ink-muted); line-height:1.6; max-width:72ch; }',
				'.routeflux-zapret-editor-kicker { display:inline-flex; align-items:center; width:max-content; max-width:100%; padding:5px 11px; border-radius:999px; background:rgba(14, 165, 233, 0.14); color:#075985; font-size:12px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }',
				'.routeflux-zapret-inline { display:flex; gap:10px; align-items:stretch; }',
				'.routeflux-zapret-inline > .cbi-input-text, .routeflux-zapret-inline > .cbi-input-select { flex:1 1 auto; min-width:0; min-height:52px; padding:0 14px; border:1px solid rgba(71, 85, 105, 0.38); border-radius:15px; background:linear-gradient(180deg, rgba(255, 255, 255, 0.99) 0%, rgba(244, 248, 252, 0.99) 100%); color:var(--routeflux-zapret-ink); box-shadow:inset 0 1px 0 rgba(255, 255, 255, 0.92), 0 10px 24px rgba(15, 23, 42, 0.08); }',
				'.routeflux-zapret-inline > .cbi-input-select { padding-right:44px; }',
				'.routeflux-zapret-inline > .cbi-input-text:focus, .routeflux-zapret-inline > .cbi-input-select:focus { border-color:rgba(14, 165, 233, 0.72); box-shadow:0 0 0 1px rgba(14, 165, 233, 0.24), 0 16px 30px rgba(14, 165, 233, 0.16); }',
				'.routeflux-zapret-inline > .cbi-button-action, .routeflux-zapret-actions .cbi-button { min-height:52px; padding:0 18px; border:1px solid rgba(56, 189, 248, 0.44); border-radius:15px; background:var(--routeflux-zapret-surface-strong); color:#eef8ff; font-weight:800; text-shadow:0 1px 0 rgba(0, 0, 0, 0.22); box-shadow:0 16px 28px rgba(3, 15, 32, 0.22); }',
				'.routeflux-zapret-inline > .cbi-button-action:hover, .routeflux-zapret-actions .cbi-button:hover { border-color:rgba(96, 165, 250, 0.66); background:linear-gradient(180deg, #214365 0%, #16304a 100%); color:#ffffff; }',
				'.routeflux-zapret-list { display:grid; gap:8px; }',
				'.routeflux-zapret-item { display:flex; gap:12px; align-items:center; padding:12px 14px; border-radius:15px; background:rgba(255, 255, 255, 0.93); border:1px solid rgba(125, 145, 168, 0.28); box-shadow:0 10px 22px rgba(15, 23, 42, 0.08); }',
				'.routeflux-zapret-item .cbi-button-negative { min-width:92px; border-radius:12px; }',
				'.routeflux-zapret-item-value { flex:1 1 auto; min-width:0; word-break:break-word; font-weight:700; color:var(--routeflux-zapret-ink); }',
				'.routeflux-zapret-badge { display:inline-flex; align-items:center; justify-content:center; min-width:58px; padding:4px 8px; border-radius:999px; background:rgba(37, 99, 235, 0.13); color:#1d4ed8; font-size:11px; font-weight:800; letter-spacing:.05em; text-transform:uppercase; }',
				'.routeflux-zapret-badge-domain { background:rgba(16, 185, 129, 0.14); color:#047857; }',
				'.routeflux-zapret-empty { padding:14px; border-radius:14px; background:rgba(255, 255, 255, 0.82); border:1px dashed rgba(125, 145, 168, 0.42); color:var(--routeflux-zapret-ink-muted); }',
				'.routeflux-zapret-summary-shell { padding:16px 18px; border:1px solid rgba(125, 145, 168, 0.34); border-radius:16px; background:rgba(255, 255, 255, 0.84); box-shadow:0 10px 22px rgba(15, 23, 42, 0.08); }',
				'.routeflux-zapret-summary-shell h3 { margin-top:0; margin-bottom:10px; color:var(--routeflux-zapret-ink); font-size:20px; letter-spacing:-0.02em; }',
				'.routeflux-zapret-summary-list { margin:0; padding-left:18px; color:var(--routeflux-zapret-ink-muted); line-height:1.55; }',
				'.routeflux-zapret-actions { display:flex; flex-wrap:wrap; gap:10px; }',
				'@media (max-width: 720px) { .routeflux-zapret-inline { flex-direction:column; } .routeflux-zapret-inline > .cbi-button-action, .routeflux-zapret-actions .cbi-button { width:100%; } .routeflux-zapret-item { align-items:flex-start; flex-direction:column; } .routeflux-zapret-item .cbi-button-negative { width:100%; } }'
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
							_('Keep this list focused. RouteFlux expands service aliases like youtube to the domains Zapret should manage, and it also accepts your own fully qualified domains.')
						])
					]),
					E('div', { 'class': 'routeflux-zapret-editor-grid' }, [
						E('div', {}, [
							E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-zapret-service-choice' }, [ _('Available presets') ]),
							E('div', { 'class': 'routeflux-zapret-inline' }, [
								E('select', {
									'id': 'routeflux-zapret-service-choice',
									'class': 'cbi-input-select',
									'change': ui.createHandlerFn(this, 'handleServiceChoice')
								}, this.renderServiceOptions(services)),
								E('button', {
									'class': 'cbi-button cbi-button-action',
									'type': 'button',
									'click': ui.createHandlerFn(this, 'handleAddService')
								}, [ _('Add preset') ])
							])
						]),
						E('div', {}, [
							E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-zapret-domain-input' }, [ _('Custom domain') ]),
							E('div', { 'class': 'routeflux-zapret-inline' }, [
								E('input', {
									'id': 'routeflux-zapret-domain-input',
									'class': 'cbi-input-text',
									'type': 'text',
									'placeholder': 'youtube.com',
									'value': this.formState.domainInput,
									'input': ui.createHandlerFn(this, 'handleDomainInput')
								}),
								E('button', {
									'class': 'cbi-button cbi-button-action',
									'type': 'button',
									'click': ui.createHandlerFn(this, 'handleAddDomain')
								}, [ _('Add domain') ])
							])
						])
					]),
					E('div', { 'class': 'routeflux-zapret-summary-shell', 'style': 'margin-top:16px;' }, [
						E('h3', {}, [ _('Current selector set') ]),
						E('ul', { 'class': 'routeflux-zapret-summary-list' }, [
							E('li', {}, [ _('Presets and domains currently selected: %d').format(selectorCount(selectors)) ]),
							E('li', {}, [ _('Zapret accepts only service aliases and fully qualified domains. IPv4 addresses, CIDRs, and host-wide selectors are not supported.') ])
						])
					]),
					E('div', { 'style': 'margin-top:16px;' }, [
						this.renderSelectorRows(selectors)
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
		return E('div', { 'id': 'routeflux-zapret-root' }, this.renderPageContent());
	},

	handleSaveApply: null,
	handleReset: null
});
