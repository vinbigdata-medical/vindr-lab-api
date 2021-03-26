package session

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

type SessionES struct {
	esClient   *elasticsearch.Client
	indexAlias string
	logger     *zap.Logger
}

func NewSessionStore(es *elasticsearch.Client, indexAlias string, logger *zap.Logger) *SessionES {
	return &SessionES{
		es, indexAlias, logger,
	}
}

type kvStr2Inf = map[string]interface{}

//Get get one ESReturn
func (store *SessionES) Get(queries map[string][]string, qs string) (*Session, *entities.ESReturn, error) {
	sessions, esReturn, err := store.GetSlice(queries, qs, 0, 1, "", nil)
	if err != nil {
		return nil, nil, err
	}
	if len(sessions) > 0 {
		return &sessions[0], esReturn, nil
	}
	return nil, esReturn, errors.New("Return is empty")
}

func (store *SessionES) Create(session Session) error {
	req := esapi.IndexRequest{
		Index:      store.indexAlias,
		DocumentID: session.SessionID,
		Body:       strings.NewReader(session.String()),
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
		return fmt.Errorf("%s ERROR indexing document ID=%s", res.Status(), session.SessionID)
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

func (store *SessionES) GetSlice(queries map[string][]string, qs string, from, size int, sort string, aggs []string) ([]Session, *entities.ESReturn, error) {
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
		es.Search.WithIndex(store.indexAlias),
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

	sessions := make([]Session, 0)
	for _, hit := range esReturn.Hits.Hits {
		var study Session
		mapData := hit.Source
		bytesData, _ := json.Marshal(mapData)
		err := json.Unmarshal(bytesData, &study)
		if err == nil {
			sessions = append(sessions, study)
		}
	}

	return sessions, &esReturn, nil
}
