import os
import sys
import re
import xml.etree.ElementTree as ET
import cffi

WIDTH = 2400
HEIGHT = 1520


def build_svg(lang="en", scale=1.0):
    is_ru = lang.lower() == "ru"
    pixel_w = int(WIDTH * scale)
    pixel_h = int(HEIGHT * scale)

    # Header texts
    title = "RouteFlux • Архитектура и принцип работы" if is_ru else "RouteFlux • Architecture &amp; System Flow"
    subtitle = (
        "Нативный менеджер Xray-подписок, интеллектуальной маршрутизации, DNS и LAN-прокси для OpenWrt / ImmortalWrt"
        if is_ru else
        "Native Xray subscription manager, intelligent routing, DNS &amp; LAN proxy for OpenWrt / ImmortalWrt"
    )
    badge_proxy = "SOCKS5 и HTTP LAN" if is_ru else "SOCKS5 &amp; HTTP LAN"
    badge_native = "Go 1.26 Native"

    # Section 1
    s1_title = "1. УПРАВЛЕНИЕ И БИЗНЕС-ЛОГИКА (CONTROL PLANE GO)" if is_ru else "1. CONTROL PLANE &amp; BUSINESS LOGIC (GO CORE)"
    
    # 1.1 Management Interfaces
    b11_title = "Интерфейсы взаимодействия" if is_ru else "Management Interfaces"
    b11_sub = "Единое состояние между всеми клиентами" if is_ru else "Single shared source of truth across all clients"
    b11_web_title = "Веб-интерфейс LuCI (OpenWrt)" if is_ru else "LuCI Web UI (OpenWrt)"
    b11_web_desc = "Вкладки: Subscriptions, Routing, DNS, Zapret, Settings" if is_ru else "Tabs: Subscriptions, Routing, DNS, Zapret, Settings"
    b11_cli_title = "CLI (Cobra) • SSH доступ" if is_ru else "CLI (Cobra) • SSH Terminal"
    b11_cli_desc = "Команды: routeflux add, connect, proxy, dns, firewall" if is_ru else "Commands: routeflux add, connect, proxy, dns, firewall"
    b11_tui_title = "TUI (Bubble Tea) и Демон" if is_ru else "TUI (Bubble Tea) &amp; Daemon"
    b11_tui_desc1 = "Интерактивный список серверов в консоли" if is_ru else "Interactive terminal server browser"
    b11_tui_desc2 = "routeflux daemon: фоновые проверки и авто-обновление" if is_ru else "routeflux daemon: background checks &amp; auto-refresh"

    # 1.2 Subscription & Parser
    b12_title = "Импорт и парсинг подписок" if is_ru else "Subscription Import &amp; Parser"
    b12_sub = "Модуль parser: авто-определение форматов и протоколов" if is_ru else "parser module: auto-detection of formats &amp; protocols"
    b12_proto_header = "Поддерживаемые протоколы и ссылки:" if is_ru else "Supported Protocols &amp; Links:"
    b12_proto_sub1 = "• URL подписок (Base64) • Share-ссылки (vless://, vmess://...)" if is_ru else "• Subscription URLs (Base64) • Share links (vless://, vmess://...)"
    b12_proto_sub2 = "• Профили клиентов 3x-ui и нативный Xray JSON" if is_ru else "• 3x-ui client configurations and raw Xray JSON profiles"
    b12_store_header = "Хранилище состояния (Store):" if is_ru else "State Persistence (Store):"
    b12_store_sub1 = "Атомарная запись JSON-файлов в /etc/routeflux/:" if is_ru else "Atomic JSON persistence in /etc/routeflux/:"
    b12_store_sub2 = "✓ Сохраняет подписки, ноды и настройки прокси при перезагрузках" if is_ru else "✓ Preserves subscriptions, nodes, and proxy settings across reboots"

    # 1.3 Probe Engine
    b13_title = "Probe Engine • Замер задержек и выбор лучшей ноды" if is_ru else "Probe Engine • Health Evaluation &amp; Auto Selection"
    b13_sub = "Модули probe, speedtest и умное anti-flap переключение" if is_ru else "probe, speedtest, and smart anti-flap switching"
    b13_test_header = "Параллельный замер доступности:" if is_ru else "Parallel Availability &amp; Speed Testing:"
    b13_test_1 = "• Фоновый TCP handshake и RTT пинг всех нод подписки" if is_ru else "• Background TCP handshake and RTT ping across all subscription nodes"
    b13_test_2 = "• Расчет скоринга нод (Health Score) по задержке, джиттеру и потерям" if is_ru else "• Dynamic health score calculated from latency, jitter, and packet loss"
    b13_test_3 = "• Проверка реального выхода через generate_204 перед активацией" if is_ru else "• Egress verification via generate_204 before active deployment"
    b13_failover_header = "Интеллектуальный Auto-Failover &amp; Anti-Flap:" if is_ru else "Intelligent Auto-Failover &amp; Anti-Flap:"
    b13_failover_1 = "• Защита от частых переключений (anti-flap): кулдаун стабильности" if is_ru else "• Anti-flap protection: cooldown window prevents rapid flipping"
    b13_failover_2 = "• Мгновенное переключение на резервную ноду при обрыве связи" if is_ru else "• Instant transparent failover to backup node if active node drops"
    b13_failover_3 = "✓ Гарантирует непрерывный доступ для клиентов сети и прокси" if is_ru else "✓ Guarantees continuous connectivity for all LAN devices and proxies"

    # 1.4 Config Generator
    b14_title = "Атомарный генератор конфигов и сервис-менеджер" if is_ru else "Atomic Config Generator &amp; Service Sync"
    b14_sub = "Оркестрация backend/xray и platform/openwrt" if is_ru else "backend/xray and platform/openwrt orchestration"
    b14_gen_header = "Транзакционная генерация Xray JSON:" if is_ru else "Transactional Xray JSON Generation:"
    b14_gen_1 = "1. Рендеринг /var/run/routeflux/xray.json с активной нодой, DNS, inbounds" if is_ru else "1. Renders /var/run/routeflux/xray.json with active node, DNS, inbounds"
    b14_gen_2 = "2. Обязательный тест валидности: xray -test -c /tmp/xray.json" if is_ru else "2. Strict validation test: xray -test -c /tmp/xray.json"
    b14_gen_3 = "3. Атомарный откат к рабочей версии при ошибках валидации" if is_ru else "3. Atomic rollback snapshot: previous config restored if validation fails"
    b14_sync_header = "Синхронизация платформы OpenWrt:" if is_ru else "OpenWrt Dataplane Synchronization:"
    b14_sync_1 = "• Настройка правил фаервола via fw4 table inet routeflux" if is_ru else "• Configures nftables ruleset via fw4 table inet routeflux"
    b14_sync_2 = "• Генерация директив nftset для службы dnsmasq" if is_ru else "• Generates dnsmasq nftset directives for dynamic domain capturing"
    b14_sync_3 = "• Мягкий перезапуск сервисов (/etc/init.d/xray reload)" if is_ru else "• Gracefully reloads services (/etc/init.d/xray reload)"

    # Section 2
    s2_title = "2. СЕТЕВОЙ ТРАФИК И ПЕРЕХВАТ (DATA PLANE OPENWRT)" if is_ru else "2. OPENWRT DATAPLANE • TRAFFIC INTERCEPTION &amp; LAN PROXY"
    
    # 2.1 LAN Clients
    b21_title = "Клиенты LAN и режимы работы" if is_ru else "LAN Clients &amp; Traffic Modes"
    b21_sub = "Устройства домашней и офисной сети (192.168.1.0/24)" if is_ru else "Home &amp; office network devices (192.168.1.0/24)"
    b21_mode_a_title = "Режим А: Прозрачный шлюз (По умолчанию)" if is_ru else "Mode A: Transparent Gateway (Default)"
    b21_mode_a_1 = "Устройства отправляют обычный трафик через шлюз" if is_ru else "Devices send standard traffic through default gateway"
    b21_mode_a_2 = "nftables прозрачно перехватывает DNS (53) и TCP/UDP" if is_ru else "nftables transparently intercepts DNS (53) and TCP/UDP"
    b21_mode_a_3 = "Не требует настроек на телефонах, ПК или смарт-ТВ" if is_ru else "Zero client configuration needed on phones, PCs, or TVs"
    b21_mode_b_title = "Режим Б: Прямой локальный и LAN-прокси (NEW)" if is_ru else "Mode B: Explicit Local &amp; LAN Proxy (NEW)"
    b21_mode_b_1 = "Routing: Off — роутер не вмешивается в чужой трафик" if is_ru else "Routing: Off — no router firewall interception"
    b21_mode_b_2 = "• SOCKS5 (:10808) и HTTP (:10809) открыты для локальной сети" if is_ru else "• SOCKS5 (:10808) &amp; HTTP (:10809) open to LAN"
    b21_mode_b_3 = "• Настраивается в браузере, Telegram, Happ, SwitchyOmega" if is_ru else "• Configured in browsers, Telegram, Happ, SwitchyOmega"
    b21_mode_b_4 = "✓ Сохраняет умный дом, банкинг и нативную скорость NAT" if is_ru else "✓ Preserves smart home, banking, and full native NAT speeds"

    # 2.2 dnsmasq nftset
    b22_title = "dnsmasq-full и динамический nftset" if is_ru else "dnsmasq-full &amp; Dynamic nftset"
    b22_sub = "Интеллектуальное сопоставление доменов без списков IP" if is_ru else "Dynamic domain-to-IP resolution without static IP lists"
    b22_how_header = "Принцип работы директивы nftset:" if is_ru else "How the nftset directive works:"
    b22_how_1 = "1. RouteFlux передает списки доменов в dnsmasq:" if is_ru else "1. RouteFlux injects target domain patterns into dnsmasq:"
    b22_how_2 = "2. Клиент запрашивает домен (например, googlevideo.com)" if is_ru else "2. Client requests domain (e.g. googlevideo.com)"
    b22_how_3 = "3. Хук ядра dnsmasq мгновенно добавляет IP в набор nftables!" if is_ru else "3. dnsmasq kernel hook adds resolved IP directly to nftables set!"
    b22_how_4 = "Последующие пакеты сразу попадают под правила прокси" if is_ru else "Subsequent packets immediately match proxy redirection rules"
    b22_dns_modes = "Режимы DNS (DNS Modes):" if is_ru else "DNS Resolution Modes:"
    b22_dns_split = "• <tspan font-weight=\"bold\" fill=\"#f8fafc\">Split (По умолч.):</tspan> .lan локально, внешний DNS шифруется (DoH)" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#f8fafc\">Split (Default):</tspan> .lan queries stay local, external via encrypted DoH"
    b22_dns_remote = "• <tspan font-weight=\"bold\" fill=\"#f8fafc\">Remote:</tspan> 100% DNS-запросов идут через удаленный DoH резолвер" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#f8fafc\">Remote:</tspan> 100% of DNS routed through remote upstream resolvers"

    # 2.3 nftables
    b23_title = "nftables (Таблица inet routeflux) • Аппаратный роутинг" if is_ru else "nftables (table inet routeflux) • Hardware-Accelerated Flow"
    b23_sub = "Фильтрация и прозрачный редирект в ядре Linux (Netfilter)" if is_ru else "In-kernel Netfilter packet evaluation and transparent redirection"
    b23_sets_header = "Динамические наборы ядра (Sets):" if is_ru else "Dynamic Kernel Sets:"
    b23_set_excl = "Исключенные хосты LAN (ТВ, консоли, рабочие ПК)" if is_ru else "Bypassed LAN devices (TV, consoles, work PCs)"
    b23_set_priv = "Локальные подсети: 10.0.0.0/8, 192.168.0.0/16, lo..." if is_ru else "Private subnets: 10.0.0.0/8, 192.168.0.0/16, lo..."
    b23_set_dir = "Прямой выход (банки, госуслуги, локальные ресурсы)" if is_ru else "Direct bypass targets (banking, government, local)"
    b23_set_pxy = "Целевые сервисы (YouTube, Discord, AI и др.)" if is_ru else "Dynamic proxy targets (YouTube, Discord, AI, etc.)"
    b23_chains_header = "Цепочки prerouting (TCP) и prerouting_mangle (UDP TProxy):" if is_ru else "Kernel Chains: prerouting (TCP) &amp; prerouting_mangle (UDP TProxy):"
    b23_rule1_out = "➔ Прямой выход (Direct WAN)" if is_ru else "➔ Direct WAN Egress"
    b23_rule2_out = "➔ Прямой выход (Direct WAN)" if is_ru else "➔ Direct WAN Egress"
    b23_rule3_desc = "Перенаправление TCP пакетов в слушатель dokodemo-door" if is_ru else "Redirects TCP packets to Xray dokodemo-door listener"
    b23_rule3_out = "➔ В Xray Inbound :12345" if is_ru else "➔ To Xray Inbound :12345"
    b23_rule4_desc = "TProxy + маршрутизация таблицы 100 для UDP (опция block-quic)" if is_ru else "TProxy + policy routing table 100 for UDP streams (block-quic option)"
    b23_rule4_out = "➔ В Xray Inbound :12345" if is_ru else "➔ To Xray Inbound :12345"

    conn_tcp_tproxy = "TCP Redirect и UDP TProxy (:12345)" if is_ru else "TCP Redirect &amp; UDP TProxy (:12345)"
    conn_lan_proxy = "Прямые входы SOCKS5 / HTTP для LAN" if is_ru else "Direct LAN SOCKS5 / HTTP Inbounds"

    # Section 3
    s3_title = "3. ИСПОЛНЯЕМЫЙ РАНТАЙМ XRAY И ВЫХОД В ИНТЕРНЕТ" if is_ru else "3. XRAY RUNTIME PROCESS &amp; GLOBAL INTERNET EGRESS"
    
    # 3.1 Inbounds
    b31_title = "Двойные Inbounds: Прозрачный и LAN-прокси" if is_ru else "Dual Inbounds: Transparent &amp; LAN Proxy"
    b31_sub = "Принимает трафик от роутера и прямые подключения из LAN" if is_ru else "Receives router-intercepted and direct LAN proxy streams"
    b31_tp_1 = "• Точка входа TProxy &amp; Redirect для сетевого фильтра роутера" if is_ru else "• TProxy &amp; Redirect entry point for router netfilter"
    b31_tp_2 = "• Sniffing: определение HTTP Host и TLS SNI доменов" if is_ru else "• Sniffing: HTTP Host &amp; TLS SNI domain inspection"
    b31_tp_3 = "✓ Не совпавшие запросы мягко уходят в прямой direct out" if is_ru else "✓ Unmatched requests fall through gently to direct out"
    b31_socks_1 = "• Протокол SOCKS5 с полной поддержкой UDP associate" if is_ru else "• SOCKS5 protocol with full UDP associate support"
    b31_socks_2 = "• HTTP forward-прокси для расширений браузера и приложений" if is_ru else "• HTTP forward proxy for browser extensions and apps"
    b31_socks_3 = "Настраиваемый бинд: 127.0.0.1 (только роутер) или 0.0.0.0 (LAN)" if is_ru else "Configurable: 127.0.0.1 (local only) or 0.0.0.0 (LAN access)"

    # 3.2 Xray Router & DNS
    b32_title = "Внутренний роутинг Xray и DNS" if is_ru else "Internal Xray Routing &amp; DNS"
    b32_sub = "Маршрутизация на основе правил по inbounds и доменам" if is_ru else "Rule-based dispatching based on inbounds and domains"
    b32_table_header = "Таблица маршрутизации (Routing Rules):" if is_ru else "Routing Rule Table:"
    b32_rule_sel = "• <tspan font-weight=\"bold\" fill=\"#a855f7\">tag: selected</tspan> ➔ Активная нода подписки" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#a855f7\">tag: selected</tspan> ➔ Active subscription server"
    b32_rule_dir = "• <tspan font-weight=\"bold\" fill=\"#10b981\">tag: direct</tspan> ➔ Прямой выход через провайдера роутера" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#10b981\">tag: direct</tspan> ➔ Direct WAN egress via router ISP"
    b32_rule_zap = "• <tspan font-weight=\"bold\" fill=\"#f59e0b\">tag: zapret</tspan> ➔ Локальный сервис обхода DPI (nfqws)" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#f59e0b\">tag: zapret</tspan> ➔ Local nfqws DPI circumvention"
    b32_rule_blk = "• <tspan font-weight=\"bold\" fill=\"#ef4444\">tag: block</tspan> ➔ Блокировка рекламы и QUIC UDP 443" if is_ru else "• <tspan font-weight=\"bold\" fill=\"#ef4444\">tag: block</tspan> ➔ Blackhole ad networks &amp; QUIC UDP 443"
    b32_doh_1 = "Встроенный DoH / DoT резолвер для внешних доменов" if is_ru else "Built-in DoH / DoT DNS resolver for remote domain resolution"
    b32_doh_2 = "Исключает утечки DNS (DNS Leaks) и подмену ответов провайдером" if is_ru else "Prevents ISP DNS poisoning and eavesdropping"

    # 3.3 Outbounds
    b33_title = "Outbounds • Выходы в глобальную сеть (WAN / Internet)" if is_ru else "Outbounds • Global Network Egress (WAN / Internet)"
    b33_sub = "Финальная зашифрованная доставка до серверов назначения" if is_ru else "Final encrypted delivery to target destination servers"
    
    b33_c1_title = "Шифрованный туннель (Proxy)" if is_ru else "Encrypted Tunnel (Selected Node)"
    b33_c1_sub = "VLESS-Reality / VMess / Trojan / Hy2"
    b33_c1_1 = "Трафик шифруется и идет на зарубежный сервер" if is_ru else "Traffic encrypted to remote overseas VPS"
    b33_c1_2 = "• Обход блокировок, DPI и замедлений" if is_ru else "• Bypasses censorship, DPI &amp; throttling"
    b33_c1_3 = "• YouTube 4K, Discord, AI, Notion, Соцсети" if is_ru else "• YouTube 4K, Discord, AI, Notion, Socials"
    b33_c1_4 = "• Полная защита от цензуры провайдера" if is_ru else "• Full privacy from local ISP logging"
    b33_c1_5 = "➔ Удаленный сервер ➔ Глобальный Интернет" if is_ru else "➔ Remote VPS ➔ Global Internet"

    b33_c2_title = "Прямой маршрут (Direct WAN)" if is_ru else "Direct Route (Direct WAN)"
    b33_c2_sub = "Без прокси через местного провайдера" if is_ru else "Bypasses proxy through local ISP"
    b33_c2_1 = "Максимальная скорость канала, минимальный пинг" if is_ru else "Native wire-speed, lowest possible ping"
    b33_c2_2 = "• Российские сервисы, Банки, Госуслуги" if is_ru else "• Banking, government portals, local media"
    b33_c2_3 = "• Исключенные устройства LAN (ТВ, приставки)" if is_ru else "• Excluded LAN devices (smart TVs, consoles)"
    b33_c2_4 = "• Весь трафик вне списков проксирования" if is_ru else "• All non-blocked domestic destinations"
    b33_c2_5 = "➔ Местный провайдер ➔ Рунет" if is_ru else "➔ Local ISP ➔ Domestic Internet"

    b33_c3_title = "Zapret Fallback (Обход DPI)" if is_ru else "Zapret Fallback (DPI Bypass)"
    b33_c3_sub = "Модификация пакетов без удаленного сервера" if is_ru else "Local packet modification without remote VPS"
    b33_c3_1 = "Пакетный хак TCP/TLS для обхода ТСПУ" if is_ru else "TCP/TLS packet desynchronization against DPI"
    b33_c3_2 = "• Нулевой расход трафика подписки / VPS" if is_ru else "• Zero VPS bandwidth consumption"
    b33_c3_3 = "• Автоматический fallback при сбоях ноды" if is_ru else "• Automatic fallback if proxy connection fails"
    b33_c3_4 = "• Обрабатывается локально через nfqws" if is_ru else "• Handled locally on router via nfqws"
    b33_c3_5 = "➔ Локальный nfqws ➔ Direct WAN" if is_ru else "➔ Local nfqws ➔ Direct WAN"

    # Footer
    f_title = "ПОШАГОВЫЙ ПРИНЦИП РАБОТЫ И ЖИЗНЕННЫЙ ЦИКЛ:" if is_ru else "STEP-BY-STEP OPERATIONAL WORKFLOW:"
    
    f_s1_title = "Импорт подписки и замер задержек" if is_ru else "Subscription Import &amp; Benchmarking"
    f_s1_1 = "Пользователь добавляет подписку через LuCI или CLI." if is_ru else "User adds subscription URL via Web UI or CLI."
    f_s1_2 = "RouteFlux параллельно пингует все ноды и выбирает" if is_ru else "RouteFlux probes all nodes in parallel and selects"
    f_s1_3 = "наиболее стабильный сервер с наименьшим RTT." if is_ru else "the lowest-latency, most reliable server."

    f_s2_title = "Атомарный тест и активация конфига" if is_ru else "Atomic Config Test &amp; Deployment"
    f_s2_1 = "Формируется Xray JSON и проверяется: xray -test." if is_ru else "Renders Xray JSON and verifies with xray -test."
    f_s2_2 = "При успехе одновременно применяются правила" if is_ru else "On success, atomic nftables and dnsmasq"
    f_s2_3 = "в таблице nftables и директивы для dnsmasq." if is_ru else "nftset directives are applied simultaneously."

    f_s3_title = "Умный DNS-перехват (nftset)" if is_ru else "Dynamic DNS Interception (nftset)"
    f_s3_1 = "Клиент запрашивает домен (YouTube, Discord и др.)." if is_ru else "Client queries a domain (e.g., YouTube, Discord)."
    f_s3_2 = "dnsmasq на лету наполняет набор @proxy_target_v4." if is_ru else "dnsmasq automatically populates kernel @proxy_target_v4."
    f_s3_3 = "Статические списки IP-адресов больше не нужны!" if is_ru else "No static IP list maintenance required!"

    f_s4_title = "Раздельная маршрутизация и LAN-прокси" if is_ru else "Dual Routing &amp; Direct LAN Proxy"
    f_s4_1 = "Прямой трафик идет в обход прокси в WAN;" if is_ru else "Transparent traffic splits between Direct and Xray;"
    f_s4_2 = "Либо при Routing Off устройства подключаются" if is_ru else "Alternatively, with Routing Off, devices connect"
    f_s4_3 = "напрямую к SOCKS5 (:10808) или HTTP (:10809)." if is_ru else "directly to SOCKS5 (:10808) or HTTP (:10809)."

    f_s5_title = "Непрерывный мониторинг и Anti-Flap" if is_ru else "Continuous Failover &amp; Anti-Flap"
    f_s5_1 = "Фоновый демон постоянно следит за состоянием." if is_ru else "Background daemon continuously monitors health."
    f_s5_2 = "При падении активной ноды RouteFlux бесшовно" if is_ru else "If the active node fails, RouteFlux seamlessly"
    f_s5_3 = "переключает трафик на резервный сервер." if is_ru else "switches traffic to the next best standby server."

    raw_svg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {WIDTH} {HEIGHT}" width="{pixel_w}" height="{pixel_h}" style="background-color: #0b0f19; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;">
  <defs>
    <!-- Background Gradient -->
    <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#090d16" />
      <stop offset="40%" stop-color="#0f172a" />
      <stop offset="100%" stop-color="#080c14" />
    </linearGradient>

    <!-- Header Gradient -->
    <linearGradient id="headerGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#38bdf8" />
      <stop offset="50%" stop-color="#818cf8" />
      <stop offset="100%" stop-color="#c084fc" />
    </linearGradient>

    <!-- Card Background Glows -->
    <linearGradient id="glowControl" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#0284c7" stop-opacity="0.18" />
      <stop offset="100%" stop-color="#0284c7" stop-opacity="0.0" />
    </linearGradient>

    <linearGradient id="glowNft" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#ea580c" stop-opacity="0.18" />
      <stop offset="100%" stop-color="#ea580c" stop-opacity="0.0" />
    </linearGradient>

    <linearGradient id="glowXray" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#7c3aed" stop-opacity="0.18" />
      <stop offset="100%" stop-color="#7c3aed" stop-opacity="0.0" />
    </linearGradient>

    <!-- Arrow Markers -->
    <marker id="arrowBlue" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
      <path d="M 0 1 L 7 4.5 L 0 8 z" fill="#38bdf8" />
    </marker>
    <marker id="arrowOrange" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
      <path d="M 0 1 L 7 4.5 L 0 8 z" fill="#f97316" />
    </marker>
    <marker id="arrowPurple" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
      <path d="M 0 1 L 7 4.5 L 0 8 z" fill="#c084fc" />
    </marker>
    <marker id="arrowGreen" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
      <path d="M 0 1 L 7 4.5 L 0 8 z" fill="#10b981" />
    </marker>
    <marker id="arrowTeal" markerWidth="9" markerHeight="9" refX="7" refY="4.5" orient="auto">
      <path d="M 0 1 L 7 4.5 L 0 8 z" fill="#14b8a6" />
    </marker>

    <!-- Drop Shadow Filter -->
    <filter id="shadow" x="-3%" y="-3%" width="106%" height="106%">
      <feDropShadow dx="0" dy="6" stdDeviation="10" flood-color="#000000" flood-opacity="0.5" />
    </filter>
  </defs>

  <!-- Background Base -->
  <rect width="{WIDTH}" height="{HEIGHT}" fill="url(#bgGrad)" />

  <!-- Subtle Blueprint Grid -->
  <g opacity="0.035" stroke="#ffffff" stroke-width="1">
    {"".join(f'<line x1="{x}" y1="0" x2="{x}" y2="{HEIGHT}"/>' for x in range(0, WIDTH, 60))}
    {"".join(f'<line x1="0" y1="{y}" x2="{WIDTH}" y2="{y}"/>' for y in range(0, HEIGHT, 60))}
  </g>

  <!-- ==================== HEADER ==================== -->
  <g transform="translate(80, 45)">
    <!-- Logo Badge -->
    <rect x="0" y="0" width="56" height="56" rx="14" fill="#1e293b" stroke="#38bdf8" stroke-width="2" />
    <circle cx="28" cy="28" r="16" fill="none" stroke="#38bdf8" stroke-width="2.5" stroke-dasharray="8,4" />
    <circle cx="28" cy="28" r="6" fill="#38bdf8" />
    <path d="M 12 28 L 22 28 M 34 28 L 44 28 M 28 12 L 28 22 M 28 34 L 28 44" stroke="#38bdf8" stroke-width="2.5" stroke-linecap="round" />

    <text x="74" y="32" font-size="32" font-weight="800" fill="url(#headerGrad)" letter-spacing="0.4">{title}</text>
    <text x="74" y="52" font-size="14.5" fill="#94a3b8">{subtitle}</text>

    <!-- Badges -->
    <g transform="translate(1330, 10)">
      <rect x="0" y="0" width="130" height="32" rx="16" fill="#1e293b" stroke="#38bdf8" stroke-width="1.2" />
      <text x="65" y="20.5" font-size="12" font-weight="700" fill="#38bdf8" text-anchor="middle">OpenWrt 22.03+</text>

      <rect x="145" y="0" width="110" height="32" rx="16" fill="#1e293b" stroke="#f97316" stroke-width="1.2" />
      <text x="200" y="20.5" font-size="12" font-weight="700" fill="#f97316" text-anchor="middle">nftables fw4</text>

      <rect x="270" y="0" width="145" height="32" rx="16" fill="#1e293b" stroke="#14b8a6" stroke-width="1.2" />
      <text x="342.5" y="20.5" font-size="12" font-weight="700" fill="#14b8a6" text-anchor="middle">dnsmasq + nftset</text>

      <rect x="430" y="0" width="110" height="32" rx="16" fill="#1e293b" stroke="#a855f7" stroke-width="1.2" />
      <text x="485" y="20.5" font-size="12" font-weight="700" fill="#a855f7" text-anchor="middle">Xray-core</text>

      <rect x="555" y="0" width="155" height="32" rx="16" fill="#1e293b" stroke="#10b981" stroke-width="1.2" />
      <text x="632.5" y="20.5" font-size="12" font-weight="700" fill="#10b981" text-anchor="middle">{badge_proxy}</text>

      <rect x="725" y="0" width="120" height="32" rx="16" fill="#1e293b" stroke="#e2e8f0" stroke-width="1.2" />
      <text x="785" y="20.5" font-size="12" font-weight="700" fill="#e2e8f0" text-anchor="middle">{badge_native}</text>
    </g>
  </g>

  <!-- ==================== SECTION 1: CONTROL PLANE (TOP) ==================== -->
  <g transform="translate(80, 125)">
    <!-- Zone Container -->
    <rect width="2240" height="360" rx="18" fill="#0f172a" stroke="#1e3a8a" stroke-width="1.6" filter="url(#shadow)" />
    <rect width="2240" height="360" rx="18" fill="url(#glowControl)" />

    <!-- Zone Label -->
    <rect x="24" y="16" width="390" height="28" rx="8" fill="#1e3a8a" />
    <text x="36" y="35" font-size="12.5" font-weight="800" fill="#93c5fd" letter-spacing="1">{s1_title}</text>

    <!-- Sub-Box 1.1: Management Interfaces -->
    <g transform="translate(25, 56)">
      <rect width="455" height="282" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#38bdf8">{b11_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b11_sub}</text>

      <!-- LuCI Item -->
      <g transform="translate(18, 66)">
        <rect width="419" height="58" rx="8" fill="#0f172a" stroke="#38bdf8" stroke-width="1" />
        <rect x="12" y="12" width="34" height="34" rx="7" fill="#0284c7" />
        <text x="29" y="34" font-size="13" font-weight="bold" fill="#ffffff" text-anchor="middle">WEB</text>
        <text x="58" y="27" font-size="13.5" font-weight="700" fill="#f8fafc">{b11_web_title}</text>
        <text x="58" y="44" font-size="11" fill="#94a3b8">{b11_web_desc}</text>
      </g>

      <!-- CLI Item -->
      <g transform="translate(18, 134)">
        <rect width="419" height="58" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <rect x="12" y="12" width="34" height="34" rx="7" fill="#334155" />
        <text x="29" y="34" font-size="13" font-weight="bold" fill="#38bdf8" text-anchor="middle">&gt;_</text>
        <text x="58" y="27" font-size="13.5" font-weight="700" fill="#f8fafc">{b11_cli_title}</text>
        <text x="58" y="44" font-size="11" fill="#94a3b8">{b11_cli_desc}</text>
      </g>

      <!-- TUI Item -->
      <g transform="translate(18, 202)">
        <rect width="419" height="66" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <rect x="12" y="16" width="34" height="34" rx="7" fill="#475569" />
        <text x="29" y="38" font-size="12" font-weight="bold" fill="#c084fc" text-anchor="middle">TUI</text>
        <text x="58" y="27" font-size="13.5" font-weight="700" fill="#f8fafc">{b11_tui_title}</text>
        <text x="58" y="44" font-size="11" fill="#94a3b8">{b11_tui_desc1}</text>
        <text x="58" y="58" font-size="10.5" fill="#38bdf8">{b11_tui_desc2}</text>
      </g>
    </g>

    <!-- Sub-Box 1.2: Subscriptions & Parser -->
    <g transform="translate(500, 56)">
      <rect width="500" height="282" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#38bdf8">{b12_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b12_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="464" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#cbd5e1">{b12_proto_header}</text>

        <rect x="16" y="34" width="102" height="23" rx="5" fill="#1e293b" />
        <text x="67" y="50" font-size="11" font-weight="700" fill="#38bdf8" text-anchor="middle">VLESS + Reality</text>

        <rect x="126" y="34" width="72" height="23" rx="5" fill="#1e293b" />
        <text x="162" y="50" font-size="11" font-weight="700" fill="#a855f7" text-anchor="middle">VMess</text>

        <rect x="206" y="34" width="66" height="23" rx="5" fill="#1e293b" />
        <text x="239" y="50" font-size="11" font-weight="700" fill="#f59e0b" text-anchor="middle">Trojan</text>

        <rect x="280" y="34" width="94" height="23" rx="5" fill="#1e293b" />
        <text x="327" y="50" font-size="11" font-weight="700" fill="#ec4899" text-anchor="middle">Hysteria 1 / 2</text>

        <rect x="382" y="34" width="66" height="23" rx="5" fill="#1e293b" />
        <text x="415" y="50" font-size="11" font-weight="700" fill="#10b981" text-anchor="middle">Socks5</text>

        <text x="16" y="77" font-size="11" fill="#94a3b8">{b12_proto_sub1}</text>
        <text x="16" y="90" font-size="10.5" fill="#64748b">{b12_proto_sub2}</text>
      </g>

      <g transform="translate(18, 172)">
        <rect width="464" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#cbd5e1">{b12_store_header}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b12_store_sub1}</text>
        <text x="16" y="60" font-size="11" font-family="monospace" fill="#38bdf8">subscriptions.json • settings.json • state.json</text>
        <text x="16" y="78" font-size="10.5" fill="#10b981">{b12_store_sub2}</text>
      </g>
    </g>

    <!-- Sub-Box 1.3: Probe Engine & Scoring -->
    <g transform="translate(1020, 56)">
      <rect width="580" height="282" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#38bdf8">{b13_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b13_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="544" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b13_test_header}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b13_test_1}</text>
        <text x="16" y="60" font-size="11" fill="#94a3b8">{b13_test_2}</text>
        <text x="16" y="78" font-size="11" fill="#94a3b8">{b13_test_3}</text>
      </g>

      <g transform="translate(18, 172)">
        <rect width="544" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b13_failover_header}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b13_failover_1}</text>
        <text x="16" y="60" font-size="11" fill="#94a3b8">{b13_failover_2}</text>
        <text x="16" y="78" font-size="10.5" fill="#10b981">{b13_failover_3}</text>
      </g>
    </g>

    <!-- Sub-Box 1.4: Config Generator & Service Manager -->
    <g transform="translate(1620, 56)">
      <rect width="595" height="282" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#38bdf8">{b14_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b14_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="559" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b14_gen_header}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b14_gen_1}</text>
        <text x="16" y="60" font-size="11" fill="#f59e0b" font-weight="bold">{b14_gen_2}</text>
        <text x="16" y="78" font-size="11" fill="#94a3b8">{b14_gen_3}</text>
      </g>

      <g transform="translate(18, 172)">
        <rect width="559" height="96" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b14_sync_header}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b14_sync_1}</text>
        <text x="16" y="60" font-size="11" fill="#94a3b8">{b14_sync_2}</text>
        <text x="16" y="78" font-size="11" fill="#94a3b8">{b14_sync_3}</text>
      </g>
    </g>
  </g>

  <!-- Control Plane Connector Lines -->
  <path d="M 760 485 L 760 525" stroke="#14b8a6" stroke-width="2.5" stroke-dasharray="5,4" marker-end="url(#arrowTeal)" fill="none" />
  <rect x="690" y="495" width="140" height="22" rx="5" fill="#0f172a" stroke="#14b8a6" stroke-width="1" />
  <text x="760" y="510" font-size="10.5" font-weight="bold" fill="#14b8a6" text-anchor="middle">dnsmasq.conf</text>

  <path d="M 1350 485 L 1350 525" stroke="#f97316" stroke-width="2.5" stroke-dasharray="5,4" marker-end="url(#arrowOrange)" fill="none" />
  <rect x="1285" y="495" width="130" height="22" rx="5" fill="#0f172a" stroke="#f97316" stroke-width="1" />
  <text x="1350" y="510" font-size="10.5" font-weight="bold" fill="#f97316" text-anchor="middle">nftables rules</text>

  <!-- ==================== SECTION 2: DATA PLANE (MIDDLE) ==================== -->
  <g transform="translate(80, 535)">
    <!-- Zone Container -->
    <rect width="2240" height="375" rx="18" fill="#0f172a" stroke="#c2410c" stroke-width="1.6" filter="url(#shadow)" />
    <rect width="2240" height="375" rx="18" fill="url(#glowNft)" />

    <!-- Zone Label -->
    <rect x="24" y="16" width="460" height="28" rx="8" fill="#9a3412" />
    <text x="36" y="35" font-size="12.5" font-weight="800" fill="#fed7aa" letter-spacing="1">{s2_title}</text>

    <!-- Sub-Box 2.1: LAN Clients & Ingress -->
    <g transform="translate(25, 56)">
      <rect width="445" height="295" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#f97316">{b21_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b21_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="409" height="90" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="24" font-size="12.5" font-weight="700" fill="#f8fafc">{b21_mode_a_title}</text>
        <text x="16" y="44" font-size="11" fill="#94a3b8">{b21_mode_a_1}</text>
        <text x="16" y="62" font-size="11" fill="#38bdf8">{b21_mode_a_2}</text>
        <text x="16" y="78" font-size="10.5" fill="#64748b">{b21_mode_a_3}</text>
      </g>

      <g transform="translate(18, 168)">
        <rect width="409" height="112" rx="8" fill="#0f172a" stroke="#10b981" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#10b981">{b21_mode_b_title}</text>
        <text x="16" y="43" font-size="11" fill="#cbd5e1">{b21_mode_b_1}</text>
        <text x="16" y="61" font-size="11" fill="#f8fafc">{b21_mode_b_2}</text>
        <text x="16" y="79" font-size="11" fill="#94a3b8">{b21_mode_b_3}</text>
        <text x="16" y="97" font-size="10.5" fill="#10b981">{b21_mode_b_4}</text>
      </g>
    </g>

    <!-- Sub-Box 2.2: DNS & Dynamic nftset -->
    <g transform="translate(490, 56)">
      <rect width="520" height="295" rx="12" fill="#1e293b" stroke="#0d9488" stroke-width="1.4" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#14b8a6">{b22_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b22_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="484" height="124" rx="8" fill="#0f172a" stroke="#0d9488" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#2dd4bf">{b22_how_header}</text>
        <text x="16" y="43" font-size="11" fill="#cbd5e1">{b22_how_1}</text>
        <text x="16" y="61" font-size="10.5" font-family="monospace" fill="#38bdf8">nftset=/youtube.com/4#inet#routeflux#proxy_target_v4</text>
        <text x="16" y="79" font-size="11" fill="#cbd5e1">{b22_how_2}</text>
        <text x="16" y="97" font-size="11" font-weight="bold" fill="#f59e0b">{b22_how_3}</text>
        <text x="16" y="113" font-size="10" fill="#64748b">{b22_how_4}</text>
      </g>

      <g transform="translate(18, 202)">
        <rect width="484" height="77" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="22" font-size="12.5" font-weight="700" fill="#f8fafc">{b22_dns_modes}</text>
        <text x="16" y="42" font-size="11" fill="#94a3b8">{b22_dns_split}</text>
        <text x="16" y="60" font-size="11" fill="#94a3b8">{b22_dns_remote}</text>
      </g>
    </g>

    <!-- Sub-Box 2.3: nftables Kernel Routing -->
    <g transform="translate(1030, 56)">
      <rect width="1185" height="295" rx="12" fill="#1e293b" stroke="#f97316" stroke-width="1.4" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#f97316">{b23_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b23_sub}</text>

      <!-- Sets sub-box -->
      <g transform="translate(18, 66)">
        <rect width="365" height="213" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b23_sets_header}</text>

        <text x="16" y="47" font-size="11" font-family="monospace" fill="#ef4444">@excluded_source_v4</text>
        <text x="16" y="62" font-size="10" fill="#94a3b8">{b23_set_excl}</text>

        <text x="16" y="86" font-size="11" font-family="monospace" fill="#10b981">@local_bypass_v4</text>
        <text x="16" y="101" font-size="10" fill="#94a3b8">{b23_set_priv}</text>

        <text x="16" y="125" font-size="11" font-family="monospace" fill="#10b981">@direct_target_v4</text>
        <text x="16" y="140" font-size="10" fill="#94a3b8">{b23_set_dir}</text>

        <text x="16" y="164" font-size="11" font-family="monospace" fill="#a855f7">@proxy_target_v4</text>
        <text x="16" y="179" font-size="10" fill="#94a3b8">{b23_set_pxy}</text>
      </g>

      <!-- Chains sub-box -->
      <g transform="translate(400, 66)">
        <rect width="767" height="213" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b23_chains_header}</text>

        <!-- Rule 1 -->
        <rect x="16" y="34" width="735" height="34" rx="6" fill="#1e293b" />
        <text x="24" y="55" font-size="10.5" font-family="monospace" fill="#94a3b8">1. ip saddr @excluded_source_v4 <tspan font-weight="bold" fill="#10b981">return</tspan> | ip daddr @local_bypass_v4 <tspan font-weight="bold" fill="#10b981">return</tspan></text>
        <text x="615" y="55" font-size="10" font-weight="bold" fill="#10b981">{b23_rule1_out}</text>

        <!-- Rule 2 -->
        <rect x="16" y="74" width="735" height="34" rx="6" fill="#1e293b" />
        <text x="24" y="95" font-size="10.5" font-family="monospace" fill="#94a3b8">2. ip daddr @direct_target_v4 <tspan font-weight="bold" fill="#10b981">return</tspan> (Keep Direct / Bypass)</text>
        <text x="615" y="95" font-size="10" font-weight="bold" fill="#10b981">{b23_rule2_out}</text>

        <!-- Rule 3 TCP -->
        <rect x="16" y="114" width="735" height="42" rx="6" fill="#1e293b" stroke="#38bdf8" stroke-width="1" />
        <text x="24" y="132" font-size="10.5" font-family="monospace" fill="#38bdf8">3. TCP: ip daddr @proxy_target_v4 tcp redirect to :12345</text>
        <text x="24" y="147" font-size="9.5" fill="#94a3b8">{b23_rule3_desc}</text>
        <text x="595" y="138" font-size="10" font-weight="bold" fill="#38bdf8">{b23_rule3_out}</text>

        <!-- Rule 4 UDP -->
        <rect x="16" y="162" width="735" height="42" rx="6" fill="#1e293b" stroke="#f97316" stroke-width="1" />
        <text x="24" y="180" font-size="10.5" font-family="monospace" fill="#f97316">4. UDP: ip daddr @proxy_target_v4 meta mark set 0x1 tproxy to :12345</text>
        <text x="24" y="195" font-size="9.5" fill="#94a3b8">{b23_rule4_desc}</text>
        <text x="595" y="186" font-size="10" font-weight="bold" fill="#f97316">{b23_rule4_out}</text>
      </g>
    </g>
  </g>

  <!-- Horizontal Arrows Between Sub-boxes in Data Plane -->
  <path d="M 470 680 L 490 680" stroke="#14b8a6" stroke-width="3" marker-end="url(#arrowTeal)" fill="none" />
  <rect x="462" y="652" width="56" height="20" rx="4" fill="#0f172a" stroke="#14b8a6" stroke-width="1" />
  <text x="490" y="666" font-size="9.5" font-weight="bold" fill="#14b8a6" text-anchor="middle">DNS 53</text>

  <path d="M 1010 680 L 1030 680" stroke="#f97316" stroke-width="3" marker-end="url(#arrowOrange)" fill="none" />
  <rect x="998" y="652" width="64" height="20" rx="4" fill="#0f172a" stroke="#f97316" stroke-width="1" />
  <text x="1030" y="666" font-size="9.5" font-weight="bold" fill="#f97316" text-anchor="middle">nftset IP</text>

  <!-- Downward Arrow from nftables to Xray Inbound -->
  <path d="M 1480 870 L 1480 945" stroke="#c084fc" stroke-width="3.5" marker-end="url(#arrowPurple)" fill="none" />
  <rect x="1350" y="895" width="260" height="26" rx="6" fill="#0f172a" stroke="#c084fc" stroke-width="1" />
  <text x="1480" y="912" font-size="11" font-weight="bold" fill="#c084fc" text-anchor="middle">{conn_tcp_tproxy}</text>

  <!-- Downward Arrow from LAN proxy direct to Xray Inbound -->
  <path d="M 230 870 L 230 945" stroke="#10b981" stroke-width="3.5" marker-end="url(#arrowGreen)" fill="none" />
  <rect x="95" y="895" width="270" height="26" rx="6" fill="#0f172a" stroke="#10b981" stroke-width="1" />
  <text x="230" y="912" font-size="11" font-weight="bold" fill="#10b981" text-anchor="middle">{conn_lan_proxy}</text>


  <!-- ==================== SECTION 3: XRAY RUNTIME & INTERNET (BOTTOM) ==================== -->
  <g transform="translate(80, 950)">
    <!-- Zone Container -->
    <rect width="2240" height="340" rx="18" fill="#0f172a" stroke="#6b21a8" stroke-width="1.6" filter="url(#shadow)" />
    <rect width="2240" height="340" rx="18" fill="url(#glowXray)" />

    <!-- Zone Label -->
    <rect x="24" y="16" width="450" height="28" rx="8" fill="#581c87" />
    <text x="36" y="35" font-size="12.5" font-weight="800" fill="#e9d5ff" letter-spacing="1">{s3_title}</text>

    <!-- Sub-Box 3.1: Inbounds -->
    <g transform="translate(25, 56)">
      <rect width="455" height="258" rx="12" fill="#1e293b" stroke="#a855f7" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#c084fc">{b31_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b31_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="419" height="80" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">transparent-in (dokodemo-door :12345):</text>
        <text x="16" y="42" font-size="10.5" fill="#94a3b8">{b31_tp_1}</text>
        <text x="16" y="58" font-size="10.5" fill="#94a3b8">{b31_tp_2}</text>
        <text x="16" y="74" font-size="10.5" fill="#10b981">{b31_tp_3}</text>
      </g>

      <g transform="translate(18, 154)">
        <rect width="419" height="80" rx="8" fill="#0f172a" stroke="#10b981" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#10b981">socks-in (:10808) &amp; http-in (:10809):</text>
        <text x="16" y="42" font-size="10.5" fill="#94a3b8">{b31_socks_1}</text>
        <text x="16" y="58" font-size="10.5" fill="#94a3b8">{b31_socks_2}</text>
        <text x="16" y="74" font-size="10" fill="#38bdf8">{b31_socks_3}</text>
      </g>
    </g>

    <!-- Sub-Box 3.2: Internal Xray Router & DNS -->
    <g transform="translate(500, 56)">
      <rect width="480" height="258" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#c084fc">{b32_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b32_sub}</text>

      <g transform="translate(18, 66)">
        <rect width="444" height="168" rx="8" fill="#0f172a" stroke="#334155" stroke-width="1" />
        <text x="16" y="23" font-size="12.5" font-weight="700" fill="#f8fafc">{b32_table_header}</text>
        <text x="16" y="44" font-size="11" fill="#94a3b8">{b32_rule_sel}</text>
        <text x="16" y="64" font-size="11" fill="#94a3b8">{b32_rule_dir}</text>
        <text x="16" y="84" font-size="11" fill="#94a3b8">{b32_rule_zap}</text>
        <text x="16" y="104" font-size="11" fill="#94a3b8">{b32_rule_blk}</text>
        <text x="16" y="128" font-size="11" fill="#38bdf8">{b32_doh_1}</text>
        <text x="16" y="146" font-size="10.5" fill="#64748b">{b32_doh_2}</text>
      </g>
    </g>

    <!-- Sub-Box 3.3: Outbounds & Egress Targets -->
    <g transform="translate(1000, 56)">
      <rect width="1215" height="258" rx="12" fill="#1e293b" stroke="#334155" stroke-width="1.2" />
      <text x="24" y="32" font-size="15.5" font-weight="700" fill="#c084fc">{b33_title}</text>
      <text x="24" y="50" font-size="11.5" fill="#94a3b8">{b33_sub}</text>

      <!-- Channel 1: Proxy Outbound -->
      <g transform="translate(18, 66)">
        <rect width="375" height="168" rx="10" fill="#0f172a" stroke="#a855f7" stroke-width="1.4" />
        <rect x="12" y="12" width="28" height="28" rx="6" fill="#7c3aed" />
        <text x="26" y="31" font-size="14" font-weight="bold" fill="#ffffff" text-anchor="middle">🔒</text>
        <text x="48" y="26" font-size="13" font-weight="700" fill="#f8fafc">{b33_c1_title}</text>
        <text x="48" y="41" font-size="10" fill="#a855f7">{b33_c1_sub}</text>
        <text x="16" y="70" font-size="11" fill="#cbd5e1">{b33_c1_1}</text>
        <text x="16" y="90" font-size="11" fill="#94a3b8">{b33_c1_2}</text>
        <text x="16" y="108" font-size="11" fill="#94a3b8">{b33_c1_3}</text>
        <text x="16" y="128" font-size="10.5" fill="#64748b">{b33_c1_4}</text>
        <text x="16" y="148" font-size="10.5" font-weight="bold" fill="#a855f7">{b33_c1_5}</text>
      </g>

      <!-- Channel 2: Direct WAN -->
      <g transform="translate(415, 66)">
        <rect width="375" height="168" rx="10" fill="#0f172a" stroke="#10b981" stroke-width="1.4" />
        <rect x="12" y="12" width="28" height="28" rx="6" fill="#059669" />
        <text x="26" y="31" font-size="14" font-weight="bold" fill="#ffffff" text-anchor="middle">🌐</text>
        <text x="48" y="26" font-size="13" font-weight="700" fill="#f8fafc">{b33_c2_title}</text>
        <text x="48" y="41" font-size="10" fill="#10b981">{b33_c2_sub}</text>
        <text x="16" y="70" font-size="11" fill="#cbd5e1">{b33_c2_1}</text>
        <text x="16" y="90" font-size="11" fill="#94a3b8">{b33_c2_2}</text>
        <text x="16" y="108" font-size="11" fill="#94a3b8">{b33_c2_3}</text>
        <text x="16" y="128" font-size="10.5" fill="#64748b">{b33_c2_4}</text>
        <text x="16" y="148" font-size="10.5" font-weight="bold" fill="#10b981">{b33_c2_5}</text>
      </g>

      <!-- Channel 3: Zapret Fallback -->
      <g transform="translate(812, 66)">
        <rect width="385" height="168" rx="10" fill="#0f172a" stroke="#f59e0b" stroke-width="1.4" />
        <rect x="12" y="12" width="28" height="28" rx="6" fill="#d97706" />
        <text x="26" y="31" font-size="14" font-weight="bold" fill="#ffffff" text-anchor="middle">⚡</text>
        <text x="48" y="26" font-size="13" font-weight="700" fill="#f8fafc">{b33_c3_title}</text>
        <text x="48" y="41" font-size="10" fill="#f59e0b">{b33_c3_sub}</text>
        <text x="16" y="70" font-size="11" fill="#cbd5e1">{b33_c3_1}</text>
        <text x="16" y="90" font-size="11" fill="#94a3b8">{b33_c3_2}</text>
        <text x="16" y="108" font-size="11" fill="#94a3b8">{b33_c3_3}</text>
        <text x="16" y="128" font-size="10.5" fill="#64748b">{b33_c3_4}</text>
        <text x="16" y="148" font-size="10.5" font-weight="bold" fill="#f59e0b">{b33_c3_5}</text>
      </g>
    </g>
  </g>

  <!-- Horizontal Arrows Inbound to Router to Outbounds -->
  <path d="M 480 1090 L 500 1090" stroke="#c084fc" stroke-width="3" marker-end="url(#arrowPurple)" fill="none" />
  <path d="M 980 1090 L 1000 1090" stroke="#c084fc" stroke-width="3" marker-end="url(#arrowPurple)" fill="none" />

  <!-- ==================== FOOTER / STEP-BY-STEP FLOW ==================== -->
  <g transform="translate(80, 1315)">
    <rect width="2240" height="155" rx="14" fill="#111827" stroke="#1f2937" stroke-width="1.2" />
    <text x="24" y="28" font-size="13.5" font-weight="800" fill="#94a3b8" letter-spacing="0.8">{f_title}</text>

    <!-- Step 1 -->
    <g transform="translate(24, 46)">
      <circle cx="16" cy="16" r="16" fill="#1e3a8a" />
      <text x="16" y="21.5" font-size="13" font-weight="bold" fill="#38bdf8" text-anchor="middle">1</text>
      <text x="44" y="16" font-size="13" font-weight="700" fill="#f8fafc">{f_s1_title}</text>
      <text x="44" y="34" font-size="11" fill="#94a3b8">{f_s1_1}</text>
      <text x="44" y="50" font-size="11" fill="#94a3b8">{f_s1_2}</text>
      <text x="44" y="66" font-size="11" fill="#94a3b8">{f_s1_3}</text>
    </g>

    <!-- Step 2 -->
    <g transform="translate(470, 46)">
      <circle cx="16" cy="16" r="16" fill="#7c2d12" />
      <text x="16" y="21.5" font-size="13" font-weight="bold" fill="#f97316" text-anchor="middle">2</text>
      <text x="44" y="16" font-size="13" font-weight="700" fill="#f8fafc">{f_s2_title}</text>
      <text x="44" y="34" font-size="11" fill="#94a3b8">{f_s2_1}</text>
      <text x="44" y="50" font-size="11" fill="#94a3b8">{f_s2_2}</text>
      <text x="44" y="66" font-size="11" fill="#94a3b8">{f_s2_3}</text>
    </g>

    <!-- Step 3 -->
    <g transform="translate(920, 46)">
      <circle cx="16" cy="16" r="16" fill="#0f766e" />
      <text x="16" y="21.5" font-size="13" font-weight="bold" fill="#14b8a6" text-anchor="middle">3</text>
      <text x="44" y="16" font-size="13" font-weight="700" fill="#f8fafc">{f_s3_title}</text>
      <text x="44" y="34" font-size="11" fill="#94a3b8">{f_s3_1}</text>
      <text x="44" y="50" font-size="11" fill="#94a3b8">{f_s3_2}</text>
      <text x="44" y="66" font-size="11" fill="#94a3b8">{f_s3_3}</text>
    </g>

    <!-- Step 4 -->
    <g transform="translate(1370, 46)">
      <circle cx="16" cy="16" r="16" fill="#581c87" />
      <text x="16" y="21.5" font-size="13" font-weight="bold" fill="#a855f7" text-anchor="middle">4</text>
      <text x="44" y="16" font-size="13" font-weight="700" fill="#f8fafc">{f_s4_title}</text>
      <text x="44" y="34" font-size="11" fill="#94a3b8">{f_s4_1}</text>
      <text x="44" y="50" font-size="11" fill="#94a3b8">{f_s4_2}</text>
      <text x="44" y="66" font-size="11" fill="#94a3b8">{f_s4_3}</text>
    </g>

    <!-- Step 5 -->
    <g transform="translate(1815, 46)">
      <circle cx="16" cy="16" r="16" fill="#065f46" />
      <text x="16" y="21.5" font-size="13" font-weight="bold" fill="#10b981" text-anchor="middle">5</text>
      <text x="44" y="16" font-size="13" font-weight="700" fill="#f8fafc">{f_s5_title}</text>
      <text x="44" y="34" font-size="11" fill="#94a3b8">{f_s5_1}</text>
      <text x="44" y="50" font-size="11" fill="#94a3b8">{f_s5_2}</text>
      <text x="44" y="66" font-size="11" fill="#94a3b8">{f_s5_3}</text>
    </g>
  </g>

