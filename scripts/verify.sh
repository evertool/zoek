#!/usr/bin/env bash
# =============================================================================
# zoekt 统一验证入口
# PRD §10.2 验证顺序：go mod tidy → Go format/vet/编译 → 单元测试 → 小程序端回归
# 后续迭代逐步加入：数据库迁移测试、API 契约测试、小程序构建、E2E
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/backend"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

PASS="${GREEN}[PASS]${NC}"
FAIL="${RED}[FAIL]${NC}"
INFO="${YELLOW}[INFO]${NC}"

echo -e "${INFO} ========================================"
echo -e "${INFO}  zoek 验证流程启动"
echo -e "${INFO}  项目根目录: ${PROJECT_ROOT}"
echo -e "${INFO} ========================================"

cd "$BACKEND_DIR"

# ---------------------------------------------------------------------------
# 1. Go mod tidy check
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [1/7] Go mod tidy 检查"
TIDY_DIFF=$(go mod tidy -diff 2>&1 || true)
if [ -n "$TIDY_DIFF" ]; then
    echo -e "${FAIL} go.mod / go.sum 不整洁，请运行: cd backend && go mod tidy"
    echo "$TIDY_DIFF"
    exit 1
fi
echo -e "${PASS} go mod tidy 通过"

# ---------------------------------------------------------------------------
# 2. Go format check
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [2/7] Go 格式检查 (gofmt)"
if [ -n "$(gofmt -l .)" ]; then
    echo -e "${FAIL} gofmt 发现未格式化文件："
    gofmt -l .
    exit 1
fi
echo -e "${PASS} gofmt 通过"

# ---------------------------------------------------------------------------
# 3. Go vet
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [3/7] Go vet 检查"
if ! go vet ./...; then
    echo -e "${FAIL} go vet 失败"
    exit 1
fi
echo -e "${PASS} go vet 通过"

# ---------------------------------------------------------------------------
# 4. Go build
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [4/7] Go 编译检查"
if ! go build -o /dev/null ./...; then
    echo -e "${FAIL} go build 失败"
    exit 1
fi
echo -e "${PASS} go build 通过"

# ---------------------------------------------------------------------------
# 5. Go test (scoring + game state machine + all packages, with race detector)
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [5/7] Go 单元测试"
if ! go test -v -count=1 -race ./...; then
    echo -e "${FAIL} go test 失败"
    exit 1
fi
echo -e "${PASS} go test 通过"

# ---------------------------------------------------------------------------
# 6. 小程序端「审核致命项」回归
#    锁住 baseURL 环境判定（审核端 envVersion 就是 develop，绝不能再据此连局域网）
#    与首页静默登录失败时的自动重试行为。
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [6/7] 小程序端回归 (check-mp-client.js)"
if ! node "$SCRIPT_DIR/check-mp-client.js"; then
    echo -e "${FAIL} 小程序端回归失败"
    exit 1
fi
echo -e "${PASS} 小程序端回归通过"

# ---------------------------------------------------------------------------
# 7. Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} ========================================"
echo -e "${PASS} 全部验证通过！"
echo -e "${INFO} ========================================"
