package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	ast "github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
	gqlhandler "github.com/graphql-go/handler"

	"magpie/internal/support"
)

const (
	envGraphQLMaxDepth           = "GRAPHQL_MAX_DEPTH"
	envGraphQLMaxFields          = "GRAPHQL_MAX_FIELDS"
	envGraphQLMaxQueryBytes      = "GRAPHQL_MAX_QUERY_BYTES"
	envGraphQLAllowIntrospection = "GRAPHQL_ALLOW_INTROSPECTION"

	defaultGraphQLMaxDepth      = 12
	defaultGraphQLMaxFields     = 250
	defaultGraphQLMaxQueryBytes = 16 << 10 // 16 KiB
)

type graphQLCost struct {
	fields int
}

func withGraphQLGuard(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opts, err := parseGraphQLRequestOptions(r)
		if err != nil {
			writeError(w, "Invalid GraphQL request", http.StatusBadRequest)
			return
		}

		query := strings.TrimSpace(opts.Query)
		if query == "" {
			writeError(w, "Missing GraphQL query", http.StatusBadRequest)
			return
		}

		maxQueryBytes := resolveGraphQLMaxQueryBytes()
		if int64(len(query)) > maxQueryBytes {
			writeError(w, fmt.Sprintf("GraphQL query exceeds %d bytes limit", maxQueryBytes), http.StatusRequestEntityTooLarge)
			return
		}

		if err := validateGraphQLQueryComplexity(query, strings.TrimSpace(opts.OperationName)); err != nil {
			writeError(w, err.Error(), http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseGraphQLRequestOptions(r *http.Request) (*gqlhandler.RequestOptions, error) {
	if r == nil {
		return &gqlhandler.RequestOptions{}, nil
	}

	if r.Method != http.MethodPost || r.Body == nil {
		return gqlhandler.NewRequestOptions(r), nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))

	return gqlhandler.NewRequestOptions(clone), nil
}

func validateGraphQLQueryComplexity(query string, operationName string) error {
	doc, err := parser.Parse(parser.ParseParams{
		Source: source.NewSource(&source.Source{
			Body: []byte(query),
			Name: "GraphQL request",
		}),
	})
	if err != nil {
		return fmt.Errorf("invalid GraphQL query")
	}

	maxDepth := resolveGraphQLMaxDepth()
	maxFields := resolveGraphQLMaxFields()
	allowIntrospection := support.GetEnvBool(envGraphQLAllowIntrospection, false)

	fragments := map[string]*ast.FragmentDefinition{}
	operations := make([]*ast.OperationDefinition, 0)

	for _, def := range doc.Definitions {
		switch typed := def.(type) {
		case *ast.FragmentDefinition:
			if typed.Name != nil && strings.TrimSpace(typed.Name.Value) != "" {
				fragments[typed.Name.Value] = typed
			}
		case *ast.OperationDefinition:
			operations = append(operations, typed)
		}
	}

	if len(operations) == 0 {
		return fmt.Errorf("GraphQL request has no operation")
	}

	selected := selectOperations(operations, operationName)
	if len(selected) == 0 {
		return fmt.Errorf("GraphQL operation %q not found", operationName)
	}

	for _, op := range selected {
		cost := &graphQLCost{}
		if err := walkSelectionSet(op.SelectionSet, 1, maxDepth, maxFields, allowIntrospection, fragments, map[string]bool{}, cost); err != nil {
			return err
		}
	}

	return nil
}

func walkSelectionSet(
	set *ast.SelectionSet,
	depth int,
	maxDepth int,
	maxFields int,
	allowIntrospection bool,
	fragments map[string]*ast.FragmentDefinition,
	fragmentStack map[string]bool,
	cost *graphQLCost,
) error {
	if set == nil {
		return nil
	}
	if depth > maxDepth {
		return fmt.Errorf("GraphQL query depth exceeds %d", maxDepth)
	}

	for _, selection := range set.Selections {
		switch node := selection.(type) {
		case *ast.Field:
			cost.fields++
			if cost.fields > maxFields {
				return fmt.Errorf("GraphQL field count exceeds %d", maxFields)
			}
			if !allowIntrospection && node.Name != nil && strings.HasPrefix(node.Name.Value, "__") {
				return fmt.Errorf("GraphQL introspection is disabled")
			}
			if err := walkSelectionSet(node.SelectionSet, depth+1, maxDepth, maxFields, allowIntrospection, fragments, fragmentStack, cost); err != nil {
				return err
			}
		case *ast.InlineFragment:
			if err := walkSelectionSet(node.SelectionSet, depth+1, maxDepth, maxFields, allowIntrospection, fragments, fragmentStack, cost); err != nil {
				return err
			}
		case *ast.FragmentSpread:
			if node.Name == nil {
				continue
			}
			name := strings.TrimSpace(node.Name.Value)
			if name == "" {
				continue
			}
			if fragmentStack[name] {
				return fmt.Errorf("GraphQL fragment cycle detected for %q", name)
			}
			fragment := fragments[name]
			if fragment == nil {
				continue
			}
			fragmentStack[name] = true
			err := walkSelectionSet(fragment.SelectionSet, depth+1, maxDepth, maxFields, allowIntrospection, fragments, fragmentStack, cost)
			delete(fragmentStack, name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func selectOperations(operations []*ast.OperationDefinition, operationName string) []*ast.OperationDefinition {
	if operationName == "" {
		return operations
	}

	selected := make([]*ast.OperationDefinition, 0, 1)
	for _, op := range operations {
		if op == nil || op.Name == nil {
			continue
		}
		if strings.TrimSpace(op.Name.Value) == operationName {
			selected = append(selected, op)
		}
	}
	return selected
}

func resolveGraphQLMaxDepth() int {
	return resolvePositiveEnvInt(envGraphQLMaxDepth, defaultGraphQLMaxDepth)
}

func resolveGraphQLMaxFields() int {
	return resolvePositiveEnvInt(envGraphQLMaxFields, defaultGraphQLMaxFields)
}

func resolveGraphQLMaxQueryBytes() int64 {
	return resolvePositiveEnvInt64(envGraphQLMaxQueryBytes, defaultGraphQLMaxQueryBytes)
}
