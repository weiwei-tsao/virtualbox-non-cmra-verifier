.PHONY: help test-ipost1-py test-ipost1-go ipost1-setup \
	docker-build docker-up docker-down docker-logs docker-rebuild-api docker-clean

help: ## 显示帮助信息
	@echo "可用命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# iPost1 相关命令（实现完成后启用）
# ipost1-crawl: ## 运行 iPost1 爬取任务
# 	@echo "🚀 启动 iPost1 爬取..."
# 	curl -X POST http://localhost:8080/api/crawl/ipost1/run

# 现有项目命令（可扩展）
api-server: ## 启动 API 服务器
	@echo "🚀 启动 API 服务器..."
	cd apps/api && go run cmd/server/main.go

api-test: ## 运行 API 单元测试
	@echo "🧪 运行 API 测试..."
	cd apps/api && go test ./...

check-firestore: ## 检查 Firestore 数据
	@echo "🔍 检查 Firestore 数据..."
	cd apps/api && go run cmd/check-firestore/main.go

# 迁移命令
migrate-clean-dry: ## 预览 iPost1 地址清洗（dry-run）
	@echo "🔍 预览 iPost1 地址清洗..."
	cd apps/api && go run cmd/migrate-clean-addresses/main.go --dry-run --source=iPost1

migrate-clean: ## 执行 iPost1 地址清洗
	@echo "🧹 执行 iPost1 地址清洗..."
	cd apps/api && go run cmd/migrate-clean-addresses/main.go --source=iPost1

migrate-clean-all-dry: ## 预览所有来源地址清洗（dry-run）
	@echo "🔍 预览所有来源地址清洗..."
	cd apps/api && go run cmd/migrate-clean-addresses/main.go --dry-run --source=

migrate-clean-all: ## 执行所有来源地址清洗
	@echo "🧹 执行所有来源地址清洗..."
	cd apps/api && go run cmd/migrate-clean-addresses/main.go --source=

# 文档命令
docs: ## 打开 iPost1 文档
	@echo "📚 iPost1 相关文档:"
	@echo "  - 实现方案: docs/ipost1_scraper_analysis.md"
	@echo "  - 快速开始: docs/ipost1_README.md"
	@echo "  - 项目 PRD: docs/US_VirtualBox_Non-CMRA_Verification_prd_en.md"

# ── Docker 命令 ──────────────────────────────────────────────────────────────

docker-build: ## 构建所有 Docker 镜像（需要 .env.docker）
	docker compose --env-file .env.docker build

docker-up: ## 构建并后台启动所有容器
	docker compose --env-file .env.docker up -d --build

docker-down: ## 停止并删除容器（保留镜像）
	docker compose down

docker-logs: ## 实时查看 API 容器日志
	docker compose logs -f api

docker-rebuild-api: ## 仅重新构建并重启 API 容器（代码改动后使用）
	docker compose --env-file .env.docker up -d --build api

docker-clean: ## 删除所有容器、镜像及构建缓存
	docker compose down --rmi local --volumes
	docker builder prune -f
