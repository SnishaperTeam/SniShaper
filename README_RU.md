# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/Лицензия-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/Документация-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Релиз)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Загрузки)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Последний%20коммит)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** — это локальный прокси-инструмент, разработанный специально для сложных сетевых условий, интегрирующий **инъекцию ECH**, **фрагментацию TLS**, **маскировку QUIC**, **миграцию сессий** и другие технологии стека протоколов, в сочетании с **виртуальным TUN-интерфейсом** для полного перехвата трафика, обеспечивая стабильный и гибкий доступ в интернет.

Это **кроссплатформенный (Windows и Linux) репозиторий**. Обе платформы используют общую кодовую базу и механизм версионирования; платформозависимая логика изолируется с помощью Go build tags.

> Нужна консольная версия без графического интерфейса? В этом репозитории встроен **SniShaper CLI** (каталог `cli/`) — кроссплатформенная (Windows / Linux / macOS) headless-версия со встроенным TUI-интерфейсом с разделённым экраном (логи в реальном времени + командная панель), сохраняющая все основные прокси-возможности и использующая общий с GUI источник версии `Package.appxmanifest`.

> Нужен клиент для Android? См. **Lumine for Android** (<https://github.com/SniShaper/lumine-for-android>) — мобильная версия с теми же идеями маршрутизации: нативный интерфейс Kotlin + Jetpack Compose (Material Design 3), ядро Go (enimul) подключается через gomobile единым AAR без встроенного WebView; поддержка подписок, редактирования правил, логов в реальном времени и фонового удержания; также доступно на F-Droid (`com.moi.lumine`).

> Нужен клиент для HarmonyOS? См. **Lumine for HarmonyOS** (<https://github.com/SnishaperTeam/lumine-for-harmonyos>) — версия для HarmonyOS с той же концепцией: нативный интерфейс ArkTS + ArkUI (светлая и тёмная темы), портативное ядро C++17 подключается через NAPI как единый `liblumine_napi.so` без встроенного WebView; поддержка VpnExtensionAbility (TUN) туннелирования, локального прокси-прослушивания (SOCKS5 / HTTP loopback inbound), управления подписками, редактирования правил, логов в реальном времени и уведомлений о состоянии работы.

---

## Возможности

- **Многорежимное прокси**: MITM, Transparent, TLS-RF (фрагментация TLS), QUIC, Migration (перенос сессий), Direct — для различных сценариев.
- **TUN виртуальный сетевой адаптер**: WinTun в Windows и сетевой стек gvisor в Linux для прозрачного глобального перехвата трафика, авто-маршрутизации и перехвата DNS.
- **Инъекция ECH**: автоматическое получение и внедрение ECH Config с DoH-обнаружением и горячей заменой.
- **Интеллектуальная маршрутизация**: автоматическое определение заблокированных доменов на основе GFWList без ручной настройки.
- **Шифрованный DNS**: встроенный защищённый DNS-резолвер с балансировкой узлов.
- **Cloudflare IP пул**: автоматическое измерение скорости, проверка работоспособности и обновление.
- **NAT64 поддержка**: гибкий IP-выход и доступ к сервисам.
- **Режим эволюции (Evolution)**: автоматическое тестирование комбинаций правил для поиска оптимального способа доступа к целевому сайту с применением в один клик.

---

## Быстрый старт

### Windows

Скачайте `snishaper-windows-amd64.7z` (портативная версия) или MSIX-установщик из [последнего релиза](https://github.com/SnishaperTeam/SniShaper/releases), распакуйте / установите и запустите `snishaper.exe`. Приложение автоматически запрашивает права администратора (требуются для TUN). Если повышение прав не удалось, TUN недоступен, но остальные функции работают.

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/ru-ru%20dark.svg" width="200"/>
</a>

### Linux

Скачайте `snishaper-linux-amd64.tar.gz` из [последнего релиза](https://github.com/SnishaperTeam/SniShaper/releases), распакуйте и запустите:

```bash
tar -xzf snishaper-linux-amd64.tar.gz
sudo ./SniShaper
```

Приложение автоматически запрашивает права root (требуются для TUN). Если повышение прав не удалось, TUN недоступен, но остальные функции (прокси и т.д.) работают. Текущая сборка предназначена для **amd64** и основана на **GTK4 + WebKitGTK 6.0** (также поддерживается GTK3).

### CLI версия (Headless)

Не нужен графический интерфейс или работаете на сервере / по SSH? В этом репозитории встроен **SniShaper CLI** (каталог `cli/`):

- **Три платформы**: Windows / Linux / macOS (amd64 + arm64).
- **TUI интерфейс**: верхняя панель — прокси-логи в реальном времени, нижняя — ввод команд (поддержка русских псевдонимов); логи никогда не перекрывают ввод.
- **Режим демона**: `snishaper start` работает постоянно; подкоманды `status` / `stop` / `logs` / `proxy` / `sysproxy` / `tun` / `config` / `ca` для удалённого управления.
- **Полное ядро**: разделяет с GUI тот же прокси-движок (инъекция ECH, фрагментация TLS, QUIC, TUN/gvisor, маршрутизация GFWList, DoH, CF IP пул, NAT64, режим эволюции).
- **Без автообновления**: подходит для длительно работающих серверов.
- **Согласованность версий**: использует `Package.appxmanifest` как единый источник версий вместе с GUI.

Инструкции по сборке см. в **[build_RU.md — Матрица артефактов](build_RU.md#матрица-артефактов-12-целей)**. Артефакты организованы в `build/bin/cli/<Platform>/<Arch>/` — просто запустите бинарник для входа в TUI.

> **Примечание для CLI Darwin / macOS:** В настоящее время у Darwin CLI нет отдельного устройства macOS для постоянного тестирования в реальных условиях, поэтому при использовании могут возникать непредвиденные или пока неизвестные проблемы. Если вы обнаружите какие-либо проблемы на Darwin / macOS, пожалуйста, своевременно создайте [Issue](https://github.com/SnishaperTeam/SniShaper/issues) или [Pull Request](https://github.com/SnishaperTeam/SniShaper/pulls), чтобы мы могли быстрее их изучить и исправить.

### Переустановка сертификата

В главном интерфейсе нажмите **Управление сертификатами → Сбросить корневой сертификат**. Для CLI-версии используйте `snishaper ca regenerate`, затем `ca install`.

### Настройка и запуск

Программа поставляется с богатым набором встроенных правил. Вы также можете настроить собственные правила на панели правил и нажать **Запустить прокси**.

---

## Документация

Для получения подробных технических принципов, руководств по развертыванию и настройке, обратитесь к [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki):

- **[Основные режимы прокси](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**: понимание принципов работы TLS-RF, QUIC и серверного режима.
- **[Руководство по правилам](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**: как разрабатывать целевые правила.
- **[Настройка GUI](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**: быстрая настройка правил в интерфейсе.
- **[Устранение неполадок](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**: решение проблем с сертификатами, правилами и другим.

---

## Сборка и разработка

Проект построен с использованием **Wails v3 + React 19 + MUI** с бэкендом на **Go**, поддерживая GUI для Windows / Linux и кроссплатформенный CLI — всего 12 целей сборки. Полное руководство по сборке выделено в **[build_RU.md](build_RU.md)**, включающее:

- **[Матрица артефактов](build_RU.md#матрица-артефактов-12-целей)**: 12 целей с платформами / архитектурами / путями артефактов, все параметры и примеры скриптов сборки.
- **[Сборка Windows](build_RU.md#сборка-windows)**: использование `build_windows.ps1`, упаковка MSIX и особенности поведения.
- **[Сборка Linux](build_RU.md#сборка-linux)**: установка зависимостей GTK4 / GTK3 и команды `build.sh`.
- **[Версия и канал выпуска](build_RU.md#версия-и-канал-выпуска)** / **[Окружение разработки](build_RU.md#окружение-разработки)**: источник версии и требования к инструментам.
- **[Непрерывная интеграция](build_RU.md#непрерывная-интеграция)**: кроссплатформенные CI и релизные пайплайны.
- **[Примечания по кроссплатформенности](build_RU.md#примечания-по-кроссплатформенности)**: Go build tags и подкаталог CLI.

---

## Инструменты

### IP Сканер (tools/scanner.py)

Универсальный инструмент сканирования IP для поиска доступных прокси-IP целевого домена.

**Использование:**

```bash
python tools/scanner.py <домен:порт> <CIDR> [макс_потоков]
```

**Параметры:**

| Параметр | Описание | Пример |
|----------|----------|--------|
| домен:порт | Цель сканирования | `open.spotify.com:443` |
| CIDR | Диапазон IP для сканирования | `35.186.224.0/24` |
| макс_потоков | Количество потоков (по умолчанию 64) | `128` |

**Примеры:**

```bash
# Сканирование доступных IP Spotify
python tools/scanner.py open.spotify.com:443 35.186.224.0/24 128

# Сканирование доступных IP Google
python tools/scanner.py google.com:443 34.0.0.0/8 256

# Сканирование любого домена
python tools/scanner.py example.com:80 1.0.0.0/16 64
```

**Результат:**

- Результаты отсортированы по скорости отклика (от быстрых к медленным)
- Логи сохраняются в каталоге `tools/logs/`
- Валидные IP сохраняются как `tools/logs/scan_*_valid.txt`

---

## Благодарности

Проект вдохновлен следующими отличными open-source проектами:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## Активность проекта и участники

### Значки активности

[![GitHub contributors](https://img.shields.io/github/contributors/SnishaperTeam/SniShaper?style=flat&label=Всего участников)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SnishaperTeam/SniShaper?style=flat&label=Коммитов в месяц)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&label=Последний коммит)](https://github.com/SnishaperTeam/SniShaper/commits/main)

### Тренд активности

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### Граф участников

<div align="center">
<a href="https://github.com/SnishaperTeam/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SnishaperTeam/SniShaper" alt="Contributors" />
</a>
</div>

---

## Лицензия

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
