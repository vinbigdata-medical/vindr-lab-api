package entities

type MetaData struct {
	StudyInstanceUID        string `json:"study_instance_uid,omitempty"`
	SeriesInstanceUID       string `json:"series_instance_uid,omitempty"`
	SOPInstanceUID          string `json:"sop_instance_uid,omitempty"`
	MaskedStudyInstanceUID  string `json:"masked_study_instance_uid,omitempty"`
	MaskedSeriesInstanceUID string `json:"masked_series_instance_uid,omitempty"`
	MaskedSOPInstanceUID    string `json:"masked_sop_instance_uid,omitempty"`
}
