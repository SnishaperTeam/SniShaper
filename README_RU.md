# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/Лицензия-MIT-blue?style=flat&logo=open-source-initiative)]() [![Wiki](https://img.shields.io/badge/Документация-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Релиз)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Загрузки)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Последний%20коммит)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** -- это локальный прокси-инструмент, разработанный специально для сложных сетевых условий, интегрирующий **инъекцию ECH**, **фрагментацию TLS**, **маскировку QUIC**, **миграцию сессий** и другие технологии стека протоколов, в сочетании с **виртуальным TUN-интерфейсом** для полного перехвата трафика, обеспечивая стабильный и гибкий доступ в интернет.

Это **кроссплатформенный (Windows и Linux) репозиторий**. Обе платформы используют общую кодовую базу и механизм версионирования; платформозависимая логика изолируется с помощью Go build tags.

---

## Возможности

- **Многорежимное прокси**: MITM, Transparent, TLS-RF (фрагментация TLS), QUIC, Migration (перенос сессий), Direct -- для различных сценариев.
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

### Переустановка сертификата

В главном интерфейсе нажмите **Управление сертификатами -> Сбросить корневой сертификат**.

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

Проект построен с использованием **Wails v3 + React 19 + MUI** с бэкендом на **Go**. Скрипты `build.sh` (Linux / macOS / WSL) и `build_windows.ps1` (Windows) используют общую матрицу целей и структуру выходных каталогов.

### Матрица артефактов (12 целей)

| Тип | Платформа | Архитектура | Артефакт |
| --- | --- | --- | --- |
| CLI | Windows | `x64` / `x86` / `arm64` | `build/bin/cli/Windows/<arch>/snishaper.exe` |
| CLI | Linux | `x64` / `arm64` | `build/bin/cli/Linux/<arch>/snishaper` |
| CLI | Darwin | `x64` / `arm64` | `build/bin/cli/Darwin/<arch>/snishaper` |
| GUI | Windows | `x64` / `x86` / `arm64` | `build/bin/gui/Windows/<arch>/snishaper.exe` |
| GUI | Linux | `x64` / `arm64` | `build/bin/gui/Linux/<arch>/SniShaper` |

7 CLI + 5 GUI = **12 целей**; каждая папка цели также содержит seed-каталоги `config/` и `rules/`. GUI не собирается для Darwin, а `x86` существует только для Windows. GUI требует GTK/WebKit (Linux) и сборку фронтенда, поэтому собирается только на нативном хосте той же ОС; CLI — чистый Go (`CGO_ENABLED=0`), все семь целей можно собрать на одном раннере без кросс-тулчейна.

Флаги (`build.sh` / `build_windows.ps1`): `--platform` / `-Platform`, `--arch` / `-Arch`, `--type` / `-Type` (повторяемые; в PowerShell используется форма со списком через запятую, например `-Type cli,gui`), `--all` / `-All`, `--dry-run` / `-DryRun`, `--ci` / `-CI` (без запросов, никогда не экспортирует кросс `CC`/`CXX`), `--cross` / `-Cross` (только локально), `--install-deps` / `-InstallDeps`, `--gtk3` / `-Gtk3`, `--wails` / `-Wails`, `--silent` / `-Silent`, `--help` / `-Help`.

```bash
# Linux / macOS / WSL
./build.sh --all                                            # все 12 целей
./build.sh --type cli --all                                 # все 7 CLI-целей
./build.sh --ci --platform linux --arch arm64 --type cli --type gui
./build.sh --dry-run --all                                  # только план
```

```powershell
# Windows
.\build_windows.ps1 -All
.\build_windows.ps1 -Type cli -All
.\build_windows.ps1 -CI -Platform windows -Arch arm64 -Type cli,gui
.\build_windows.ps1 -Platform windows -Arch x64 -Type gui -InstallDeps -BuildMsix
```

**CI: ARM64 собирается только нативно.** Обычный CI (build.yml, push/PR) только собирает и прогоняет smoke-тесты — без упаковки и выгрузки артефактов. Матрица GUI запускает по одной паре (платформа, архитектура) на задание, поэтому ARM64 GUI компилируется только на нативных ARM-раннерах: `linux/arm64` — `ubuntu-24.04-arm`, `linux/x64` — `ubuntu-latest`, `windows/x64|x86|arm64` — `windows-latest` (Go формирует `windows/arm64` с `CGO_ENABLED=0`, без кросс-тулчейна). Все семь CLI-целей — чистый Go (`CGO_ENABLED=0`) и собираются один раз заданием `cli-build`, поэтому дублирования нет. Упаковка (MSIX / 7z / tar.gz) выполняется только в release-пайплайне (по тегу или вручную). В режиме `--ci` скрипты не экспортируют `CC`/`CXX`, поэтому в журналах ARM64 не может появиться `aarch64-linux-gnu-gcc` или `osxcross`; задание CI дополнительно проверяет журнал и падает при обнаружении. Версионный ресурс Windows GUI перегенерируется go-winres только для amd64 (release/MSIX); цели arm64/x86 используют закоммиченный архитектурно-нейтральный `snishaper.syso` (go-winres на amd64-хосте не умеет выдавать arm64-объекты).

