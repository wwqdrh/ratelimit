package registry

type NodeInfo struct {
	Addr      string
	Cap       int
	Avaliable int
}

// 中心节点的接口，使用方可以自行选择redis或者etcd
type IRegistry interface {
	Initial(info NodeInfo) bool
	AllNodeInfo() []NodeInfo
	Borrow(addr, target string, cap int) bool // 向其他节点借
	GetCap(addr string) int                   // 获取容量更新自己节点的容量值
	SetCap(addr string, cap int) bool
}
