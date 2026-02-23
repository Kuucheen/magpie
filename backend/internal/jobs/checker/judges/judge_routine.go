package judges

import (
	"context"
	"magpie/internal/config"
	"time"
)

func StartJudgeRoutine(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		judgeList := GetSortedJudgesByID()
		if len(judgeList) > 0 {
			betweenTime := getTimeBetweenJudgeChecks(uint64(len(judgeList)))

			for _, judge := range judgeList {
				select {
				case <-ctx.Done():
					return
				default:
				}

				judge.UpdateIp()

				select {
				case <-ctx.Done():
					return
				case <-time.After(betweenTime):
				}
			}
		} else {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func getTimeBetweenJudgeChecks(count uint64) time.Duration {
	var periodTime uint64

	if config.InProductionMode {
		periodTime = config.CalculateMillisecondsOfCheckingPeriod(config.GetConfig().Checker.JudgeTimer) / count
	} else {
		periodTime = config.CalculateMillisecondsOfCheckingPeriod(config.GetConfig().Checker.CheckerTimer) / count / 2 // Twice per period_time
	}

	return time.Duration(periodTime) * time.Millisecond
}
