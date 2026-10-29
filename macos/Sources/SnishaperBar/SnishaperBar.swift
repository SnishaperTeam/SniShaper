// SniShaperBar — macOS 菜单栏应用。
//
// 本应用不直接实现代理功能：它驱动 headless `snishaper` CLI 二进制
// （`snishaper start/stop/proxy on/off/sysproxy on/off/status --json/diag`），
// 状态经 `status --json` 的稳定 JSON 契约轮询获得。代理引擎、规则与
// TUN 全部在 CLI 进程内完成，菜单栏应用只做控制与展示。
//
// 构建：swift build（纯文本，无 Xcode 工程）。打包 .app：scripts/macos-bundle.sh。

import AppKit
import Foundation

// MARK: - 状态契约（对应 CLI statusJSON）

struct StatusInfo: Decodable {
    let service: Bool
    let proxy: Bool?
    let port: String?
    let mode: String?
    let tun: Bool?
    let tunMessage: String?
    let sysproxy: Bool?
    let sysproxyServer: String?

    enum CodingKeys: String, CodingKey {
        case service, proxy, port, mode, tun
        case tunMessage = "tun_message"
        case sysproxy
        case sysproxyServer = "sysproxy_server"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        service = try c.decode(Bool.self, forKey: .service)
        proxy = try c.decodeIfPresent(Bool.self, forKey: .proxy)
        port = try c.decodeIfPresent(String.self, forKey: .port)
        mode = try c.decodeIfPresent(String.self, forKey: .mode)
        tun = try c.decodeIfPresent(Bool.self, forKey: .tun)
        tunMessage = try c.decodeIfPresent(String.self, forKey: .tunMessage)
        sysproxy = try c.decodeIfPresent(Bool.self, forKey: .sysproxy)
        sysproxyServer = try c.decodeIfPresent(String.self, forKey: .sysproxyServer)
    }
}

// MARK: - CLI 封装

enum CLI {
    /// 二进制查找顺序：用户覆盖（defaults）、应用包内 Resources/bin、
    /// Homebrew、/usr/local/bin、PATH。
    static func binaryPath() -> String {
        if let custom = UserDefaults.standard.string(forKey: "binaryPath"), !custom.isEmpty {
            return custom
        }
        var candidates: [String] = []
        if let res = Bundle.main.resourceURL {
            candidates.append(res.appendingPathComponent("bin/snishaper").path)
        }
        candidates.append(contentsOf: [
            "/opt/homebrew/bin/snishaper",
            "/usr/local/bin/snishaper",
            "snishaper", // 依赖 PATH
        ])
        for path in candidates {
            if path == "snishaper" || FileManager.default.isExecutableFile(atPath: path) {
                return path
            }
        }
        return candidates.last!
    }

    /// 运行一条 CLI 媌命令并返回 (stdout, 退出码)。
    @discardableResult
    static func run(_ arguments: [String]) -> (String, Int32) {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = [binaryPath()] + arguments
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = Pipe() // 错误流不弹到控制台
        do {
            try process.run()
        } catch {
            return ("", 127)
        }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return (String(data: data, encoding: .utf8) ?? "", process.terminationStatus)
    }

    /// 轮询用的状态查询；服务未运行也返回合法 JSON（仅 service 字段）。
    static func status() -> StatusInfo? {
        let (out, code) = run(["status", "--json"])
        guard code == 0, !out.isEmpty else { return nil }
        return try? JSONDecoder().decode(StatusInfo.self, from: Data(out.utf8))
    }
}

// MARK: - 菜单栏控制器

final class MenuBarController: NSObject, NSMenuDelegate {
    private var statusItem: NSStatusItem!
    private var status: StatusInfo?
    private var pollTimer: Timer?

