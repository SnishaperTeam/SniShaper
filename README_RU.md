[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

# SniShaper

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/coolapijust/snishaper/wiki)

**SniShaper** — это локальный прокси-инструмент, разработанный специально для сложных сетевых условий. Он интегрирует различные технологии обхода блокировок, включая **инъекцию ECH**, **фрагментацию TLS-RF**, **реконструкцию соединений QUIC** и **легковесное проксирование в режиме сервера**, обеспечивая стабильный доступ в интернет.

---

## Возможности

- **Шесть режимов проксирования**: поддержка широкого спектра режимов от легковесного `transparent` до продвинутого `server` проксирования для любых задач.
- **Гибкие стратегии**:
  - **TLS-RF (фрагментация TLS)**: обход точечных блокировок по SNI с помощью фрагментации.
  - **Реплей QUIC**: обход стандартного обнаружения SNI с помощью функций `quic-go`.
  - **Инъекция ECH**: автоматическое получение и внедрение echconfig.
- **Оптимизация IP и WARP**: интегрированная оптимизация пула IP-адресов Cloudflare и туннель WARP Masque.
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
# Клонирование репозитория
git clone https://github.com/coolapijust/snishaper.git
cd snishaper

# Установка зависимостей фронтенда
cd frontend
npm install

# Сборка фронтенд-ресурсов
npm run build
cd ..

# Сборка Windows GUI исполняемого файла
go build -ldflags="-H windowsgui" -o build/bin/snishaper.exe .
```

Файл `snishaper.syso` хранится в репозитории, поэтому при `go build` иконка Windows и метаданные версии встраиваются автоматически без дополнительного шага упаковки ресурсов.

Рекомендуемое окружение:

- `Go 1.25+`
- `Node.js 24+`
- `npm 11+`

Результат сборки:

- Ресурсы фронтенда: `frontend/dist`
- Исполняемый файл: `build/bin/snishaper.exe`

---

## Благодарности

Этот проект вдохновлен и использует наработки следующих отличных open-source проектов:

- [SNIBypassGUI](https://github.com/coolapijust/SniViewer)
- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

## Лицензия

[MIT License](LICENSE)
