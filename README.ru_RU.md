[English](README.md) | [Русский](README.ru_RU.md)

# RouteFlux

RouteFlux — это нативный для OpenWrt менеджер Xray-подписок для роутеров и пограничных устройств.

Он помогает импортировать прокси-подписки, выбирать подходящую ноду, применять правила маршрутизации трафика роутера и управлять DNS без ручного редактирования Xray JSON. Если RouteFlux экономит вам время, поставьте звезду репозиторию и поделитесь им с другими пользователями OpenWrt.

## Обзор

RouteFlux создан для тех, кому нужен практичный сценарий работы с прокси на OpenWrt:

- Импортируйте URL подписки, сырую ссылку `vless://`, `vmess://` или `trojan://`, либо валидный JSON-конфиг 3x-ui/Xray.
- Подключайтесь к конкретной ноде вручную или позвольте RouteFlux автоматически выбрать лучшую.
- Управляйте всем из CLI, веб-интерфейса LuCI или локального TUI.
- Держите настройки маршрутизации роутера и DNS в понятном виде, а не прячьте их внутри сгенерированного Xray-конфига.

Текущая целевая среда выполнения — OpenWrt и совместимые форки, такие как ImmortalWrt. Практический базовый вариант — OpenWrt `22.03+` с `nftables`.

## Возможности

- Быстрый импорт подписок, share-ссылок и поддерживаемых JSON-файлов 3x-ui/Xray.
- Поддержка нод VLESS, VMess и Trojan.
- Безопасные обновления runtime через `xray -test`, резервную копию последней рабочей конфигурации и контролируемую перезагрузку сервиса.
- Автоматический режим с проверками состояния, live failover, anti-flap логикой и восстановлением runtime после перезагрузки.
- Простые правила прозрачной маршрутизации для LAN-хостов, CIDR, диапазонов или целевых адресов.
- Отдельные DNS-команды с разумным профилем по умолчанию для повседневного использования роутера.
- Общее состояние для CLI, LuCI и TUI, поэтому можно переключаться между интерфейсами без потери контекста.

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

- LuCI: `Services -> RouteFlux`
- CLI: по SSH через `routeflux ...`
- TUI: `routeflux tui`

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
```

Создайте артефакты релиза:

```bash
make package-release
```

Для ручной установки на роутер скопируйте сгенерированный tarball на роутер и распакуйте его в `/`:

```bash
VERSION="$(git describe --tags --always --dirty | sed 's/^v//')"
scp -O "./dist/routeflux_${VERSION}_mipsel_24kc.tar.gz" root@router:/tmp/
ssh root@router "tar -xzf /tmp/routeflux_${VERSION}_mipsel_24kc.tar.gz -C / && rm -f /tmp/luci-indexcache && rm -rf /tmp/luci-modulecache && /etc/init.d/rpcd reload && /etc/init.d/uhttpd reload"
```

## Использование

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

### Полезные команды для DNS и фаервола

```bash
routeflux dns get
routeflux dns set default
routeflux dns explain

routeflux firewall get
routeflux firewall set hosts 192.168.1.150
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux services list
routeflux firewall explain
```

### Другие полезные команды

```bash
routeflux refresh --all
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

- `hosts`: отправлять весь TCP-трафик выбранных LAN-устройств через RouteFlux
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

- `block-quic`: блокирует UDP/443 в режиме `hosts`, или для LAN-клиентов в режиме `targets`, чтобы приложения не обходили прокси-маршрутизацию через QUIC
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

Дополнительная документация проекта:

- [docs/config.md](docs/config.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/tui-flow.md](docs/tui-flow.md)

## Лицензия

MIT
