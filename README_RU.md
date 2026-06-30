# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/coolapijust/snishaper/wiki)

**SniShaper** — это локальный прокси-инструмент, разработанный специально для сложных сетевых условий. Он интегрирует различные технологии обхода блокировок, включая **инъекцию ECH**, **фрагментацию TLS-RF**, **реконструкцию соединений QUIC** и **легковесное проксирование в режиме сервера**, обеспечивая стабильный доступ в интернет.

---

## Возможности

- **Шесть режимов проксирования**: поддержка широкого спектра режимов от легковесного `transparent` до продвинутого `server` проксирования для любых задач.
- **Гибкие стратегии**:
  - **TLS-RF (фрагментация TLS)**: обход точечных блокировок по SNI с помощью фрагментации.
  - **Реплей QUIC**: обход стандартного обнаружения SNI с помощью функций quic-go.
  - **Инъекция ECH**: автоматическое получение и внедрение echconfig.
- **Интеллектуальная маршрутизация**: автоматическое определение заблокированных доменов на основе GFWList, позволяющее подключаться к большинству сайтов вне правил без ручной настройки.

---

## Быстрый старт

### 1. Запуск
Скачайте [последнюю версию](https://github.com/coolapijust/snishaper/releases) и запустите `snishaper.exe`.

### 2. Переустановка сертификата
В главном интерфейсе нажмите «Управление сертификатами» -> «**Нажмите для переустановки сертификата**».

### 3. Настройка и запуск
Программное обеспечение поставляется с богатым набором официальных правил. Вы также можете настроить собственные правила на панели правил и нажать кнопку «**Запустить прокси**».

---

## Документация

Для получения подробных технических принципов, руководств по развертыванию и настройке, пожалуйста, обратитесь к [**GitHub Wiki**](https://github.com/coolapijust/snishaper/wiki):

- **[Основные режимы прокси](https://github.com/coolapijust/snishaper/wiki/Core-Proxy-Modes)**: понимание принципов работы TLS-RF, QUIC и серверного режима.
- **[Руководство по правилам](https://github.com/coolapijust/snishaper/wiki/Custom-Rules-Guide)**: как разрабатывать целевые правила.
- **[Настройка GUI](https://github.com/coolapijust/snishaper/wiki/GUI-Configuration)**: быстрая настройка правил в интерфейсе.
- **[Развертывание сервера](https://github.com/coolapijust/snishaper/wiki/Server-Deployment)**: настройка собственного серверного узла на CF Workers или VPS.
- **[Устранение неполадок](https://github.com/coolapijust/snishaper/wiki/FAQ)**: решение проблем с сертификатами, правилами и другим.

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
go build -ldflags="-s -w" -o "build/bin/snishaper.exe"
```

### Параметры командной строки скрипта сборки

`build_windows.ps1` поддерживает следующие параметры для пропуска интерактивных запросов:

| Параметр | Значения | Описание |
|----------|----------|----------|
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
- [usque](https://github.com/Diniboy1123/usque)

## Участники

Благодарим следующих участников за их вклад в этот репозиторий:

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :---: | :---: | :---: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |

## Лицензия

[MIT License](LICENSE)