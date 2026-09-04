# Makefile for SniShaper builds
# 使用 -Command 方式调用 PowerShell，避免 -File 对数组参数的处理差异
#
# 产物布局：build/bin/{cli,gui}/{Windows,Linux,Darwin}/{x64,x86,arm64}/
# 完整矩阵：CLI 7 个 + GUI 5 个 = 12 个目标

PS = powershell -NoProfile -ExecutionPolicy Bypass -Command

# ---------- Windows GUI ----------
win-gui-x64:
	$(PS) "& { .\build_windows.ps1 -Platform windows -Arch x64 -Type gui -Silent -Lang cn }"

win-gui-x86:
	$(PS) "& { .\build_windows.ps1 -Platform windows -Arch x86 -Type gui -Silent -Lang cn }"

win-gui-arm64:
	$(PS) "& { .\build_windows.ps1 -Platform windows -Arch arm64 -Type gui -Silent -Lang cn }"

# ---------- Linux GUI（通过 WSL 原生构建） ----------
linux-gui-x64:
	$(PS) "& { .\build_windows.ps1 -Platform linux -Arch x64 -Type gui -Silent -Lang cn }"

linux-gui-arm64:
	$(PS) "& { .\build_windows.ps1 -Platform linux -Arch arm64 -Type gui -Silent -Lang cn }"

# ---------- CLI ----------
cli-x64:
	$(PS) "& { .\build_windows.ps1 -Platform all -Arch x64 -Type cli -Silent -Lang cn }"

cli-arm64:
	$(PS) "& { .\build_windows.ps1 -Platform all -Arch arm64 -Type cli -Silent -Lang cn }"

cli-x86:
	$(PS) "& { .\build_windows.ps1 -Platform windows -Arch x86 -Type cli -Silent -Lang cn }"

cli-all:
	$(PS) "& { .\build_windows.ps1 -Type cli -All -Silent -Lang cn }"

# ---------- 全构建 ----------
all: win-gui-x64 win-gui-x86 win-gui-arm64 linux-gui-x64 linux-gui-arm64 cli-all
	@echo "===== All builds completed successfully! ====="

# ---------- 辅助 ----------
dry-run:
	$(PS) "& { .\build_windows.ps1 -DryRun -All }"

.PHONY: win-gui-x64 win-gui-x86 win-gui-arm64 linux-gui-x64 linux-gui-arm64 cli-x64 cli-arm64 cli-x86 cli-all all dry-run
