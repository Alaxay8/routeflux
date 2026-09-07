[English](README.md) | [Русский](README.ru_RU.md)

# RouteFlux

RouteFlux — это нативный для OpenWrt менеджер Xray-подписок для роутеров и перефирийных устройств.

Он помогает импортировать прокси-подписки, выбирать подходящую ноду, применять правила маршрутизации трафика роутера и управлять DNS без ручного редактирования Xray JSON. Если RouteFlux экономит вам время, поставьте звезду репозиторию и поделитесь им с другими пользователями OpenWrt.

## Обзор

RouteFlux создан для тех, кому нужен практичный сценарий работы с прокси на OpenWrt:

- Импортируйте URL подписки, сырую ссылку `vless://`, `vmess://`, `trojan://`, `socks5://`, `hysteria://` или `hy2://`, либо валидный JSON-конфиг 3x-ui/Xray.
- Подключайтесь к конкретной ноде вручную или позвольте RouteFlux автоматически выбрать лучшую.
- Управляйте всем из CLI, веб-интерфейса LuCI или локального TUI.
- Держите настройки маршрутизации роутера и DNS в понятном виде, а не прячьте их внутри сгенерированного Xray-конфига.

Текущая целевая среда выполнения — OpenWrt и совместимые форки, такие как ImmortalWrt. Практический базовый вариант — OpenWrt `22.03+` с `nftables`.

## Возможности

- Быстрый импорт подписок, share-ссылок и поддерживаемых JSON-файлов 3x-ui/Xray.
- Поддержка нод VLESS, VMess, Trojan, Socks5, Hysteria и Hysteria 2.
- Безопасные обновления runtime через `xray -test`, резервную копию последней рабочей конфигурации и контролируемую перезагрузку сервиса.
- Автоматический режим с проверками состояния, live failover, anti-flap логикой и восстановлением runtime после перезагрузки.
- Удобный список серверов (Server List) для оптимизированного управления подписками и отдельными серверами.
- Режим «Только выбранные устройства» (Only Selected Devices) для перенаправления через прокси только указанных LAN-хостов.
- Параллельный замер задержки и автоматическое переподключение к лучшему серверу при наличии дублирующихся имен нод.
- Простые правила прозрачной маршрутизации для LAN-хостов, CIDR, диапазонов или целевых адресов.
- Режим локального и LAN-прокси с выделенными портами SOCKS5 (:10808) и HTTP (:10809), управляемыми через LuCI и CLI.
- Отдельные DNS-команды с разумным профилем по умолчанию для повседневного использования роутера.
- Общее состояние для CLI, LuCI и TUI, поэтому можно переключаться между интерфейсами без потери контекста.

## Архитектура и принцип работы

RouteFlux разделяет управление, сетевую фильтрацию ядра Linux и рантайм проксирования на три независимых уровня:

[![RouteFlux • Архитектура и принцип работы](docs/images/architecture-diagram-ru.png)](docs/images/architecture-diagram-ru.png)

> **Совет:** Нажмите на схему для увеличения или откройте [векторную SVG-версию](docs/images/architecture-diagram-ru.svg) для бесконечного масштабирования без потери четкости.

### Уровни архитектуры

1. **Управление и бизнес-логика (Control Plane Go)**
   - **Единые интерфейсы:** Общее состояние между веб-интерфейсом LuCI, CLI (Cobra), интерактивным консольным TUI и системным демоном.
   - **Парсер и хранилище:** Авто-определение форматов подписок и протоколов (VLESS Reality, VMess, Trojan, Hysteria 1/2, Socks5) с атомарным сохранением JSON-состояния в `/etc/routeflux/`.
   - **Движок замера (Probe Engine):** Параллельный замер доступности (RTT, TCP handshake), скоринг стабильности нод, проверка выхода `generate_204` и защита от частых переключений (anti-flap failover).
   - **Атомарный генератор конфигов:** Предварительное тестирование конфигурации через `xray -test -c` с автоматическим откатом на рабочую версию при ошибках.

