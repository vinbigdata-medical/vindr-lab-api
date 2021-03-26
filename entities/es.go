package entities

type ESReturn struct {
	ScrollID     string                  `json:"_scroll_id`
	Took         int                     `json:"took"`
	TimedOut     bool                    `json:"timed_out"`
	Shards       Shards                  `json:"_shards"`
	Hits         HitsGLobal              `json:"hits"`
	Aggregations *map[string]Aggregation `json:"aggregations,omitempty"`
}
type Shards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}
type Total struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}
type Data struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Meta struct {
	StudyInstanceUID  string `json:"study_instance_uid"`
	SeriesInstanceUID string `json:"series_instance_uid"`
	SopInstanceUID    string `json:"sop_instance_uid"`
}
type Source struct {
	ID        string `json:"id"`
	LabelID   string `json:"label_id"`
	ObjectID  string `json:"object_id"`
	CreatorID string `json:"creator_id"`
	Note      string `json:"note"`
	Data      []Data `json:"data"`
	Type      string `json:"type"`
	Event     string `json:"event"`
	Created   int64  `json:"created"`
	Meta      Meta   `json:"meta"`
}
type HitsLocal struct {
	Index  string                 `json:"_index"`
	Type   string                 `json:"_type"`
	ID     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}
type HitsGLobal struct {
	Total    Total       `json:"total"`
	MaxScore float64     `json:"max_score"`
	Hits     []HitsLocal `json:"hits"`
}

type Buckets struct {
	Key      string `json:"key"`
	DocCount int    `json:"doc_count"`
}
type Aggregation struct {
	DocCountErrorUpperBound int       `json:"doc_count_error_upper_bound"`
	SumOtherDocCount        int       `json:"sum_other_doc_count"`
	Buckets                 []Buckets `json:"buckets"`
}

type ESError struct {
	Error struct {
		RootCause []struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"root_cause"`
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"error"`
	Status int `json:"status"`
}

type ESBulkResponse struct {
	Errors bool `json:"errors"`
	Items  []struct {
		Index struct {
			ID     string `json:"_id"`
			Result string `json:"result"`
			Status int    `json:"status"`
			Error  struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
				Cause  struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"caused_by"`
			} `json:"error"`
		} `json:"index"`
	} `json:"items"`
}
