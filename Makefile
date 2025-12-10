.PHONY: help test-ipost1-py test-ipost1-go ipost1-setup

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

# 文档命令
docs: ## 打开 iPost1 文档
	@echo "📚 iPost1 相关文档:"
	@echo "  - 实现方案: docs/ipost1_scraper_analysis.md"
	@echo "  - 快速开始: docs/ipost1_README.md"
	@echo "  - 项目 PRD: docs/US_VirtualBox_Non-CMRA_Verification_prd_en.md"
