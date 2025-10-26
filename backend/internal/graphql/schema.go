package graphql

import (
	"context"
	"fmt"
	"time"

	gql "github.com/graphql-go/graphql"

	"magpie/internal/api/dto"
	"magpie/internal/database"
	"magpie/internal/domain"
)

type viewerData struct {
	user     domain.User
	settings map[string]interface{}
}

func NewSchema() (gql.Schema, error) {
	simpleJudgeType := gql.NewObject(gql.ObjectConfig{
		Name: "SimpleUserJudge",
		Fields: gql.Fields{
			"url":   &gql.Field{Type: gql.NewNonNull(gql.String)},
			"regex": &gql.Field{Type: gql.NewNonNull(gql.String)},
		},
	})

	userSettingsType := gql.NewObject(gql.ObjectConfig{
		Name: "UserSettings",
		Fields: gql.Fields{
			"httpProtocol":               &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"httpsProtocol":              &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"socks4Protocol":             &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"socks5Protocol":             &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"timeout":                    &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"retries":                    &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"useHttpsForSocks":           &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"autoRemoveFailingProxies":   &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"autoRemoveFailureThreshold": &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"judges":                     &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(simpleJudgeType)))},
			"scrapingSources":            &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(gql.String)))},
		},
	})

	proxyType := gql.NewObject(gql.ObjectConfig{
		Name: "Proxy",
		Fields: gql.Fields{
			"id":             &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"ip":             &gql.Field{Type: gql.NewNonNull(gql.String)},
			"port":           &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"estimatedType":  &gql.Field{Type: gql.NewNonNull(gql.String)},
			"responseTime":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"country":        &gql.Field{Type: gql.NewNonNull(gql.String)},
			"anonymityLevel": &gql.Field{Type: gql.NewNonNull(gql.String)},
			"protocol":       &gql.Field{Type: gql.NewNonNull(gql.String)},
			"alive":          &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
			"latestCheck":    &gql.Field{Type: gql.DateTime},
		},
	})

	proxyHistoryType := gql.NewObject(gql.ObjectConfig{
		Name: "ProxyHistoryEntry",
		Fields: gql.Fields{
			"count":      &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"recordedAt": &gql.Field{Type: gql.NewNonNull(gql.DateTime)},
		},
	})

	proxySnapshotType := gql.NewObject(gql.ObjectConfig{
		Name: "ProxySnapshotEntry",
		Fields: gql.Fields{
			"count":      &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"recordedAt": &gql.Field{Type: gql.NewNonNull(gql.DateTime)},
		},
	})

	proxySnapshotCollectionType := gql.NewObject(gql.ObjectConfig{
		Name: "ProxySnapshotCollection",
		Fields: gql.Fields{
			"alive": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(proxySnapshotType))),
			},
			"scraped": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(proxySnapshotType))),
			},
		},
	})

	proxyPageType := gql.NewObject(gql.ObjectConfig{
		Name: "ProxyPage",
		Fields: gql.Fields{
			"page":       &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"pageSize":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"totalCount": &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"items":      &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(proxyType)))},
		},
	})

	judgeValidProxyType := gql.NewObject(gql.ObjectConfig{
		Name: "JudgeValidProxy",
		Fields: gql.Fields{
			"judgeUrl":           &gql.Field{Type: gql.NewNonNull(gql.String)},
			"eliteProxies":       &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"anonymousProxies":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"transparentProxies": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		},
	})

	countryBreakdownType := gql.NewObject(gql.ObjectConfig{
		Name: "ProxyCountryBreakdown",
		Fields: gql.Fields{
			"country": &gql.Field{Type: gql.NewNonNull(gql.String)},
			"count":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
		},
	})

	dashboardType := gql.NewObject(gql.ObjectConfig{
		Name: "DashboardInfo",
		Fields: gql.Fields{
			"totalChecks":      &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"totalScraped":     &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"totalChecksWeek":  &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"totalScrapedWeek": &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"countryBreakdown": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(countryBreakdownType))),
			},
			"judgeValidProxies": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(judgeValidProxyType))),
			},
		},
	})

	scrapeSiteType := gql.NewObject(gql.ObjectConfig{
		Name: "ScrapeSite",
		Fields: gql.Fields{
			"id":         &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"url":        &gql.Field{Type: gql.NewNonNull(gql.String)},
			"proxyCount": &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"addedAt":    &gql.Field{Type: gql.DateTime},
		},
	})

	scrapeSitePageType := gql.NewObject(gql.ObjectConfig{
		Name: "ScrapeSitePage",
		Fields: gql.Fields{
			"page":       &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"pageSize":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"totalCount": &gql.Field{Type: gql.NewNonNull(gql.Int)},
			"items":      &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(scrapeSiteType)))},
		},
	})

	viewerType := gql.NewObject(gql.ObjectConfig{
		Name: "Viewer",
		Fields: gql.Fields{
			"id": &gql.Field{
				Type: gql.NewNonNull(gql.ID),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return fmt.Sprintf("%d", data.user.ID), nil
					}
					return nil, nil
				},
			},
			"email": &gql.Field{
				Type: gql.NewNonNull(gql.String),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return data.user.Email, nil
					}
					return nil, nil
				},
			},
			"role": &gql.Field{
				Type: gql.NewNonNull(gql.String),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return data.user.Role, nil
					}
					return nil, nil
				},
			},
			"settings": &gql.Field{
				Type: gql.NewNonNull(userSettingsType),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return data.settings, nil
					}
					return nil, nil
				},
			},
			"scrapeSourceUrls": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(gql.String))),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						if urls, ok := data.settings["scrapingSources"].([]string); ok {
							return urls, nil
						}
					}
					return []string{}, nil
				},
			},
			"dashboard": &gql.Field{
				Type: gql.NewNonNull(dashboardType),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						info := database.GetDashboardInfo(data.user.ID)
						return buildDashboard(info), nil
					}
					return nil, nil
				},
			},
			"proxyCount": &gql.Field{
				Type: gql.NewNonNull(gql.Int),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return int(database.GetAllProxyCountOfUser(data.user.ID)), nil
					}
					return 0, nil
				},
			},
			"proxies": &gql.Field{
				Type: gql.NewNonNull(proxyPageType),
				Args: gql.FieldConfigArgument{
					"page": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.Int)},
				},
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					page := 1
					if raw, ok := p.Args["page"].(int); ok && raw > 0 {
						page = raw
					}
					if data, ok := p.Source.(*viewerData); ok {
						return buildProxyPage(data.user.ID, page), nil
					}
					return nil, nil
				},
			},
			"scrapeSourceCount": &gql.Field{
				Type: gql.NewNonNull(gql.Int),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					if data, ok := p.Source.(*viewerData); ok {
						return int(database.GetAllScrapeSiteCountOfUser(data.user.ID)), nil
					}
					return 0, nil
				},
			},
			"proxyHistory": &gql.Field{
				Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(proxyHistoryType))),
				Args: gql.FieldConfigArgument{
					"limit": &gql.ArgumentConfig{Type: gql.Int},
				},
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					limit := 168
					if raw, ok := p.Args["limit"].(int); ok && raw > 0 {
						limit = raw
					}
					if data, ok := p.Source.(*viewerData); ok {
						return buildProxyHistory(data.user.ID, limit), nil
					}
					return []map[string]interface{}{}, nil
				},
			},
			"proxySnapshots": &gql.Field{
				Type: gql.NewNonNull(proxySnapshotCollectionType),
				Args: gql.FieldConfigArgument{
					"limit": &gql.ArgumentConfig{Type: gql.Int},
				},
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					limit := 0
					if raw, ok := p.Args["limit"].(int); ok && raw > 0 {
						limit = raw
					}
					if data, ok := p.Source.(*viewerData); ok {
						alive := database.GetProxySnapshotEntries(data.user.ID, domain.ProxySnapshotMetricAlive, limit)
						alive = ensureLatestAliveSnapshot(alive, database.GetCurrentAliveProxyCount(data.user.ID))

						scraped := database.GetProxySnapshotEntries(data.user.ID, domain.ProxySnapshotMetricScraped, limit)
						return map[string]interface{}{
							"alive":   buildProxySnapshots(alive),
							"scraped": buildProxySnapshots(scraped),
						}, nil
					}
					return map[string]interface{}{
						"alive":   []map[string]interface{}{},
						"scraped": []map[string]interface{}{},
					}, nil
				},
			},
			"scrapeSources": &gql.Field{
				Type: gql.NewNonNull(scrapeSitePageType),
				Args: gql.FieldConfigArgument{
					"page": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.Int)},
				},
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					page := 1
					if raw, ok := p.Args["page"].(int); ok && raw > 0 {
						page = raw
					}
					if data, ok := p.Source.(*viewerData); ok {
						return buildScrapeSitePage(data.user.ID, page), nil
					}
					return nil, nil
				},
			},
		},
	})

	queryType := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"viewer": &gql.Field{
				Type: viewerType,
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					return fetchViewer(p.Context)
				},
			},
		},
	})

	judgeInputType := gql.NewInputObject(gql.InputObjectConfig{
		Name: "SimpleUserJudgeInput",
		Fields: gql.InputObjectConfigFieldMap{
			"url":   &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
			"regex": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		},
	})

	updateSettingsInput := gql.NewInputObject(gql.InputObjectConfig{
		Name: "UpdateUserSettingsInput",
		Fields: gql.InputObjectConfigFieldMap{
			"httpProtocol":               &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"httpsProtocol":              &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"socks4Protocol":             &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"socks5Protocol":             &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"timeout":                    &gql.InputObjectFieldConfig{Type: gql.Int},
			"retries":                    &gql.InputObjectFieldConfig{Type: gql.Int},
			"useHttpsForSocks":           &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"autoRemoveFailingProxies":   &gql.InputObjectFieldConfig{Type: gql.Boolean},
			"autoRemoveFailureThreshold": &gql.InputObjectFieldConfig{Type: gql.Int},
			"judges": &gql.InputObjectFieldConfig{
				Type: gql.NewList(gql.NewNonNull(judgeInputType)),
			},
			"scrapingSources": &gql.InputObjectFieldConfig{
				Type: gql.NewList(gql.NewNonNull(gql.String)),
			},
		},
	})

	mutationType := gql.NewObject(gql.ObjectConfig{
		Name: "Mutation",
		Fields: gql.Fields{
			"updateUserSettings": &gql.Field{
				Type: userSettingsType,
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(updateSettingsInput)},
				},
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					rawInput, _ := p.Args["input"].(map[string]interface{})
					if err := applyUserSettings(p.Context, rawInput); err != nil {
						return nil, err
					}
					viewer, err := fetchViewer(p.Context)
					if err != nil {
						return nil, err
					}
					if data, ok := viewer.(*viewerData); ok {
						return data.settings, nil
					}
					return nil, nil
				},
			},
		},
	})

	return gql.NewSchema(gql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
}

