package annotation

import (
	"testing"
	"vindr-lab-api/constants"

	"github.com/stretchr/testify/assert"
)

var label = Label{
	ID:             "id",
	Type:           constants.LabelTypeImpression,
	Scope:          constants.LabelScopeImage,
	AnnotationType: constants.AntnTypeBox,
	CreatorID:      "creator",
	LabelGroupID:   "label_group",
	Color:          "#000000",
	Name:           "name",
}

func TestValidateLabelScope(t *testing.T) {
	{
		assert.Equal(t, true, label.IsValidLabelScope())
	}
	{
		label := label
		label.Scope = "x"
		assert.Equal(t, false, label.IsValidLabelScope())
	}
}

func TestValidateLabelType(t *testing.T) {
	{
		assert.Equal(t, true, label.IsValidLabelType())
	}
	{
		label := label
		label.Type = "x"
		assert.Equal(t, false, label.IsValidLabelType())
	}
}

func TestValidateAntnType(t *testing.T) {
	{
		assert.Equal(t, true, label.IsValidAnnotationType())
	}
	{
		label := label
		label.AnnotationType = "x"
		assert.Equal(t, false, label.IsValidAnnotationType())
	}
}

func TestValidateLabel(t *testing.T) {
	{
		assert.Equal(t, true, label.IsValidLabel())
	}
	{
		label := label
		label.AnnotationType = "x"
		assert.Equal(t, false, label.IsValidLabel())
	}
}

func TestLabelToString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", label.String())
	}
	{
		label := Label{}
		assert.Equal(t, "{\"id\":\"\",\"type\":\"\",\"scope\":\"\",\"annotation_type\":\"\",\"name\":\"\",\"short_name\":\"\",\"parent_label_id\":\"\",\"description\":\"\",\"color\":\"\",\"creator_id\":\"\"}", label.String())
	}
}