2. **Сетевой трафик и перехват (Data Plane OpenWrt)**
   - **Динамический `nftset` в `dnsmasq-full`:** Автоматическое наполнение наборов ядра IP-адресами запрошенных доменов без необходимости вручную вести списки IP.
   - **Аппаратный роутинг в ядре (`nftables`):** Таблица `inet routeflux`, цепочки prerouting (TCP redirect) и tproxy (UDP) с поддержкой исключения локальных подсетей и устройств.
   - **Два режима работы:**
     - **Режим А (Прозрачный шлюз):** Роутер сам прозрачно перехватывает DNS и трафик целевых доменов без настройки клиентов сети.
     - **Режим Б (Локальный и LAN-прокси):** Прямой доступ к портам SOCKS5 (`:10808`) и HTTP (`:10809`) для приложений (браузеры, Telegram, Happ) с сохранением нативной скорости NAT роутера.

3. **Рантайм Xray и выход в интернет**
   - **Двойные точки входа (Inbounds):** Прием как перехваченного трафика от сетевого фильтра роутера, так и прямых SOCKS5/HTTP-подключений от устройств локальной сети.
   - **Внутренний DoH / DoT резолвер:** Защищенное разрешение доменных имен, исключающее перехват и подмену DNS провайдером.
   - **Раздельные каналы выхода (Outbounds):** Маршрутизация в шифрованный туннель к удаленному серверу (VPS), прямой выход через домашнего провайдера (Direct WAN) или локальный сервис обхода DPI (Zapret / `nfqws`).

## Быстрый старт

Установите последний стабильный релиз на ваш роутер:

```bash
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh" && sh /tmp/routeflux-install.sh
```

Затем импортируйте подписку и подключитесь:

```bash
routeflux add https://provider.example/subscription
routeflux list subscriptions
routeflux connect --auto --subscription sub-1234567890
```

После установки можно использовать:

- LuCI: `Services -> RouteFlux` открывает `Subscriptions`
- CLI: по SSH через `routeflux ...`
- TUI: `routeflux tui`

## Веб-интерфейс

RouteFlux включает интерфейс LuCI для повседневного управления подписками.

![RouteFlux LuCI Subscriptions](docs/images/luci-subscriptions-1.png)

Экран профиля показывает метаданные подписки, быстрые действия, auto exclusions и список доступных нод.

![RouteFlux LuCI Subscription Profile](docs/images/luci-subscriptions-2.png)

Таблица нод позволяет сравнить задержку, посмотреть детали транспорта, подключиться вручную, перепроверить маршрут или исключить ноду из auto mode.

![RouteFlux LuCI Nodes Table](docs/images/luci-subscriptions-3.png)

На странице Routing также есть экран Keep Direct для bypass selectors, где можно оставить выбранные домены или IPv4-цели на прямом маршруте, пока активен bypass mode.

![RouteFlux LuCI Keep Direct](docs/images/keep-direrct.png)

Для сценариев split routing есть экран Excluded Devices, где можно оставить выбранные LAN-хосты вне proxy path и управлять ими прямо из LuCI.

![RouteFlux LuCI Excluded Devices](docs/images/exclude-devices.png)

В LuCI также доступен экран управления Zapret fallback, где видны автоматическое переключение, test mode и текущее состояние транспорта.

![RouteFlux LuCI Zapret](docs/images/zapret.png)

На странице Settings есть переключение внешнего вида, чтобы можно было сменить тему RouteFlux внутри LuCI без изменения остального интерфейса OpenWrt.

![RouteFlux LuCI Appearance](docs/images/appearance.png)

## Установка

### Установка из релиза GitHub

Используйте последний стабильный установщик:

```bash
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh" && sh /tmp/routeflux-install.sh
```

Чтобы обновить уже установленный RouteFlux поверх текущей версии без потери подписок, пользовательских service alias и preset-ов из `/etc/routeflux`:

