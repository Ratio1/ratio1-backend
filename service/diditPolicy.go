package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/biter777/countries"
	"github.com/google/uuid"
)

type DiditQuestionnaireFields struct {
	FirstName   uuid.UUID
	LastName    uuid.UUID
	CompanyName uuid.UUID
	TaxId       uuid.UUID
	VatNumber   uuid.UUID
	Address     uuid.UUID
	City        uuid.UUID
	PostalCode  uuid.UUID
	State       uuid.UUID
	Country     uuid.UUID
}

type DiditVerificationPolicy struct {
	ApprovalPolicy      DiditApprovalPolicy
	QuestionnaireFields DiditQuestionnaireFields
}

type DiditPolicySet struct {
	Kyc DiditVerificationPolicy
	Kyb DiditVerificationPolicy
}

func NewDiditPolicySet(cfg config.DiditConfig) (DiditPolicySet, error) {
	kyc, err := newDiditVerificationPolicy(
		cfg.KycWorkflowId,
		cfg.KycWorkflowVersion,
		model.DiditSessionKindUser,
		cfg.KycQuestionnaire,
		[]string{"ID_VERIFICATION", "LIVENESS", "FACE_MATCH", "POA", "QUESTIONNAIRE", "AML"},
	)
	if err != nil {
		return DiditPolicySet{}, fmt.Errorf("invalid Didit KYC policy: %w", err)
	}
	kyb, err := newDiditVerificationPolicy(
		cfg.KybWorkflowId,
		cfg.KybWorkflowVersion,
		model.DiditSessionKindBusiness,
		cfg.KybQuestionnaire,
		[]string{"KYB_REGISTRY", "AML", "KYB_DOCUMENTS", "KYB_KEY_PEOPLE", "QUESTIONNAIRE"},
	)
	if err != nil {
		return DiditPolicySet{}, fmt.Errorf("invalid Didit KYB policy: %w", err)
	}
	kyc.ApprovalPolicy.ResidenceCountryQuestionId = kyc.QuestionnaireFields.Country
	kyc.ApprovalPolicy.RestrictedResidenceCountries = Ratio1RestrictedResidenceCountries()
	kyb.ApprovalPolicy.RequireCompanyRegistrationNumber = true
	return DiditPolicySet{Kyc: kyc, Kyb: kyb}, nil
}

func (p DiditPolicySet) ForApplicantType(applicantType string) (DiditVerificationPolicy, error) {
	switch applicantType {
	case model.IndividualCustomer:
		return p.Kyc, nil
	case model.BusinessCustomer:
		return p.Kyb, nil
	default:
		return DiditVerificationPolicy{}, errors.New("unsupported applicant type")
	}
}

func newDiditVerificationPolicy(
	workflowIdValue string,
	workflowVersion int,
	sessionKind model.DiditSessionKind,
	questionnaire config.DiditQuestionnaireConfig,
	requiredFeatures []string,
) (DiditVerificationPolicy, error) {
	workflowId, err := uuid.Parse(workflowIdValue)
	if err != nil || workflowVersion <= 0 {
		return DiditVerificationPolicy{}, errors.New("workflow id and version are required")
	}
	questionnaireId, err := uuid.Parse(questionnaire.QuestionnaireId)
	if err != nil || questionnaire.QuestionnaireVersion <= 0 {
		return DiditVerificationPolicy{}, errors.New("questionnaire id and version are required")
	}

	fields := DiditQuestionnaireFields{}
	if sessionKind == model.DiditSessionKindUser {
		fields.FirstName, err = requiredDiditQuestionId(questionnaire.FirstNameQuestionId, "first name")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.LastName, err = requiredDiditQuestionId(questionnaire.LastNameQuestionId, "last name")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.TaxId, err = requiredDiditQuestionId(questionnaire.TaxIdQuestionId, "tax id")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.Address, err = requiredDiditQuestionId(questionnaire.AddressQuestionId, "address")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.City, err = requiredDiditQuestionId(questionnaire.CityQuestionId, "city")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.PostalCode, err = requiredDiditQuestionId(questionnaire.PostalCodeQuestionId, "postal code")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.State, err = requiredDiditQuestionId(questionnaire.StateQuestionId, "state")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
		fields.Country, err = requiredDiditQuestionId(questionnaire.CountryQuestionId, "country")
		if err != nil {
			return DiditVerificationPolicy{}, err
		}
	}

	requiredItems := make([]uuid.UUID, 0, 8+len(questionnaire.AdditionalRequiredQuestionIds))
	if sessionKind == model.DiditSessionKindUser {
		requiredItems = append(
			requiredItems,
			fields.TaxId,
			fields.Address,
			fields.City,
			fields.PostalCode,
			fields.State,
			fields.Country,
		)
		requiredItems = append(requiredItems, fields.FirstName, fields.LastName)
	}
	for _, value := range questionnaire.AdditionalRequiredQuestionIds {
		id, parseErr := requiredDiditQuestionId(value, "additional required")
		if parseErr != nil {
			return DiditVerificationPolicy{}, parseErr
		}
		requiredItems = append(requiredItems, id)
	}
	seenQuestionIds := make(map[uuid.UUID]struct{}, len(requiredItems))
	for _, id := range requiredItems {
		if _, duplicate := seenQuestionIds[id]; duplicate {
			return DiditVerificationPolicy{}, fmt.Errorf(
				"questionnaire question id %s is configured more than once",
				id,
			)
		}
		seenQuestionIds[id] = struct{}{}
	}

	return DiditVerificationPolicy{
		QuestionnaireFields: fields,
		ApprovalPolicy: DiditApprovalPolicy{
			WorkflowId:                 workflowId,
			WorkflowVersion:            workflowVersion,
			SessionKind:                sessionKind,
			RequiredFeatures:           requiredFeatures,
			QuestionnaireId:            questionnaireId,
			QuestionnaireVersion:       questionnaire.QuestionnaireVersion,
			RequiredQuestionnaireItems: requiredItems,
		},
	}, nil
}