    // 菜单条目引用：轮询后只改 title，不重建菜单（避免点击瞬间菜单跳动）。
    private let serviceLine = NSMenuItem(title: "服务: …", action: nil, keyEquivalent: "")
    private let proxyLine = NSMenuItem(title: "代理: …", action: nil, keyEquivalent: "")
    private let sysProxyLine = NSMenuItem(title: "系统代理: …", action: nil, keyEquivalent: "")
    private let tunLine = NSMenuItem(title: "TUN: …", action: nil, keyEquivalent: "")
    private let serviceToggle = NSMenuItem(title: "启动服务", action: #selector(toggleService), keyEquivalent: "s")
    private let proxyToggle = NSMenuItem(title: "开启代理", action: #selector(toggleProxy), keyEquivalent: "p")
    private let sysProxyToggle = NSMenuItem(title: "开启系统代理", action: #selector(toggleSysProxy), keyEquivalent: "y")

    func start() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.button?.title = "⬢"

        let menu = NSMenu()
        menu.delegate = self
        menu.autoenablesItems = false

        for item in [serviceLine, proxyLine, sysProxyLine, tunLine] {
            item.isEnabled = false
            menu.addItem(item)
        }

        menu.addItem(.separator())
        serviceToggle.target = self
        menu.addItem(serviceToggle)
        proxyToggle.target = self
        menu.addItem(proxyToggle)
        sysProxyToggle.target = self
        menu.addItem(sysProxyToggle)

        let diag = NSMenuItem(title: "复制诊断 JSON", action: #selector(copyDiagnostics), keyEquivalent: "d")
        diag.target = self
        menu.addItem(diag)

        let logs = NSMenuItem(title: "打开日志文件夹", action: #selector(openLogs), keyEquivalent: "l")
        logs.target = self
        menu.addItem(logs)

        menu.addItem(.separator())
        let quit = NSMenuItem(title: "退出 SniShaper", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        menu.addItem(quit)

        statusItem.menu = menu
        refresh()

        pollTimer = Timer.scheduledTimer(withTimeInterval: 3.0, repeats: true) { [weak self] _ in
            self?.refresh()
        }
    }

    /// 轮询状态并更新菜单文案。菜单未打开时只更新数据，打开时才刷
    /// title，避免后台持续触发无障碍通知。
    private func refresh() {
        let fresh = CLI.status()
        DispatchQueue.main.async { self.apply(fresh) }
    }

    private func apply(_ info: StatusInfo?) {
        status = info
        let on = "开", off = "关", unknown = "未知"
        guard let info else {
            statusItem.button?.title = "⬢!"
            serviceLine.title = "服务: 无法获取（未找到 snishaper？）"
            for item in [proxyToggle, sysProxyToggle] { item.isEnabled = false }
            serviceToggle.title = "启动服务"
            serviceToggle.isEnabled = false
            return
        }
        statusItem.button?.title = info.service ? "⬢" : "⬢"
        serviceLine.title = "服务: \(info.service ? "运行中" : "未运行")"
        proxyLine.title = "代理: \(info.proxy == true ? on : (info.proxy == false ? off : unknown))"
        sysProxyLine.title = "系统代理: \(info.sysproxy == true ? on : (info.sysproxy == false ? off : unknown))"
        tunLine.title = "TUN: \(info.tun == true ? on : (info.tun == false ? off : unknown))"

        serviceToggle.isEnabled = true
        serviceToggle.title = info.service ? "停止服务" : "启动服务"
        proxyToggle.isEnabled = info.service
        proxyToggle.title = (info.proxy == true) ? "停止代理" : "开启代理"
        sysProxyToggle.isEnabled = info.service
        sysProxyToggle.title = (info.sysproxy == true) ? "关闭系统代理" : "开启系统代理"
    }

    // MARK: 动作

    @objc private func toggleService() {
        let running = status?.service ?? false
        _ = CLI.run(running ? ["stop"] : ["start"])
        // start 是分离进程，轮询会追上状态；这里立即刷一次缩短反馈延迟。
        DispatchQueue.global().async { [weak self] in
            Thread.sleep(forTimeInterval: 0.8)
            self?.refresh()
        }
    }

    @objc private func toggleProxy() {
        guard let info = status, info.service else { return }
        _ = CLI.run((info.proxy == true) ? ["proxy", "off"] : ["proxy", "on"])
        refresh()
    }

    @objc private func toggleSysProxy() {
        guard let info = status, info.service else { return }
        _ = CLI.run((info.sysproxy == true) ? ["sysproxy", "off"] : ["sysproxy", "on"])
        refresh()
    }

    @objc private func copyDiagnostics() {
        let (out, code) = CLI.run(["diag"])
        let text = code == 0 ? out : "diag 失败（退出码 \(code)）"
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    @objc private func openLogs() {
        // 日志目录约定：与二进制同级的 log/ 目录（CLI cmdStart 同样写入此处）。
        let dir = URL(fileURLWithPath: CLI.binaryPath()).deletingLastPathComponent()
            .appendingPathComponent("log")
        NSWorkspace.shared.open(dir)
    }
}

// MARK: - 应用入口

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let controller = MenuBarController()

    func applicationDidFinishLaunching(_ notification: Notification) {
        controller.start()
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
