package logtest_test

import (
	"sync"
	"testing"
	"time"

	"github.com/aidnetic/logger-go"
	"github.com/aidnetic/logger-go/logtest"
)

func TestRecorder(t *testing.T) {
	log, rec := logtest.New()
	reqLog := log.With(logger.F("request_id", "r1"))
	reqLog.Debug("cache miss", logger.Group("cache", logger.F("key", "user:1")))
	reqLog.Warn("slow query", logger.F("took", 2*time.Second), logger.F("rows", 10))
	log.Fatal("boom")

	entries := rec.Entries()
	if len(entries) != 3 {
		t.Fatalf("got %d entries: %v", len(entries), entries)
	}
	e, ok := rec.Find("slow")
	if !ok || e.Level != logger.WARN {
		t.Fatalf("Find: %v %v", e, ok)
	}
	if e.Fields["request_id"] != "r1" || e.Fields["took"] != 2*time.Second || e.Fields["rows"] != int64(10) {
		t.Errorf("fields = %v", e.Fields)
	}
	if got := entries[0].Fields["cache.key"]; got != "user:1" {
		t.Errorf("group field = %v", got)
	}
	if codes := rec.ExitCodes(); len(codes) != 1 || codes[0] != 1 {
		t.Errorf("exit codes = %v", codes)
	}
	if n := len(rec.FindAll("o")); n != 2 {
		t.Errorf("FindAll matched %d entries, want 2", n)
	}
	rec.Reset()
	if rec.Len() != 0 || len(rec.ExitCodes()) != 0 {
		t.Error("Reset did not clear the recorder")
	}
}

func TestRecorderConcurrent(t *testing.T) {
	log, rec := logtest.New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				log.With(logger.F("j", j)).Info("x")
			}
		}()
	}
	wg.Wait()
	if rec.Len() != 1000 {
		t.Errorf("recorded %d entries, want 1000", rec.Len())
	}
}
