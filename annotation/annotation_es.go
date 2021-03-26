package annotation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

type AnnotationES struct {
	esClient      *elasticsearch.Client
	indexPrefix   string
	indexTemplate string
	logger        *zap.Logger
}

func NewAnnotationStore(client *elasticsearch.Client, indexPrefix, indexTemplate string, logger *zap.Logger) *AnnotationES {
	return &AnnotationES{
		client, indexPrefix, indexTemplate, logger,
	}
}

func getIndexName(indexPrefix string, antn Annotation) string {
	indexTime := utils.ConvertTimeStampToTime(antn.Created)
	index := strings.ToLower(fmt.Sprintf("%s_%s_%d%02d", indexPrefix, antn.Type, indexTime.Year(), indexTime.Month()))
	return index
}

func getIndexWildcard(indexPrefix string) string {
	return fmt.Sprintf("%s_*", indexPrefix)
}

//Query function
func (store *AnnotationES) Query(queries map[string][]string, qs string, from int, size int, sort string, aggs []string, f func([]Annotation, entities.ESReturn)) error {
	from1 := from
	size1 := size
	for true {
		annotations, esReturn, err := store.GetSlice(queries, qs, from1, size1, sort, aggs)
		if err != nil {
			return err
		}

		f(annotations, *esReturn)

		if len(annotations) < size1 {
			break
		}

		from1 += size1
	}
	return nil
}

//GetSlice function
func (store *AnnotationES) GetSlice(queries map[string][]string, qs string, from int, size int, sort string, aggs []string) ([]Annotation, *entities.ESReturn, error) {
	es := store.esClient

	var (
		esReturn entities.ESReturn
		esError  entities.ESError
		buf      bytes.Buffer
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
	utils.LogDebug("[%s] %d hits; took: %dms", res.Status(), esReturn.Hits.Total.Value, esReturn.Took)

	antns := make([]Annotation, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var antn Annotation
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &antn)
		if err == nil {
			antns = append(antns, antn)
		}
	}

	return antns, &esReturn, nil
}

//Get get one ESReturn
func (store *AnnotationES) Get(queries map[string][]string, qs string) (*Annotation, *entities.ESReturn, error) {
	studies, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(studies) > 0 {
		return &studies[0], esReturn, nil
	}
	return nil, esReturn, nil
}

// Create function
func (store *AnnotationES) Create(antn Annotation) error {
	// utils.LogDebug(antn.String())
	req := esapi.IndexRequest{
		Index:      getIndexName(store.indexPrefix, antn),
		DocumentID: antn.ID,
		Body:       strings.NewReader(antn.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), antn.ID)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		fmt.Printf("Error parsing the response body: %s", err)
		return err
	}

	if resMap["result"] == "created" {
		return nil
	}

	return err
}

