package study

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

	"github.com/dustin/go-humanize"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"go.uber.org/zap"
)

type TaskES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewTaskStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *TaskES {
	return &TaskES{
		es, indexPrefix, logger,
	}
}

func getTaskIndexName(indexPrefix string, task Task) string {
	indexTime := utils.ConvertTimeStampToTime(task.Created)
	index := fmt.Sprintf("%s_%d%02d", indexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func getTaskIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

// Create function
func (store *TaskES) Create(task Task) error {
	req := esapi.IndexRequest{
		Index:      getTaskIndexName(store.indexPrefix, task),
		DocumentID: task.ID,
		Body:       strings.NewReader(task.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), task.ID)
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

func (store *TaskES) Bulk(tasks []Task) error {
	var (
		buf bytes.Buffer
		res *esapi.Response
		raw map[string]interface{}
		blk *entities.ESBulkResponse

		indexName = getTaskIndexName(store.indexPrefix, tasks[0])

		numItems   int
		numErrors  int
		numIndexed int
		numBatches int
		currBatch  int
	)

	count := len(tasks)
	batch := 10

	utils.LogInfo("\x1b[1mBulk\x1b[0m: documents [%s] batch size [%s]",
		humanize.Comma(int64(count)), humanize.Comma(int64(batch)))

	es := store.esClient

	if count%batch == 0 {
		numBatches = (count / batch)
	} else {
		numBatches = (count / batch) + 1
	}

	start := time.Now().UTC()

	for i, object := range tasks {
		numItems++

		currBatch = i / batch
		if i == count-1 {
			currBatch++
		}

		meta := []byte(fmt.Sprintf(`{ "index" : { "_id" : "%s" } }%s`, object.ID, "\n"))

		data, err := json.Marshal(object)
		if err != nil {
			log.Fatalf("Cannot encode %s: %s", object.ID, err)
		}

		data = append(data, "\n"...)

		buf.Grow(len(meta) + len(data))
		buf.Write(meta)
		buf.Write(data)

		if i > 0 && i%batch == 0 || i == count-1 {
			utils.LogDebug("[%d/%d] ", currBatch, numBatches)

			res, err = es.Bulk(bytes.NewReader(buf.Bytes()), es.Bulk.WithIndex(indexName), es.Bulk.WithRefresh("true"))
			if err != nil {
				log.Fatalf("Failure indexing batch %d: %s", currBatch, err)
				return err
			}

			if res.IsError() {
				numErrors += numItems
				if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
					log.Fatalf("Failure to to parse response body: %s", err)
				} else {
					utils.LogDebug("  Error: [%d] %s: %s",
						res.StatusCode,
						raw["error"].(map[string]interface{})["type"],
						raw["error"].(map[string]interface{})["reason"],
					)
				}
			} else {
				if err := json.NewDecoder(res.Body).Decode(&blk); err != nil {
					log.Fatalf("Failure to to parse response body: %s", err)
				} else {
					for _, d := range blk.Items {
						if d.Index.Status > 201 {
							numErrors++
							utils.LogDebug("  Error: [%d]: %s: %s: %s: %s",
								d.Index.Status, d.Index.Error.Type, d.Index.Error.Reason, d.Index.Error.Cause.Type, d.Index.Error.Cause.Reason,
							)
						} else {
							numIndexed++
						}
					}
				}
			}

			res.Body.Close()

			buf.Reset()
			numItems = 0
		}
	}

	dur := time.Since(start)

	if numErrors > 0 {
		utils.LogDebug("Indexed [%s] documents with [%s] errors in %s (%s docs/sec)",
			humanize.Comma(int64(numIndexed)),
			humanize.Comma(int64(numErrors)),
			dur.Truncate(time.Millisecond),
			humanize.Comma(int64(1000.0/float64(dur/time.Millisecond)*float64(numIndexed))),
		)
	} else {
		utils.LogDebug("Sucessfuly indexed [%s] documents in %s (%s docs/sec)",
			humanize.Comma(int64(numIndexed)),
			dur.Truncate(time.Millisecond),
			humanize.Comma(int64(1000.0/float64(dur/time.Millisecond)*float64(numIndexed))))
	}
	return nil
}

// Get function
func (store *TaskES) Get(queries map[string][]string, qs string) (*Task, *entities.ESReturn, error) {
	tasks, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(tasks) == 0 {
		return nil, esReturn, errors.New("Item not found")
	}
	return &tasks[0], esReturn, nil
}

func (store *TaskES) Query(queries map[string][]string, qs string, from, size int, sort string, aggs []string, f func(tasks []Task, es entities.ESReturn)) error {
	from1 := from
	size1 := size
	for true {
		tasks, esReturn, err := store.GetSlice(queries, qs, from1, size1, sort, aggs)
		if err != nil {
			return err
		}

		f(tasks, *esReturn)

		if len(tasks) < size1 {
			break
		}

		from1 += size1
	}
	return nil
}

// GetSlice function
func (store *TaskES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]Task, *entities.ESReturn, error) {
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
		es.Search.WithIndex(getTaskIndexWildcard(store.indexPrefix)),
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
		}
		// Print the response status and error information.
		return nil, nil, fmt.Errorf("[%s] %s: %s", res.Status(), esError.Error.Type, esError.Error.Reason)
	}

	if err := json.NewDecoder(res.Body).Decode(&esReturn); err != nil {
		return nil, nil, fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status, number of results, and request duration.
	tasks := make([]Task, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var task Task
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &task)
		if err == nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, &esReturn, nil
}

// Delete function
func (store *TaskES) Delete(queries map[string][]string, qs string) error {
	var buf bytes.Buffer
	body := utils.ConvertInputsToESQueryBody(queries, qs, -1, -1, "", nil)
	utils.LogDebug(utils.ConvertMapToString(*body))

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{getTaskIndexWildcard(store.indexPrefix)},
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
func (store *TaskES) Update(task Task, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", task.ID))
	if err != nil {
		return err
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)
	update["modified"] = now

	indexName := esReturn.Hits.Hits[0].Index

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: task.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), task.ID)
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