func requiredDiditQuestionId(value, label string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s question id is required", label)
	}
	return id, nil
}

func MapDiditDecisionToUserInfo(
	decision model.DiditDecision,
	policy DiditVerificationPolicy,
	blockchainAddress, email string,
) (*model.UserInfo, string, bool, error) {
	if EvaluateDiditApprovalEvidence(
		decision,
		policy.ApprovalPolicy.WorkflowVersion,
		policy.ApprovalPolicy,
	) != DiditEvidenceEligible {
		return nil, "", false, errors.New("Didit approval evidence is incomplete")
	}
	answers := diditQuestionnaireAnswers(decision)
	value := func(id uuid.UUID) (string, error) {
		answer := strings.TrimSpace(answers[id])
		if answer == "" {
			return "", errors.New("required Didit questionnaire answer is missing")
		}
		return answer, nil
	}

	userInfo := &model.UserInfo{
		BlockchainAddress: blockchainAddress,
		Email:             email,
	}
	country := ""
	viesRegistered := false
	switch decision.SessionKind {
	case model.DiditSessionKindUser:
		address, addressErr := value(policy.QuestionnaireFields.Address)
		if addressErr != nil {
			return nil, "", false, addressErr
		}
		city, cityErr := value(policy.QuestionnaireFields.City)
		if cityErr != nil {
			return nil, "", false, cityErr
		}
		if _, postalErr := value(policy.QuestionnaireFields.PostalCode); postalErr != nil {
			return nil, "", false, postalErr
		}
		state, stateErr := value(policy.QuestionnaireFields.State)
		if stateErr != nil {
			return nil, "", false, stateErr
		}
		var countryErr error
		country, countryErr = value(policy.QuestionnaireFields.Country)
		if countryErr != nil {
			return nil, "", false, countryErr
		}
		country = strings.ToUpper(country)
		if len(country) != 3 {
			return nil, "", false, errors.New("billing country must use ISO alpha-3")
		}
		firstName, nameErr := value(policy.QuestionnaireFields.FirstName)
		if nameErr != nil {
			return nil, "", false, nameErr
		}
		lastName, nameErr := value(policy.QuestionnaireFields.LastName)
		if nameErr != nil {
			return nil, "", false, nameErr
		}
		if len(decision.IdVerifications) != 1 ||
			!strings.EqualFold(strings.TrimSpace(decision.IdVerifications[0].FirstName), firstName) ||
			!strings.EqualFold(strings.TrimSpace(decision.IdVerifications[0].LastName), lastName) {
			return nil, "", false, errors.New("billing name does not match verified identity")
		}
		taxId, taxErr := value(policy.QuestionnaireFields.TaxId)
		if taxErr != nil {
			return nil, "", false, taxErr
		}
		userInfo.Name = &firstName
		userInfo.Surname = &lastName
		userInfo.IdentificationCode = taxId
		userInfo.Address = address
		userInfo.City = city
		userInfo.State = state
		userInfo.Country = country
	case model.DiditSessionKindBusiness:
		company, companyErr := diditApprovedCompany(decision)
		if companyErr != nil {
			return nil, "", false, companyErr
		}
		submitted := diditKybSubmittedCompany(company.UserProvidedData)
		companyName := firstNonEmpty(company.CompanyName, submitted.CompanyName)
		taxId := firstNonEmpty(company.TaxNumber, submitted.TaxNumber)
		vatNumber := firstNonEmpty(company.VatNumber, submitted.VatNumber)
		address := firstNonEmpty(company.RegisteredAddress, submitted.LegalAddress)
		city := firstNonEmpty(company.LocationOfRegistration, submitted.City)
		country, companyErr = normalizeDiditCountryCode(
			firstNonEmpty(company.CountryCode, submitted.CountryCode),
		)
		if companyErr != nil {
			return nil, "", false, companyErr
		}
		region := firstNonEmpty(submitted.Region, submitted.State)
		if region == "" {
			region = city
		}
		if companyName == "" ||
			taxId == "" ||
			address == "" ||
			city == "" ||
			region == "" ||
			len(country) != 3 {
			return nil, "", false, errors.New("Didit native KYB invoicing data is incomplete")
		}
		identificationCode := taxId
		if isUeCountry(country) {
			if vatNumber == "" {
				return nil, "", false, errors.New("Didit native KYB VAT number is required for an EU company")
			}
			identificationCode = vatNumber
			viesRegistered = strings.EqualFold(company.VatValidationStatus, "valid")
		}
		userInfo.CompanyName = &companyName
		userInfo.IdentificationCode = identificationCode
		userInfo.Address = address
		userInfo.City = city
		userInfo.State = region
		userInfo.Country = country
		userInfo.IsCompany = true
	default:
		return nil, "", false, errors.New("unsupported Didit session kind")
	}
	if err := ValidateData(*userInfo); err != nil {
		return nil, "", false, err
	}
	return userInfo, country, viesRegistered, nil
}

