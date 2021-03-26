package object

import (
	"testing"

	"vindr-lab-api/constants"
	"vindr-lab-api/entities"

	"github.com/stretchr/testify/assert"
)

var object = Object{
	ID:      "id",
	Created: 0,
	Meta: &entities.MetaData{
		MaskedSOPInstanceUID:    "",
		MaskedSeriesInstanceUID: "",
		MaskedStudyInstanceUID:  "",
		SOPInstanceUID:          "",
		SeriesInstanceUID:       "",
		StudyInstanceUID:        "",
	},
	ProjectID: "project",
	StudyID:   "study",
	Type:      constants.ObjectTypeStudy,
}

var objectBig = ObjectBig{
	ProjectID:             "project",
	ListSOPInstanceUID:    &[]string{},
	ListSeriesInstanceUID: &[]string{},
	ListStudyInstanceUID:  &[]string{},
	StudyID:               "study",
}

func TestIsValidType(t *testing.T) {
	{
		object := Object{
			Type: "",
		}
		assert.Equal(t, false, object.IsValidType())
	}
	{
		assert.Equal(t, true, object.IsValidType())
	}
}

func TestIsValidObject(t *testing.T) {
	{
		assert.Equal(t, false, object.IsValidObject())
	}
	{
		object := object
		object.Type = constants.ObjectTypeStudy
		object.Meta.StudyInstanceUID = "study"
		assert.Equal(t, true, object.IsValidObject())
	}
	{
		object := object
		object.Type = constants.ObjectTypeSeries
		object.Meta.StudyInstanceUID = "study"
		object.Meta.SeriesInstanceUID = "series"
		assert.Equal(t, true, object.IsValidObject())
	}
	{
		object := object
		object.Type = constants.ObjectTypeImage
		object.Meta.StudyInstanceUID = "study"
		object.Meta.SeriesInstanceUID = "series"
		object.Meta.SOPInstanceUID = "sop"
		assert.Equal(t, true, object.IsValidObject())
	}
}

func TestToString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", object.String())
	}
	{
		assert.NotEqual(t, "{}", objectBig.String())
	}
}
