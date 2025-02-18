package model

type SumsubEvent struct {
	ApplicantID    string       `json:"applicantId"`
	InspectionID   string       `json:"inspectionId"`
	CorrelationID  string       `json:"correlationId"`
	LevelName      string       `json:"levelName"`
	ExternalUserID string       `json:"externalUserId"`
	ApplicantType  string       `json:"applicantType,omitempty"`
	Type           string       `json:"type"`
	ReviewResult   ReviewResult `json:"reviewResult,omitempty"`
	SandboxMode    bool         `json:"sandboxMode"`
	ReviewStatus   string       `json:"reviewStatus"`
	CreatedAt      string       `json:"createdAt"`
	CreatedAtMs    string       `json:"createdAtMs"`
	ClientID       string       `json:"clientId"`
}

type ReviewResult struct {
	ModerationComment string   `json:"moderationComment"`
	ClientComment     string   `json:"clientComment"`
	ReviewAnswer      string   `json:"reviewAnswer"`
	RejectLabels      []string `json:"rejectLabels"`
	ReviewRejectType  string   `json:"reviewRejectType"`
}
type ApplicantProfile struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	//ClientID          *string       `json:"clientId"`
	//InspectionID      *string       `json:"inspectionId"`
	ExternalUserID string `json:"externalUserId"`
	//SourceKey         *string       `json:"sourceKey"`
	//Info      ApplicantInfo `json:"info"`
	FixedInfo ApplicantInfo `json:"fixedInfo"`
	//Email             *string       `json:"email"`
	//Phone             *string       `json:"phone"`
	//ApplicantPlatform *string       `json:"applicantPlatform"`
	//IPCountry         *string       `json:"ipCountry"`
	//AuthCode          *string       `json:"authCode"`
	//Lang              *string       `json:"lang"`
	Type string `json:"type"`
	//Tags              *[]string     `json:"tags"`
}

type ApplicantInfo struct {
	CompanyInfo *CompanyInfo `json:"companyInfo,omitempty"`
	FirstName   string       `json:"firstName"`
	FirstNameEn string       `json:"firstNameEn"`
	//MiddleName     string       `json:"middleName"`
	//MiddleNameEn   string       `json:"middleNameEn"`
	LastName   string `json:"lastName"`
	LastNameEn string `json:"lastNameEn"`
	//LegalName      string       `json:"legalName"`
	//Gender         string       `json:"gender,omitempty"`
	//DOB            string       `json:"dob"`
	//PlaceOfBirth   string       `json:"placeOfBirth"`
	//CountryOfBirth string       `json:"countryOfBirth"`
	//StateOfBirth   string       `json:"stateOfBirth"`
	//Country        string       `json:"country"`
	//Nationality    string       `json:"nationality"`
	Addresses []Address `json:"addresses"`
	Tin       string    `json:"tin"`
}

type CompanyInfo struct {
	CompanyName          string   `json:"companyName"`
	RegistrationNumber   string   `json:"registrationNumber"`
	Country              string   `json:"country"`
	LegalAddress         string   `json:"legalAddress"`
	IncorporatedOn       string   `json:"incorporatedOn"`
	Type                 string   `json:"type"`
	Email                string   `json:"email"`
	Phone                string   `json:"phone"`
	ControlScheme        string   `json:"controlScheme"`
	ApplicantPosition    string   `json:"applicantPosition"`
	TaxID                string   `json:"taxId"`
	RegistrationLocation string   `json:"registrationLocation"`
	Website              string   `json:"website"`
	PostalAddress        string   `json:"postalAddress"`
	Address              *Address `json:"address,omitempty"`
	NoUBOs               bool     `json:"noUBOs"`
	NoShareholders       bool     `json:"noShareholders"`
}

type Address struct {
	Street  string `json:"street"`
	Town    string `json:"town"`
	State   string `json:"state"`
	Country string `json:"country"`
	//PostalCode *string `json:"postalCode"`
}
