package support

import "magpie/internal/domain"

func GetUserIdsFromList(users []domain.User) []uint {
	userIds := make([]uint, 0, len(users))
	for _, user := range users {
		if user.ID == 0 {
			continue
		}
		userIds = append(userIds, user.ID)
	}

	return userIds
}
