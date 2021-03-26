package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"

	"github.com/gin-gonic/gin"
)

func ConvertTimeStampToTime(timestamp int64) time.Time {
	i, err := strconv.ParseInt(strconv.FormatInt(timestamp/1000, 10), 10, 64)
	if err != nil {
		panic(err)
	}
	return time.Unix(i, 0)
}

func ConvertESReturnToObject(esReturn entities.ESReturn, aType interface{}) *interface{} {
	object := &aType
	if len(esReturn.Hits.Hits) > 0 {
		mapData := esReturn.Hits.Hits[0].Source
		bytesData, _ := json.Marshal(mapData)
		json.Unmarshal(bytesData, &object)
		return object
	}
	return nil
}

func ConvertGinRequestToParams(c *gin.Context) (map[string][]string, string, int, int, string, []string) {
	queryStr := c.Query(constants.ParamSearch)
	queryStr, _ = url.QueryUnescape(queryStr)
	fmt.Println(queryStr)

	size, err := strconv.Atoi(c.Query(constants.ParamLimit))
	if err != nil {
		size = constants.DefaultLimit
	}
	from, err := strconv.Atoi(c.Query(constants.ParamOffset))
	if err != nil {
		from = constants.DefaultOffset
	}
	sort := c.Query(constants.ParamSort)
	aggs := c.QueryArray(constants.ParamAggregation)

	queries := ConvertQueryParamsToMap(c)

	return queries, queryStr, from, size, sort, aggs
}

// FindInSlice takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func FindInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func ConvertMapToString(m map[string]interface{}) string {
	jsonString, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("%v", m)
	}
	return string(jsonString)
}