```bash
ROUTEFLUX_TAG=v0.1.5
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

Если нужен зафиксированный релиз:

```bash
ROUTEFLUX_TAG=v0.1.5
wget -O /tmp/routeflux-install.sh "https://github.com/Alaxay8/routeflux/releases/download/${ROUTEFLUX_TAG}/install.sh" && sh /tmp/routeflux-install.sh
```

Установщик автоматически ставит встроенный runtime Xray, если роутер ещё не предоставляет рабочий бинарный файл Xray и сервис.
Он обновляет RouteFlux поверх текущей установки и сохраняет существующие state-файлы в `/etc/routeflux`.

Сейчас готовые артефакты для простой установки публикуются для:

- `mipsel_24kc`
- `x86_64`
- `aarch64_cortex-a53`

Чтобы удалить RouteFlux и встроенный runtime Xray:

```bash
wget -O /tmp/routeflux-uninstall.sh "https://github.com/Alaxay8/routeflux/releases/latest/download/uninstall.sh" && sh /tmp/routeflux-uninstall.sh
```

### Сборка из исходников

Требования:

- Go `1.26` или новее
- OpenWrt или ImmortalWrt с `nftables`

Соберите локальный бинарник:

```bash
make build
```

Кросс-сборка для OpenWrt:

```bash
make build-openwrt
make build-openwrt-x86_64
make build-openwrt-aarch64_cortex-a53
```

Создайте артефакты релиза:

```bash
make package-release
```

Для ручной установки на роутер скопируйте сгенерированный tarball на роутер и распакуйте его в `/`:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')"
ARCH="${ARCH:-aarch64_cortex-a53}"
scp -O "./dist/routeflux_${VERSION}_${ARCH}.tar.gz" root@router:/tmp/
ssh root@router "tar -xzf /tmp/routeflux_${VERSION}_${ARCH}.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload"
```

## Использование

### Локальный и LAN прокси

RouteFlux предоставляет SOCKS5 (TCP и UDP) и HTTP прокси через выбранный сервер
подписки. По умолчанию они слушают на `127.0.0.1:10808` и `127.0.0.1:10809` и
принимают подключения только с самого роутера.

В LuCI откройте **RouteFlux → Settings → Local / LAN Proxy**, включите **Allow
connections from LAN** и сохраните. Там же можно изменить порты SOCKS5 и HTTP.
Сохранение настроек прокси не зависит от переключения темы оформления.

Те же настройки доступны из командной строки (CLI):

```bash
routeflux proxy get
routeflux --json proxy get
routeflux proxy set --allow-lan true --socks-port 10808 --http-port 10809
routeflux proxy set --allow-lan false
```

Изменяются только указанные параметры. Порты должны различаться, находиться в
диапазоне от 1 до 65535 и не конфликтовать с портами DNS или прозрачного прокси.
Настройки сохраняются при обновлении подписок, смене серверов и перезагрузке.
Изменения применяются к активному соединению Xray немедленно. Если Xray отключен
или RouteFlux использует Zapret, настройки вступят в силу при следующем подключении
Xray. При ошибке применения рантайм откатывается к предыдущим настройкам.

Доступ из LAN запускает прослушивание на всех интерфейсах IPv4 (`0.0.0.0`), включая
localhost, без авторизации по логину и паролю. Ограничивайте доступ доверенными
клиентами локальной сети с помощью системного фаервола OpenWrt. RouteFlux не
открывает правила в зоне WAN. Прослушивание IPv6 этой настройкой не поддерживается.

Для прямого использования прокси без прозрачной маршрутизации оставьте RouteFlux
подключенным и выполните `routeflux firewall disable` (или выберите **Routing → Off**).
Это отключает перехват трафика в RouteFlux, а не системный фаервол OpenWrt.
На странице Routing отображаются эндпоинты LAN и ссылка на Settings. Указывайте
в клиентах реальный LAN-адрес роутера и сохранённые порты. Например, для роутера
с адресом `192.168.1.1`:

```bash
curl --proxy http://192.168.1.1:10809 https://example.com
curl --proxy socks5h://192.168.1.1:10808 https://example.com
```

Не редактируйте сгенерированный файл конфигурации Xray вручную: RouteFlux перегенерирует
его при изменениях. Для работы SOCKS5 UDP также требуется поддержка UDP со стороны
выбранного вышестоящего сервера.

### Повседневный сценарий

```bash
routeflux add https://provider.example/subscription
routeflux list subscriptions
routeflux list nodes --subscription sub-1234567890
routeflux connect --subscription sub-1234567890 --node abcdef123456
routeflux status
routeflux disconnect
```

### Автоматический режим

```bash
routeflux connect --auto --subscription sub-1234567890
routeflux daemon
```

Используйте `routeflux daemon --once`, если хотите выполнить одно обновление и одну проверку состояния без постоянного фонового сервиса.

