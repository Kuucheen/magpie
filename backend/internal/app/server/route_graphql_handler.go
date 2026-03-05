package server

import (
	"net/http"
	"sync"

	"github.com/charmbracelet/log"
	gqlhandler "github.com/graphql-go/handler"

	"magpie/internal/auth"
	gqlschema "magpie/internal/graphql"
)

var (
	graphQLHandler     http.Handler
	graphQLHandlerOnce sync.Once
	graphQLHandlerErr  error
)

func getGraphQLHandler() (http.Handler, error) {
	graphQLHandlerOnce.Do(func() {
		schema, err := gqlschema.NewSchema()
		if err != nil {
			graphQLHandlerErr = err
			return
		}

		base := gqlhandler.New(&gqlhandler.Config{
			Schema:   &schema,
			Pretty:   true,
			GraphiQL: false,
		})

		graphQLHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if userID, err := auth.GetUserIDFromRequest(r); err == nil && userID > 0 {
				ctx = gqlschema.WithUserID(ctx, userID)
			} else if err != nil {
				log.Debug("GraphQL token rejected", "error", err)
			}

			base.ContextHandler(ctx, w, r)
		})
	})

	return graphQLHandler, graphQLHandlerErr
}
