// swift-tools-version:5.9
// SniShaper macOS 菜单栏应用的 Swift Package 清单。
// 纯文本构建（swift build），无需 Xcode 工程文件；打包为 .app 由
// scripts/macos-bundle.sh 负责，CI 中在真实 macOS runner 上编译验证。
import PackageDescription

let package = Package(
    name: "SnishaperBar",
    platforms: [
        .macOS(.v13)
    ],
    targets: [
        .executableTarget(
            name: "SnishaperBar",
            path: "Sources/SnishaperBar"
        )
    ]
)