type diditCompanyProjection struct {
	CompanyName            string          `json:"company_name"`
	CountryCode            string          `json:"country_code"`
	RegistrationNumber     string          `json:"registration_number"`
	RegisteredAddress      string          `json:"registered_address"`
	LocationOfRegistration string          `json:"location_of_registration"`
	TaxNumber              string          `json:"tax_number"`
	VatNumber              string          `json:"vat_number"`
	VatValidationStatus    string          `json:"vat_validation_status"`
	UserProvidedData       json.RawMessage `json:"user_provided_data"`
}

type diditKybSubmittedCompanyProjection struct {
	CompanyName  string `json:"company_name"`
	CountryCode  string `json:"country_code"`
	Region       string `json:"region"`
	State        string `json:"state"`
	City         string `json:"city"`
	LegalAddress string `json:"legal_address"`
	TaxNumber    string `json:"tax_number"`
	VatNumber    string `json:"vat_number"`
}

func diditKybSubmittedCompany(raw json.RawMessage) diditKybSubmittedCompanyProjection {
	var data diditKybSubmittedCompanyProjection
	if len(raw) == 0 || json.Unmarshal(raw, &data) != nil {
		return diditKybSubmittedCompanyProjection{}
	}
	return data
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeDiditCountryCode(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if len(normalized) != 2 && len(normalized) != 3 {
		return "", errors.New("Didit company country code must use ISO 3166-1 alpha-2 or alpha-3")
	}
	for _, character := range normalized {
		if character < 'A' || character > 'Z' {
			return "", errors.New("Didit company country code must use ISO 3166-1 alpha-2 or alpha-3")
		}
	}
	code := countries.ByName(normalized)
	if code == countries.Unknown {
		return "", errors.New("Didit company country code is not a valid ISO 3166-1 code")
	}
	return code.Alpha3(), nil
}

func diditApprovedCompany(decision model.DiditDecision) (diditCompanyProjection, error) {
	if len(decision.RegistryChecks) != 1 {
		return diditCompanyProjection{}, errors.New("Didit company registry evidence is missing")
	}
	var company diditCompanyProjection
	if err := json.Unmarshal(decision.RegistryChecks[0].Company, &company); err != nil ||
		strings.TrimSpace(company.CompanyName) == "" ||
		strings.TrimSpace(company.CountryCode) == "" ||
		strings.TrimSpace(company.RegistrationNumber) == "" {
		return diditCompanyProjection{}, errors.New("Didit company registry evidence is incomplete")
	}
	return company, nil
}

func diditQuestionnaireAnswers(decision model.DiditDecision) map[uuid.UUID]string {
	answers := make(map[uuid.UUID]string)
	for _, response := range decision.QuestionnaireResponses {
		for _, section := range response.Sections {
			for _, item := range section.Items {
				if item.Answer == nil {
					continue
				}
				switch {
				case item.Answer.Value != nil:
					answers[item.Uuid] = *item.Answer.Value
				case item.Answer.Text != nil:
					answers[item.Uuid] = *item.Answer.Text
				}
			}
		}
	}
	return answers
}
