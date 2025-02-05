package picker_test

import (
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	xconfig "github.com/edgewize/edgeQ/pkg/config"
	"math"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

/*
## 调度器策略
调度器策略指标包括：
- 调度器策略的性能指标（这部分单独测试）
- 调度器策略的负载均衡情况（通过方差来判断）

调度器策略包括：
- RR：Round Robin 随机策略，ServiceGroup 中的每个 Service 依次被选中
- WRR：Weighted Round Robin 加权随机策略，ServiceGroup 中的每个 Service 依次被选中，但是根据权重来决定被选中的概率
- RWRR：Random Weighted Round Robin 随机加权策略，ServiceGroup 中的每个 Service 依次被选中，但是根据权重来决定被选中的概率，且每次选中的 Service 都是随机的
*/

var _ = Describe("Picker", func() {
	var (
		b   picker.PickerBuilder
		pbi *picker.PickerBuildInfo
		p   picker.Picker
		c   *Count
	)

	Context("RR", func() {
		var cnt = 300000
		BeforeEach(func() {
			b = picker.Builder("RR")
			pbi = picker.NewPickerBuildInfo()
			pbi.AddResource(xconfig.ServiceGroup{Name: "service10", Weight: 10})
			pbi.AddResource(xconfig.ServiceGroup{Name: "service20", Weight: 20})
			pbi.AddResource(xconfig.ServiceGroup{Name: "service30", Weight: 30})
			p = b.Build(pbi)
			c = NewCount()

			for i := 0; i < cnt; i++ {
				ret, err := p.Pick(picker.PickInfo{
					Candidates: []picker.Resource{
						&xconfig.ServiceGroup{Name: "service10"},
						&xconfig.ServiceGroup{Name: "service20"},
						&xconfig.ServiceGroup{Name: "service30"},
					},
				})
				Expect(err).To(BeNil())
				name := ret.Resource.ResourceName()
				c.Add(name)
			}

			DeferCleanup(func() {
				b = nil
				p = nil
			})
		})
		When("ServiceGroup 中的每个 Service 依次被选中", func() {
			It(fmt.Sprintf("service10 should be select 10/30 * %d = %d", cnt, cnt*1/3), func() {
				Expect(c.count["service10"]).To(Equal(cnt * 1 / 3))
			})
			It(fmt.Sprintf("service20 should be select 10/30 * %d = %d", cnt, cnt*1/3), func() {
				Expect(c.count["service20"]).To(Equal(cnt * 1 / 3))
			})
			It(fmt.Sprintf("service30 should be select 10/30 * %d = %d", cnt, cnt*1/3), func() {
				Expect(c.count["service30"]).To(Equal(cnt * 1 / 3))
			})
		})
	})

	Context("WRR", func() {
		var cnt = 300000
		BeforeEach(func() {
			b = picker.Builder("WRR")
			pbi = picker.NewPickerBuildInfo()
			pbi.AddResource(xconfig.ServiceGroup{Name: "service10", Weight: 10})
			pbi.AddResource(xconfig.ServiceGroup{Name: "service20", Weight: 20})
			pbi.AddResource(xconfig.ServiceGroup{Name: "service30", Weight: 30})
			p = b.Build(pbi)
			c = NewCount()

			for i := 0; i < cnt; i++ {
				ret, err := p.Pick(picker.PickInfo{
					Candidates: []picker.Resource{
						&xconfig.ServiceGroup{Name: "service10"},
						&xconfig.ServiceGroup{Name: "service20"},
						&xconfig.ServiceGroup{Name: "service30"},
					},
				})
				Expect(err).To(BeNil())
				name := ret.Resource.ResourceName()
				c.Add(name)
			}

			DeferCleanup(func() {
				b = nil
				p = nil
			})
		})
		When("ServiceGroup 中的每个 Service 按照 wight 被选中", func() {
			It(fmt.Sprintf("service10 should be select 10/60 * %d = %d", cnt, cnt*1/6), func() {
				Expect(isWithin5Percent(float64(c.count["service10"]), float64(cnt*10/60))).To(BeTrue())
			})
			It(fmt.Sprintf("service20 should be select 20/60 * %d = %d", cnt, cnt*2/6), func() {
				Expect(isWithin5Percent(float64(c.count["service20"]), float64(cnt*20/60))).To(BeTrue())
			})
			It(fmt.Sprintf("service30 should be select 30/60 * %d = %d", cnt, cnt*3/6), func() {
				Expect(isWithin5Percent(float64(c.count["service30"]), float64(cnt*30/60))).To(BeTrue())
			})
		})
	})

})

func isWithin5Percent(a, b float64) bool {
	if a == 0 && b == 0 {
		return true // 都为0时视为相等
	}
	// 避免除以0错误，取较大值作为基准
	base := math.Max(math.Abs(a), math.Abs(b))
	if base == 0 {
		return false // 至少一个为0时无法比较
	}
	diff := math.Abs(a - b)
	percentage := (diff / base) * 100
	return percentage <= 5
}

type Count struct {
	sync.Locker
	count map[string]int
}

func NewCount() *Count {
	return &Count{
		Locker: &sync.Mutex{},
		count:  map[string]int{},
	}
}

func (c *Count) Add(s string) {
	c.Lock()
	defer c.Unlock()
	c.count[s]++
}

func (c *Count) Dec(s string) {
	c.Lock()
	defer c.Unlock()
	c.count[s]--
}

func (c *Count) Set(s string, n int) {
	c.Lock()
	defer c.Unlock()
	c.count[s] = n
}

func (c *Count) Reset() {
	c.Lock()
	defer c.Unlock()
	c.count = map[string]int{}
}
