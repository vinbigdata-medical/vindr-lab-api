package study

import (
	"encoding/json"

	"vindr-lab-api/constants"
)

var mapStudyStatus = map[string]int{
	constants.StudyStatusUnassigned: 0,
	constants.StudyStatusAssigned:   1,
	constants.StudyStatusCompleted:  2,
}

type Study struct {
	ID           string     `json:"id"`
	Modified     int64      `json:"modified"`
	TimeInserted int64      `json:"time_inserted"`
	Code         string     `json:"code"`
	Status       string     `json:"status"`
	ProjectID    string     `json:"project_id"`
	CreatorID    string     `json:"creator_id"`
	DICOMTags    *DICOMTags `json:"dicom_tags,omitempty"`
}

type DICOMTags struct {
	AccessionNumber                    []string                   `json:"AccessionNumber,omitempty"`
	BitsAllocated                      []string                   `json:"BitsAllocated,omitempty"`
	BitsStored                         []string                   `json:"BitsStored,omitempty"`
	BodyPartExamined                   []string                   `json:"BodyPartExamined,omitempty"`
	Columns                            []string                   `json:"Columns,omitempty"`
	ContentDate                        []string                   `json:"ContentDate,omitempty"`
	ContentTime                        []string                   `json:"ContentTime,omitempty"`
	ConversionType                     []string                   `json:"ConversionType,omitempty"`
	DeidentificationMethod             []string                   `json:"DeidentificationMethod,omitempty"`
	Exposure                           []string                   `json:"Exposure,omitempty"`
	ExposureTime                       []string                   `json:"ExposureTime,omitempty"`
	HighBit                            []string                   `json:"HighBit,omitempty"`
	ImageAndFluoroscopyAreaDoseProduct []string                   `json:"ImageAndFluoroscopyAreaDoseProduct,omitempty"`
	ImageType                          []string                   `json:"ImageType,omitempty"`
	ImagerPixelSpacing                 []string                   `json:"ImagerPixelSpacing,omitempty"`
	InstanceNumber                     []string                   `json:"InstanceNumber,omitempty"`
	KVP                                []string                   `json:"KVP,omitempty"`
	Manufacturer                       []string                   `json:"Manufacturer,omitempty"`
	Modality                           []string                   `json:"Modality,omitempty"`
	NumberOfFrames                     []string                   `json:"NumberOfFrames,omitempty"`
	PatientBirthDate                   []string                   `json:"PatientBirthDate,omitempty"`
	PatientBreedDescription            []string                   `json:"PatientBreedDescription,omitempty"`
	PatientID                          []string                   `json:"PatientID,omitempty"`
	PatientIdentityRemoved             []string                   `json:"PatientIdentityRemoved,omitempty"`
	PatientName                        []string                   `json:"PatientName,omitempty"`
	PatientSex                         []string                   `json:"PatientSex,omitempty"`
	PatientSpeciesDescription          []string                   `json:"PatientSpeciesDescription,omitempty"`
	PhotometricInterpretation          []string                   `json:"PhotometricInterpretation,omitempty"`
	PixelData                          []interface{}              `json:"PixelData,omitempty"`
	PixelRepresentation                []string                   `json:"PixelRepresentation,omitempty"`
	PixelSpacing                       []string                   `json:"PixelSpacing,omitempty"`
	ReferencedImageSequence            *[]ReferencedImageSequence `json:"ReferencedImageSequence,omitempty"`
	ReferringPhysicianName             []string                   `json:"ReferringPhysicianName,omitempty"`
	RescaleIntercept                   []string                   `json:"RescaleIntercept,omitempty"`
	RescaleSlope                       []string                   `json:"RescaleSlope,omitempty"`
	RescaleType                        []string                   `json:"RescaleType,omitempty"`
	Rows                               []string                   `json:"Rows,omitempty"`
	SOPClassUID                        []string                   `json:"SOPClassUID,omitempty"`
	SOPInstanceUID                     []string                   `json:"SOPInstanceUID,omitempty"`
	SamplesPerPixel                    []string                   `json:"SamplesPerPixel,omitempty"`
	SeriesInstanceUID                  []string                   `json:"SeriesInstanceUID,omitempty"`
	SeriesNumber                       []string                   `json:"SeriesNumber,omitempty"`
	SpecificCharacterSet               []string                   `json:"SpecificCharacterSet,omitempty"`
	StudyDate                          []string                   `json:"StudyDate,omitempty"`
	StudyID                            []string                   `json:"StudyID,omitempty"`
	StudyInstanceUID                   []string                   `json:"StudyInstanceUID,omitempty"`
	StudyTime                          []string                   `json:"StudyTime,omitempty"`
	WindowCenter                       []string                   `json:"WindowCenter,omitempty"`
	WindowCenterWidthExplanation       []string                   `json:"WindowCenterWidthExplanation,omitempty"`
	WindowWidth                        []string                   `json:"WindowWidth,omitempty"`
	XRayTubeCurrent                    []string                   `json:"XRayTubeCurrent,omitempty"`
}
type ReferencedImageSequence struct {
	ReferencedSOPClassUID    string `json:"ReferencedSOPClassUID,omitempty"`
	ReferencedSOPInstanceUID string `json:"ReferencedSOPInstanceUID,omitempty"`
}

func IsValidStatus(studyStatus string) bool {
	_, found := mapStudyStatus[studyStatus]
	return found
}

func (study *Study) String() string {
	b, _ := json.Marshal(study)
	return string(b)
}
