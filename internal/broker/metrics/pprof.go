package metrics

import (
	"context"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net/http"
	_ "net/http/pprof"
	"time"
)

type PprofServer struct {
	*http.Server
	Enable bool
}

func NewPprofSrv(enable bool) *PprofServer {
	return &PprofServer{&http.Server{Addr: "0.0.0.0:6060", Handler: nil}, enable}
}

func (pp *PprofServer) Run() error {
	if !pp.Enable {
		return nil
	}

	if err := pp.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (pp *PprofServer) Stop() {
	if !pp.Enable {
		return
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	err := pp.Shutdown(shutdownCtx)
	if err != nil {
		klog.Errorf("stop pprof server failed, [%v]", err)
	}
}
