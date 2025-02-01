package holder

import (
	"context"
	"github.com/edgewize/modelmesh/pkg/utils"
	"reflect"
	"testing"
	"time"
)

func Test_holder_Wait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	holder := New(ctx, time.Duration(5)*time.Second)

	reqID := utils.ReqUUID()

	// Mock a response
	go func() {
		time.Sleep(3 * time.Second)
		holder.Notify(&Response{
			ID:     reqID,
			Status: StatusOK,
		})
	}()

	t.Logf("wait reqID: %s", reqID)
	ret := holder.Wait(ctx, reqID)
	t.Logf("wait reqID: %s, ret: %v", reqID, ret)

	if !reflect.DeepEqual(ret.ID, reqID) {
		t.Errorf("ret ID = %v, reqID = %v", ret.ID, reqID)
	}
}
