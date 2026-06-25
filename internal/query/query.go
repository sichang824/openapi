package query

import (
	"sort"
	"strings"

	"openapi/internal/spec"
)

type Endpoint struct {
	Method    string
	Path      string
	Operation spec.Operation
	Score     int
}

func Search(doc *spec.Document, keyword string, limit int) []Endpoint {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" {
		return nil
	}

	results := make([]Endpoint, 0)
	for path, item := range doc.Paths {
		pathLower := strings.ToLower(path)
		for method, op := range item {
			score := matchScore(kw, strings.ToLower(method), pathLower, op)
			if score <= 0 {
				continue
			}
			results = append(results, Endpoint{
				Method:    strings.ToUpper(method),
				Path:      path,
				Operation: op,
				Score:     score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].Method < results[j].Method
	})

	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func ListAll(doc *spec.Document) []Endpoint {
	endpoints := make([]Endpoint, 0)
	for path, item := range doc.Paths {
		for method, op := range item {
			endpoints = append(endpoints, Endpoint{
				Method:    strings.ToUpper(method),
				Path:      path,
				Operation: op,
			})
		}
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})

	return endpoints
}

func matchScore(kw, method, path string, op spec.Operation) int {
	score := 0
	if strings.Contains(path, kw) || strings.Contains(method, kw) {
		score += 100
	}
	if strings.Contains(strings.ToLower(op.Summary), kw) {
		score += 60
	}
	if strings.Contains(strings.ToLower(op.Description), kw) {
		score += 40
	}
	if strings.Contains(strings.ToLower(op.OperationID), kw) {
		score += 50
	}
	for _, tag := range op.Tags {
		if strings.Contains(strings.ToLower(tag), kw) {
			score += 30
		}
	}
	return score
}
