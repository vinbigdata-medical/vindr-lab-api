package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var project = Project{
	ID:            "id",
	Created:       0,
	CreatorID:     "creator",
	Description:   "desc",
	LabelGroupIDs: []string{},
	Name:          "name",
}

func TestString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", project.String())
	}
}
