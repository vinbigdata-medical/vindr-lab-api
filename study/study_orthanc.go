package study

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"vindr-lab-api/entities"
	"vindr-lab-api/utils"

	"github.com/gojektech/heimdall/v6/httpclient"
)

type StudyOrthanC struct {
	uri        string
	httpClient *httpclient.Client
}

func NewStudyOrthanC(uri string) *StudyOrthanC {
	timeout := 5000 * time.Millisecond

	tr := http.DefaultTransport.(*http.Transport)
	tr.TLSClientConfig.InsecureSkipVerify = true
	client := &http.Client{Transport: tr}

	httpClient := httpclient.NewClient(
		httpclient.WithHTTPClient(client),
		httpclient.WithHTTPTimeout(timeout),
		httpclient.WithRetryCount(3),
	)

	return &StudyOrthanC{
		uri:        uri,
		httpClient: httpClient,
	}
}

func (orthanc *StudyOrthanC) FindObjectByUID(scope, uid string) (string, error) {
	var buf bytes.Buffer

	level := ""
	switch scope {
	case "Study":
		level = "Study"
		break
	case "Series":
		level = "Series"
		break
	case "SOP":
		level = "Instance"
		break
	}

	body := &kvStr2Inf{
		"Level": level,
		"Limit": 100,
		"Query": kvStr2Inf{
			fmt.Sprintf("%sInstanceUID", scope): uid,
		},
	}
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", fmt.Errorf("Error encoding query: %s", err)
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/tools/find", orthanc.uri), &buf)
	if err != nil {
		return "", err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", errors.New(res.Status)
	}

	studies := make([]string, 0)
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&studies); err != nil {
		return "", fmt.Errorf("Error parsing the response body: %s", err)
	}

	if len(studies) > 0 {
		return studies[0], nil
	}

	return "", errors.New("[MANUAL] Data is empty")
}

func (orthanc *StudyOrthanC) DeleteDicomFile(studyOrthanC *StudyOrthanC, s Study) error {
	orthancStudyID, err := studyOrthanC.FindObjectByUID("Study", fmt.Sprintf("%s.%s", s.ProjectID, s.DICOMTags.StudyInstanceUID[0]))
	if err != nil {
		utils.LogError(err)
		return err
	}

	err = orthanc.DeleteStudy(orthancStudyID)
	if err != nil {
		utils.LogError(err)
		return err
	}

	return nil
}

func (orthanc *StudyOrthanC) DeleteStudy(orthancStudyID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/studies/%s", orthanc.uri, orthancStudyID), nil)
	if err != nil {
		return err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}

	return nil
}

func (orthanc *StudyOrthanC) GetInstancesByStudy(orthancStudyID string) (*[]entities.OrthancInstance, error) {
	uri := fmt.Sprintf("%s/studies/%s/instances", orthanc.uri, orthancStudyID)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	tags := make([]entities.OrthancInstance, 0)
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	return &tags, nil
}

func (orthanc *StudyOrthanC) GetInstancesBySeries(orthancStudyID string) (*[]entities.OrthancInstance, error) {
	uri := fmt.Sprintf("%s/series/%s/instances", orthanc.uri, orthancStudyID)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	tags := make([]entities.OrthancInstance, 0)
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	return &tags, nil
}

func (orthanc *StudyOrthanC) GetSimplifiedTagsByID(orthancImageID string) (*entities.OrthancSimplfiedTags, error) {
	uri := fmt.Sprintf("%s/instances/%s/simplified-tags", orthanc.uri, orthancImageID)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	tags := entities.OrthancSimplfiedTags{}
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	return &tags, nil
}

// DownloadStudy from OrthanC
// format: dicom, zip
func (orthanc *StudyOrthanC) DownloadStudy(orthancStudyID, format, filepath string) error {
	dlType := "archive"
	if format == "zip" {
		dlType = "media"
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/studies/%s/%s", orthanc.uri, orthancStudyID, dlType), nil)
	if err != nil {
		return err
	}
	res, err := orthanc.httpClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, res.Body)

	return err
}