func fetchViewer(ctx context.Context) (interface{}, error) {
	userID, err := UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user := database.GetUserFromId(userID)
	if user.ID == 0 {
		return nil, fmt.Errorf("user %d not found", userID)
	}

	judges := database.GetUserJudges(userID)
	sources := database.GetScrapingSourcesOfUsers(userID)

	settings := buildUserSettings(user, judges, sources)

	return &viewerData{
		user:     user,
		settings: settings,
	}, nil
}

func buildUserSettings(user domain.User, judges []dto.SimpleUserJudge, sources []string) map[string]interface{} {
	dtoSettings := user.ToUserSettings(judges, sources)

	judgeList := make([]map[string]interface{}, 0, len(dtoSettings.SimpleUserJudges))
	for _, judge := range dtoSettings.SimpleUserJudges {
		judgeList = append(judgeList, map[string]interface{}{
			"url":   judge.Url,
			"regex": judge.Regex,
		})
	}

	return map[string]interface{}{
		"httpProtocol":               dtoSettings.HTTPProtocol,
		"httpsProtocol":              dtoSettings.HTTPSProtocol,
		"socks4Protocol":             dtoSettings.SOCKS4Protocol,
		"socks5Protocol":             dtoSettings.SOCKS5Protocol,
		"timeout":                    int(dtoSettings.Timeout),
		"retries":                    int(dtoSettings.Retries),
		"useHttpsForSocks":           dtoSettings.UseHttpsForSocks,
		"autoRemoveFailingProxies":   dtoSettings.AutoRemoveFailingProxies,
		"autoRemoveFailureThreshold": int(dtoSettings.AutoRemoveFailureThreshold),
		"judges":                     judgeList,
		"scrapingSources":            dtoSettings.ScrapingSources,
	}
}