Запуск без флагов выбора открывает интерактивный мастер (язык, тип, платформа, архитектура, опциональный кросс-тулчейн, подтверждение плана). Проверка артефактов: `file build/bin/gui/Linux/arm64/SniShaper` (ELF aarch64) либо на Windows `dumpbin /headers ...` / поле machine в PE-заголовке.

### Сборка Windows

```powershell
# Клонировать репозиторий
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Полная компиляция (интерактивный режим, автоустановка зависимостей, опционально MSIX)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# Или с PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1
```

#### Параметры командной строки скрипта сборки

`build_windows.ps1` поддерживает следующие параметры для пропуска интерактивных запросов:

| Параметр | Значения | Описание |
| ------------ | -------------------------------- | ------------------------------------------------------------------ |
| `-Build` | `<система> <режим> <объём>` | Трёхкомпонентная спецификация. **Система**: `windows` / `linux` / `all`; **Режим**: `gui` / `cli` / `all`; **Объём**: `frontend` / `backend` / `all`. Если не указано — интерактивное меню; старый формат `-Build frontend/backend/all` по-прежнему работает |
| `-Lang` | `en` / `cn` / `ru` | Язык подсказок, по умолчанию английский |
| `-Arch` | `x64` / `arm64` / `x86` | Целевая архитектура (принимает псевдонимы amd64/386), по умолчанию хост. `x86` собирает только Windows — Linux (вкл. CLI) и Darwin пропускаются |
| `-InstallDeps` | без значений (флаг) | Выполнить `npm install` перед сборкой фронтенда |
| `-BuildMsix` | без значений (флаг) | Собрать MSIX-пакет после компиляции (требуется WinApp CLI) |
| `-SkipSign` | без значений (флаг) | Пропустить подпись MSIX, выходной файл получит префикс `unsigned_` (требуется `-BuildMsix`) |
| `-Cli` | без значений (флаг) | Дополнительно собрать headless CLI (целевые платформы определяются `-Arch`) в `build/bin/cli/<Platform>/<Arch>/` |
| `-Gtk3` | без значений (флаг) | Использовать GTK3 + webkit2gtk-4.1 для сборки Linux (WSL) (аналог `build.sh --gtk3`) |
| `-Silent` | без значений (флаг) | Тихий режим без интерактивных запросов; по умолчанию `-Build windows gui all` и `-Lang en` |

**Особенности поведения:**

- **Автоповышение прав**: Скрипту требуются права администратора. При запуске от обычного пользователя он перезапускает себя через запрос UAC, передавая все параметры без изменений.
- **Очистка перед сборкой**: Все запущенные процессы `snishaper` принудительно завершаются перед сборкой, чтобы избежать блокировки файлов.
- **Синхронизация версии**: Перед компиляцией бэкенда версия и канал выпуска читаются из `Package.appxmanifest`, синхронизируются в ресурс версии через go-winres и внедряются через ldflags; при сбое go-winres сохраняется текущий ресурс версии и сборка продолжается. `go mod download` выполняется всегда.
- **Упаковка MSIX**: Требуется WinApp CLI (`winget install Microsoft.WinAppCLI`); если сертификат `devcert.pfx` отсутствует, он автоматически генерируется из manifest и устанавливается. Результат сохраняется в каталоге `Apppackage/`.
- **Сборка Linux (WSL)**: `-Build linux` делегирует `build.sh` через WSL (GTK4 по умолчанию, `-Gtk3` переключает на GTK3, `-Arch` передаётся как `--arch`); если WSL не найден, выводится предупреждение и сборка Linux пропускается.

**Примеры использования:**

```powershell
# Windows GUI, всё (фронтенд + бэкенд)
.\build_windows.ps1 -Build windows,gui,all

# Windows GUI, только фронтенд
.\build_windows.ps1 -Build windows,gui,frontend

# Linux GUI через WSL, всё
.\build_windows.ps1 -Build linux,gui,all

# Только CLI (headless, кроссплатформенный)
.\build_windows.ps1 -Build windows,cli,all

# Windows + Linux GUI
.\build_windows.ps1 -Build all,gui,all

# Всё: все платформы, GUI + CLI
.\build_windows.ps1 -Build all,all,all

# arm64 + упаковка MSIX
.\build_windows.ps1 -Build windows,gui,all -Arch arm64 -BuildMsix

# Тихий режим (для CI/CD, без взаимодействия)
.\build_windows.ps1 -Silent

# Старый формат по-прежнему работает
.\build_windows.ps1 -Build frontend -Lang cn
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# Без параметров = интерактивный режим
.\build_windows.ps1
```

