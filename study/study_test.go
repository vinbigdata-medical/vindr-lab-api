package study

import (
	"testing"

	"vindr-lab-api/constants"

	"github.com/stretchr/testify/assert"
)

var study = Study{
	Code:         "code",
	CreatorID:    "cteator",
	DICOMTags:    &DICOMTags{},
	ID:           "id",
	Modified:     0,
	ProjectID:    "project",
	Status:       constants.StudyStatusAssigned,
	TimeInserted: 0,
}

func TestIsValidStatus(t *testing.T) {
	{
		study := Study{
			Status: "this_is_wrong",
		}
		assert.Equal(t, false, IsValidStatus(study.Status))
	}
	{
		assert.Equal(t, true, IsValidStatus(study.Status))
	}
}

func TestString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", study.String())
	}
	{
		study := Study{}
		assert.Equal(t, "{\"id\":\"\",\"modified\":0,\"time_inserted\":0,\"code\":\"\",\"status\":\"\",\"project_id\":\"\",\"creator_id\":\"\"}", study.String())
	}
}
