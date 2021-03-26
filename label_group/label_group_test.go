package label_group

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var labelGroup = LabelGroup{
	ID:        "id",
	Color:     "#000000",
	Name:      "name",
	Created:   0,
	CreatorID: "creator",
}

func TestToString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", labelGroup.String())
	}
	{
		labelGroup := LabelGroup{}
		assert.Equal(t, "{\"id\":\"\",\"name\":\"\",\"color\":\"\",\"created\":0,\"creator_id\":\"\"}", labelGroup.String())
	}
}
