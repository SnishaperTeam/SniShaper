# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/SniShaper/SniShaper/wiki)
[![GitHub Release](https://img.shields.io/github/v/release/SniShaper/SniShaper?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/SniShaper/total?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&logo=git)](https://github.com/SniShaper/SniShaper/commits/main)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SniShaper/SniShaper/build.yml?style=flat-square&logo=githubactions&label=CI)](https://github.com/SniShaper/SniShaper/actions)

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

Скачайте `snishaper-windows-amd64.7z` (портативная версия) или MSIX-установщик из [последнего релиза](https://github.com/SniShaper/SniShaper/releases), распакуйте / установите и запустите `snishaper.exe`. Приложение автоматически запрашивает права администратора (требуются для TUN). Если повышение прав не удалось, TUN недоступен, но остальные функции работают.

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/ru-ru%20dark.svg" width="200"/>
</a>

### Linux

Скачайте `snishaper-linux-amd64.tar.gz` из [последнего релиза](https://github.com/SniShaper/SniShaper/releases), распакуйте и запустите:

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

Для получения подробных технических принципов, руководств по развертыванию и настройке, обратитесь к [**GitHub Wiki**](https://github.com/SniShaper/SniShaper/wiki):

- **[Основные режимы прокси](https://github.com/SniShaper/SniShaper/wiki/Core-Proxy-Modes)**: понимание принципов работы TLS-RF, QUIC и серверного режима.
- **[Руководство по правилам](https://github.com/SniShaper/SniShaper/wiki/Custom-Rules-Guide)**: как разрабатывать целевые правила.
- **[Настройка GUI](https://github.com/SniShaper/SniShaper/wiki/GUI-Configuration)**: быстрая настройка правил в интерфейсе.
- **[Устранение неполадок](https://github.com/SniShaper/SniShaper/wiki/FAQ)**: решение проблем с сертификатами, правилами и другим.

---

## Сборка и разработка

Проект построен с использованием **Wails v3 + React 19 + MUI** с бэкендом на **Go**. Один и тот же репозиторий производит исполняемые файлы для Windows и Linux.

### Сборка Windows

```powershell
# Клонировать репозиторий
git clone https://github.com/SniShaper/SniShaper.git
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
| `-Build` | `frontend` / `backend` / `all` | Цель сборки |
| `-Lang` | `en` / `cn` / `ru` | Язык интерфейса |
| `-InstallDeps` | без значений (флаг) | Установить npm зависимости |
| `-BuildMsix` | без значений (флаг) | Собрать MSIX-пакет |
| `-SkipSign` | без значений (флаг) | Пропустить подпись MSIX, выходной файл будет иметь префикс `unsigned_` (требуется `-BuildMsix`) |
| `-Silent` | без значений (флаг) | Тихий режим, пропуск всех интерактивных запросов |

**Примеры использования:**

```powershell
# Собрать только фронтенд (китайский интерфейс)
.\build_windows.ps1 -Build frontend -Lang cn

# Собрать только бэкенд (английский интерфейс)
.\build_windows.ps1 -Build backend -Lang en

# Собрать всё и установить зависимости
.\build_windows.ps1 -Build all -Lang cn -InstallDeps

# Собрать всё и создать MSIX-пакет (подписан по умолчанию)
.\build_windows.ps1 -Build all -BuildMsix

# Собрать всё и создать неподписанный MSIX (пропустить подпись)
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# Тихий режим (для CI/CD, без взаимодействия)
.\build_windows.ps1 -Silent

# Тихий режим со сборкой и созданием пакета (пропуск подписи)
.\build_windows.ps1 -Build all -Silent -BuildMsix -SkipSign

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
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# Скомпилировать только бэкенд (использует существующий frontend/dist)
./build_linux.sh

# Сначала собрать фронтенд, затем бэкенд
./build_linux.sh --with-frontend

# Использовать GTK3 + webkit2gtk-4.1
./build_linux.sh --gtk3
```

Результат сборки записывается в `build/bin/SniShaper` (включая seed-файлы `rules/` и `config/`). TUN / системный прокси требуют root; запускайте с `sudo ./build/bin/SniShaper`.

### Версия и канал выпуска

Номер версии и канал выпуска (`release` / `beta` / `alpha` / `rc`) **унифицированы** в корневом файле `Package.appxmanifest`:

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

И Windows, и Linux сборки читают из этого файла и внедряют значения через ldflags (`snishaper/app.buildVersion`, `snishaper/app.buildChannel`). Отдельного JSON-файла версии в репозитории нет.

### Окружение разработки

- `Go 1.25+`
- `Node.js 24+` / `npm 11+`
- Windows: инструментарий MSVC (Wails v3), WinApp CLI (упаковка MSIX)
- Linux: пакеты разработки GTK4 / WebKitGTK или GTK3 (см. выше)
- Режим TUN зависит от сетевого стека gvisor (в Windows включается через build tag `with_gvisor`)

Результаты сборки:

- Ресурсы фронтенда находятся в `frontend/dist`
- Исполняемый файл Windows: `build/bin/snishaper.exe`
- Исполняемый файл Linux: `build/bin/SniShaper`

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

[![GitHub contributors](https://img.shields.io/github/contributors/SniShaper/SniShaper?style=flat-square&label=Всего участников)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SniShaper/SniShaper?style=flat-square&label=Коммитов в месяц)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&label=Последний коммит)](https://github.com/SniShaper/SniShaper/commits/main)

### Тренд активности

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### Основные участники

<div align="center">
<a href="https://github.com/SniShaper/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SniShaper/SniShaper" alt="Contributors" />
</a>
</div>

---

## Лицензия

[MIT License](LICENSE)
