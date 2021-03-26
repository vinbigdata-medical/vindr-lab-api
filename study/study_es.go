package study

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"vindr-lab-api/entities"
	"vindr-lab-api/utils"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"go.uber.org/zap"
)

type StudyES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewStudyStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *StudyES {
	return &StudyES{
		es, indexPrefix, logger,
	}
}

type kvStr2Inf = map[string]interface{}

func getStudyIndexName(indexPrefix string, study Study) string {
	indexTime := utils.ConvertTimeStampToTime(study.TimeInserted)
	index := fmt.Sprintf("%s_%d%02d", indexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func getStudyIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

// Create function
func (store *StudyES) Create(study Study) error {
	req := esapi.IndexRequest{
		Index:      getStudyIndexName(store.indexPrefix, study),
		DocumentID: study.ID,
		Body:       strings.NewReader(study.String()),
		Refresh:    "true",
	}

	// Return an API response object from request
	ctx := context.Background()
	res, err := req.Do(ctx, store.esClient.Transport)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("IndexRequest ERROR: %s", err))
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), study.ID)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	if resMap["result"] == "created" {
		return nil
	}

	return err
}

//Get get one ESReturn
func (store *StudyES) Get(queries map[string][]string, qs string) (*Study, *entities.ESReturn, error) {
	studies, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(studies) > 0 {
		return &studies[0], esReturn, nil
	}
	return nil, esReturn, errors.New("Return is empty")
}

// GetSlice function
func (store *StudyES) GetSlice(queries map[string][]string, qs string,
	from, size int, sort string, aggs []string) ([]Study, *entities.ESReturn, error) {
	es := store.esClient

	var (
		esReturn entities.ESReturn
		buf      bytes.Buffer
		esError  entities.ESError
	)

	body := utils.ConvertInputsToESQueryBody(queries, qs, from, size, sort, aggs)
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, nil, fmt.Errorf("Error encoding query: %s", err)
	}
	utils.LogDebug(utils.ConvertMapToString(*body))

	// Perform the search request.
	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(getStudyIndexWildcard(store.indexPrefix)),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)

	if err != nil {
		fmt.Println(fmt.Errorf("Error getting response: %s", err))
		return nil, nil, fmt.Errorf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if err := json.NewDecoder(res.Body).Decode(&esError); err != nil {
			return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
		} else {
			return nil, nil, fmt.Errorf("[%s] %s: %s", res.Status(), esError.Error.Type, esError.Error.Reason)
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&esReturn); err != nil {
		return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status, number of results, and request duration.
	utils.LogDebug("[%s] %d hits; took: %dms", res.Status(), esReturn.Hits.Total.Value, esReturn.Took)

	studies := make([]Study, 0)
	for _, hit := range esReturn.Hits.Hits {
		var study Study
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &study)
		if err == nil {
			studies = append(studies, study)
		}
	}

	return studies, &esReturn, nil
}

// Query get all
func (store *StudyES) Query(queries map[string][]string, qs string, from, size int, sort string, aggs []string, f func(studies []Study, es entities.ESReturn)) error {
	from1 := from
	size1 := size

	for true {
		studies, esReturn, err := store.GetSlice(queries, qs, from1, size1, sort, aggs)
		if err != nil {
			return err
		}

		f(studies, *esReturn)

		if len(studies) < size1 {
			break
		}

		from1 += size1
	}
	return nil
}

// Delete function
func (store *StudyES) Delete(queries map[string][]string, qs string) error {
	var buf bytes.Buffer
	body := utils.ConvertInputsToESQueryBody(queries, qs, -1, -1, "", nil)
	utils.LogDebug(utils.ConvertMapToString(*body))

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{getStudyIndexWildcard(store.indexPrefix)},
		Body:    &buf,
		Refresh: &refresh,
	}

	// Return an API response object from request
	ctx := context.Background()
	res, err := req.Do(ctx, store.esClient)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("UpdateRequest ERROR: %s", err))
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%s ERROR deleting document", res.Status())
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	if resMap["result"] == "created" {
		return nil
	}

	return err
}

// Update function
func (store *StudyES) Update(study Study, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", study.ID))
	if err != nil {
		return err
	}

	indexName := esReturn.Hits.Hits[0].Index

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: study.ID,
		Refresh:    "true",
		Body:       &buf,
	}

	// Return an API response object from request
	ctx := context.Background()
	res, err := req.Do(ctx, store.esClient)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("UpdateRequest ERROR: %s", err))
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), study.ID)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	if resMap["result"] == "created" {
		return nil
	}

	return err
}
