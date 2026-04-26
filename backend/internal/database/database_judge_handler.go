package database

import (
	"magpie/internal/domain"
	"time"
)

func GetJudgesRegexFromString(judges []*domain.Judge) []*domain.JudgeWithRegex {
	fullStrings := getStringListFromJudges(judges)

	var results []struct {
		ID         uint      `gorm:"column:id"`
		FullString string    `gorm:"column:full_string"`
		CreatedAt  time.Time `gorm:"column:created_at"`
		Regex      string    `gorm:"column:regex"`
	}

	if err := DB.Table("user_judges").
		Select("judges.id, judges.full_string, judges.created_at, user_judges.regex").
		Joins("JOIN judges ON user_judges.judge_id = judges.id").
		Where("judges.full_string IN (?)", fullStrings).
		Scan(&results).Error; err != nil {
		return nil
	}

	ptrResults := make([]*domain.JudgeWithRegex, len(results))
	for i, item := range results {
		judge := &domain.Judge{
			ID:         item.ID,
			FullString: item.FullString,
			CreatedAt:  item.CreatedAt,
		}

		judge.SetUp()
		judge.UpdateIp()
		ptrResults[i] = &domain.JudgeWithRegex{
			Judge: judge,
			Regex: item.Regex,
		}
	}

	return ptrResults
}

func GetJudgesFromString(judges []*domain.Judge) []*domain.Judge {
	fullStrings := getStringListFromJudges(judges)

	DB.Where("full_string IN (?)", fullStrings).Find(&judges)
	return judges
}

func AddJudges(judges []*domain.Judge) error {
	result := DB.Create(judges)
	return result.Error
}

func getStringListFromJudges(judges []*domain.Judge) []string {
	fullStrings := make([]string, len(judges))
	for i, j := range judges {
		fullStrings[i] = j.FullString
	}
	return fullStrings
}
