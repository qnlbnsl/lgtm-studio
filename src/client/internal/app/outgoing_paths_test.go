package app

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/link"
)

func TestOutgoingDirectReadMasksRecordAtEmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const value = "log-emission-sign-in-fixture"
		reading, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		markReading := sync.OnceFunc(func() { close(reading) })
		releaseRead := sync.OnceFunc(func() { close(release) })
		defer releaseRead()
		filter := link.NewRefreshingOutgoingFilter(func() []string {
			markReading()
			<-release
			return []string{value}
		})
		var output bytes.Buffer
		logger := newOutgoingLogger(&output, filter)
		go func() {
			defer close(done)
			logger.Info("fixture log", "value", directLogValue{
				started: make(chan struct{}),
				value:   "before-" + value + "-after",
			})
		}()

		synctest.Wait()
		if output.Len() != 0 {
			t.Error("the writer received a log record before the sign-in read returned")
		}
		select {
		case <-reading:
		default:
			t.Error("the log emission did not start a fresh sign-in read")
		}

		releaseRead()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("the log emission did not finish after release")
		}
		var row struct {
			Msg   string `json:"msg"`
			Value string `json:"value"`
		}
		decoder := json.NewDecoder(&output)
		if err := decoder.Decode(&row); err != nil || row.Msg != "fixture log" ||
			row.Value != "before-[redacted]-after" || decoder.Decode(&row) != io.EOF {
			t.Error("the writer did not receive exactly one masked log record after release")
		}
	})
}
