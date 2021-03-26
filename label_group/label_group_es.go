package label_group

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"vindr-lab-api/entities"
	"vindr-lab-api/utils"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"go.uber.org/zap"
)

type LabelGroupES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewLabelGroupStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *LabelGroupES {
	return &LabelGroupES{
		esClient:    es,
		indexPrefix: indexPrefix,
		logger:      logger,
	}
}

type kvStr2Inf = map[string]interface{}

func getIndexName(indexPrefix string, labelGroup LabelGroup) string {
	indexTime := utils.ConvertTimeStampToTime(labelGroup.Created)
	index := fmt.Sprintf("%s_%d%02d", indexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func getIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

// CreateLabelGroup function
func (store *LabelGroupES) CreateLabelGroup(labelGroup LabelGroup) error {
	req := esapi.IndexRequest{
		Index:      getIndexName(store.indexPrefix, labelGroup),
		DocumentID: labelGroup.ID,
		Body:       strings.NewReader(labelGroup.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), labelGroup.ID)
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

// GetSliceByMap function
func (store *LabelGroupES) GetSliceByMap(queries map[string][]string, qs string, from, size int, sort string, aggs []string) (*map[string]LabelGroup, error) {
	labelGroups, _, err := store.GetSlice(queries, qs, from, size, sort, aggs)
	mapLabelGroups := make(map[string]LabelGroup)
	for _, v := range labelGroups {
		mapLabelGroups[v.ID] = v
	}
	return &mapLabelGroups, err
}

func (store *LabelGroupES) Get(queries map[string][]string, qs string) (*LabelGroup, *entities.ESReturn, error) {
	studies, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(studies) > 0 {
		return &studies[0], esReturn, nil
	}
	return nil, esReturn, nil
}

// GetSlice function
func (store *LabelGroupES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]LabelGroup, *entities.ESReturn, error) {
	es := store.esClient

	var (
		esReturn entities.ESReturn
		buf      bytes.Buffer
		esError  entities.ESError
	)

	body := utils.ConvertInputsToESQueryBody(queries, qs, from, size, sort, aggs)
	utils.LogDebug(utils.ConvertMapToString(*body))

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, nil, fmt.Errorf("Error encoding query: %s", err)
	}

	// Perform the search request.
	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(getIndexWildcard(store.indexPrefix)),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)

	if err != nil {
		log.Fatalf("Error getting response: %s", err)
		return nil, nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		if err := json.NewDecoder(res.Body).Decode(&esError); err != nil {
			return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			return nil, nil, fmt.Errorf("[%s] %s: %s", res.Status(), esError.Error.Type, esError.Error.Reason)
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&esReturn); err != nil {
		return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status, number of results, and request duration.
	utils.LogInfo("[%s] %d hits; took: %dms", res.Status(), esReturn.Hits.Total.Value, esReturn.Took)

	labelGroups := make([]LabelGroup, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var labelGroup LabelGroup
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &labelGroup)
		if err == nil {
			labelGroups = append(labelGroups, labelGroup)
		}
	}

	return labelGroups, &esReturn, nil
}

// Delete function
func (store *LabelGroupES) Delete(labelGroup LabelGroup) error {
	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["query"] = kvStr2Inf{
		"match": kvStr2Inf{
			"id": labelGroup.ID,
		},
	}

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{getIndexWildcard(store.indexPrefix)},
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), labelGroup.ID)
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
func (store *LabelGroupES) Update(labelGroup LabelGroup, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", labelGroup.ID))
	if err != nil {
		return err
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	update["modified"] = now

	indexName := esReturn.Hits.Hits[0].Index

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	fmt.Println(labelGroup.String())
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: labelGroup.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), labelGroup.ID)
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
