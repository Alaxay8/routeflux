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

function parseList(raw) {
	var value = trim(raw);

	if (value === '')
		return [];

	return value.split(/[\s,]+/).filter(function(item) {
		return trim(item) !== '';
	});
}

function notificationParagraph(message) {
	return E('p', {}, [ message ]);
}

function serviceSelectors(service) {
	var values = [];

	if (Array.isArray(service.services))
		values = values.concat(service.services);
	if (Array.isArray(service.domains))
		values = values.concat(service.domains);
	if (Array.isArray(service.cidrs))
		values = values.concat(service.cidrs);

	return values;
}

function serviceSummary(service) {
	var lines = [];
	var services = Array.isArray(service.services) ? service.services : [];
	var domains = Array.isArray(service.domains) ? service.domains : [];
	var cidrs = Array.isArray(service.cidrs) ? service.cidrs : [];

	if (services.length > 0)
		lines.push(_('Included services: %s').format(services.join(', ')));
	lines.push(_('Domains: %s').format(domains.length > 0 ? domains.join(', ') : '-'));
	lines.push(_('IPv4 targets: %s').format(cidrs.length > 0 ? cidrs.join(', ') : '-'));

	return lines.join('\n');
}

return view.extend({
	load: function() {
		return Promise.all([
			this.execJSON([ '--json', 'services', 'list' ]).catch(function(err) {
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

	handleSaveService: function(ev) {
		if (ev)
			ev.preventDefault();

		var nameElement = document.querySelector('#routeflux-service-name');
		var selectorsElement = document.querySelector('#routeflux-service-selectors');
		var name = trim(nameElement && nameElement.value);
		var selectors = parseList(selectorsElement && selectorsElement.value);

		if (name === '') {
			ui.addNotification(null, notificationParagraph(_('Enter a service alias name.')));
			return Promise.resolve();
		}

		if (selectors.length === 0) {
			ui.addNotification(null, notificationParagraph(_('Enter at least one service alias, domain, IPv4 address, CIDR, or IPv4 range.')));
			return Promise.resolve();
		}

		return this.runCommands([
			[ 'services', 'set', name ].concat(selectors)
		], _('Target service saved.'));
	},

	handleDeleteService: function(name, ev) {
		if (ev)
			ev.preventDefault();

		if (!window.confirm(_('Delete target service %s?').format(name)))
			return Promise.resolve();

		return this.runCommands([
			[ 'services', 'delete', name ]
		], _('Target service deleted.'));
	},

	handleEditService: function(service, ev) {
		if (ev)
			ev.preventDefault();

		var nameElement = document.querySelector('#routeflux-service-name');
		var selectorsElement = document.querySelector('#routeflux-service-selectors');

		if (nameElement)
			nameElement.value = trim(service.name);
		if (selectorsElement)
			selectorsElement.value = serviceSelectors(service).join('\n');

		if (nameElement)
			nameElement.focus();

		window.scrollTo({ top: 0, behavior: 'smooth' });
	},

	handleClearForm: function(ev) {
		if (ev)
			ev.preventDefault();

		var nameElement = document.querySelector('#routeflux-service-name');
		var selectorsElement = document.querySelector('#routeflux-service-selectors');

		if (nameElement)
			nameElement.value = '';
		if (selectorsElement)
			selectorsElement.value = '';

		return Promise.resolve();
	},

	renderServiceEntry: function(service, readonly) {
		var actions = [];

		if (!readonly) {
			actions.push(E('button', {
				'class': 'cbi-button',
				'type': 'button',
				'click': ui.createHandlerFn(this, 'handleEditService', service)
			}, [ _('Edit') ]));
			actions.push(E('button', {
				'class': 'cbi-button cbi-button-remove',
				'type': 'button',
				'click': ui.createHandlerFn(this, 'handleDeleteService', service.name)
			}, [ _('Delete') ]));
		}

		return E('div', { 'class': 'routeflux-service-entry' }, [
			E('div', { 'class': 'routeflux-service-entry-head' }, [
				E('div', {}, [
					E('strong', {}, [ trim(service.name) || '-' ]),
					E('div', { 'class': 'cbi-value-description' }, [
						readonly ? _('Built-in preset') : _('Custom alias')
					])
				]),
				E('div', { 'class': 'routeflux-service-entry-actions' }, actions)
			]),
			E('pre', { 'class': 'routeflux-service-pre' }, [ serviceSummary(service) ])
		]);
	},

	render: function(data) {
		var services = Array.isArray(data[0]) ? data[0] : [];
		var builtinServices = services.filter(function(service) { return service.readonly === true; });
		var customServices = services.filter(function(service) { return service.readonly !== true; });
		var content = [];

		if (data[0] && data[0].__error__)
			ui.addNotification(null, notificationParagraph(_('Services error: %s').format(data[0].__error__)));

		content.push(routefluxUI.renderSharedStyles());
		content.push(E('style', { 'type': 'text/css' }, [
			'.routeflux-services-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:12px; margin-bottom:12px; }',
			'.routeflux-services-grid textarea { min-height:140px; width:100%; }',
			'.routeflux-service-entry { border:1px solid rgba(98, 112, 129, 0.28); border-radius:12px; padding:12px; margin-bottom:10px; background:rgba(255, 255, 255, 0.02); }',
			'.routeflux-service-entry-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; }',
			'.routeflux-service-entry-actions { display:flex; gap:8px; flex-wrap:wrap; }',
			'.routeflux-service-pre { white-space:pre-wrap; margin:10px 0 0; }',
			'.routeflux-services-actions { display:flex; flex-wrap:wrap; gap:10px; }'
		]));

		content.push(E('h2', {}, [ _('RouteFlux - Services') ]));
		content.push(E('p', { 'class': 'cbi-section-descr' }, [
			_('Create reusable aliases and bundles for firewall targets. Built-in presets stay readonly; custom entries let you save domains, IPv4 selectors, and existing service aliases once, then reuse them inside Firewall -> Targets.')
		]));

		content.push(E('div', { 'class': 'routeflux-overview-grid' }, [
			routefluxUI.renderSummaryCard(_('Built-in Presets'), String(builtinServices.length)),
			routefluxUI.renderSummaryCard(_('Custom Services'), String(customServices.length)),
			routefluxUI.renderSummaryCard(_('Alias Rules'), _('lowercase, digits, hyphens'))
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Custom Service Form') ]),
			E('div', { 'class': 'routeflux-services-grid' }, [
				E('div', { 'class': 'cbi-value' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-service-name' }, [ _('Name') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('input', {
							'id': 'routeflux-service-name',
							'class': 'cbi-input-text',
							'type': 'text',
							'placeholder': 'openai'
						})
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Use lowercase letters, digits, and hyphens. Built-in names like youtube and telegram are reserved.')
					])
				]),
				E('div', { 'class': 'cbi-value', 'style': 'grid-column:1 / -1' }, [
					E('label', { 'class': 'cbi-value-title', 'for': 'routeflux-service-selectors' }, [ _('Selectors') ]),
					E('div', { 'class': 'cbi-value-field' }, [
						E('textarea', {
							'id': 'routeflux-service-selectors',
							'class': 'cbi-input-textarea',
							'placeholder': _('Examples: youtube openai chatgpt.com oaistatic.com 104.18.0.0/15')
						}, [])
					]),
					E('div', { 'class': 'cbi-value-description' }, [
						_('Accepted selectors: existing service aliases, domains, IPv4 addresses, CIDRs, and IPv4 ranges. Self-references, alias cycles, URLs, and wildcard domains are not supported.')
					])
				])
			]),
			E('div', { 'class': 'routeflux-services-actions' }, [
				E('button', {
					'class': 'cbi-button cbi-button-apply',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleSaveService')
				}, [ _('Save Service') ]),
				E('button', {
					'class': 'cbi-button',
					'type': 'button',
					'click': ui.createHandlerFn(this, 'handleClearForm')
				}, [ _('Clear Form') ])
			])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, [ _('Custom Services') ]),
			customServices.length > 0
				? E('div', {}, customServices.map(L.bind(function(service) {
					return this.renderServiceEntry(service, false);
				}, this)))
				: E('p', { 'class': 'cbi-value-description' }, [
					_('No custom services yet. Create one above, then use its name inside Firewall -> Targets.')
				])
		]));

		content.push(E('div', { 'class': 'cbi-section' }, [
			E('details', { 'open': 'open' }, [
				E('summary', {}, [ _('Built-in Presets') ]),
				E('div', {}, builtinServices.map(L.bind(function(service) {
					return this.renderServiceEntry(service, true);
				}, this)))
			])
		]));

		return E(content);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