На OpenWrt включите сервис, если хотите автоматическое обновление, мониторинг failover и восстановление после перезагрузки:

```bash
/etc/init.d/routeflux enable
/etc/init.d/routeflux start
```

### Страницы LuCI

- `Subscriptions`: импорт провайдеров, просмотр профилей и подключение.
- `Routing`: упрощённый повседневный flow для `Off`, `Bypass`, прямых доменов, IPv4 selector-ов и выбора DNS preset.
- `DNS`: полный контроль DNS для `system`, `remote`, `split` и `disabled`.
- `Zapret`: только fallback-домены. Используйте fully qualified domains вроде `youtube.com` или `googlevideo.com`.
- `Settings`: настройка локального LAN-прокси и выбор темы оформления.
- `services` в CLI остаются advanced alias-ами для firewall targets.

### Полезные команды для DNS и фаервола

На OpenWrt `routeflux dns set default` применяет Recommended DNS preset. Это preset, а не пятый DNS-режим. Режимы `remote|split` и остальные реальные DNS-режимы по-прежнему влияют на реальный DNS роутера и LAN, пока подключена нода. RouteFlux перенаправляет `dnsmasq` в локальный Xray DNS runtime, сохраняет локальные имена вроде `.lan` локальными в режиме split и возвращает system DNS при disconnect.

```bash
routeflux dns get
routeflux dns set default
routeflux dns explain

routeflux firewall get
routeflux firewall set hosts 192.168.1.150
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux services list
routeflux zapret get
routeflux zapret set selectors youtube.com googlevideo.com
routeflux firewall explain
```

### Другие полезные команды

```bash
routeflux refresh --all
routeflux move sub-1234567890 up
routeflux restart
routeflux remove sub-1234567890
routeflux diagnostics
routeflux logs
routeflux settings get
routeflux services list
routeflux version
routeflux tui
```

## Примеры

Импортируйте сырую share-ссылку:

```bash
routeflux add 'vless://uuid@example.com:443?...#Example'
```

Импортируйте валидный JSON-конфиг 3x-ui или Xray:

```bash
routeflux add < ./client-config.json
```

Направьте один LAN-девайс через активное соединение RouteFlux:

```bash
routeflux firewall set hosts 192.168.1.150
routeflux connect --subscription sub-1234567890 --node 90c42d5dd302
```

Направьте всю приватную LAN-сеть через RouteFlux:

```bash
routeflux firewall set hosts all
routeflux connect --subscription sub-1234567890 --node 90c42d5dd302
```

Используйте зашифрованный DNS для внешних доменов, сохранив локальные имена на роутере:

```bash
routeflux dns set default
```

Создайте свой alias для targets один раз и затем переиспользуйте его:

```bash
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux firewall set targets openai youtube
```

## Конфигурация

По умолчанию RouteFlux хранит состояние в `/etc/routeflux` на OpenWrt. Для локальной разработки он использует `./.routeflux`.

Полезные переменные окружения:

- `ROUTEFLUX_ROOT`: переопределяет директорию состояния
- `ROUTEFLUX_XRAY_CONFIG`: переопределяет путь к сгенерированному Xray-конфигу
- `ROUTEFLUX_XRAY_SERVICE`: переопределяет скрипт управления сервисом Xray
- `ROUTEFLUX_XRAY_BINARY`: переопределяет бинарный файл Xray, используемый для проверки
- `ROUTEFLUX_FIREWALL_RULES`: переопределяет путь к сгенерированному файлу правил `nftables`

Основные сохраняемые файлы:

- `/etc/routeflux/subscriptions.json`
- `/etc/routeflux/settings.json`
- `/etc/routeflux/state.json`

Для понятных объяснений лучше использовать встроенную справку:

- `routeflux dns explain`
- `routeflux firewall explain`
- `routeflux settings --help`

## Режимы DNS

CLI help показывает только короткий основной путь. Этот раздел — подробный справочник по DNS.

Если не хочется разбираться в деталях DNS, используйте это:

```bash
routeflux dns set default
```

Это лучший повседневный вариант для большинства пользователей: локальные имена остаются локальными, а внешний DNS шифруется.

- `system`: оставить DNS как есть
Пример: DNS на роутере уже работает нормально, и вы не хотите, чтобы RouteFlux что-то менял.

