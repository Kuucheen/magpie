package bootstrap

import (
	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/jobs/checker/judges"
	"magpie/internal/support"
	"sync"

	"github.com/charmbracelet/log"
)

var addDefaultJudgeMutex sync.Mutex

// AddDefaultJudgesToUsers gets empty judges list of users and adds the default judges (from config) to the db
// this is the ugliest function I have ever written. I really need to make this better
func AddDefaultJudgesToUsers() {
	addDefaultJudgeMutex.Lock()
	defer addDefaultJudgeMutex.Unlock()

	cfg := config.GetConfig()
	users := database.GetUsersThatDontHaveJudges()

	judgesWithRegex := make([]*domain.JudgeWithRegex, 0, len(cfg.Checker.Judges))
	judgeList := make([]*domain.Judge, 0, len(cfg.Checker.Judges))

	for _, judge := range cfg.Checker.Judges {
		if config.IsWebsiteBlocked(judge.URL) {
			log.Info("Skipping default judge because website is blocked", "url", judge.URL)
			continue
		}

		entry := &domain.JudgeWithRegex{
			Judge: &domain.Judge{
				FullString: judge.URL,
			},
			Regex: judge.Regex,
		}

		judgeList = append(judgeList, entry.Judge)
		judgesWithRegex = append(judgesWithRegex, entry)

		err := entry.Judge.SetUp()
		if err != nil {
			log.Error("Error setting up judge", "error", err)
		}
		entry.Judge.UpdateIp()
	}

	if len(judgesWithRegex) == 0 {
		log.Info("No default judges added; all entries were blocked or missing")
		return
	}

	jwr := database.GetJudgesRegexFromString(judgeList) // Get ids
	if len(jwr) == 0 {
		err := database.AddJudges(judgeList) // Sets id too
		if err != nil {
			database.GetJudgesFromString(judgeList) // Sets id if not added judgeList
		}

		for i, judge := range judgeList {
			setUpAndUpdateJudgeIp(judge)
			judgesWithRegex[i].Judge = judge
		}
	} else {
		judgesWithRegex = jwr

		for _, judge := range judgesWithRegex {
			setUpAndUpdateJudgeIp(judge.Judge)
		}
	}

	err := database.AddUserJudgesRelation(users, judgesWithRegex)
	if err != nil {
		log.Error("Error adding user judgeList to database", "error", err)
	} else {
		judgesNonPointer := make([]domain.JudgeWithRegex, len(judgesWithRegex))
		for i, j := range judgesWithRegex {
			judgesNonPointer[i] = *j
		}

		judges.AddJudgesToUsers(support.GetUserIdsFromList(users), judgesNonPointer)
	}
}

func addJudgeRelationsToCache() {
	userJudges, jwr := database.GetAllUserJudgeRelations()

	for _, userJudge := range userJudges {
		for _, judge := range jwr {
			if userJudge.JudgeID == judge.Judge.ID {
				if config.IsWebsiteBlocked(judge.Judge.FullString) {
					log.Info("Skipping cached judge because website is blocked", "url", judge.Judge.FullString, "user_id", userJudge.UserID)
					continue
				}
				judges.AddUserJudge(userJudge.UserID, judge.Judge, judge.Regex)
			}
		}
	}
}

func setUpAndUpdateJudgeIp(judge *domain.Judge) {
	judge.SetUp()
	judge.UpdateIp()
}