### Сборка Linux

Сборка Linux использует `build_linux.sh` и выполняется на хосте Linux (или WSL2 в Windows).

#### Зависимости (Ubuntu / Debian)

```bash
# GTK4 + WebKitGTK 6.0 (по умолчанию)
sudo apt-get update
sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev

# Или использовать GTK3 + webkit2gtk-4.1
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

#### Команды сборки

```bash
# Клонировать репозиторий
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Интерактивное меню (1 GUI / 2 CLI / 3 GUI+CLI + выбор архитектуры)
./build.sh

# GUI (использует существующий frontend/dist)
./build.sh --gui

# Сначала собрать фронтенд, затем бэкенд
./build.sh --with-frontend

# Использовать GTK3 + webkit2gtk-4.1
./build.sh --gtk3

# Только CLI (headless, кроссплатформенный)
./build.sh --cli

# GUI + CLI
./build.sh --all

# Указать архитектуру (для GUI и CLI)
./build.sh --gui --arch arm64

# x86: только Windows CLI (Linux/Darwin пропускаются)
./build.sh --cli --arch x86
```

Результат сборки записывается в `build/bin/gui/Linux/<arch>/SniShaper` (включая seed-файлы `rules/` и `config/`). TUN / системный прокси требуют root; запускайте с `sudo ./build/bin/gui/Linux/x64/SniShaper`. CLI-бинарники находятся в `build/bin/cli/<Platform>/<Arch>/`.

### Версия и канал выпуска

Номер версии и канал выпуска (`release` / `beta` / `alpha` / `rc`) **унифицированы** в корневом файле `Package.appxmanifest`:

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

И Windows, и Linux сборки читают из этого файла и внедряют значения через ldflags (`snishaper/app.buildVersion`, `snishaper/app.buildChannel`). Отдельного JSON-файла версии в репозитории нет.

### Окружение разработки

- `Go 1.27+`
- `Node.js 24+` / `npm 11+`
- Windows: инструментарий MSVC (Wails v3), WinApp CLI (упаковка MSIX)
- Linux: пакеты разработки GTK4 / WebKitGTK или GTK3 (см. выше)
- Режим TUN зависит от сетевого стека gvisor (в Windows включается через build tag `with_gvisor`)

Результаты сборки:

- Ресурсы фронтенда находятся в `frontend/dist`
- GUI Windows: `build/bin/gui/Windows/<arch>/snishaper.exe` (по умолчанию x64)
- GUI Linux: `build/bin/gui/Linux/<arch>/SniShaper`
- CLI: `build/bin/cli/{Windows,Linux,Darwin}/<arch>/snishaper[.exe]`

---

## Непрерывная интеграция

Кроссплатформенные CI-конвейеры:

- **`build.yml`**: запускается при каждом push / PR. Собирает Windows на `windows-2025` и Linux на `ubuntu-24.04`, затем выполняет компиляцию и smoke-тест бинарного файла.
- **`_release_pipeline.yml`**: конвейер релиза. Windows-runner создаёт MSIX и портативный архив `snishaper-windows-amd64.7z`, Ubuntu-runner создаёт `snishaper-linux-amd64.tar.gz`, и в конце Windows-runner объединяет артефакты обеих платформ и создаёт GitHub Release. Release notes сначала генерируются локальным экземпляром Ollama на runner (по умолчанию `qwen3.5:2b`); если Ollama недоступна, используется классифицированный список коммитов.

---

## Примечания по кроссплатформенности

Windows и Linux собираются из одного репозитория, платформозависимые реализации изолируются через Go build tags (например, `//go:build linux` / `windows`). Отдельного Linux-репозитория посещать не нужно.

## Благодарности

Проект вдохновлен следующими отличными open-source проектами:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## Участники

Благодарим следующих участников за их вклад в этот репозиторий:

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |
| <a href="https://github.com/lzpls"><img src="https://avatars.githubusercontent.com/lzpls" width="40" height="40" style="border-radius: 50%;" alt="lzpls" /></a> |
| [lzpls](https://github.com/lzpls) |

## История звёзд

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
 </picture>
</a>

---

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

### Основные участники

<div align="center">
<a href="https://github.com/SnishaperTeam/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SnishaperTeam/SniShaper" alt="Contributors" />
</a>
</div>

---

## Лицензия

[MIT License](LICENSE)
