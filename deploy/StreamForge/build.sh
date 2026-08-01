#!/usr/bin/env bash
# build.sh —— 本地构建三个服务的产物，输出到 deploy/StreamForge/ 下
# 使用方法（在项目根目录，Windows 请用 Git Bash 或 PowerShell 版 build.ps1）：
#   bash deploy/StreamForge/build.sh
# 前置要求：
#   - JDK 17+（user-service 使用了 record，低于 17 会报 "class, interface, or enum expected"）
#   - Go 1.21+
#   - Node 22 或 24
# 注意：在 WSL 里跑时，用的是 WSL 内部的 java/go/node，不是 Windows 侧的。
#       如果 WSL 里没装 JDK 17，请：sudo apt install openjdk-17-jdk
#       或者切到 Git Bash / PowerShell 用 Windows 侧的 JDK 17 构建。
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/../.." &> /dev/null && pwd)"

echo "==> 项目根目录: ${PROJECT_ROOT}"
echo "==> 部署包目录: ${SCRIPT_DIR}"

# ---------------- 0. 固定使用 Windows 侧的 JDK 17 ----------------
JDK_WIN='C:\Users\admin\.jdks\ms-17.0.16'

# 检测当前 shell 环境，把 Windows 风格 JDK 路径转成对应的 Unix 风格
SHELL_ENV=unknown
if grep -qiE '(microsoft|wsl)' /proc/version 2>/dev/null; then
  SHELL_ENV=wsl
  JDK_UNIX="/mnt/c/Users/admin/.jdks/ms-17.0.16"
elif [[ -n "${MSYSTEM:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]]; then
  SHELL_ENV=gitbash
  JDK_UNIX="/c/Users/admin/.jdks/ms-17.0.16"
else
  echo "[error] 指定的 JDK 路径 ${JDK_WIN} 只在 Windows 上可用。" >&2
  echo "        请在 Windows 的 Git Bash / WSL / PowerShell 里运行；纯 Linux 请自行 export JAVA_HOME。" >&2
  exit 1
fi

if [[ ! -d "${JDK_UNIX}" ]]; then
  echo "[error] JDK 目录不存在：${JDK_UNIX}" >&2
  exit 1
fi

echo "==> 指定 JDK：${JDK_WIN} (shell=${SHELL_ENV}, unix=${JDK_UNIX})"

command -v go   >/dev/null 2>&1 || { echo "[error] 未找到 go，需要 Go 1.21+。" >&2; exit 1; }
command -v npm  >/dev/null 2>&1 || { echo "[error] 未找到 npm，需要 Node 22 或 24。" >&2; exit 1; }

# ---------------- 前端 API 基地址（构建期注入） ----------------
# 前端在生产环境应通过同源的 Nginx 反代访问后端，因此这里置空，
# 让 axios 走相对路径 /api/...
export VITE_USER_API_BASE_URL="${VITE_USER_API_BASE_URL:-}"
export VITE_MEDIA_API_BASE_URL="${VITE_MEDIA_API_BASE_URL:-}"

# ---------------- 1. user-service（Java / Maven） ----------------
echo ""
echo "==> [1/3] 构建 user-service (Spring Boot, 使用 ${JDK_WIN})"
cd "${PROJECT_ROOT}/user-service"

# 让 Maven Wrapper 使用指定的 JDK
export JAVA_HOME="${JDK_UNIX}"

if [[ "${SHELL_ENV}" == "wsl" ]]; then
  # 【为什么走 cmd.exe】WSL 里的 mvnw（Bash 版）会用 ${JAVA_HOME}/bin/java 去启动 Maven，
  # 但 Windows JDK 目录里只有 java.exe（Windows PE），WSL 里的 bash 无法直接执行 .exe 的加载器
  # （Java 里会崩：Unable to find any JVMs matching...）。
  # 最稳的办法：通过 cmd.exe 调 mvnw.cmd，一切都在 Windows 侧完成。
  # 把项目 Unix 路径换成 Windows 路径传给 cmd.exe。
  PROJECT_WIN="$(wslpath -w "$(pwd)")"
  # JAVA_HOME 在 Windows 侧要用 Windows 路径
  cmd.exe /c "set \"JAVA_HOME=${JDK_WIN}\" && cd /d \"${PROJECT_WIN}\" && mvnw.cmd -B -DskipTests package"
else
  # Git Bash：可以直接跑 mvnw.cmd（或 mvnw），JAVA_HOME 用 Unix 风格即可
  export PATH="${JDK_UNIX}/bin:${PATH}"
  cmd //c "mvnw.cmd -B -DskipTests package"
fi

cp target/user-service-0.0.1-SNAPSHOT.jar "${SCRIPT_DIR}/user-service/user-service.jar"
echo "==> user-service.jar 已复制"

# ---------------- 2. media-service（Go，交叉编译到 linux/amd64） ----------------
echo ""
echo "==> [2/3] 构建 media-service (Go, linux/amd64)"
cd "${PROJECT_ROOT}/media-service"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" \
  -o "${SCRIPT_DIR}/media-service/media-service" \
  ./cmd/media-service
echo "==> media-service 二进制已生成"

# ---------------- 3. frontend（Vite 静态资源） ----------------
echo ""
echo "==> [3/3] 构建 frontend (Vite)"
cd "${PROJECT_ROOT}/frontend"
if [[ ! -d node_modules ]]; then
  npm ci
fi
npm run build
rm -rf "${SCRIPT_DIR}/frontend/dist"
cp -r dist "${SCRIPT_DIR}/frontend/dist"
echo "==> dist/ 已复制"

echo ""
echo "======================================================"
echo "构建完成。下一步："
echo "  1) 打包上传： tar -czf StreamForge.tgz -C deploy StreamForge"
echo "  2) 上传到服务器并解压后进入 StreamForge 目录"
echo "  3) cp .env.example .env  &&  编辑 .env"
echo "  4) bash deploy.sh"
echo "======================================================"