```bash
routeflux dns set mode system
```

- `remote`: отправлять каждый DNS-запрос на выбранные DNS-серверы
Пример: вы хотите, чтобы весь DNS шёл через Cloudflare или Google DNS.

```bash
routeflux dns set mode remote
routeflux dns set transport doh
routeflux dns set servers "1.1.1.1,1.0.0.1"
```

- `split`: оставлять локальные имена на роутере, а интернет-домены отправлять на выбранный DNS
Пример: `router.lan` остаётся локальным, а `google.com` уходит на зашифрованный DNS.

```bash
routeflux dns set default
```

- `disabled`: не записывать DNS-настройки RouteFlux в Xray config
Пример: полезно только для кастомных сценариев, где DNS управляется в другом месте.

```bash
routeflux dns set mode disabled
```

Транспорт DNS:

- `plain`: обычный DNS, без шифрования
- `doh`: зашифрованный DNS Over HTTPS

## Режимы фаервола

CLI help показывает только короткий основной путь. Этот раздел — подробный справочник по фаерволу.

- `disabled`: не перенаправлять трафик роутера через RouteFlux  
Пример: RouteFlux установлен, но ни одно устройство не  направляется принудительно через прокси.

```bash
routeflux firewall disable
```

Что продолжает работать, когда фаервол выключен:

- можно добавлять, обновлять, удалять и просматривать подписки
- можно подключаться к ноде вручную или в автоматическом режиме
- RouteFlux всё равно генерирует и применяет конфиг Xray для выбранной ноды
- DNS-настройки продолжают работать
- CLI, LuCI, TUI, daemon, проверки состояния и failover продолжают работать

Что не происходит, когда фаервол выключен:

- RouteFlux не добавляет правила перенаправления `nftables`
- трафик роутера не отправляется через прокси автоматически
- LAN-устройства не используют выбранную ноду, пока вы не включите `hosts` или `targets`
- режим прозрачного прокси не включается для перехватываемого трафика

Простыми словами: RouteFlux продолжает управлять подписками и активным runtime Xray, но сам по себе не перехватывает трафик роутера или вашей LAN.

- `targets`: отправлять трафик через RouteFlux только тогда, когда адрес назначения совпадает с выбранными сервисами, доменами или IPv4-целями
Пример: через прокси должен идти трафик только к конкретным сервисам.

```bash
routeflux firewall set targets youtube instagram 1.1.1.1
```

Селекторы targets:

- service preset: `discord`, `facetime`, `gemini`, `gemini-mobile`, `instagram`, `netflix`, `notebooklm`, `notebooklm-mobile`, `telegram`, `telegram-web`, `twitter`, `whatsapp`, `youtube`
- пользовательский alias сервиса: `openai`
- домен: `youtube.com`
- IPv4-адрес: `1.1.1.1`
- подсеть: `8.8.8.0/24`
- диапазон: `203.0.113.10-203.0.113.20`

Замечания для доменных targets:

- Создавайте свои alias через `routeflux services set <name> <domain-or-ip...>`, а затем используйте это имя в `routeflux firewall set targets ...`.
- Пользовательский alias может содержать только домены, IPv4-адреса, CIDR и IPv4-диапазоны.
- Имена встроенных preset-ов зарезервированы и остаются read-only.
- RouteFlux трактует `youtube.com` как сам домен и его поддомены.
- Популярные preset-ы вроде `youtube`, `instagram`, `discord`, `twitter`, `netflix`, `whatsapp`, `gemini`, `gemini-mobile`, `notebooklm` и `notebooklm-mobile` автоматически разворачиваются во внутренние доменные семейства.
- Популярные root-домены вроде `youtube.com`, `instagram.com`, `netflix.com`, `x.com`, `gemini.google.com` и `notebooklm.google.com` тоже автоматически разворачиваются во внутренние доменные семейства.
- Используйте `gemini-mobile` и `notebooklm-mobile` для Android и iOS приложений, когда web preset оказывается слишком узким.
- Для mobile preset-ов Google AI может добавляться небольшой набор IPv4 targets поверх доменов, потому что часть трафика приложения не раскрывает пригодный hostname.
- `gemini`, `gemini-mobile`, `notebooklm`, `notebooklm-mobile`, `telegram`, `facetime`, `twitter` и `netflix` это best-effort preset-ы, потому что их приложения могут использовать прямые IP или более широкую общую инфраструктуру вендора.
- Мобильные Google AI preset-ы намеренно шире и могут зацеплять общую Google-инфраструктуру. Если и этого недостаточно, добавьте недостающие Google-домены в свой custom alias и маршрутизируйте уже его.
- Для доменных targets нужен `dnsmasq` с поддержкой `nftset`, обычно это `dnsmasq-full` на OpenWrt.
- Доменные targets зависят от DNS-ответов, которые видит роутер. Если клиенты используют собственный DoH или DoT напрямую, набор IP может остаться пустым.
- На shared CDN RouteFlux теперь откатывает несоответствующий прозрачный трафик на `direct`, а не отправляет весь совпавший IP через выбранную ноду.

