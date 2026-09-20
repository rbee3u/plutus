package jobs

const (
	// KindProjection rebuilds derived portfolio state.
	KindProjection = "projection"
	// KindCollection collects platform observations.
	KindCollection = "collection"

	statusQueued  = "queued"
	statusRunning = "running"

	dedupeFullRebuild  = "projection:full"
	defaultPriority    = int16(10)
	defaultMaxAttempts = uint32(5)

	outcomeSucceeded = "succeeded"
	outcomeFailed    = "failed"
)

// FullProjectionKinds are the families rebuilt by a full-Portfolio projection job.
var FullProjectionKinds = []string{
	"latest_balance",
	"latest_position",
	"portfolio_valuation",
	"exposure",
	"contributed_capital",
	"value_change",
	"period_summary",
}
