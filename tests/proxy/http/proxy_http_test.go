package http

import (
	"context"
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	pe "github.com/edgewize/edgeQ/internal/proxy/endpoint"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/endpoint"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"
)

var _ = Describe("ReverseProxy", func() {
	var (
		backendServer *httptest.Server
		proxyServer   endpoint.QosEndpoint
	)

	BeforeEach(func() {
		// 初始化模拟后端
		backendServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("Hello World"))
		}))

		backendURL, err := url.Parse(backendServer.URL)
		if err != nil {
			Fail(fmt.Sprintf("初始化测试后端服务器失败 %v", err))
		}

		err = os.Setenv("TARGET_PORT", backendURL.Port())
		if err != nil {
			Fail(fmt.Sprintf("获取后端服务器端口失败 %v", err))
		}

		err = os.Setenv("TARGET_ADDRESS", backendURL.Hostname())
		if err != nil {
			Fail(fmt.Sprintf("获取后端服务器地址失败 %v", err))
		}

		proxyConf := proxyCfg.New()
		proxyConf.Proxy.Type = constants.HTTPEndpoint
		proxyServer, err = pe.GetProxyEndpoint(proxyConf)
		if err != nil {
			Fail(fmt.Sprintf("创建 proxy 服务器失败 %v", err))
		}

		go func() {
			_ = proxyServer.Start(context.Background())
		}()
	})

	AfterEach(func() {
		backendServer.Close()
		proxyServer.Stop()
		time.Sleep(time.Second * 3)
	})

	Context("正常请求场景", func() {
		It("应成功转发请求", func() {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s", constants.DefaultProxyContainerPort))
			Expect(err).To(BeNil())
			defer resp.Body.Close()

			By("验证状态码")
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))

			body, err := io.ReadAll(resp.Body)
			By("检查返回报文")
			Expect(err).To(BeNil())
			Expect(string(body)).Should(Equal("Hello World"))
		})
	})

	Context("异常处理场景", func() {
		It("应返回502状态码当后端不可用时", func() {
			// 显式关闭后端服务
			backendServer.Close()

			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s", constants.DefaultProxyContainerPort))
			if err != nil {
				fmt.Printf("返回错误 %v\n", err)
			}
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			By("验证网关错误状态")
			Expect(resp.StatusCode).Should(Equal(http.StatusBadGateway))
		})
	})
})
