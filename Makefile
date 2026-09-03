# =============================================================================
# zoekt — 雀友记 麻将牌局积分记录工具
# Makefile 提供统一验证入口 (PRD §10.2)
# =============================================================================

.PHONY: verify backend-test backend-build backend-vet backend-fmt backend-tidy backend-run clean

## verify: 执行全量验证流程（PRD §10.2）
verify:
	@bash scripts/verify.sh

## run: 启动后端开发服务器（使用默认配置）
run: backend-run

## run-config: 使用指定配置文件启动后端
# 用法: make run-config CONFIG=backend/config.yaml
run-config:
	@cd backend && go run ./cmd/server -config=$(CONFIG)

## backend-fmt: Go 格式检查
backend-fmt:
	@cd backend && gofmt -l .

## backend-vet: Go vet 检查
backend-vet:
	@cd backend && go vet ./...

## backend-tidy: Go mod tidy 检查
backend-tidy:
	@cd backend && go mod tidy -diff 2>/dev/null || (echo "go.mod 不整洁，请运行 go mod tidy" && exit 1)

## backend-build: Go 编译
backend-build:
	@cd backend && go build -o /dev/null ./...

## backend-test: Go 单元测试（含竞态检测）
backend-test:
	@cd backend && go test -v -count=1 -race ./...

## backend-run: 启动后端 API 服务（端口 8080）
backend-run:
	@cd backend && go run ./cmd/server -config=config.yaml

## clean: 清理构建产物
clean:
	@cd backend && go clean

# 生成 JWT secret，复制到 configs/config.yaml 的 jwt.secret
jwt-secret:
	@openssl rand -base64 32