func buildProxyHistory(userID uint, limit int) []map[string]interface{} {
	entries := database.GetProxyHistoryEntries(userID, limit)
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		result = append(result, map[string]interface{}{
			"count":      entry.Count,
			"recordedAt": entry.RecordedAt,
		})
	}
	return result
}

func buildProxySnapshots(entries []dto.ProxySnapshotEntry) []map[string]interface{} {
	if len(entries) == 0 {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		result = append(result, map[string]interface{}{
			"count":      entry.Count,
			"recordedAt": entry.RecordedAt,
		})
	}
	return result
}

func ensureLatestAliveSnapshot(entries []dto.ProxySnapshotEntry, currentCount int64) []dto.ProxySnapshotEntry {
	now := time.Now()
	if len(entries) == 0 {
		return append(entries, dto.ProxySnapshotEntry{
			Count:      currentCount,
			RecordedAt: now,
		})
	}

	lastIndex := len(entries) - 1
	if entries[lastIndex].Count == currentCount {
		entries[lastIndex].RecordedAt = now
		return entries
	}

	return append(entries, dto.ProxySnapshotEntry{
		Count:      currentCount,
		RecordedAt: now,
	})
}

func buildProxyPage(userID uint, page int) map[string]interface{} {
	proxies := database.GetProxyInfoPage(userID, page)
	items := make([]map[string]interface{}, 0, len(proxies))
	for _, proxy := range proxies {
		items = append(items, map[string]interface{}{
			"id":             proxy.Id,
			"ip":             proxy.IP,
			"port":           int(proxy.Port),
			"estimatedType":  proxy.EstimatedType,
			"responseTime":   int(proxy.ResponseTime),
			"country":        proxy.Country,
			"anonymityLevel": proxy.AnonymityLevel,
			"alive":          proxy.Alive,
			"latestCheck":    proxy.LatestCheck,
		})
	}

	return map[string]interface{}{
		"page":       page,
		"pageSize":   len(items),
		"totalCount": int(database.GetAllProxyCountOfUser(userID)),
		"items":      items,
	}
}

