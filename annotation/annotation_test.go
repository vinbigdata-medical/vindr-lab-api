package annotation

import (
	"testing"
	"vindr-lab-api/constants"

	"github.com/stretchr/testify/assert"
)

var box = []Point2D{
	{
		X: 1,
		Y: 1,
	},
	{
		X: 1,
		Y: 1,
	},
}
var polygon = []Point2D{
	{
		X: 1,
		Y: 1,
	},
	{
		X: 2,
		Y: 2,
	},
	{
		X: 2,
		Y: 2,
	},
}

var media = Media{
	Bytes:    "LzlqLzRBQ",
	MimeType: "png",
}

var annotation = Annotation{
	CreatorID:   "creator",
	Description: "desc",
	Event:       constants.EventCreate,
	LabelIDs: []string{
		"l1",
		"l2",
		"l3",
	},
	// Meta: &entities.MetaData{
	// 	MaskedStudyInstanceUID:  "",
	// 	MaskedSeriesInstanceUID: "",
	// 	MaskedSOPInstanceUID:    "",
	// },
	ObjectID:  "object",
	ProjectID: "project",
	Created:   0,
	ID:        "id",
	Type:      constants.AntnTypeBox,
	StudyID:   "study",
	TaskID:    "task",
}

func TestNewAnnotation(t *testing.T) {
	{
		annotation1 := Annotation{}
		annotation1.NewAnnotation()
		assert.Greater(t, annotation1.Created, (int64)(0))
	}
}

func TestVerifyBoundingBox(t *testing.T) {
	{
		box := box
		ret := IsBoundingBox(box)
		assert.Equal(t, true, ret)
	}
	{
		box := box
		box = box[:1]
		ret := IsBoundingBox(box)
		assert.Equal(t, false, ret)
	}
}

func TestVerifyPolygon(t *testing.T) {
	polygon := polygon
	ret := IsPolytgon(polygon)
	assert.Equal(t, true, ret)
}

func TestToString(t *testing.T) {
	{
		assert.NotEqual(t, "{}", annotation.String())
	}
	{
		annotation := Annotation{}
		assert.Equal(t, "{\"id\":\"\",\"object_id\":\"\",\"project_id\":\"\",\"creator_id\":\"\",\"description\":\"\",\"type\":\"\",\"event\":\"\",\"created\":0}", annotation.String())
	}
}

func TestAnnotationType(t *testing.T) {
	{
		assert.Equal(t, true, annotation.IsValidAnnotationType())
	}
	{
		annotation := annotation
		annotation.Type = "x"
		assert.Equal(t, false, annotation.IsValidAnnotationType())
	}
}

func TestAnnotationValidate(t *testing.T) {
	//false test
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeBox
		annotation.Data = []interface{}{}
		assert.Equal(t, false, annotation.IsValidAnnotation())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypePolygon
		annotation.Data = []interface{}{}
		assert.Equal(t, false, annotation.IsValidAnnotation())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		assert.Equal(t, false, annotation.IsValidAnnotation())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeTag
		assert.Equal(t, true, annotation.IsValidAnnotation())
	}
	//true test
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeBox
		annotation.Data = []interface{}{
			box,
		}
		assert.Equal(t, false, annotation.IsValidAnnotation())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypePolygon
		annotation.Data = []interface{}{
			polygon,
		}
		assert.Equal(t, false, annotation.IsValidAnnotation())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		annotation.Data = Media{
			Bytes:    "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYA",
			MimeType: "png",
		}
		assert.Equal(t, true, annotation.IsValidAnnotation())
	}
}

func TestIsValidMeta(t *testing.T) {
	{
		assert.Equal(t, true, annotation.IsValidMeta())
	}
	{
		annotation := annotation
		annotation.Meta = nil
		assert.Equal(t, false, annotation.IsValidMeta())
	}
}

func TestValidData(t *testing.T) {

	boxInterface := []interface{}{
		map[string]interface{}{
			"x": box[0].X,
			"y": box[0].Y,
		},
		map[string]interface{}{
			"x": box[1].X,
			"y": box[1].Y,
		},
	}
	polygonInterface := []interface{}{
		map[string]interface{}{
			"x": polygon[0].X,
			"y": polygon[0].Y,
		},
		map[string]interface{}{
			"x": polygon[1].X,
			"y": polygon[1].Y,
		},
		map[string]interface{}{
			"x": polygon[2].X,
			"y": polygon[2].Y,
		},
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeBox
		annotation.Data = boxInterface
		assert.Equal(t, true, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeBox
		annotation.Data = boxInterface[:1]
		assert.Equal(t, false, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypePolygon
		annotation.Data = polygonInterface
		assert.Equal(t, true, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypePolygon
		annotation.Data = polygonInterface[:1]
		assert.Equal(t, false, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		annotation.Data = kvStr2Inf{
			"mime_type": "png",
			"bytes":     "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYA",
		}
		assert.Equal(t, true, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		annotation.Data = kvStr2Inf{
			"bytes":     "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYA",
			"mime_type": "x",
		}
		assert.Equal(t, false, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		annotation.Data = kvStr2Inf{
			"bytes":     "x",
			"mime_type": "png",
		}
		assert.Equal(t, false, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeMask
		annotation.Data = kvStr2Inf{
			"bytes":     "x",
			"mime_type": "x",
		}
		assert.Equal(t, false, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = constants.AntnTypeTag
		assert.Equal(t, true, annotation.IsValidData())
	}
	{
		annotation := annotation
		annotation.Type = "x"
		assert.Equal(t, false, annotation.IsValidData())
	}
}
