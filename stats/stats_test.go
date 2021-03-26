package stats

import (
	"testing"

	"gopkg.in/go-playground/assert.v1"
)

var labelExport = LabelExport{
	CreatorID: "creator",
	Created:   0,
	FilePath:  "path",
	ID:        "id",
	ProjectID: "project",
	Tag:       "tag.json",
}

func TestToString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", labelExport.String())
	}
	{
		labelExport := LabelExport{}
		assert.Equal(t, "{\"id\":\"\",\"created\":0,\"file_path\":\"\",\"creator_id\":\"\",\"project_id\":\"\",\"tag\":\"\"}", labelExport.String())
	}
}
