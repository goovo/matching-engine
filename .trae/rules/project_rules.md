**项目架构设计**
- 根目录核心文件与目录
  - `main.go`：进程入口，启动 gRPC 服务并注册处理器
  - `engine/`：撮合核心逻辑与数据结构（订单、订单簿、撮合流程、节点与类型）
  - `server/`：gRPC 服务实现，把 RPC 请求转换为引擎操作
  - `engineGrpc/`：由 `engine.proto` 生成的 gRPC 代码
  - `util/`：工具库，这里封装了高精度十进制运算
  - 其他：`engine.proto`（服务与消息定义）、`Dockerfile`、`Makefile`（proto 编译）、`go.mod`、`go.sum`
- 组织思路
  - 清晰分层：`server` 负责 RPC 接入与入参校验，`engine` 负责核心业务算法与状态管理，`util` 提供通用数值工具
  - 每个交易对维护一个独立的订单簿：`map[string]*engine.OrderBook`，确保不同 `pair` 的撮合相互隔离（`server/engine.go:15–22`）

**主要依赖库**
- 直接依赖（摘自 `go.mod`）
  - `github.com/Pantelwar/binarytree v1.0.0`：二叉树数据结构，用于价格层级管理（`engine/order_book.go:11`）
  - `github.com/Pantelwar/matching-engine v1.0.6`：自引用模块，承载核心引擎逻辑
  - `github.com/golang/protobuf v1.5.2`：Protocol Buffers（旧版 API）
  - `github.com/shopspring/decimal v1.3.1`：高精度十进制，封装在 `util.StandardBigDecimal`（`util/bigdecimal.go:4–10`）
  - `google.golang.org/grpc v1.48.0`：gRPC 框架（`main.go:19–23`）
- 间接依赖
  - `golang.org/x/net`、`x/sys`、`x/text`、`google.golang.org/genproto`、`google.golang.org/protobuf` 等

**入口文件和执行流程**
- 入口与启动
  - `main.go` 中创建 gRPC 服务器，注册引擎服务并开启监听（`main.go:19–34`）
  - 关键步骤
    - `grpc.NewServer()` 启动 gRPC 服务（`main.go:20`）
    - `server.NewEngine()` 构造业务处理器（`main.go:21`）
    - `engineGrpc.RegisterEngineServer(gs, cs)` 注册 RPC（`main.go:22`）
    - `reflection.Register(gs)` 开启反射，便于工具查询服务（`main.go:24`）
    - `net.Listen("tcp", ":9000")` 并 `Serve`（`main.go:26–33`）
- 请求处理总览（以 `Process` 为例）
  - `server.Engine.Process` 接收 `Order`，组装为引擎内部 `Order` 并校验（`server/engine.go:25–41`）
  - 基于 `pair` 选择或新建订单簿（`server/engine.go:48–54`）
  - 调用 `OrderBook.Process` 进行限价撮合，返回成交与剩余部分（`server/engine.go:56–79`）
  - 结果序列化为字符串返回（`OutputOrders` 的两个字段，`server/engine.go:58–79`）

**配置文件结构**
- 未使用外部配置文件或环境变量，端口常量硬编码为 `":9000"`（`main.go:15–17`）
- 容器暴露端口为 `9099`，与应用监听端口不一致（`Dockerfile:25` vs `main.go:16`）
- 不使用 `viper`/`flag`/`env` 等配置机制；如需多环境支持，建议引入环境变量或启动参数来配置端口与其他选项

**数据库连接方式**
- 未使用外部数据库，撮合状态完全在内存中维护
  - 每个 `pair` 一个 `OrderBook`（`server/engine.go:15–22, 48–54`）
  - 订单簿内部用两棵二叉树分别维护买、卖价层（`engine/order_book.go:16–22, 211–223`）
  - 每个价格节点存放一个 `OrderNode`，包含订单队列与聚合成交量（`engine/order_node.go:8–17`）
- 并发控制
  - 使用 `sync.Mutex` 保护订单索引映射 `orders` 的读写（`engine/order_book.go:21–22, 256–258, 291–293`）
  - 树操作本身未见全局锁，默认在单线程或受控并发环境下运行；若高并发 RPC 写入同一 `OrderBook`，需进一步加锁或队列化

**API 路由设计**
- 协议与服务
  - gRPC 服务名 `Engine`，定义在 `engine.proto`（`engine.proto:3–8`）
- 方法与语义
  - `Process(Order) returns (OutputOrders)`：限价单撮合（`engine.proto:4`，实现 `server/engine.go:24–80`）
  - `ProcessMarket(Order) returns (OutputOrders)`：市价单撮合（`engine.proto:5`，实现 `server/engine.go:123–174`）
  - `Cancel(Order) returns (Order)`：按 `ID` 撤单（`engine.proto:6`，实现 `server/engine.go:82–121`）
  - `FetchBook(BookInput) returns (BookOutput)`：查询买卖盘聚合数据（`engine.proto:7`，实现 `server/engine.go:176–230`）
- 消息结构
  - `Order`：`Type`/`ID`/`Amount`/`Price`/`Pair`（`engine.proto:10–16`）
  - `OutputOrders`：字符串化的已处理订单与剩余订单（`engine.proto:18–21`）
  - `BookInput`：`pair` 与 `limit`（`engine.proto:28–31`）
  - `BookOutput`：买卖盘数组，每项为 `BookArray`（`engine.proto:37–40`）
- 返回格式说明
  - `OutputOrders` 中两个字段是 JSON 字符串而非嵌套消息，便于与引擎内部结构直接序列化对接（`server/engine.go:58–79, 157–173`）

**中间件使用情况**
- 未使用 gRPC 拦截器或其他中间件（创建服务器时未配置 `UnaryInterceptor`/`StreamInterceptor`）
- 启用了 gRPC 反射，方便用 `grpcurl` 等调试（`main.go:24`）
- 如需日志、鉴权、速率限制等，可通过 gRPC 拦截器链式挂载

**补充观察**
- 订单解析与校验
  - `engine.Order.UnmarshalJSON` 做了字段校验与数值解析（`engine/order.go:56–100`）
  - `server.Process` 通过手工拼接 JSON 再反序列化为引擎 `Order`，可以改为直接构造 `engine.Order` 以减少转换开销（`server/engine.go:27–36`）
- 订单簿展示
  - `FetchBook` 返回的是价格与累积量的字符串数组，买盘倒序、卖盘正序（`engine/order_book.go:98–145`）
- 可用测试
  - `engine/` 下含多组测试覆盖核心撮合与类型逻辑（如 `order_book_test.go`, `process_limit_order_test.go` 等）