package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"vindr-lab-api/utils"

	"github.com/gojektech/heimdall/v6/httpclient"
)

type IDGenerator struct {
	uri    string
	client *httpclient.Client
}

func NewIDGenerator(uri string) *IDGenerator {
	timeout := 1000 * time.Millisecond
	return &IDGenerator{
		uri:    uri,
		client: httpclient.NewClient(httpclient.WithHTTPTimeout(timeout)),
	}
}

func (idGen *IDGenerator) GenNew(key string) (int, error) {
	client := idGen.client
	url := fmt.Sprintf("%s/id_generator/%s/tap", idGen.uri, key)
	utils.LogInfo(url)

	var buf bytes.Buffer
	body := make(map[string]interface{})
	json.NewEncoder(&buf).Encode(body)

	req, err := http.NewRequest("PUT", url, &buf)
	if err != nil {
		utils.LogError(err)
		return -1, err
	}
	res, err := client.Do(req)
	if err != nil {
		utils.LogError(err)
		return -1, err
	}
	defer res.Body.Close()

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return -1, err
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		utils.LogError(err)
		return -1, err
	}

	resp := make(map[string]interface{})
	err = json.Unmarshal(bytes, &resp)
	ret := int(resp["last_insert_id"].(float64))

	return ret, nil
}
