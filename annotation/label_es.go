package annotation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type LabelES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewLabelStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *LabelES {
	return &LabelES{
		es, indexPrefix, logger,
	}
}

type kvStr2Inf = map[string]interface{}

func (store *LabelES) getIndexName(IndexPrefix string, label Label) string {
	indexTime := utils.ConvertTimeStampToTime(label.Created)
	index := fmt.Sprintf("%s_%d%02d", IndexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func (store *LabelES) getIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

//Get get one ESReturn
func (store *LabelES) Get(queries map[string][]string, qs string) (*Label, *entities.ESReturn, error) {
	projects, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(projects) > 0 {
		return &projects[0], esReturn, nil
	}
	return nil, esReturn, errors.New("Return is empty")
}

// Create function
func (store *LabelES) Create(label Label) error {
	req := esapi.IndexRequest{
		Index:      store.getIndexName(store.indexPrefix, label),
		DocumentID: label.ID,
		Body:       strings.NewReader(label.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), label.ID)
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

// GetSlice function
func (store *LabelES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]Label, *entities.ESReturn, error) {
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
		es.Search.WithIndex(store.getIndexWildcard(store.indexPrefix)),
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
			fmt.Println(res.Body)
			return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
		}
		// Print the response status and error information.
		return nil, nil, fmt.Errorf("[%s] %s: %s", res.Status(), esError.Error.Type, esError.Error.Reason)
	}

	if err := json.NewDecoder(res.Body).Decode(&esReturn); err != nil {
		return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status, number of results, and request duration.
	utils.LogDebug("[%s] %d hits; took: %dms", res.Status(), esReturn.Hits.Total.Value, esReturn.Took)

	labels := make([]Label, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var label Label
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &label)
		if err == nil {
			labels = append(labels, label)
		}
	}

	return labels, &esReturn, nil
}

func (store *LabelES) Query(queries map[string][]string, qs string, from, size int, sort string, aggs []string, f func([]Label, entities.ESReturn)) error {
	from1 := from
	size1 := size
	for true {
		labels, esReturn, err := store.GetSlice(queries, qs, from1, size1, sort, aggs)
		if err != nil {
			return err
		}

		f(labels, *esReturn)

		if len(labels) < size1 {
			break
		}

		from1 += size1
	}
	return nil
}

// Delete function
func (store *LabelES) Delete(queries map[string][]string, qs string) error {
	var buf bytes.Buffer
	body := utils.ConvertInputsToESQueryBody(queries, qs, -1, -1, "", nil)
	utils.LogInfo(utils.ConvertMapToString(*body))

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
func (store *LabelES) Update(label Label, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", label.ID))
	if err != nil {
		return err
	}
	indexName := esReturn.Hits.Hits[0].Index

	now := time.Now().UnixNano() / int64(time.Millisecond)
	update["modified"] = now

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	utils.LogInfo(indexName)
	utils.LogInfo(label.String())
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: label.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), label.ID)
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
