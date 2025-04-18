package database

import (
	"gorm.io/gorm"
	"magpie/helper"

	"magpie/models"
)

// GetScrapingSourcesOfUsers returns all URLs associated with the given user.
func GetScrapingSourcesOfUsers(userID uint) []string {
	var user models.User
	if err := DB.Preload("ScrapeSites").First(&user, userID).Error; err != nil {
		return nil
	}

	out := make([]string, 0, len(user.ScrapeSites))
	for _, s := range user.ScrapeSites {
		out = append(out, s.URL)
	}
	return out
}

// SaveScrapingSourcesOfUsers replaces the user’s current list with `sources`.
func SaveScrapingSourcesOfUsers(userID int, sources []string) ([]models.ScrapeSite, error) {
	var sites []models.ScrapeSite

	err := DB.Transaction(func(tx *gorm.DB) error {
		sites = make([]models.ScrapeSite, 0, len(sources))

		for _, raw := range sources {
			if raw == "" || !helper.IsValidURL(raw) {
				continue
			}

			var site models.ScrapeSite
			if err := tx.
				Where("url = ?", raw).
				FirstOrCreate(&site, &models.ScrapeSite{URL: raw}).Error; err != nil {
				return err
			}
			sites = append(sites, site)
		}

		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		return tx.Model(&user).
			Association("ScrapeSites").
			Replace(&sites)
	})

	return sites, err
}

func GetAllScrapeSites() ([]models.ScrapeSite, error) {
	var allProxies []models.ScrapeSite
	const batchSize = maxParamsPerBatch

	collectedProxies := make([]models.ScrapeSite, 0)

	err := DB.Preload("Users").Order("id").FindInBatches(&allProxies, batchSize, func(tx *gorm.DB, batch int) error {
		collectedProxies = append(collectedProxies, allProxies...)
		return nil
	})

	if err.Error != nil {
		return nil, err.Error
	}

	return collectedProxies, nil
}
