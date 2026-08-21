package collector

import syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"

const MaxBatchSize = 1024 * 64

const (
	LimitCol            = 300
	LimitDeck           = 300
	LimitNoteType       = 64
	LimitNote           = 64
	LimitProcessingNote = 64
	LimitCard           = 400
	LimitReviewLog      = 500
)

type PullCollector struct {
	syncChanges []*syncv1.SyncChange

	// 实际大小，单位为字节
	actualSize int
	maxUSN     int64
}

type CollectResult struct {
	SyncCursorUsn int64
	HasMore       bool
	SizeExceeded  bool
}

func NewPullCollector() *PullCollector {
	return &PullCollector{
		syncChanges: make([]*syncv1.SyncChange, 0, 64),
		actualSize:  0,
		maxUSN:      0,
	}
}

func (c *PullCollector) recordMaxUSN(usn int64) {
	c.maxUSN = max(usn, c.maxUSN)
}

func (c *PullCollector) MaxUSN() int64 {
	return c.maxUSN
}

func (c *PullCollector) IsFull() bool {
	return c.actualSize >= MaxBatchSize
}

func (c *PullCollector) Changes() []*syncv1.SyncChange {
	return c.syncChanges
}
