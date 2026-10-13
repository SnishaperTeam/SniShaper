# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/Лицензия-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/Документация-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Релиз)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Загрузки)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Последний%20коммит)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** — это локальный прокси-инструмент для сложных сетевых условий, использующий **инъекцию ECH**, **фрагментацию TLS**, **преобразование QUIC**, **миграцию сессий** и другие решения, обеспечивая стабильный и гибкий доступ в интернет.

Проект поддерживает кроссплатформенность. Подробности — в **[Platform_RU.md](docs/Platform_RU.md)**.

---

## Смежный проект: FlowWeaver

[**FlowWeaver**](https://github.com/SnishaperTeam/FlowWeaver) — полнофункциональная ветка этого проекта, которую ведёт тот же мейнтейнер. Оба проекта **активно поддерживаются**:

| | SniShaper (этот репозиторий) | [FlowWeaver](https://github.com/SnishaperTeam/FlowWeaver) |
|---|---|---|
| Направленность | Лёгкий прокси-сервер | Полнофункциональный клиент / шлюз |
| Сильные стороны | Инъекция ECH, фрагментация TLS, миграция сессий | Подписки Clash, мультипротокольные узлы, WireGuard, TUN-шлюз |

Проекты используют разные кодовые базы и развиваются независимо. Форматы файлов подписок и правил совместимы. Если вам нужны **импорт подписок, выбор узлов разных протоколов или туннели WireGuard**, используйте FlowWeaver.

---

## Сообщество

Присоединяйтесь к группе QQ **[Snishaper and FlowWeaver building](https://qm.qq.com/q/GtBOkAOiME)**, чтобы напрямую общаться с пользователями и разработчиками SniShaper и FlowWeaver, сообщать о проблемах и предлагать идеи.

---

## Возможности

- **Многорежимное прокси**: MITM (человек посередине), Transparent, TLS-RF (фрагментация TLS), QUIC, Migration (миграция сессий), Direct — для различных сценариев работы с сайтами.
- **TUN виртуальный сетевой адаптер**: прозрачный глобальный перехват трафика, авто-маршрутизация и перехват DNS.
- **Инъекция ECH**: автоматическое получение и внедрение ECH Config с DoH-обнаружением и горячей заменой.
- **Интеллектуальная маршрутизация**: автоматическое определение заблокированных доменов на основе GFWList с автоматическим покрытием множества сайтов, не включённых в правила.
- **Шифрованный DNS**: встроенный защищённый DNS-резолвер с балансировкой узлов для стабильного разрешения.
- **Cloudflare IP пул**: автоматическое измерение скорости, проверка работоспособности и обновление.
- **NAT64 поддержка**: более гибкий IP-выход, обеспечивающий доступ к сервисам при IP-блокировке.
- **Режим эволюции**: автоматическое тестирование комбинаций правил для поиска оптимального способа доступа к целевому сайту с применением в один клик.

---

## Быстрый старт

Проект изначально разрабатывался только под Windows, а позднее была добавлена поддержка Linux. Сейчас доступны как GUI, так и CLI.

Для большинства пользователей мы рекомендуем скачать последнюю стабильную сборку для Windows прямо из [Releases](https://github.com/SnishaperTeam/SniShaper/releases).

По остальным платформам и инструкциям по сборке обратитесь к следующим документам:

- **[Platform_RU.md](docs/Platform_RU.md)** — быстрый старт для платформ, использование CLI и мобильные версии.
- **[build_RU.md](docs/build_RU.md)** — руководство по сборке и матрица из 12 целей.

---

## Документация

Для получения подробных технических принципов, руководств по развертыванию и настройке, обратитесь к [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki):

- **[Основные режимы прокси](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**: понимание принципов работы TLS-RF, QUIC и серверного режима.
- **[Руководство по правилам](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**: как разрабатывать целевые правила.
- **[Настройка GUI](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**: быстрая настройка правил в интерфейсе.
- **[Устранение неполадок](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**: решение проблем с сертификатами, правилами и другим.
- **[Соглашение для соавторов (Collaborator Agreement)](docs/COLLABORATOR_AGREEMENT.md)**: условия, приглашение и порядок принятия статуса соавтора репозитория.

---

## Сборка и разработка

В настоящее время фронтенд построен на **Wails v3 + React 19 + MUI**, а ядро разрабатывается на **Go**, поддерживая GUI для Windows / Linux и кроссплатформенный CLI — всего 12 целей сборки. Полное руководство по сборке — в **[build_RU.md](docs/build_RU.md)**.

Мы завершим реализацию нативного GUI в одном из следующих стабильных релизов, чтобы сократить потребление памяти фронтендом.

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

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=timeline&logscale=&releases=&legend=bottom-right">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&theme=dark&logscale&legend=bottom-right" />
<source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&logscale&legend=bottom-right" />
<img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&logscale&legend=bottom-right" />
</picture>
</a>

---

## Лицензия

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
