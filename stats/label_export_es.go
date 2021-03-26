package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"vindr-lab-api/entities"
	"vindr-lab-api/utils"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"go.uber.org/zap"
)

type kvStr2Inf = map[string]interface{}

type LabelExportES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewLabelExportStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *LabelExportES {
	return &LabelExportES{
		es, indexPrefix, logger,
	}
}

func (store *LabelExportES) getIndexName(IndexPrefix string, label LabelExport) string {
	indexTime := utils.ConvertTimeStampToTime(label.Created)
	index := fmt.Sprintf("%s_%d%02d", IndexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func (store *LabelExportES) getIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

// Create function
func (store *LabelExportES) Create(exportLabel LabelExport) error {
	req := esapi.IndexRequest{
		Index:      store.getIndexName(store.indexPrefix, exportLabel),
		DocumentID: exportLabel.ID,
		Body:       strings.NewReader(exportLabel.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), exportLabel.ID)
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
func (store *LabelExportES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]LabelExport, *entities.ESReturn, error) {
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
	utils.LogInfo("[%s] %d hits; took: %dms", res.Status(), esReturn.Hits.Total.Value, esReturn.Took)

	labels := make([]LabelExport, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var label LabelExport
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &label)
		if err == nil {
			labels = append(labels, label)
		}
	}

	return labels, &esReturn, nil
}

// Delete function
func (store *LabelExportES) Delete(label LabelExport) error {
	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["query"] = kvStr2Inf{
		"match": kvStr2Inf{
			"id": label.ID,
		},
	}

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{store.getIndexWildcard(store.indexPrefix)},
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
