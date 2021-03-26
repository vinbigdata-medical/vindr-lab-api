package project

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

type ProjectES struct {
	esClient    *elasticsearch.Client
	indexPrefix string
	logger      *zap.Logger
}

func NewProjectStore(es *elasticsearch.Client, indexPrefix string, logger *zap.Logger) *ProjectES {
	return &ProjectES{
		es, indexPrefix, logger,
	}
}

type kvStr2Inf = map[string]interface{}

func getIndexName(IndexPrefix string, project Project) string {
	indexTime := utils.ConvertTimeStampToTime(project.Created)
	index := fmt.Sprintf("%s_%d%02d", IndexPrefix, indexTime.Year(), indexTime.Month())
	return index
}

func getIndexWildcard(IndexPrefix string) string {
	index := fmt.Sprintf("%s_*", IndexPrefix)
	return index
}

// Create function
func (store *ProjectES) Create(project Project) error {
	req := esapi.IndexRequest{
		Index:      getIndexName(store.indexPrefix, project),
		DocumentID: project.ID,
		Body:       strings.NewReader(project.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), project.ID)
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
func (store *ProjectES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]Project, *entities.ESReturn, error) {
	es := store.esClient

	var (
		esReturn entities.ESReturn
		buf      bytes.Buffer
		esError  entities.ESError
	)

	body := utils.ConvertInputsToESQueryBody(queries, qs, from, size, sort, aggs)
	bytes, _ := json.Marshal(body)
	fmt.Println(string(bytes))

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

	projects := make([]Project, 0)
	// Cast to array
	for _, hit := range esReturn.Hits.Hits {
		var project Project
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &project)
		if err == nil {
			projects = append(projects, project)
		}

	}

	return projects, &esReturn, nil
}

// Query get all
func (store *ProjectES) Query(queries map[string][]string, qs string, from, size int, sort string, aggs []string, f func(projects []Project, es entities.ESReturn)) error {
	from1 := from
	size1 := size
	for true {
		projects, esReturn, err := store.GetSlice(queries, qs, from1, size1, sort, aggs)
		if err != nil {
			return err
		}

		f(projects, *esReturn)

		if len(projects) < size1 {
			break
		}

		from1 += size1
	}
	return nil
}

//Get get one ESReturn
func (store *ProjectES) Get(queries map[string][]string, qs string) (*Project, *entities.ESReturn, error) {
	projects, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(projects) > 0 {
		return &projects[0], esReturn, nil
	}
	return nil, esReturn, nil
}

// Delete function
func (store *ProjectES) Delete(project Project) error {
	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["query"] = kvStr2Inf{
		"match": kvStr2Inf{
			"id": project.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), project.ID)
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
func (store *ProjectES) Update(project Project, update map[string]interface{}) error {
	_, esReturn, err := store.Get(nil, fmt.Sprintf("_id:%s", project.ID))
	if err != nil {
		return err
	}
	indexName := esReturn.Hits.Hits[0].Index

	now := time.Now().UnixNano() / int64(time.Millisecond)
	update["modified"] = now

	var buf bytes.Buffer
	body := kvStr2Inf{}
	body["doc"] = update

	fmt.Println(project.String())
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: project.ID,
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
		return fmt.Errorf("%s ERROR updating document ID=%s", res.Status(), project.ID)
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

type ProjectUpdateRequest struct {
	Index   string
	ID      string
	Project Project
}

type ESUpdateBody struct {
	Doc Project `json:"doc"`
}

func (store *ProjectES) Persist(ctx context.Context, p *ProjectUpdateRequest) error {
	var body = ESUpdateBody{
		Doc: p.Project,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return fmt.Errorf("Error encoding query: %s", err)
	}
	req := esapi.UpdateRequest{
		Index:      p.Index,
		DocumentID: p.ID,
		Refresh:    "true",
		Body:       &buf,
	}

	// Return an API response object from request
	_, err := req.Do(ctx, store.esClient)
	return err
}