func buildScrapeSitePage(userID uint, page int) map[string]interface{} {
	sites := database.GetScrapeSiteInfoPage(userID, page)
	items := make([]map[string]interface{}, 0, len(sites))
	for _, site := range sites {
		items = append(items, map[string]interface{}{
			"id":         int(site.Id),
			"url":        site.Url,
			"proxyCount": int(site.ProxyCount),
			"addedAt":    site.AddedAt,
		})
	}

	return map[string]interface{}{
		"page":       page,
		"pageSize":   len(items),
		"totalCount": int(database.GetAllScrapeSiteCountOfUser(userID)),
		"items":      items,
	}
}

func buildDashboard(info dto.DashboardInfo) map[string]interface{} {
	countries := make([]map[string]interface{}, 0, len(info.CountryBreakdown))
	for _, entry := range info.CountryBreakdown {
		countries = append(countries, map[string]interface{}{
			"country": entry.Country,
			"count":   int(entry.Count),
		})
	}

	entries := make([]map[string]interface{}, 0, len(info.JudgeValidProxies))
	for _, entry := range info.JudgeValidProxies {
		entries = append(entries, map[string]interface{}{
			"judgeUrl":           entry.JudgeUrl,
			"eliteProxies":       int(entry.EliteProxies),
			"anonymousProxies":   int(entry.AnonymousProxies),
			"transparentProxies": int(entry.TransparentProxies),
		})
	}

	return map[string]interface{}{
		"totalChecks":       int(info.TotalChecks),
		"totalScraped":      int(info.TotalScraped),
		"totalChecksWeek":   int(info.TotalChecksWeek),
		"totalScrapedWeek":  int(info.TotalScrapedWeek),
		"countryBreakdown":  countries,
		"judgeValidProxies": entries,
	}
}

