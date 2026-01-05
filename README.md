# Superfast Matching Engine
Improved matching engine written in Go (Golang)


## Installation

```javascript
go get github.com/goovo/matching-engine/engine
```


#### 1. 纯内存基准测试 (`cmd/bench-core`)，结果显示核心撮合引擎的性能非常优异：

* **单线程处理能力**：约 **167 万 TPS**

* **多线程并发能力**：约 **125 万 TPS**

* **市价单处理能力**：约 **934 万 TPS**

#### 2. 完整基准测试 (`cmd/bench`)，结果表明：算法效率逼近物理极限。


测试结果数据：

| 测试场景                | 耗时 (ns/op) | 内存分配 (B/op) | 吞吐量估算 (TPS)   |
|:------------------------|:-------------|:----------------|:-------------------|
| RDMA Limit Match (新)   | 383.7 ns     | 358 B           | ~260 万            |
| RDMA Market Match (新)  | 379.7 ns     | 377 B           | ~263 万            |
| Logic Limit Match (基准)| 480.7 ns     | 180 B           | ~208 万            |
| Logic Market Match (基准)| 480.3 ns     | 183 B           | ~208 万            |

#### 3. 结果分析
- 性能提升显著 ：RDMA 通道相比原有的基准测试快了约 20% （380ns vs 480ns）。
