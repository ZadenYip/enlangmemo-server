package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type ReviewLogRow struct {
	ID            []byte
	Usn           int64
	CardID        []byte
	ReviewTime    int64
	ScheduledDays int32
	Rating        int32
	Difficulty    float64
	Stability     float64
	LearningSteps int32
	State         int8
	Duration      int8
}

const (
	ReviewIDSize            = 16
	ReviewUsnSize           = 8
	ReviewCardIDSize        = 16
	ReviewReviewTimeSize    = 8
	ReviewScheduledDaysSize = 4
	ReviewRatingSize        = 4
	ReviewDifficultySize    = 8
	ReviewStabilitySize     = 8
	ReviewLearningStepsSize = 4
	ReviewStateSize         = 4
	ReviewDurationSize      = 4
)

func (c *PullCollector) AddReviewLogChanges(rows *sql.Rows, limit int) (CollectResult, error) {
	var result CollectResult = CollectResult{
		SyncCursorUsn: 0,
		HasMore:       false,
		SizeExceeded:  false,
	}
	count := 0
	for rows.Next() {
		if count == limit {
			result.HasMore = true
			break
		}
		if c.IsFull() {
			result.HasMore = true
			result.SizeExceeded = true
			break
		}
		var row ReviewLogRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.CardID,
			&row.ReviewTime,
			&row.ScheduledDays,
			&row.Rating,
			&row.Difficulty,
			&row.Stability,
			&row.LearningSteps,
			&row.State,
			&row.Duration,
		)
		if err != nil {
			return CollectResult{}, err
		}
		const size = ReviewIDSize + ReviewUsnSize + ReviewCardIDSize +
			ReviewReviewTimeSize + ReviewScheduledDaysSize + ReviewRatingSize +
			ReviewDifficultySize + ReviewStabilitySize + ReviewLearningStepsSize +
			ReviewStateSize + ReviewDurationSize
		c.actualSize += size

		payload := syncv1.ReviewLogPayload{
			CardId:        row.CardID,
			ReviewTime:    row.ReviewTime,
			ScheduledDays: row.ScheduledDays,
			Rating:        row.Rating,
			Difficulty:    row.Difficulty,
			Stability:     row.Stability,
			LearningSteps: row.LearningSteps,
			State:         int32(row.State),
			Duration:      int32(row.Duration),
		}

		syncChange := &syncv1.SyncChange{
			EntityId:   row.ID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        row.Usn,
			Payload:    &syncv1.SyncChange_ReviewLog{ReviewLog: &payload},
		}
		c.syncChanges = append(c.syncChanges, syncChange)
		result.SyncCursorUsn = row.Usn + 1
		c.recordMaxUSN(row.Usn)

		count++
	}

	if err := rows.Err(); err != nil {
		return CollectResult{}, err
	}
	if c.IsFull() {
		result.SizeExceeded = true
	}
	return result, nil
}
