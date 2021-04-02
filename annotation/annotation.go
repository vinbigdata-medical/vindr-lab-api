package annotation

import (
	"encoding/json"
	"fmt"
	"time"

	"vindr-lab-api/constants"

	"github.com/google/uuid"
)

type Annotation struct {
	ID          string                 `json:"id"`
	ObjectID    string                 `json:"object_id"`
	ProjectID   string                 `json:"project_id"`
	CreatorID   string                 `json:"creator_id"`
	Description string                 `json:"description"`
	Data        interface{}            `json:"data,omitempty"`
	Type        string                 `json:"type"`
	Event       string                 `json:"event"`
	Created     int64                  `json:"created"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
	LabelIDs    []string               `json:"label_ids,omitempty"`
	Labels      *[]Label               `json:"labels,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
	StudyID     string                 `json:"study_id,omitempty"`
	CreatorName string                 `json:"creator_name"`
}
type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Point3D struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type Media struct {
	MimeType string `json:"mime_type"`
	Bytes    string `json:"bytes"`
}

func (antn *Annotation) NewAnnotation() {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	antn.ID = uuid.New().String()
	antn.Created = now
	antn.Event = constants.EventCreate
}

func (antn *Annotation) String() string {
	b, _ := json.Marshal(antn)

	return string(b)
}

func (antn *Annotation) IsValidAnnotationType() bool {
	_, found := AnnotationType[antn.Type]
	return found
}

func (antn *Annotation) IsValidMeta() bool {
	if antn.Meta == nil {
		return false
	}

	return true
}

func (antn *Annotation) IsValidData() bool {
	antnType := antn.Type
	switch antnType {
	case constants.AntnTypeBox, constants.AntnTypePolygon:

		objects := antn.Data.([]interface{})
		points := make([]Point2D, 0)

		for _, v := range objects {
			pair, ok := v.(map[string]interface{})
			if !ok {
				return false
			}

			point := Point2D{X: pair["x"].(float64), Y: pair["y"].(float64)}
			points = append(points, point)
		}

		if antnType == constants.AntnTypeBox {
			return IsBoundingBox(points)
		}

		return IsPolytgon(points)

	case constants.AntnTypeMask:
		str := fmt.Sprintf("%v", antn.Data)

		if str == "" {
			return false
		}

		return true

	case constants.AntnTypeTag:
		if antn.Data != nil {
			return false
		}
		return true

	case constants.AntnType3DBox:
		if antn.Data == nil {
			return false
		}
		return true

	default:
		break
	}

	return false
}

func IsBoundingBox(points []Point2D) bool {
	if len(points) != 2 {
		return false
	}
	return true
}

func IsPolytgon(points []Point2D) bool {
	if len(points) < 3 {
		return false
	}
	return true
}

func (antn *Annotation) IsValidAnnotation() bool {
	fmt.Println(antn.LabelIDs, antn.CreatorID, antn.ObjectID, antn.ProjectID, antn.StudyID, antn.TaskID,
		antn.IsValidMeta(), antn.IsValidAnnotationType(), antn.IsValidData())
	if antn.LabelIDs == nil || antn.CreatorID == "" || antn.ProjectID == "" ||
		antn.StudyID == "" || antn.TaskID == "" ||
		!antn.IsValidMeta() || !antn.IsValidAnnotationType() || !antn.IsValidData() {
		return false
	}
	return true
}
