'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const views = path.resolve(__dirname, '../../luci-app-routeflux/htdocs/luci-static/resources/view/routeflux');
const saved = { allow_lan: false, socks_port: 10808, http_port: 10809 };
let calls = [], fail = false, themeChanges = 0, notices = [];
const env = {
 view: { extend: v => v },
 fs: { exec: async (bin, args) => { calls.push(args); if (fail) return { code: 1, stderr: 'port occupied' }; return { code: 0, stdout: JSON.stringify(saved) }; } },
 rpc: { declare: () => async () => ({ 'ipv4-address': [{ address: '10.0.0.1' }] }) },
 ui: { addNotification: (...args) => notices.push(args), createHandlerFn: (v,k) => v[k].bind(v) },
 dom: {}, document: { querySelector: () => null }, window: {},
 routefluxUI: { currentTheme: () => 'dark', setThemePreference: () => themeChanges++, renderSharedStyles: () => '', withThemeClass: v => v },
 E: (tag,attrs,children) => ({tag,attrs,children}), _: s => s, L: {url: (...parts) => '/' + parts.join('/'), bind: (fn,obj) => fn.bind(obj)},
};
function load(name) { return vm.runInNewContext('(function(){'+fs.readFileSync(path.join(views,name),'utf8')+'})()',env); }
function flatten(x) { if (x == null) return ''; if (typeof x !== 'object') return String(x); return Array.isArray(x) ? x.map(flatten).join(' ') : flatten(x.children); }
(async () => {
 const page = load('settings.js');
 const data = await page.load();
 const rendered = page.render(data);
 assert.match(flatten(rendered), /Local \/ LAN Proxy/);
 assert.ok(calls.some(args => args.join(' ') === '--json proxy get'));
 page.proxyDraft = {allow_lan: true, socks_port: '20808', http_port: '20809'};
 await page.handleSaveProxy();
 assert.equal(calls.at(-1).join(' '), '--json proxy set --allow-lan true --socks-port 20808 --http-port 20809');
 assert.equal(themeChanges, 0);
 page.proxyDraft = {allow_lan: true, socks_port: '20808', http_port: '20808'};
 let count = calls.length;
 await page.handleSaveProxy();
 assert.equal(calls.length, count, 'invalid ports must not call CLI');
 fail = true;
 page.proxyDraft.http_port = '20809';
 await page.handleSaveProxy();
 assert.match(flatten(notices.at(-1)), /port occupied/);
 assert.equal(page.proxyDraft.allow_lan, true, 'failed save must retain draft');
 assert.equal(page.proxySaving, false);
 const routing = load('firewall.js');
 routing.proxySettings = { allow_lan: true, socks_port: 20808, http_port: 20809 };
 routing.lanStatus = {'ipv4-address': [{address:'10.0.0.1'}]};
 let hint = flatten(routing.renderProxyHint());
 assert.match(hint,/10\.0\.0\.1:20808/); assert.match(hint,/10\.0\.0\.1:20809/);
 routing.proxySettings.allow_lan = false;
 assert.match(flatten(routing.renderProxyHint()), /Settings/);
 routing.proxySettings.allow_lan = true; routing.lanStatus = {};
 assert.match(flatten(routing.renderProxyHint()), /LAN address/);
 console.log('Proxy settings and Routing behavior passed');
})().catch(err => { console.error(err); process.exitCode = 1; });
