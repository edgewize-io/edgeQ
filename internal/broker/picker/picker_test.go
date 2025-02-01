package picker_test

import (
	"fmt"
	"github.com/edgewize/modelmesh/internal/broker/picker"
	xconfig "github.com/edgewize/modelmesh/pkg/config"
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
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service10", Weight: 10})
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service20", Weight: 20})
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service30", Weight: 30})
			p = b.Build(pbi)
			c = NewCount()

			for i := 0; i < cnt; i++ {
				ret, err := p.Pick(picker.PickInfo{})
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
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service10", Weight: 10})
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service20", Weight: 20})
			pbi.AddResource(&xconfig.ServiceGroup{Name: "service30", Weight: 30})
			p = b.Build(pbi)
			c = NewCount()

			for i := 0; i < cnt; i++ {
				ret, err := p.Pick(picker.PickInfo{})
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
				Expect(c.count["service10"]).To(Equal(cnt * 10 / 60))
			})
			It(fmt.Sprintf("service20 should be select 20/60 * %d = %d", cnt, cnt*2/6), func() {
				Expect(c.count["service20"]).To(Equal(cnt * 20 / 60))
			})
			It(fmt.Sprintf("service30 should be select 30/60 * %d = %d", cnt, cnt*3/6), func() {
				Expect(c.count["service30"]).To(Equal(cnt * 30 / 60))
			})
		})
	})

})

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
