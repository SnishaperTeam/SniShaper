# Makefile for SniShaper builds
# 使用 -Command 方式传递数组参数，避免 -File 解析问题

# ---------- Windows GUI ----------
win-gui-x64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'windows','gui','all' -Arch x64 -Silent -Lang cn }"

win-gui-x86:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'windows','gui','all' -Arch x86 -Silent -Lang cn }"

win-gui-arm64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'windows','gui','all' -Arch arm64 -Silent -Lang cn }"

# ---------- Linux GUI ----------
linux-gui-x64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'linux','gui','all' -Arch x64 -Silent -Lang cn }"

linux-gui-arm64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'linux','gui','all' -Arch arm64 -Silent -Lang cn }"

# ---------- CLI ----------
cli-x64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'all','cli','all' -Arch x64 -Silent -Lang cn }"

cli-arm64:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'all','cli','all' -Arch arm64 -Silent -Lang cn }"

cli-x86:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "& { .\build_windows.ps1 -Build 'all','cli','all' -Arch x86 -Silent -Lang cn }"

# ---------- 全构建 ----------
all: win-gui-x64 win-gui-x86 win-gui-arm64 linux-gui-x64 linux-gui-arm64 cli-x64 cli-arm64 cli-x86
	@echo "===== All builds completed successfully! ====="

.PHONY: win-gui-x64 win-gui-x86 win-gui-arm64 linux-gui-x64 linux-gui-arm64 cli-x64 cli-arm64 cli-x86 all