- `split`: использовать отдельные таблицы для трафика через RouteFlux, direct-исключений и полностью исключённых устройств
Пример: YouTube и рабочие сервисы идут через RouteFlux, банковские сайты остаются direct, а один ноутбук или ТВ вообще не перехватывается.

```bash
routeflux firewall set split --proxy youtube openai --bypass gosuslugi.ru sberbank.ru --exclude-host 192.168.1.50
```

Селекторы split:

- service preset: `discord`, `facetime`, `gemini`, `gemini-mobile`, `instagram`, `netflix`, `notebooklm`, `notebooklm-mobile`, `telegram`, `telegram-web`, `twitter`, `whatsapp`, `youtube`
- пользовательский alias сервиса: `openai`
- домен: `gosuslugi.ru`
- IPv4-адрес: `1.1.1.1`
- подсеть: `8.8.8.0/24`
- диапазон: `203.0.113.10-203.0.113.20`
- исключённое устройство: `192.168.1.50`, `192.168.1.0/24`, `192.168.1.10-192.168.1.20` или `all`

Замечания для split:

- Split использует тот же парсинг селекторов и то же разворачивание alias, что и `targets`.
- `Keep Direct` имеет приоритет над `Route Through RouteFlux`, если совпадают одни и те же назначения.
- В LuCI и основном CLI unmatched split-трафик по умолчанию остаётся `direct`.
- Для доменных split-правил на OpenWrt нужен `dnsmasq` с поддержкой `nftset`, если домены должны попадать в nftables destination sets.
- `routeflux firewall set anti-target ...` остаётся как legacy alias для `split` с bypass-only селекторами и proxy fallback.
- `block-quic` управляет обработкой проксируемого QUIC. Включайте его только если хотите намеренно блокировать проксируемый QUIC и заставить клиентов перейти на TCP.

- `hosts`: отправлять весь трафик выбранных LAN-устройств через RouteFlux
Пример: направить через прокси один телефон, телевизор или ноутбук.

```bash
routeflux firewall set hosts 192.168.1.150
```

Селекторы хостов:

- одно устройство: `192.168.1.150`
- подсеть: `192.168.1.0/24`
- диапазон: `192.168.1.150-192.168.1.159`
- вся приватная LAN: `all`

Примеры:

```bash
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts 192.168.1.150-192.168.1.159
routeflux firewall set hosts all
```

Другие параметры фаервола:

- `block-quic`: блокировать проксируемый QUIC/UDP и при необходимости принуждать клиентов к TCP fallback
- `port`: меняет порт прозрачного редиректа

## Разработка

Форматирование, vet и тесты:

```bash
make fmt
make lint
go test ./...
```

Сборка и runtime coverage:

```bash
make build
make coverage-runtime
```

Интеграционный набор OpenWrt:

```bash
make test-integration
```

### Интеграционные тесты локального прокси

При установленном Xray и доступном в `PATH` запустите тесты HTTP CONNECT, SOCKS5 TCP
и SOCKS5 UDP на локальных серверах Xray и echo:

```bash
ROUTEFLUX_RUN_XRAY_INTEGRATION=1 go test ./internal/backend/xray -run TestProxyProtocolsIntegration -v
```

Тесты поведения интерфейса LuCI запускаются командой `go test ./test/luci` при наличии Node.js в `PATH`.

Дополнительная документация проекта:

- [docs/config.md](docs/config.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/tui-flow.md](docs/tui-flow.md)

## Лицензия

MIT
