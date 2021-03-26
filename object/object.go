package object

import (
	"encoding/json"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"
)

var mapObjectType = map[string]int{
	constants.ObjectTypeStudy:  0,
	constants.ObjectTypeSeries: 1,
	constants.ObjectTypeImage:  2,
}

type Object struct {
	ID        string             `json:"id"`
	Created   int64              `json:"created"`
	Type      string             `json:"type"`
	ProjectID string             `json:"project_id"`
	StudyID   string             `json:"study_id"`
	Meta      *entities.MetaData `json:"meta,omitempty"`
}

type ObjectBig struct {
	ProjectID             string    `json:"project_id"`
	StudyID               string    `json:"study_id"`
	ListStudyInstanceUID  *[]string `json:"list_study_instance_uid,omitempty"`
	ListSeriesInstanceUID *[]string `json:"list_series_instance_uid,omitempty"`
	ListSOPInstanceUID    *[]string `json:"list_sop_instance_uid,omitempty"`
}

func (object *Object) IsValidObject() bool {
	switch object.Type {
	case "STUDY":
		if object.Meta.StudyInstanceUID != "" && object.Meta.SeriesInstanceUID == "" && object.Meta.SOPInstanceUID == "" {
			return true
		}
	case "SERIES":
		if object.Meta.StudyInstanceUID != "" && object.Meta.SeriesInstanceUID != "" && object.Meta.SOPInstanceUID == "" {
			return true
		}
	case "IMAGE":
		if object.Meta.StudyInstanceUID != "" && object.Meta.SeriesInstanceUID != "" && object.Meta.SOPInstanceUID != "" {
			return true
		}
	}
	return false
}

func (study *Object) IsValidType() bool {
	_, found := mapObjectType[study.Type]
	return found
}

func (objectBig *ObjectBig) String() string {
	b, _ := json.Marshal(objectBig)
	return string(b)
}

func (object *Object) String() string {
	b, _ := json.Marshal(object)
	return string(b)
}
