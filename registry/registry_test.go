package registry

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testdefaultRegistry = testRegistry{nodes: map[string]NodeInfo{}}

type testRegistry struct {
	nodes map[string]NodeInfo
}

func (r *testRegistry) Initial(info NodeInfo) bool {
	r.nodes[info.Addr] = info
	return true
}

func (r *testRegistry) AllNodeInfo() []NodeInfo {
	res := []NodeInfo{}
	for _, val := range r.nodes {
		res = append(res, val)
	}
	return res
}

func (r *testRegistry) Borrow(addr, target string, cap int) bool { // 向其他节点借
	cap1, cap2 := r.GetCap(addr), r.GetCap(target)
	if cap2 < cap {
		return false
	}
	r.SetCap(addr, cap1+cap)
	r.SetCap(target, cap2-cap)
	return true
}

func (r *testRegistry) GetCap(addr string) int { // 更新自己节点的容量值
	if val, ok := r.nodes[addr]; ok {
		return val.Cap
	}
	return -1
}

func (r *testRegistry) SetCap(addr string, cap int) bool { // 更新自己节点的容量值
	if val, ok := r.nodes[addr]; ok {
		val.Cap = cap
		r.nodes[addr] = val
		return true
	}
	return false
}

func TestMain(m *testing.M) {
	// mock 3 node
	testdefaultRegistry.Initial(NodeInfo{Addr: "app1", Cap: 100, Avaliable: 100})
	testdefaultRegistry.Initial(NodeInfo{Addr: "app2", Cap: 100, Avaliable: 100})
	testdefaultRegistry.Initial(NodeInfo{Addr: "app3", Cap: 100, Avaliable: 100})

	os.Exit(m.Run())
}

func TestRegistryNodeInfo(t *testing.T) {
	assert.Equal(t, testdefaultRegistry.GetCap("app1"), 100)
	assert.Equal(t, testdefaultRegistry.GetCap("app2"), 100)
	assert.Equal(t, testdefaultRegistry.GetCap("app3"), 100)

	assert.True(t, testdefaultRegistry.Borrow("app1", "app2", 50))
	assert.False(t, testdefaultRegistry.Borrow("app1", "app2", 51))

	assert.Equal(t, testdefaultRegistry.GetCap("app1"), 150)
	assert.Equal(t, testdefaultRegistry.GetCap("app2"), 50)
}
