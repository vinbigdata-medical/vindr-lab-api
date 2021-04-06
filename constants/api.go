package constants

const (
	ENV = "API_ENV"

	ParamID           = "id"
	ParamIDs          = "ids"
	ParamLabelID      = "label_id"
	PramAnnotationID  = "annotation_id"
	ParamObjectID     = "object_id"
	ParamStudyID      = "study_id"
	ParamProjectID    = "project_id"
	ParamSessionID    = "session_id"
	ParamLabelGroupID = "label_group_id"
	ParamStudyStatus  = "study_status"
	ParamTaskStatus   = "task_status"
	ParamAuth         = "Authorization"

	ParamLimit       = "_limit"
	ParamOffset      = "_offset"
	ParamSort        = "_sort"
	ParamSearch      = "_search"
	ParamAggregation = "_agg"
	ParamRole        = "_role"

	EventCreate = "CREATED"
	EventUpdate = "UPDATED"
	EventDelete = "DELETED"

	DefaultLimit  = 100
	DefaultOffset = 0

	TaskStatusNew       = "NEW"
	TaskStatusDoing     = "DOING"
	TaskStatusCompleted = "COMPLETED"

	TaskTypeAnnotate = "ANNOTATE"
	TaskTypeReview   = "REVIEW"

	StudyStatusUnassigned = "UNASSIGNED"
	StudyStatusAssigned   = "ASSIGNED"
	StudyStatusCompleted  = "COMPLETED"

	SessionItemTypeTask  = "TASK"
	SessionItemTypeStudy = "STUDY"

	LabelTypeImpression = "IMPRESSION"
	LabelTypeFinding    = "FINDING"

	LabelScopeStudy  = "STUDY"
	LabelScopeSeries = "SERIES"
	LabelScopeImage  = "IMAGE"

	AntnTypeTag     = "TAG"
	AntnTypeBox     = "BOUNDING_BOX"
	AntnTypePolygon = "POLYGON"
	AntnTypeMask    = "MASK"
	AntnType3DBox   = "BOUNDING_BOX_3D"

	ObjectTypeStudy  = "STUDY"
	ObjectTypeSeries = "SERIES"
	ObjectTypeImage  = "IMAGE"

	ProjWorkflowSingle   = "SINGLE"
	ProjWorkflowTriangle = "TRIANGLE"

	ProjRoleAnnotator    = "ANNOTATOR"
	ProjRoleReviewer     = "REVIEWER"
	ProjRoleProjectOwner = "PROJECT_OWNER"

	LabelSelectTypeNone     = ""
	LabelSelectTypeRadio    = "RADIO"
	LabelSelectTypeCheckbox = "CHECKBOX"

	ExportStatusPending = "PENDING"
	ExportStatusDone    = "DONE"

	ASSIGN_STRATEGY_ALL     = "ALL"
	ASSIGN_STRATEGY_EQUALLY = "EQUALLY"

	ASSIGN_SOURCE_SELECTED = "SELECTED"
	ASSIGN_SOURCE_FILE     = "FILE"
	ASSIGN_SOURCE_SEARCH   = "SEARCH"
)