</svg>
"""
    # Verify XML well-formedness before returning
    ET.fromstring(raw_svg)
    return raw_svg


def render_svg_to_png(svg_string, out_path, scale=2.0):
    pixel_w = int(WIDTH * scale)
    pixel_h = int(HEIGHT * scale)

    # Ensure root svg attributes match target pixel canvas size
    svg_string = re.sub(
        r'(<svg\b[^>]*?\bwidth=)"[^"]*"',
        rf'\g<1>"{pixel_w}"',
        svg_string,
        count=1
    )
    svg_string = re.sub(
        r'(<svg\b[^>]*?\bheight=)"[^"]*"',
        rf'\g<1>"{pixel_h}"',
        svg_string,
        count=1
    )

    ffi = cffi.FFI()
    ffi.cdef('''
    void* objc_getClass(const char *name);
    void* sel_registerName(const char *name);
    void* objc_msgSend(void *self, void *op, ...);
    typedef double CGFloat;
    typedef struct CGSize { CGFloat width; CGFloat height; } CGSize;
    typedef struct CGPoint { CGFloat x; CGFloat y; } CGPoint;
    typedef struct CGRect { CGPoint origin; CGSize size; } CGRect;
    ''')

    libobjc = ffi.dlopen('/usr/lib/libobjc.A.dylib')
    appkit = ffi.dlopen('/System/Library/Frameworks/AppKit.framework/AppKit')

    NSData = libobjc.objc_getClass(b'NSData')
    sel_dataWithBytes = libobjc.sel_registerName(b'dataWithBytes:length:')
    msg_data = ffi.cast('void* (*)(void*, void*, const char*, size_t)', libobjc.objc_msgSend)

    svg_bytes = svg_string.encode('utf-8')
    ns_data = msg_data(NSData, sel_dataWithBytes, svg_bytes, len(svg_bytes))

    NSImage = libobjc.objc_getClass(b'NSImage')
    sel_alloc = libobjc.sel_registerName(b'alloc')
    sel_initWithData = libobjc.sel_registerName(b'initWithData:')
    msg_alloc = ffi.cast('void* (*)(void*, void*)', libobjc.objc_msgSend)
    msg_init = ffi.cast('void* (*)(void*, void*, void*)', libobjc.objc_msgSend)
    img = msg_init(msg_alloc(NSImage, sel_alloc), sel_initWithData, ns_data)

    sel_setSize = libobjc.sel_registerName(b'setSize:')
    msg_setSize = ffi.cast('void (*)(void*, void*, CGSize)', libobjc.objc_msgSend)
    cg_size = ffi.new('CGSize*', {'width': pixel_w, 'height': pixel_h})[0]
    msg_setSize(img, sel_setSize, cg_size)

    NSBitmapImageRep = libobjc.objc_getClass(b'NSBitmapImageRep')
    sel_initBitmap = libobjc.sel_registerName(b'initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bytesPerRow:bitsPerPixel:')
    msg_initBitmap = ffi.cast('void* (*)(void*, void*, void*, long, long, long, long, bool, bool, void*, long, long)', libobjc.objc_msgSend)

    NSString = libobjc.objc_getClass(b'NSString')
    sel_stringWithUTF8 = libobjc.sel_registerName(b'stringWithUTF8String:')
    msg_str = ffi.cast('void* (*)(void*, void*, const char*)', libobjc.objc_msgSend)
    cs_name = msg_str(NSString, sel_stringWithUTF8, b'NSCalibratedRGBColorSpace')

    rep = msg_initBitmap(msg_alloc(NSBitmapImageRep, sel_alloc), sel_initBitmap,
                         ffi.NULL, pixel_w, pixel_h, 8, 4, True, False, cs_name, 0, 32)

    NSGraphicsContext = libobjc.objc_getClass(b'NSGraphicsContext')
    sel_graphicsContextWithBitmap = libobjc.sel_registerName(b'graphicsContextWithBitmapImageRep:')
    msg_gc = ffi.cast('void* (*)(void*, void*, void*)', libobjc.objc_msgSend)
    gctx = msg_gc(NSGraphicsContext, sel_graphicsContextWithBitmap, rep)

    sel_saveGraphicsState = libobjc.sel_registerName(b'saveGraphicsState')
    sel_restoreGraphicsState = libobjc.sel_registerName(b'restoreGraphicsState')
    sel_setCurrentContext = libobjc.sel_registerName(b'setCurrentContext:')
    msg_void1 = ffi.cast('void (*)(void*, void*)', libobjc.objc_msgSend)
    msg_void2 = ffi.cast('void (*)(void*, void*, void*)', libobjc.objc_msgSend)

    msg_void1(NSGraphicsContext, sel_saveGraphicsState)
    msg_void2(NSGraphicsContext, sel_setCurrentContext, gctx)

    sel_drawInRect = libobjc.sel_registerName(b'drawInRect:fromRect:operation:fraction:')
    msg_draw = ffi.cast('void (*)(void*, void*, CGRect, CGRect, unsigned long, CGFloat)', libobjc.objc_msgSend)
    rect = ffi.new('CGRect*', {'origin': {'x': 0, 'y': 0}, 'size': {'width': pixel_w, 'height': pixel_h}})[0]
    zero_rect = ffi.new('CGRect*', {'origin': {'x': 0, 'y': 0}, 'size': {'width': 0, 'height': 0}})[0]
    msg_draw(img, sel_drawInRect, rect, zero_rect, 2, 1.0)

    msg_void1(NSGraphicsContext, sel_restoreGraphicsState)

    sel_repUsingType = libobjc.sel_registerName(b'representationUsingType:properties:')
    msg_png = ffi.cast('void* (*)(void*, void*, unsigned long, void*)', libobjc.objc_msgSend)
    png_data = msg_png(rep, sel_repUsingType, 4, ffi.NULL)

    sel_bytes = libobjc.sel_registerName(b'bytes')
    sel_len = libobjc.sel_registerName(b'length')
    msg_bytes = ffi.cast('const char* (*)(void*, void*)', libobjc.objc_msgSend)
    msg_len = ffi.cast('size_t (*)(void*, void*)', libobjc.objc_msgSend)
    data_bytes = ffi.buffer(msg_bytes(png_data, sel_bytes), msg_len(png_data, sel_len))[:]

    os.makedirs(os.path.dirname(os.path.abspath(out_path)), exist_ok=True)
    with open(out_path, 'wb') as f:
        f.write(data_bytes)
    print(f"Rendered PNG ({pixel_w}x{pixel_h}) to {out_path} ({len(data_bytes)} bytes)")


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Generate RouteFlux architecture diagrams in high resolution")
    parser.add_argument("args", nargs="*", help="Optional language ('en'/'ru') and output paths")
    parser.add_argument("--lang", choices=["en", "ru", "both"], default=None, help="Language to generate")
    parser.add_argument("--scale", type=float, default=2.0, help="Resolution scale factor (default: 2.0 -> 4800x3040)")
    parser.add_argument("--no-svg", action="store_true", help="Skip saving SVG files")

    cli_args = parser.parse_args()
    scale = cli_args.scale
    save_svg = not cli_args.no_svg
    pos = cli_args.args

    if cli_args.lang:
        langs = [cli_args.lang] if cli_args.lang in ("en", "ru") else ["en", "ru"]
        for l in langs:
            sfx = "-ru" if l == "ru" else ""
            svg_data = build_svg(l)
            if save_svg:
                svg_p = f"docs/images/architecture-diagram{sfx}.svg"
                with open(svg_p, "w", encoding="utf-8") as f:
                    f.write(svg_data)
                print(f"Saved SVG to {svg_p}")
            render_svg_to_png(svg_data, f"docs/images/architecture-diagram{sfx}.png", scale=scale)
    elif len(pos) >= 2 and pos[0] in ("en", "ru"):
        lang = pos[0]
        for path in pos[1:]:
            render_svg_to_png(build_svg(lang), path, scale=scale)
    elif len(pos) == 1 and pos[0] in ("en", "ru"):
        lang = pos[0]
        sfx = "-ru" if lang == "ru" else ""
        svg_data = build_svg(lang)
        if save_svg:
            svg_p = f"docs/images/architecture-diagram{sfx}.svg"
            with open(svg_p, "w", encoding="utf-8") as f:
                f.write(svg_data)
            print(f"Saved SVG to {svg_p}")
        render_svg_to_png(svg_data, f"docs/images/architecture-diagram{sfx}.png", scale=scale)
    elif pos:
        for path in pos:
            lang = "ru" if "-ru" in path or "_ru" in path else "en"
            render_svg_to_png(build_svg(lang), path, scale=scale)
    else:
        for l in ("en", "ru"):
            sfx = "-ru" if l == "ru" else ""
            svg_data = build_svg(l)
            if save_svg:
                svg_p = f"docs/images/architecture-diagram{sfx}.svg"
                with open(svg_p, "w", encoding="utf-8") as f:
                    f.write(svg_data)
                print(f"Saved SVG to {svg_p}")
            png_p = f"docs/images/architecture-diagram{sfx}.png"
            render_svg_to_png(svg_data, png_p, scale=scale)
