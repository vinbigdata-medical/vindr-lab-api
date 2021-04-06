package utils

import (
	"fmt"
	"strings"

	"vindr-lab-api/constants"

	"github.com/gin-gonic/gin"
)

var Meta = map[string]bool{
	"study_instance_uid":         true,
	"sop_instance_uid":           true,
	"series_instance_uid":        true,
	"masked_study_instance_uid":  true,
	"masked_sop_instance_uid":    true,
	"masked_series_instance_uid": true,
}

func ConvertQueryParamsToQueryString(context *gin.Context) string {
	mapQuery := context.Request.URL.Query()
	queries := make([]string, 0)
	for k, v := range mapQuery {
		switch k {
		case constants.ParamLimit, constants.ParamOffset, constants.ParamSort, constants.ParamSearch:
			continue
		default:
			if _, ok := Meta[k]; ok {
				queries = append(queries, fmt.Sprintf("meta.%s:%s", k, v[0]))
			} else {
				queries = append(queries, fmt.Sprintf("%s:%s", k, v[0]))
			}
		}
	}
	return strings.Join(queries, " AND ")
}

func ConvertQueryParamsToMap(context *gin.Context) map[string][]string {
	mapQuery := context.Request.URL.Query()
	queries := make(map[string][]string)
	for k, v := range mapQuery {
		switch k {
		case constants.ParamLimit, constants.ParamOffset, constants.ParamSort, constants.ParamSearch, constants.ParamAggregation:
			continue
		default:
			kw := ""
			if _, isKeywordField := nonKeywordFields[k]; !isKeywordField {
				kw = ".keyword"
			}

			if _, ok := Meta[k]; ok {
				queries[fmt.Sprintf("meta.%s%s", k, kw)] = v
			} else {
				queries[fmt.Sprintf("%s%s", k, kw)] = v
			}
		}
	}
	return queries
}

type kvStr2Inf = map[string]interface{}

var nonKeywordFields = map[string]bool{
	"created":       true,
	"time_inserted": true,
	"modified":      true,
	"archived":      true,
}

func MakeSortQuery(sortRaw string) []kvStr2Inf {
	if sortRaw == "" {
		return nil
	}

	sorts := strings.Split(sortRaw, ",")
	sortQuery := make([]kvStr2Inf, 0)
	for _, sort := range sorts {
		var order string
		var criteria string
		if strings.HasPrefix(sort, "-") {
			order = "desc"
			criteria = strings.TrimPrefix(sort, "-")
		} else {
			order = "asc"
			criteria = sort
		}

		if _, found := nonKeywordFields[criteria]; !found {
			criteria += ".keyword"
		}

		sortQuery = append(sortQuery, kvStr2Inf{
			criteria: kvStr2Inf{
				"order": order,
			},
		})
	}

	return sortQuery
}

func ConvertInputsToESQueryBody(queries map[string][]string, qs string, from, size int, sort string, aggs []string) *kvStr2Inf {
	body := kvStr2Inf{}

	if size != -1 {
		body["size"] = size
	}
	if from != -1 {
		body["from"] = from
	}

	filter := make([]kvStr2Inf, 0)
	should := make([]kvStr2Inf, 0)
	must := make([]kvStr2Inf, 0)

	if len(queries) > 0 {
		for k, v := range queries {
			if len(v) == 1 {
				filter = append(filter, kvStr2Inf{
					"term": kvStr2Inf{
						k: v[0],
					},
				})
			} else if len(v) > 1 {
				for i := range v {
					should = append(should, kvStr2Inf{
						"term": kvStr2Inf{
							k: v[i],
						},
					})
				}
			}
		}
	}

	if qs != "" {
		must = append(must, kvStr2Inf{
			"query_string": kvStr2Inf{
				"query": qs,
			},
		})
	}
	boolQ := make(kvStr2Inf)
	boolQ = kvStr2Inf{
		"must":   must,
		"filter": filter,
		"should": should,
	}

	if len(should) > 0 {
		boolQ["minimum_should_match"] = 1
	}

	body["query"] = kvStr2Inf{
		"bool": boolQ,
	}

	if sortParam := MakeSortQuery(sort); sortParam != nil {
		body["sort"] = sortParam
	}

	if len(aggs) > 0 {
		aggsQ := make(kvStr2Inf)
		for _, agg := range aggs {
			aggsQ[agg] = kvStr2Inf{
				"terms": kvStr2Inf{
					"field": fmt.Sprintf("%s.keyword", agg),
					"size":  constants.DefaultLimit,
				},
			}
		}
		body["aggs"] = aggsQ
	}

	return &body
}
