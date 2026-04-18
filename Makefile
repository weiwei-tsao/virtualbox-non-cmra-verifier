.PHONY: help start \
	api-server api-test \
	check-firestore check-rdi \
	migrate-clean-dry migrate-clean migrate-clean-all-dry migrate-clean-all \
	migrate-add-source migrate-validation-fields \
	force-revalidate fix-phantom-runs \
	docker-build docker-up docker-down docker-logs docker-rebuild-api docker-clean

help: ## 显示帮助信息
	@echo "可用命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'

# ── 开发命令 ──────────────────────────────────────────────────────────────────

start: ## 一键启动本地开发环境（API + Web，自动创建 .env.local）
	@bash start.sh

api-server: ## 仅启动 API 服务器
	@echo "启动 API 服务器..."
	cd apps/api && go run ./cmd/server

api-test: ## 运行 API 单元测试
	@echo "运行 API 测试..."
	cd apps/api && go test ./...

# ── 数据检查命令 ──────────────────────────────────────────────────────────────

check-firestore: ## 检查 Firestore 数据
	@echo "检查 Firestore 数据..."
	cd apps/api && go run ./cmd/check-firestore

check-rdi: ## 检查 RDI 数据分布
	@echo "检查 RDI 数据..."
	cd apps/api && go run ./cmd/check-rdi

# ── 迁移命令 ──────────────────────────────────────────────────────────────────

migrate-clean-dry: ## 预览地址清洗（iPost1，dry-run）
	@echo "预览 iPost1 地址清洗..."
	cd apps/api && go run ./cmd/migrate-clean-addresses --dry-run --source=iPost1

migrate-clean: ## 执行地址清洗（iPost1）
	@echo "执行 iPost1 地址清洗..."
	cd apps/api && go run ./cmd/migrate-clean-addresses --source=iPost1

migrate-clean-all-dry: ## 预览所有来源地址清洗（dry-run）
	@echo "预览所有来源地址清洗..."
	cd apps/api && go run ./cmd/migrate-clean-addresses --dry-run --source=

migrate-clean-all: ## 执行所有来源地址清洗
	@echo "执行所有来源地址清洗..."
	cd apps/api && go run ./cmd/migrate-clean-addresses --source=

migrate-add-source: ## 迁移：为现有记录补填 source 字段
	@echo "迁移 source 字段..."
	cd apps/api && go run ./cmd/migrate-add-source

migrate-validation-fields: ## 迁移：初始化 validation 状态字段
	@echo "迁移 validation 字段..."
	cd apps/api && go run ./cmd/migrate-validation-fields

# ── 运维脚本 ──────────────────────────────────────────────────────────────────

force-revalidate: ## 强制将指定数量地址标记为 needs_revalidation（默认 1000 条）
	@echo "强制重验地址..."
	cd apps/api && go run ./scripts/force_revalidate

fix-phantom-runs: ## 修复卡住的 validation run（状态永久 running 的异常记录）
	@echo "修复 phantom runs..."
	cd apps/api && go run ./scripts/fix_phantom_runs

# ── Docker 命令 ──────────────────────────────────────────────────────────────

docker-build: ## 构建所有 Docker 镜像（需要 .env）
	docker compose build

docker-up: ## 构建并后台启动所有容器
	docker compose up -d --build

docker-down: ## 停止并删除容器（保留镜像）
	docker compose down

docker-logs: ## 实时查看 API 容器日志
	docker compose logs -f api

docker-rebuild-api: ## 仅重新构建并重启 API 容器（代码改动后使用）
	docker compose up -d --build api

docker-clean: ## 删除所有容器、镜像及构建缓存
	docker compose down --rmi local --volumes
	docker builder prune -f
