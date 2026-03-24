[English](README.md) | [Русский](README.ru_RU.md)

# RouteFlux

## Обзор

RouteFlux — это легковесное Go-приложение для OpenWrt, которое управляет proxy-подписками Xray и V2Ray-совместимых сервисов на роутерах и edge-устройствах. Оно подходит тем, кому нужен менеджер VLESS, VMess, Trojan, 3x-ui, Xray, маршрутизации трафика через роутер и подписок без ручного редактирования Xray JSON.

RouteFlux импортирует URL подписок, raw `vless://`, `vmess://`, `trojan://` и `ss://` ссылки, а также валидные JSON-конфиги 3x-ui или Xray. Приложение нормализует поддерживаемые proxy outbounds в единую модель ноды, безопасно хранит локальное состояние и генерирует runtime-конфиг для Xray на OpenWrt и совместимых системах, например ImmortalWrt.

## Возможности

- Импортирует proxy-подписки из URL, raw share-link, stdin или валидного 3x-ui/Xray JSON.
- Парсит VLESS, VMess, Trojan и Shadowsocks share-links.
- Нормализует поддерживаемые proxy outbounds из 3x-ui/Xray в RouteFlux-ноды.
- Позволяет добавлять, просматривать, обновлять, подключать, отключать, удалять одну подписку или удалять все подписки из CLI или TUI.
- Поддерживает ручной выбор ноды и автоматический выбор лучшей ноды на основе health checks и anti-flap логики.
- Генерирует runtime-конфиг Xray и перезагружает OpenWrt `init.d` service.
- Настраивает простую маршрутизацию через nftables для выбранных IP-адресов, CIDR, диапазонов или LAN-хостов.
- Управляет DNS через отдельную команду `routeflux dns`, а не смешивает ее с общими настройками.
- Стартует с практичным DNS-профилем по умолчанию: split DNS, DoH к Cloudflare и локальные `.lan` имена через локальный DNS.
- Сохраняет подписки, настройки, runtime state и telemetry через атомарные JSON-записи.

## Быстрый старт

1. Установите бинарник RouteFlux на роутер.
2. Добавьте подписку, share-link или валидный 3x-ui/Xray JSON-конфиг.
3. Подключите ноду вручную или включите auto mode.

