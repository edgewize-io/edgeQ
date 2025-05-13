package grpc

import (
	"context"
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	pg "github.com/edgewize/edgeQ/internal/proxy/endpoint/grpc"
	"github.com/edgewize/edgeQ/pkg/constants"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"io"
	"net"
	"time"

	proxyTest "github.com/edgewize/edgeQ/tests/proto"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("双向流代理测试", func() {
	var (
		proxyServer   *pg.GRPCProxyServer
		backendServer *grpc.Server
		userClient    proxyTest.BackendServiceClient
		//backendConn   *grpc.ClientConn
	)

	BeforeEach(func() {
		// 启动后端服务
		var err error
		backendServer = grpc.NewServer()
		proxyTest.RegisterBackendServiceServer(backendServer, &mockBackend{})
		lis, err := net.Listen("tcp", "0.0.0.0:12001")
		Expect(err).To(BeNil())

		go backendServer.Serve(lis)

		// 初始化代理服务
		proxyConf := proxyCfg.New()
		proxyConf.Proxy.Type = constants.GRPCEndpoint
		proxyConf.Proxy.GRPC.BackendAddr = lis.Addr().String()

		proxyServer, err = pg.NewGRPCProxyServer(proxyConf)
		if err != nil {
			Fail(fmt.Sprintf("创建 proxy 服务器失败 %v", err))
		}

		go proxyServer.Start(context.Background())

		conn, err := grpc.Dial(
			fmt.Sprintf("127.0.0.1:%s", constants.DefaultProxyContainerPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			Fail(fmt.Sprintf("创建 客户端 -> 代理服务器 失败 %v", err))
		}

		userClient = proxyTest.NewBackendServiceClient(conn)
	})

	AfterEach(func() {
		proxyServer.Stop()
		backendServer.Stop()
		time.Sleep(time.Second * 2)
	})

	Context("正常流式传输", func() {
		It("应正确转发双向数据", func() {
			stream, err := userClient.BidirectionalStream(context.Background())
			Expect(err).NotTo(HaveOccurred())

			By("发送测试数据")
			go func() {
				for i := 0; i < 3; i++ {
					stream.Send(&proxyTest.DataChunk{Content: []byte{byte(i)}})
				}
				stream.CloseSend()
			}()

			By("接收响应验证")
			for i := 0; i < 3; i++ {
				data, err := stream.Recv()
				Expect(err).NotTo(HaveOccurred())
				Expect(data.Content[0]).To(Equal(byte(i + 10))) // 假设后端返回数据+10
			}

			_, err = stream.Recv()
			Expect(err).To(Equal(io.EOF))
		})
	})

})

// 模拟后端服务实现
type mockBackend struct{}

func (m *mockBackend) BidirectionalStream(stream proxyTest.BackendService_BidirectionalStreamServer) error {
	// 处理元数据
	stream.SendHeader(metadata.Pairs("proxy-header", "ginkgo-test"))

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 模拟处理逻辑：每个字节+10
		modified := &proxyTest.DataChunk{Content: []byte{data.Content[0] + 10}}
		if err := stream.Send(modified); err != nil {
			return err
		}
	}
}