func applyUserSettings(ctx context.Context, input map[string]interface{}) error {
	if input == nil {
		return fmt.Errorf("missing input")
	}

	userID, err := UserIDFromContext(ctx)
	if err != nil {
		return err
	}

	user := database.GetUserFromId(userID)
	if user.ID == 0 {
		return fmt.Errorf("user %d not found", userID)
	}

	currentJudges := database.GetUserJudges(userID)
	currentSources := database.GetScrapingSourcesOfUsers(userID)

	settings := user.ToUserSettings(currentJudges, currentSources)

	if v, ok := input["httpProtocol"].(bool); ok {
		settings.HTTPProtocol = v
	}
	if v, ok := input["httpsProtocol"].(bool); ok {
		settings.HTTPSProtocol = v
	}
	if v, ok := input["socks4Protocol"].(bool); ok {
		settings.SOCKS4Protocol = v
	}
	if v, ok := input["socks5Protocol"].(bool); ok {
		settings.SOCKS5Protocol = v
	}
	if v, ok := input["timeout"].(int); ok && v >= 0 {
		settings.Timeout = uint16(v)
	}
	if v, ok := input["retries"].(int); ok && v >= 0 {
		settings.Retries = uint8(v)
	}
	if v, ok := input["useHttpsForSocks"].(bool); ok {
		settings.UseHttpsForSocks = v
	}
	if v, ok := input["autoRemoveFailingProxies"].(bool); ok {
		settings.AutoRemoveFailingProxies = v
	}
	if v, ok := input["autoRemoveFailureThreshold"].(int); ok && v >= 0 {
		if v > 255 {
			v = 255
		}
		settings.AutoRemoveFailureThreshold = uint8(v)
	}

	if rawJudges, ok := input["judges"].([]interface{}); ok {
		judges := make([]dto.SimpleUserJudge, 0, len(rawJudges))
		for _, rawJudge := range rawJudges {
			if judgeMap, ok := rawJudge.(map[string]interface{}); ok {
				judge := dto.SimpleUserJudge{}
				if url, ok := judgeMap["url"].(string); ok {
					judge.Url = url
				}
				if regex, ok := judgeMap["regex"].(string); ok {
					judge.Regex = regex
				}
				judges = append(judges, judge)
			}
		}
		settings.SimpleUserJudges = judges
	}

	if rawSources, ok := input["scrapingSources"].([]interface{}); ok {
		sources := make([]string, 0, len(rawSources))
		for _, raw := range rawSources {
			if s, ok := raw.(string); ok {
				sources = append(sources, s)
			}
		}
		settings.ScrapingSources = sources
	}

	if err := database.UpdateUserSettings(userID, settings); err != nil {
		return err
	}

	return nil
}
