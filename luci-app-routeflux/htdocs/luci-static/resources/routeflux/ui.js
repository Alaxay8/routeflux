'use strict';
'require baseclass';

function trim(value) {
	if (value == null)
		return '';

	return String(value).trim();
}

function hasContent(value) {
	if (Array.isArray(value))
		return value.length > 0;

	return trim(value) !== '';
}

return baseclass.extend({
	formatTimestamp: function(value) {
		var normalized = trim(value);
		var match = normalized.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})/);

		return match ? match[1] : normalized;
	},

	statusTone: function(connected) {
		return connected === true ? 'connected' : 'disconnected';
	},

	renderSharedStyles: function() {
		return E('style', { 'type': 'text/css' }, [
			'.routeflux-overview-grid { display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:10px; margin-bottom:16px; }',
			'.routeflux-card { border:1px solid rgba(98, 112, 129, 0.46); border-radius:12px; padding:11px 13px; min-height:88px; background:linear-gradient(180deg, rgba(235, 240, 246, 0.98) 0%, rgba(225, 231, 239, 0.98) 100%); box-shadow:0 14px 28px rgba(8, 15, 26, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.46); overflow:hidden; }',
			'.routeflux-card-primary { border-color:rgba(71, 85, 105, 0.58); box-shadow:0 18px 36px rgba(8, 15, 26, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.5); }',
			'.routeflux-card-accent { height:4px; border-radius:999px; margin-bottom:11px; background:linear-gradient(90deg, #38bdf8 0%, #60a5fa 100%); }',
			'.routeflux-card-label { color:var(--text-color-secondary, #56657a); font-size:10px; margin-bottom:7px; text-transform:uppercase; letter-spacing:.14em; font-weight:800; }',
			'.routeflux-card-value { color:var(--text-color-primary, #17263a); font-size:15px; font-weight:700; line-height:1.42; word-break:break-word; }',
			'.routeflux-card-primary .routeflux-card-value { font-size:16px; }',
			'.routeflux-card-connected { background:linear-gradient(180deg, rgba(223, 237, 229, 0.98) 0%, rgba(210, 228, 219, 0.98) 100%); border-color:rgba(34, 112, 83, 0.46); }',
			'.routeflux-card-connected .routeflux-card-label { color:#355a49; }',
			'.routeflux-card-connected .routeflux-card-value { color:#153f30; }',
			'.routeflux-card-connected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, #22c55e 0%, #14b8a6 100%); }',
			'.routeflux-card-disconnected { background:linear-gradient(180deg, rgba(229, 235, 243, 0.98) 0%, rgba(220, 227, 236, 0.98) 100%); border-color:rgba(90, 103, 121, 0.52); }',
			'.routeflux-card-disconnected .routeflux-card-label { color:#526175; }',
			'.routeflux-card-disconnected .routeflux-card-value { color:#1f2d40; }',
			'.routeflux-card-disconnected.routeflux-card-primary .routeflux-card-accent { background:linear-gradient(90deg, #64748b 0%, #94a3b8 100%); }'
		]);
	},

	renderSummaryCard: function(label, value, options) {
		var settings = options || {};
		var className = 'routeflux-card';
		var content = value;
		var attrs = {
			'class': className
		};

		if (trim(settings.id) !== '')
			attrs.id = settings.id;

		if (trim(settings.tone) !== '')
			attrs['class'] += ' routeflux-card-' + trim(settings.tone);

		if (settings.primary === true)
			attrs['class'] += ' routeflux-card-primary';

		if (!hasContent(content))
			content = settings.fallback != null ? settings.fallback : '-';

		if (!Array.isArray(content))
			content = [ content ];

		var valueAttrs = { 'class': 'routeflux-card-value' };
		if (trim(settings.valueId) !== '')
			valueAttrs.id = settings.valueId;

		var children = [];

		if (settings.primary === true)
			children.push(E('div', { 'class': 'routeflux-card-accent' }, []));

		children.push(E('div', { 'class': 'routeflux-card-label' }, [ label ]));
		children.push(E('div', valueAttrs, content));

		return E('div', attrs, children);
	}
});
