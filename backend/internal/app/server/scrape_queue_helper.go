package server

import (
	"context"
	"errors"
	"fmt"

	"magpie/internal/api/dto"
	"magpie/internal/database"
	"magpie/internal/domain"
	sitequeue "magpie/internal/jobs/queue/sites"
)

var enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
	return sitequeue.PublicScrapeSiteQueue.AddToQueue(sites)
}
var getScrapeSourceDetailForUser = func(userID uint, sourceID uint64) (*dto.ScrapeSiteDetail, error) {
	return database.GetScrapeSiteDetail(userID, sourceID)
}

var deleteScrapeSiteRelations = database.DeleteScrapeSiteRelation
var removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error {
	return sitequeue.PublicScrapeSiteQueue.RemoveFromQueue(sites)
}
var deleteOrphanScrapeSites = database.DeleteOrphanScrapeSites

func enqueueScrapeSitesOrRollback(userID uint, sites []domain.ScrapeSite) error {
	if len(sites) == 0 {
		return nil
	}

	if err := enqueueScrapeSites(sites); err != nil {
		if rollbackErr := rollbackScrapeSiteEnqueue(userID, sites); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}

	return nil
}

func rollbackScrapeSiteEnqueue(userID uint, sites []domain.ScrapeSite) error {
	siteIDs := scrapeSiteIDsToIntSlice(sites)
	if len(siteIDs) == 0 {
		return nil
	}

	_, orphanedSites, rollbackErr := deleteScrapeSiteRelations(userID, siteIDs)
	if rollbackErr != nil {
		return fmt.Errorf("rollback scrape-site relations failed: %w", rollbackErr)
	}

	if len(orphanedSites) == 0 {
		return nil
	}

	var cleanupErr error
	if err := removeScrapeSitesFromQueue(orphanedSites); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("rollback scrape-site queue cleanup failed: %w", err))
	}
	if _, err := deleteOrphanScrapeSites(context.Background()); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("rollback orphan scrape-site cleanup failed: %w", err))
	}
	return cleanupErr
}

func scrapeSiteIDsToIntSlice(sites []domain.ScrapeSite) []int {
	ids := make([]int, 0, len(sites))
	seen := make(map[uint64]struct{}, len(sites))
	for _, site := range sites {
		if site.ID == 0 {
			continue
		}
		if _, exists := seen[site.ID]; exists {
			continue
		}
		seen[site.ID] = struct{}{}
		ids = append(ids, int(site.ID))
	}
	return ids
}