См. разделы [Установка](#установка) и [Использование](#использование).

## Установка

1. Установите Go `1.22` или новее, если собираете проект локально.
2. Используйте OpenWrt или ImmortalWrt с доступным `nftables`. Практический минимум для текущей firewall-интеграции — OpenWrt `22.03+`.
3. Соберите RouteFlux:

```bash
make build-openwrt
```

4. Скопируйте бинарник на роутер. На многих устройствах OpenWrt `scp -O` работает надежнее, чем режим SFTP по умолчанию:

```bash
scp -O ./bin/openwrt/routeflux root@router:/usr/bin/routeflux
```

Если на роутере нет SFTP, передайте файл через SSH:

```bash
ssh root@router 'cat > /tmp/routeflux.new && chmod 0755 /tmp/routeflux.new && mv /tmp/routeflux.new /usr/bin/routeflux' < ./bin/openwrt/routeflux
```

5. Опционально установите LuCI frontend, если хотите использовать веб-интерфейс.

Соберите deploy bundle:

```bash
make package-openwrt
```

Скопируйте его на роутер и распакуйте в `/`:

```bash
scp -O ./dist/routeflux_0.1.0_all.tar.gz root@router:/tmp/
ssh root@router 'tar -xzf /tmp/routeflux_0.1.0_all.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload'
```

Будут установлены:

- `/usr/bin/routeflux`
- `/usr/share/luci/menu.d/luci-app-routeflux.json`
- `/usr/share/rpcd/acl.d/luci-app-routeflux.json`
- `/www/luci-static/resources/view/routeflux/*.js`

6. Установите Xray Core позже, когда будете использовать `connect`, `disconnect` или runtime-конфиг. Убедитесь, что service script доступен по пути `/etc/init.d/xray`, либо переопределите его через `ROUTEFLUX_XRAY_SERVICE`.

## Использование

Примеры CLI:

```bash
routeflux add
routeflux add https://provider.example/subscription
routeflux add 'vless://uuid@example.com:443?...#Example'
routeflux add < 3x-ui-config.json
routeflux list subscriptions
routeflux list nodes --subscription sub-1234567890
routeflux remove sub-1234567890
routeflux remove --all
routeflux refresh --subscription sub-1234567890
routeflux refresh --all
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux connect --auto --subscription sub-1234567890
routeflux disconnect
routeflux status
routeflux diagnostics
routeflux logs
routeflux settings get
routeflux settings set refresh-interval 1h
routeflux settings set auto-mode true
routeflux firewall get
routeflux firewall explain
routeflux firewall set targets 1.1.1.1 8.8.8.8/32
routeflux firewall set hosts 192.168.1.150
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts 192.168.1.150-192.168.1.159
routeflux firewall set hosts all
routeflux firewall set block-quic true
routeflux dns get
routeflux dns explain
routeflux dns set default
routeflux dns set mode split
routeflux dns set transport doh
routeflux dns set servers "dns.google,1.1.1.1"
routeflux tui
```

## Примеры

Импорт валидного 3x-ui или Xray JSON-конфига:

```bash
routeflux add < ./client-config.json
```

Удаление одной сохраненной подписки:

```bash
routeflux remove sub-8b9f930214
```

Удаление всех сохраненных подписок:

```bash
routeflux remove --all
```

Включение автоматического выбора лучшей ноды:

```bash
routeflux connect --auto --subscription sub-8b9f930214
```

Маршрутизация всего TCP-трафика одного LAN-устройства через RouteFlux:

```bash
routeflux firewall set hosts 192.168.1.150
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Маршрутизация пула LAN-устройств через RouteFlux:

```bash
routeflux firewall set hosts 192.168.1.32/27
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Маршрутизация всех типовых private LAN-клиентов через RouteFlux:

```bash
routeflux firewall set hosts all
routeflux connect --subscription sub-8b9f930214 --node 90c42d5dd302
```

Использование зашифрованного DNS для внешних доменов с сохранением локальных имен дома:

```bash
routeflux dns set default
```

Текущий рекомендуемый профиль RouteFlux:

```text
mode=split
transport=doh
servers=1.1.1.1,1.0.0.1
bootstrap=
direct-domains=domain:lan,full:router.lan
```

## Конфигурация

По умолчанию RouteFlux хранит состояние в `/etc/routeflux` на OpenWrt. Для локальной разработки используется `./.routeflux`.

Важные переменные окружения:

- `ROUTEFLUX_ROOT`: переопределяет каталог состояния.
- `ROUTEFLUX_XRAY_CONFIG`: переопределяет путь к сгенерированному Xray config.
- `ROUTEFLUX_XRAY_SERVICE`: переопределяет service control script для Xray.
- `ROUTEFLUX_FIREWALL_RULES`: переопределяет путь к сгенерированному nftables rules file.

Сохраняемые файлы:

- `/etc/routeflux/subscriptions.json`
- `/etc/routeflux/settings.json`
- `/etc/routeflux/state.json`

DNS workflow:

- RouteFlux стартует с практичным профилем по умолчанию: `split + doh + 1.1.1.1/1.0.0.1 + domain:lan,full:router.lan`.
- Используйте `routeflux dns get`, чтобы посмотреть текущие DNS-настройки.
- Используйте `routeflux dns explain`, чтобы получить понятное описание каждого DNS mode и transport.
- Используйте `routeflux dns set default`, чтобы одной командой вернуть рекомендуемый повседневный DNS-профиль.
- Используйте `routeflux dns set ...`, чтобы изменить DNS-поведение.
- Для общих настроек приложения, например refresh interval, auto mode и log level, продолжайте использовать `routeflux settings`.

Firewall workflow:

- Используйте `routeflux firewall get`, чтобы посмотреть текущие настройки transparent routing.
- Используйте `routeflux firewall explain`, чтобы получить понятное описание host mode, target mode и QUIC blocking.
- Используйте `routeflux firewall set hosts ...`, чтобы маршрутизировать выбранные LAN-клиенты через RouteFlux.
- Используйте `routeflux firewall set targets ...`, чтобы маршрутизировать выбранные destination IPv4 addresses или ranges через RouteFlux.
- Используйте `routeflux firewall set port ...` и `routeflux firewall set block-quic ...`, чтобы подстроить активное firewall-поведение.
- `routeflux firewall host ...` и `routeflux firewall status` сохранены как compatibility aliases.

DNS modes:

- `system`: RouteFlux не трогает DNS и не пишет DNS-блок в Xray config.
- `remote`: все DNS-запросы уходят на выбранные DNS-серверы.
- `split`: локальные домашние имена остаются локальными, а остальное уходит на выбранные DNS-серверы.
- `disabled`: RouteFlux пропускает запись DNS-блока. Обычно это полезно только когда нужна полностью явная конфигурация.

DNS transports:

- `plain`: обычный DNS без шифрования.
- `doh`: DNS over HTTPS. Это рабочий зашифрованный DNS-вариант в текущем backend.
- `dot`: DNS over TLS. Настройка существует, но текущий Xray backend ее пока не применяет.

## Разработка

Сборка и тесты локально:

```bash
make fmt
make test
make coverage
make build
```

Кросс-сборка для OpenWrt:

```bash
make build-openwrt
```

Заметки по проекту:

- Parser, selector, firewall integration, DNS rendering и Xray config generation покрыты unit tests и golden files.
- `routeflux add` принимает URL, raw share-links или stdin, поэтому хорошо подходит для copy-paste и shell pipelines.
- Импорт 3x-ui и Xray JSON, включая JSON-массивы полных конфигов, нормализуется в RouteFlux-ноды, а не копируется как полный runtime-config.
- Общие настройки и DNS-настройки намеренно разделены. Используйте `routeflux settings` для поведения приложения и `routeflux dns` для runtime DNS.
- `routeflux firewall set hosts` принимает отдельные IPv4-адреса, IPv4 CIDR-пулы, IPv4-диапазоны и алиасы `all` или `*` для типовых private LAN ranges.

## Архитектура

Кодовая база разделена на слои domain, parser, store, probe, backend, app, CLI и TUI. Подробнее см. [docs/architecture.md](docs/architecture.md).

## Поддерживаемые протоколы

- VLESS
- VMess
- Trojan
- Shadowsocks share-link parsing

## TUI

MVP TUI управляется с клавиатуры и использует навигацию provider-first:

- `j` / `k`: перемещение между VPN-сервисами
- `h` / `l`: перемещение между профилями внутри выбранного сервиса
- `n` / `p`: перемещение между локациями внутри выбранного профиля
- `c`: подключить выбранную ноду
- `a`: включить auto selection для выбранной подписки
- `r`: обновить выбранную подписку
- `s`: открыть настройки
- `d`: отключиться
- `q`: выйти

Placeholder screenshots:

- `docs/images/tui-main.txt`
- `docs/images/tui-settings.txt`

## Развертывание на OpenWrt

1. Соберите проект через `make build-openwrt`.
2. Скопируйте бинарник на роутер.
3. Создайте `/etc/routeflux`, если каталога еще нет.
4. Установите Xray только когда будете проксировать трафик, и проверьте, что `/etc/init.d/xray` умеет `reload`, `start` и `stop`.
5. Запустите `routeflux add`, импортируйте валидную подписку или 3x-ui/Xray JSON-конфиг и подключитесь к ноде.
6. Если нужен зашифрованный DNS, настройте его через `routeflux dns` после того, как runtime уже заработал.

## Ограничения

- Для `connect`, `disconnect` и генерации runtime-конфига требуется Xray.
- Импорт 3x-ui/Xray JSON читает только поддерживаемые proxy outbounds. Он не сохраняет полные секции `dns`, `routing`, `inbounds`, outbound chaining и другие вспомогательные runtime-части.
- Текущий Xray backend умеет подключать VLESS, VMess и Trojan ноды. Парсинг Shadowsocks уже есть, но полная генерация outbound для Shadowsocks в Xray пока не подключена.
- `dns.transport=dot` определен в настройках, но текущий Xray backend его не применяет.
- Полностью автоматизированный transparent router traffic interception в MVP еще не закончен.
- Текущее firewall routing поддерживает destination IPv4 targets, source IPv4 hosts, CIDR-пулы, IPv4-диапазоны и LAN-wide shortcut `all` или `*`. QUIC blocking доступен только в host mode.
- LuCI MVP находится в `luci-app-routeflux` и уже содержит страницы `Overview`, `Subscriptions`, `Firewall`, `DNS`, `Settings`, `Diagnostics` и `Logs`.

## Лицензия

MIT
