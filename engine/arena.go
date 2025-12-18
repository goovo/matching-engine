package engine

// IndexType 定义 Arena 索引类型
type IndexType int32

const (
	// NullIndex 表示空索引（类似于 nil）
	NullIndex IndexType = -1

	// 分页参数：2^16 = 65536
	PageShift = 16
	PageSize  = 1 << PageShift
	PageMask  = PageSize - 1
)

// OrderArena 订单内存池（分页式连续内存布局）
type OrderArena struct {
	pages    [][]Order // 二维切片：页 -> 订单数组
	freeHead IndexType // 空闲链表头
}

// NewOrderArena 创建一个新的 Arena
// capacity: 初始预估容量（仅用于预分配第一页或 pages 数组，实际上按需增长）
func NewOrderArena(capacity int) *OrderArena {
	// 估算需要多少页
	numPages := (capacity + PageSize - 1) / PageSize
	if numPages < 1 {
		numPages = 1
	}
	
	arena := &OrderArena{
		pages:    make([][]Order, 0, numPages), // pages 索引本身的容量
		freeHead: NullIndex,
	}
	// 预分配第一页
	arena.pages = append(arena.pages, make([]Order, 0, PageSize))
	return arena
}

// Alloc 分配一个新的订单槽位，返回索引
func (a *OrderArena) Alloc() IndexType {
	// 1. 优先从空闲链表复用
	if a.freeHead != NullIndex {
		idx := a.freeHead
		// 解析位置
		pageIdx := int(idx) >> PageShift
		offset := int(idx) & PageMask
		
		// 此时 Next 字段存储的是下一个空闲节点的索引
		a.freeHead = a.pages[pageIdx][offset].Next
		return idx
	}

	// 2. 追加新节点
	lastPageIdx := len(a.pages) - 1
	// 检查当前页是否已满
	if len(a.pages[lastPageIdx]) >= PageSize {
		// 新开一页
		a.pages = append(a.pages, make([]Order, 0, PageSize))
		lastPageIdx++
	}

	// 在当前页追加
	// 注意：cap 足够，append 不会触发重新分配底层数组，只会更新 slice header
	// 指针稳定性得到保证
	idxInPage := len(a.pages[lastPageIdx])
	a.pages[lastPageIdx] = append(a.pages[lastPageIdx], Order{})

	return IndexType((lastPageIdx << PageShift) | idxInPage)
}

// Free 释放指定索引的订单
func (a *OrderArena) Free(idx IndexType) {
	if idx == NullIndex {
		return
	}
	pageIdx := int(idx) >> PageShift
	offset := int(idx) & PageMask
	
	// 简单的边界检查
	if pageIdx >= len(a.pages) || offset >= len(a.pages[pageIdx]) {
		return
	}

	// 将该节点插入空闲链表头部
	a.pages[pageIdx][offset].Next = a.freeHead
	a.freeHead = idx
}

// Get 通过索引获取订单指针
// 这是一个极高频的操作，必须内联且高效
func (a *OrderArena) Get(idx IndexType) *Order {
	// 使用位运算快速定位
	// Go 编译器通常能很好地优化这种模式
	return &a.pages[int(idx)>>PageShift][int(idx)&PageMask]
}

// Reset 重置 Arena（用于测试）
func (a *OrderArena) Reset() {
	// 简单粗暴：清空 pages 列表，旧内存由 GC 回收
	// 或者保留 pages 结构但重置 len=0 (复用内存)
	// 为了最彻底的清洁，我们只保留第一页并重置
	if len(a.pages) > 0 {
		// 复用第一页的底层数组，避免反复 alloc
		a.pages[0] = a.pages[0][:0]
		a.pages = a.pages[:1]
	} else {
		a.pages = append(a.pages, make([]Order, 0, PageSize))
	}
	a.freeHead = NullIndex
}
