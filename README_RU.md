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

---

## Основной поток обработки запросов

```mermaid
flowchart TD
    A[Запрос браузера/приложения] --> B[ProxyServer.handleRequest]
    B --> C{Сопоставление правил matchRule}
    C --> D[Получение Effective Mode]
    D --> E{Тип запроса}
    E -->|CONNECT| F[handleConnect]
    E -->|HTTP| G[handleHTTP]
    F --> H{Effective Mode}
    H -->|mitm| I[handleMITM - Man-in-the-Middle]
    H -->|tls-rf| J[handleTLSFragment - Фрагментация TLS]
    H -->|quic| K[handleQUICMITM - QUIC туннель]
    H -->|migration| L[handleMigration - API миграции сессий]
    H -->|transparent/direct| M[handleTransparent - Сквозной туннель]
    G --> N{Режим}
    N -->|direct| O[transport.RoundTrip прямой прокси]
    N -->|mitm/quic| P[HTTP to HTTPS редирект]
    N -->|другие| Q[Upstream кандидат + RoundTrip]
    I --> R[Целевой сервер]
    J --> R
    K --> R
    L --> R
    M --> R
    O --> R
    Q --> R
```

## Поток TUN виртуального сетевого адаптера

```mermaid
flowchart LR
    A[Системный трафик] --> B[TUN виртуальный адаптер]
    B --> C[PlanTUNFlow]
    C --> D{Протокол}
    D -->|UDP| E[UDP стратегия]
    D -->|TCP| F[TCP обработка]
    E --> G{Effective Mode}
    G -->|quic| H[native-quic]
    G -->|warp| I[warp]
    G -->|другие| J[passthrough]
    F --> K{Режим}
    K -->|mitm/tls-rf| L[TCP туннель]
    K -->|transparent| M[Прозрачная пересылка]
    K -->|direct| N[Прямое соединение]
    L --> O[Выход через физический сетевой адаптер]
    M --> O
    N --> O
    H --> O
    I --> O
    J --> O
```

---

## Возможности

- **Многорежимное прокси**: MITM, Transparent, TLS-RF (фрагментация TLS), QUIC, Migration (перенос сессий), Direct -- для различных сценариев.
- **TUN виртуальный сетевой адаптер**: нативная поддержка TUN для прозрачного глобального перехвата трафика, авто-маршрутизации и перехвата DNS.
- **Инъекция ECH**: автоматическое получение и внедрение ECH Config с DoH-обнаружением и горячей заменой.
- **Интеллектуальная маршрутизация**: автоматическое определение заблокированных доменов на основе GFWList без ручной настройки.
- **Шифрованный DNS**: встроенный защищённый DNS-резолвер с балансировкой узлов.
- **Cloudflare IP пул**: автоматическое измерение скорости, проверка работоспособности и обновление.
- **NAT64 поддержка**: гибкий IP-выход и доступ к сервисам.

---

## Быстрый старт

### 1. Запуск
Скачайте [последнюю версию](https://github.com/SniShaper/SniShaper/releases) и запустите `snishaper.exe`. Приложение автоматически запрашивает права администратора (требуются для TUN). Если повышение прав не удалось, TUN недоступен, но остальные функции работают.

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/ru-ru%20dark.svg" width="200"/>
</a>

### 2. Переустановка сертификата
В главном интерфейсе нажмите **Управление сертификатами -> Сбросить корневой сертификат**.

### 3. Настройка и запуск
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

Проект построен с использованием **Wails v3**.

```powershell
# Клонировать репозиторий
git clone https://github.com/SniShaper/snishaper.git
cd snishaper

# Установить зависимости фронтенда
cd frontend
npm install

# Собрать статические ресурсы фронтенда
npm run build
cd ..

# Полная компиляция за один шаг (интерактивный режим)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# Или с PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1

# Компиляция основной программы на Go (скрипт автоматически выполняет go mod download)
go build -tags with_gvisor -ldflags="-s -w" -o "build/bin/snishaper.exe"
```

### Параметры командной строки скрипта сборки

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

Рекомендации по окружению разработки:

- `Go 1.25+`
- `Node.js 24+`
- `npm 11+`
- `gVisor` (требуется для TUN режима, Linux: установить пакет `gvisor`)

Результаты сборки:

- Ресурсы фронтенда находятся в `frontend/dist`
- Исполняемый файл находится в `build/bin/snishaper.exe`

---

## Кроссплатформенность

Программа поддерживает платформы Windows и Linux. Для версии Linux обратитесь к [Linux версия](https://github.com/dongzheyu/SniShaper-Linux/).

## Благодарности

Проект вдохновлен следующими отличными open-source проектами:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## Участники

Благодарим следующих участников за их вклад в этот репозиторий:

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |

## История звёзд

<a href="https://www.star-history.com/?repos=snishaper/snishaper&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
   <img alt="Диаграмма истории звёзд" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
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