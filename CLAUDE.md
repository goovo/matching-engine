# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供该代码库的使用指南。

## 项目概述

这是一个用 Go 语言编写的高性能加密货币撮合引擎，用于在数字资产交易所撮合买卖订单。引擎支持：
- 限价订单
- 市价订单
- 订单取消
- 订单簿管理

## 架构设计

撮合引擎使用嵌套二叉树结构以实现高效的订单撮合：
- **OrderBook**：包含 BuyTree 和 SellTree，每个都是按价格作为键的二叉树
  - 买单：以最高买价作为根节点存储（降序排列）
  - 卖单：以最低卖价作为根节点存储（升序排列）
- **OrderType**：每个价格水平包含另一个按订单 ID 作为键的二叉树，用于实现先进先出撮合
- **Orders**：使用 `StandardBigDecimal`（shopspring/decimal 的封装）进行精确的财务计算
- **并发性**：每个 OrderBook 拥有自己的互斥锁，确保线程安全

`engine/` 目录中的关键数据结构：
- `order_book.go` - 主要 OrderBook 协调逻辑
- `order.go` - 订单结构体，包含 JSON 序列化功能
- `order_type.go` - OrderType，用于管理同一价格水平的订单
- `process_limit_order.go` - 核心限价订单撮合逻辑
- `process_market_order.go` - 市价订单执行逻辑
- `cancel_limit_order.go` - 订单取消逻辑
- `trade.go` - 交易执行（目前功能较少）
- `side.go` - 买卖方向枚举

## gRPC API

服务器通过 gRPC 在 9000 端口暴露四个主要接口：
- `Process` - 提交限价订单
- `ProcessMarket` - 提交市价订单
- `Cancel` - 取消已有订单
- `FetchBook` - 获取订单簿状态

Protocol Buffer 定义位于 `engine.proto`。gRPC 服务实现在 `server/engine.go` 中。

## 构建与开发

```bash
# 安装 Go 依赖
go mod download

# 构建项目
go build -o engine

# 运行测试
go test ./...

# 生成 protobuf 代码（需要 protoc-gen-go）
make proto
```

## 测试

项目拥有全面的测试覆盖。运行特定测试文件：
```bash
# 运行所有 engine 测试
go test ./engine/...

# 运行特定测试
go test -run TestProcessLimitOrder ./engine

# 运行基准测试
go test -bench=. ./engine/
```

## Docker

```bash
# 构建 Docker 镜像
docker build -t matching-engine .

# 运行容器
docker run -p 9000:9000 matching-engine
```

## 关键实现细节

- 订单撮合遵循价格-时间优先原则：优先最佳价格，然后是最早的订单
- 来自 `github.com/goovo/binarytree` 的二叉树结构实现了 O(log n) 操作复杂度
- 所有财务计算使用十进制算术，避免浮点数精度问题
- 每个交易对都在 Engine 的 `book` 映射中有自己独立的 OrderBook 实例
- Order 结构体的 JSON 标签便于测试和调试时的序列化