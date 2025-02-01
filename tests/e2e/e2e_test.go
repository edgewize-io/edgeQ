package e2e_test

import (
	"context"
	"fmt"
	broker "github.com/edgewize/modelmesh/cmd/broker/app"
	brokeroptions "github.com/edgewize/modelmesh/cmd/broker/app/options"
	proxy "github.com/edgewize/modelmesh/cmd/proxy/app"
	proxyoptions "github.com/edgewize/modelmesh/cmd/proxy/app/options"
	brokerconfig "github.com/edgewize/modelmesh/internal/broker/config"
	proxyconfig "github.com/edgewize/modelmesh/internal/proxy/config"
	mindspore_serving_proto "github.com/edgewize/modelmesh/mindspore_serving/proto"
	"github.com/edgewize/modelmesh/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/smallnest/deepcopy"
	"go.uber.org/ratelimit"
	"k8s.io/klog/v2"
	"math"
	"time"
)

var ()

var _ = BeforeSuite(func() {

})

var _ = Describe("E2E test", func() {
	var (
		fss           *e2e.FakeServingServer
		serviceGroups []*brokerconfig.ServiceGroup
		brokerOpt     *brokeroptions.ServerRunOptions
		proxyOpts     []*proxyoptions.ServerRunOptions
		proxyOrigConf *proxyconfig.Config
		ctx           context.Context
		Metric        *e2e.Metric
		cancel        context.CancelFunc
		err           error
	)

	// 1 初始化 broker 和 proxy 的配置
	BeforeEach(func() {
		{
			conf, err := brokerconfig.TryLoadFromDisk()
			Expect(err).To(BeNil())
			conf.ServiceGroups = serviceGroups
			brokerOpt = &brokeroptions.ServerRunOptions{
				Config: conf,
			}
		}
		{
			proxyOrigConf, err = proxyconfig.TryLoadFromDisk()
			Expect(err).To(BeNil())
		}
	})

	// 3 启动 broker 和 proxy
	JustBeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(func() {
			cancel()
		})

		GoSleep(func() {
			Expect(fss).NotTo(BeNil())
			e2e.RunFakeServingServer(ctx, fss)
		}, 2)
		GoSleep(func() {
			Expect(brokerOpt).NotTo(BeNil())
			broker.Run(brokerOpt, brokerconfig.WatchConfigChange(), ctx)
		}, 2)
		GoSleep(func() {
			Expect(proxyOpts).NotTo(BeNil())
			for _, proxyOpt := range proxyOpts {
				go proxy.Run(proxyOpt, proxyconfig.WatchConfigChange(), ctx)
			}
		}, 1)

	})

	Context("WRR", func() {
		requestList := []e2e.RequestInfo{}
		// 2 再启动服务前初始化调度策略 & 设定 服务 Handle
		BeforeEach(func() {
			serviceGroups = []*brokerconfig.ServiceGroup{
				&brokerconfig.ServiceGroup{Name: "service-weight-2", Weight: 2},
				&brokerconfig.ServiceGroup{Name: "service-weight-20", Weight: 20},
				&brokerconfig.ServiceGroup{Name: "service-weight-200", Weight: 200},
			}
			for i := 0; i < len(serviceGroups); i++ {
				conf := deepcopy.Copy[*proxyconfig.Config](proxyOrigConf)
				conf.ProxyServer.Addr = fmt.Sprintf("127.0.0.1:%d", 8080+i)
				conf.ServiceGroup = serviceGroups[i]
				proxyOpts = append(proxyOpts, &proxyoptions.ServerRunOptions{Config: conf})
			}

			Expect(brokerOpt).NotTo(BeNil())
			brokerOpt.Config.Schedule.Method = "WRR"
			brokerOpt.Config.ServiceGroups = serviceGroups
			Expect(fss).To(BeNil())
			fss = &e2e.FakeServingServer{
				Handler: func(ctx context.Context, req *mindspore_serving_proto.PredictRequest) error {
					spec := req.ServableSpec
					requestList = append(requestList, e2e.RequestInfo{
						Name:          spec.Name,
						MethodName:    spec.MethodName,
						VersionNumber: int64(spec.VersionNumber),
						Count:         1,
					})
					time.Sleep(1 * time.Millisecond)
					return nil
				},
			}
			DeferCleanup(func() {
				fss = nil
			})
		})
		// 4 启动 client 测试
		JustBeforeEach(func() {
			for i := 0; i < len(proxyOpts); i++ {
				rl := ratelimit.New(1000)
				go (func(idx int) {
					opt := proxyOpts[idx]
					client, err := e2e.FakeClent(ctx, opt)
					method := fmt.Sprintf("weight-%d", opt.Config.ServiceGroup.Weight)
					Expect(err).To(BeNil())
					cnt := 0
					for {
						go func() {
							_, err := client.Predict(ctx, &mindspore_serving_proto.PredictRequest{
								ServableSpec: &mindspore_serving_proto.ServableSpec{
									Name:          opt.Config.ServiceGroup.Name,
									MethodName:    method,
									VersionNumber: uint64(cnt),
								},
							})
							if err != nil {
								cnt++
							}
						}()
						//time.Sleep(1 * time.Microsecond)
						rl.Take()
					}
				})(i)
			}
			time.Sleep(10 * time.Second)
			Metric = e2e.MetricFromStruct(requestList)
		})
		It("检查 broker 接收到的请求分布", func() {
			By("收集请求数据，size 为收到的总请求数，grouped 为按 Name 分组的请求数量")
			size := float64(Metric.NRow())
			grouped := Metric.GroupBy("Name")

			By("检查 service-weight-2 接收到的请求分布，期望为 2/(2+20+200)")
			GinkgoWriter.Println(grouped["service-weight-2"], size, float64(grouped["service-weight-2"])/size, float64(grouped["service-weight-2"])/size-2.0/(2+20+200))
			Expect(math.Abs(float64(grouped["service-weight-2"])/size-2.0/(2+20+200)) < 0.1).To(BeTrue())

			By("检查 service-weight-20 接收到的请求分布，期望为 20/(2+20+200)")
			GinkgoWriter.Println(grouped["service-weight-20"], size, float64(grouped["service-weight-20"])/size, float64(grouped["service-weight-20"])/size-20.0/(2+20+200))
			Expect(math.Abs(float64(grouped["service-weight-20"])/size-20.0/(2+20+200)) < 0.1).To(BeTrue())

			By("检查 service-weight-200 接收到的请求分布，期望为 200/(2+20+200)")
			GinkgoWriter.Println(grouped["service-weight-200"], size, float64(grouped["service-weight-200"])/size, float64(grouped["service-weight-200"])/size-200.0/(2+20+200))
			Expect(math.Abs(float64(grouped["service-weight-200"])/size-200.0/(2+20+200)) < 0.1).To(BeTrue())
		})
	})
})

func GoSleep(fn func(), sleep time.Duration) {
	go fn()
	klog.V(0).Infof("sleeping %d second...", sleep)
	time.Sleep((sleep) * time.Second)
}