// BulkCreate function
func (store *AnnotationES) BulkCreate(objects []Annotation) error {
	var (
		buf bytes.Buffer
		res *esapi.Response
		raw map[string]interface{}
		blk *entities.ESBulkResponse

		indexName = getIndexName(store.indexPrefix, objects[0])

		numItems   int
		numErrors  int
		numIndexed int
		numBatches int
		currBatch  int
	)

	count := len(objects)
	batch := 10

	utils.LogDebug("\x1b[1mBulk\x1b[0m: documents [%s] batch size [%s]",
		humanize.Comma(int64(count)), humanize.Comma(int64(batch)))

	es := store.esClient

	if count%batch == 0 {
		numBatches = (count / batch)
	} else {
		numBatches = (count / batch) + 1
	}

	start := time.Now().UTC()

	// Loop over the collection
	//
	for i, object := range objects {
		numItems++

		currBatch = i / batch
		if i == count-1 {
			currBatch++
		}

		// Prepare the metadata payload
		//
		meta := []byte(fmt.Sprintf(`{ "index" : { "_id" : "%s" } }%s`, object.ID, "\n"))

		// Prepare the data payload: encode article to JSON
		//
		data, err := json.Marshal(object)
		if err != nil {
			log.Fatalf("Cannot encode %s: %s", object.ID, err)
		}

		// Append newline to the data payload
		//
		data = append(data, "\n"...) // <-- Comment out to trigger failure for batch

		// Append payloads to the buffer (ignoring write errors)
		//
		buf.Grow(len(meta) + len(data))
		buf.Write(meta)
		buf.Write(data)

		// When a threshold is reached, execute the Bulk() request with body from buffer
		//
		if i > 0 && i%batch == 0 || i == count-1 {
			utils.LogDebug("[%d/%d] ", currBatch, numBatches)

			res, err = es.Bulk(bytes.NewReader(buf.Bytes()), es.Bulk.WithIndex(indexName), es.Bulk.WithRefresh("true"))
			if err != nil {
				log.Fatalf("Failure indexing batch %d: %s", currBatch, err)
				return err
			}
			// If the whole request failed, print error and mark all documents as failed
			//
			if res.IsError() {
				numErrors += numItems
				if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
					log.Fatalf("Failure to to parse response body: %s", err)
				} else {
					log.Printf("Error: [%d] %s: %s",
						res.StatusCode,
						raw["error"].(map[string]interface{})["type"],
						raw["error"].(map[string]interface{})["reason"],
					)
				}
				// A successful response might still contain errors for particular documents...
				//
			} else {
				if err := json.NewDecoder(res.Body).Decode(&blk); err != nil {
					log.Fatalf("Failure to to parse response body: %s", err)
				} else {
					for _, d := range blk.Items {
						// ... so for any HTTP status above 201 ...
						//
						if d.Index.Status > 201 {
							// ... increment the error counter ...
							//
							numErrors++

							// ... and print the response status and error information ...
							utils.LogDebug("Error: [%d]: %s: %s: %s: %s",
								d.Index.Status, d.Index.Error.Type, d.Index.Error.Reason, d.Index.Error.Cause.Type, d.Index.Error.Cause.Reason,
							)
						} else {
							// ... otherwise increase the success counter.
							//
							numIndexed++
						}
					}
				}
			}

			// Close the response body, to prevent reaching the limit for goroutines or file handles
			//
			res.Body.Close()

			// Reset the buffer and items counter
			//
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

// Update function
func (store *AnnotationES) Update(antn Annotation, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", antn.ID))
	if err != nil {
		return err
	}
	indexName := esReturn.Hits.Hits[0].Index

	now := time.Now().UnixNano() / int64(time.Millisecond)
	update["modified"] = now

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: antn.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), antn.ID)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status and indexed document version.
	// fmt.Println("Status:", res.Status(), "Result:", resMap["result"], "Version:", int(resMap["_version"].(float64)), resMap)

	if resMap["result"] == "created" {
		return nil
	}

	return err
}

// DeleteAnnotation function
func (store *AnnotationES) Delete(queries map[string][]string, qs string) error {
	var buf bytes.Buffer
	body := utils.ConvertInputsToESQueryBody(queries, qs, -1, -1, "", nil)
	utils.LogDebug(utils.ConvertMapToString(*body))

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

func (store *AnnotationES) PutIndexTemplate() error {

	file := strings.Join([]string{"templates", store.indexTemplate + ".json"}, "/")
	dat, err := ioutil.ReadFile(file)
	m := make(map[string]interface{})
	json.Unmarshal(dat, &m)

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(m); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}

	create := true

	req := esapi.IndicesPutIndexTemplateRequest{
		Name:   store.indexTemplate,
		Body:   &buf,
		Create: &create,
		Human:  true,
		Pretty: true,
	}

	// Return an API response object from request
	ctx := context.Background()
	res, err := req.Do(ctx, store.esClient)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("PutTemplate ERROR: %s", err))
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%s ERROR putting template %s", res.Status(), store.indexTemplate)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status and indexed document version.
	// fmt.Println("Status:", res.Status(), "Result:", resMap["result"], "Version:", int(resMap["_version"].(float64)), resMap)

	if resMap["result"] == "created" {
		return nil
	}

	return err
}

func (store *AnnotationES) PutMapping() error {

	file := strings.Join([]string{"mappings", store.indexTemplate + ".json"}, "/")
	dat, err := ioutil.ReadFile(file)
	m := make(map[string]interface{})
	json.Unmarshal(dat, &m)

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(m); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}

	req := esapi.IndicesPutMappingRequest{
		Index:  []string{getIndexWildcard(store.indexPrefix)},
		Body:   &buf,
		Human:  true,
		Pretty: true,
	}

	// Return an API response object from request
	ctx := context.Background()
	res, err := req.Do(ctx, store.esClient)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("PutTemplate ERROR: %s", err))
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("%s ERROR putting template %s", res.Status(), store.indexTemplate)
	}

	// Deserialize the response into a map.
	var resMap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resMap); err != nil {
		return fmt.Errorf("Error parsing the response body: %s", err)
	}

	// Print the response status and indexed document version.
	// fmt.Println("Status:", res.Status(), "Result:", resMap["result"], "Version:", int(resMap["_version"].(float64)), resMap)

	if resMap["result"] == "created" {
		return nil
	}

	return err
}
