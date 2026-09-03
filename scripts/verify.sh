#!/usr/bin/env bash
# =============================================================================
# zoekt 统一验证入口
# PRD §10.2 验证顺序：go mod tidy → Go format/vet/编译 → 单元测试
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
echo -e "${INFO} [1/6] Go mod tidy 检查"
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
echo -e "${INFO} [2/6] Go 格式检查 (gofmt)"
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
echo -e "${INFO} [3/6] Go vet 检查"
if ! go vet ./...; then
    echo -e "${FAIL} go vet 失败"
    exit 1
fi
echo -e "${PASS} go vet 通过"

# ---------------------------------------------------------------------------
# 4. Go build
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [4/6] Go 编译检查"
if ! go build -o /dev/null ./...; then
    echo -e "${FAIL} go build 失败"
    exit 1
fi
echo -e "${PASS} go build 通过"

# ---------------------------------------------------------------------------
# 5. Go test (scoring + game state machine + all packages, with race detector)
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} [5/6] Go 单元测试"
if ! go test -v -count=1 -race ./...; then
    echo -e "${FAIL} go test 失败"
    exit 1
fi
echo -e "${PASS} go test 通过"

# ---------------------------------------------------------------------------
# 6. Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${INFO} ========================================"
echo -e "${PASS} 全部验证通过！"
echo -e "${INFO} ========================================"
