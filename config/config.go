package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/Ratio1/edge_sdk_go/pkg/r1fs"
)

var (
	Config         GeneralConfig
	BackendVersion string
	_              = Config
)

type GeneralConfig struct {
	Api                            ApiConfig
	Database                       DatabaseConfig
	Jwt                            JwtConfig
	Mail                           MailConfig
	Verification                   VerificationConfig
	Sumsub                         SumsubConfig
	Didit                          DiditConfig
	MailerLite                     MailerLiteConfig
	AcceptedDomains                AcceptedDomains
	ChainID                        int
	Oblio                          Oblio
	Infura                         Infura
	DeeployApi                     string
	OraclesApi                     string
	NDContractAddress              string
	R1ContractAddress              string
	USDCContractAddress            string
	PoaiManagerAddress             string
	ReaderAddress                  string
	NaeuralAddress                 string
	TeamAddresses                  []string
	BuyLicenseInvoiceCronJobTiming map[string]string
	DailyCronJobTiming             map[string]string
	OfflineNodesCronJobTiming      map[string]string
	MonthlyCronJobTiming           map[string]string
	AdminAddresses                 []string
	EmailTemplatesPath             string
	BuyLimitUSD                    BuyLimitUSDConfig
	ViesApi                        ViesConfig
	InvoiceMessageEmail            string
	Ratio1redirectUrl              Ratio1redirectUrl
	FreeCurrencyApiKey             string
	R1fsClient                     *r1fs.Client
}

type ApiConfig struct {
	Address    string
	DevTesting bool
	AdminKey   string
}

type DatabaseConfig struct {
	User         string
	Password     string
	Host         string
	Port         int
	DbName       string
	MaxOpenConns int
	MaxIdleConns int
	SslMode      string
}

type JwtConfig struct {
	ExpiryMins        int
	Issuer            string
	KeySeedHex        string
	Secret            string
	ConfirmSecret     string
	ConfirmExpiryMins int
}

type MailConfig struct {
	ApiUrl     string
	ApiKey     string
	ConfirmUrl string
	FromEmail  string
}

type SumsubConfig struct {
	ApiUrl             string
	ApiEndpoint        string
	CustomerLevelName  string
	BusinessLevelName  string
	SumsubAppToken     string
	SumsubSecretKey    string
	SumsubJwtSecretKey string
}

type VerificationConfig struct {
	Provider                    string
	LegacySumsubWebhooksEnabled bool
	WorkerPollSeconds           int
	WorkerBatchSize             int
	WorkerMaxAttempts           int
}

type DiditQuestionnaireConfig struct {
	QuestionnaireId               string
	QuestionnaireVersion          int
	FirstNameQuestionId           string
	LastNameQuestionId            string
	CompanyNameQuestionId         string
	TaxIdQuestionId               string
	VatNumberQuestionId           string
	AddressQuestionId             string
	CityQuestionId                string
	PostalCodeQuestionId          string
	StateQuestionId               string
	CountryQuestionId             string
	AdditionalRequiredQuestionIds []string
}

type DiditConfig struct {
	ApiUrl                string
	Environment           string
	ApplicationId         string
	CallbackUrl           string
	KycWorkflowId         string
	KycWorkflowVersion    int
	KybWorkflowId         string
	KybWorkflowVersion    int
	KycQuestionnaire      DiditQuestionnaireConfig
	KybQuestionnaire      DiditQuestionnaireConfig
	ApiKey                string `json:"-"`
	WebhookSecret         string `json:"-"`
	PreviousWebhookSecret string `json:"-"`
}

type MailerLiteConfig struct {
	Url     string
	GroupId string
	ApiKey  string
}

type AcceptedDomains struct {
	Inner []AcceptedDomain
}

type AcceptedDomain struct {
	Domain string
}

type Oblio struct {
	AuthUrl      string
	InvoiceUrl   string
	ClientSecret string
}

type Infura struct {
	ApiUrl string
	Secret string
}

type BuyLimitUSDConfig struct {
	Individual int
	Company    int
}

type ViesConfig struct {
	BaseUrl  string
	User     string
	Password string
}

type Ratio1redirectUrl struct {
	OperatorUrl string
	CspUrl      string
}

func (d DatabaseConfig) Url() string {
	format := "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s"
	return fmt.Sprintf(format, d.Host, d.Port, d.User, d.Password, d.DbName, d.SslMode)
}
func LoadNodes(filePath string) (map[string]string, error) {
	var nodes = make(map[string]string)
	err := core.LoadJsonFile(&nodes, filePath)
	if err != nil {
		return nil, errors.New("error while loading addresses from file: " + err.Error())
	}

	return nodes, nil
}

func LoadConfig(filePath string) (*GeneralConfig, error) {
	cfg := &GeneralConfig{}
	err := core.LoadJsonFile(cfg, filePath)
	if err != nil {
		return nil, errors.New("error while loading config from file: " + err.Error())
	}

	/*	DATABASE ENV VARIABLES	*/
	cfg.Database.DbName = os.Getenv("DATABASE_NAME")
	if cfg.Database.DbName == "" {
		return nil, errors.New("DATABASE_NAME is not set")
	}
	cfg.Database.User = os.Getenv("DATABASE_USER")
	if cfg.Database.User == "" {
		return nil, errors.New("DATABASE_USER is not set")
	}
	cfg.Database.Host = os.Getenv("DATABASE_HOST")
	if cfg.Database.Host == "" {
		return nil, errors.New("DATABASE_HOST is not set")
	}
	portAsString := os.Getenv("DATABASE_PORT")
	portAsInt, err := strconv.Atoi(portAsString)
	if err != nil {
		return nil, errors.New("DATABASE_PORT return error: " + err.Error())
	}
	cfg.Database.Port = portAsInt
	cfg.Database.Password = os.Getenv("DATABASE_PASSWORD")
	if cfg.Database.Password == "" {
		return nil, errors.New("DATABASE_PASSWORD is not set")
	}

	/*	JWT ENV VARIABLES	*/
	cfg.Jwt.KeySeedHex = os.Getenv("JWT_KEYSEED_HEX")
	if cfg.Jwt.KeySeedHex == "" {
		return nil, errors.New("JWT_KEYSEED_HEX is not set")
	}
	cfg.Jwt.Secret = os.Getenv("JWT_SECRET")
	if cfg.Jwt.Secret == "" {
		return nil, errors.New("JWT_SECRET is not set")
	}
	cfg.Jwt.ConfirmSecret = os.Getenv("JWT_CONFIRM_SECRET")
	if cfg.Jwt.ConfirmSecret == "" {
		return nil, errors.New("JWT_CONFIRM_SECRET is not set")
	}

	/*	MAIL ENV VARIABLES	*/
	cfg.Mail.ApiKey = os.Getenv("MAIL_API_KEY")
	if cfg.Mail.ApiKey == "" {
		return nil, errors.New("MAIL_API_KEY is not set")
	}

	/*	SUMSUB ENV VARIABLES */
	cfg.Verification.Provider = strings.ToLower(strings.TrimSpace(os.Getenv("VERIFICATION_PROVIDER")))
	if cfg.Verification.Provider == "" {
		cfg.Verification.Provider = "sumsub"
	}
	if cfg.Verification.Provider != "sumsub" && cfg.Verification.Provider != "didit" {
		return nil, errors.New("VERIFICATION_PROVIDER must be sumsub or didit")
	}
	cfg.Verification.LegacySumsubWebhooksEnabled = true
	if value := strings.TrimSpace(os.Getenv("LEGACY_SUMSUB_WEBHOOKS_ENABLED")); value != "" {
		enabled, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			return nil, errors.New("LEGACY_SUMSUB_WEBHOOKS_ENABLED must be true or false")
		}
		cfg.Verification.LegacySumsubWebhooksEnabled = enabled
	}
	if cfg.Verification.WorkerPollSeconds <= 0 {
		cfg.Verification.WorkerPollSeconds = 2
	}
	if cfg.Verification.WorkerBatchSize <= 0 {
		cfg.Verification.WorkerBatchSize = 20
	}
	if cfg.Verification.WorkerMaxAttempts <= 0 {
		cfg.Verification.WorkerMaxAttempts = 10
	}

	cfg.Sumsub.SumsubAppToken = os.Getenv("SUMSUB_APP_TOKEN")
	if cfg.Sumsub.SumsubAppToken == "" && cfg.Verification.Provider == "sumsub" {
		return nil, errors.New("SUMSUB_APP_TOKEN is not set")
	}
	cfg.Sumsub.SumsubSecretKey = os.Getenv("SUMSUB_SECRET_KEY")
	if cfg.Sumsub.SumsubSecretKey == "" && cfg.Verification.Provider == "sumsub" {
		return nil, errors.New("SUMSUB_SECRET_KEY is not set")
	}
	cfg.Sumsub.SumsubJwtSecretKey = os.Getenv("SUMSUB_JWT_SECRET_KEY")
	if cfg.Sumsub.SumsubJwtSecretKey == "" &&
		(cfg.Verification.Provider == "sumsub" || cfg.Verification.LegacySumsubWebhooksEnabled) {
		return nil, errors.New("SUMSUB_JWT_SECRET_KEY is not set")
	}

	/*
		DIDIT ENV VARIABLES

		Didit remains optional until the provider cutover. The client constructor
		validates these values when the Didit integration is instantiated.
	*/
	cfg.Didit.ApiKey = os.Getenv("DIDIT_API_KEY")
	cfg.Didit.WebhookSecret = os.Getenv("DIDIT_WEBHOOK_SECRET")
	cfg.Didit.PreviousWebhookSecret = os.Getenv("DIDIT_PREVIOUS_WEBHOOK_SECRET")
	if apiUrl := os.Getenv("DIDIT_API_URL"); apiUrl != "" {
		cfg.Didit.ApiUrl = apiUrl
	}
	if workflowId := os.Getenv("DIDIT_KYC_WORKFLOW_ID"); workflowId != "" {
		cfg.Didit.KycWorkflowId = workflowId
	}
	if workflowId := os.Getenv("DIDIT_KYB_WORKFLOW_ID"); workflowId != "" {
		cfg.Didit.KybWorkflowId = workflowId
	}
	if err := overrideDiditConfigFromEnvironment(&cfg.Didit); err != nil {
		return nil, err
	}

	/*	INFURA ENV VARIABLES */
	cfg.Infura.Secret = os.Getenv("INFURA_SECRET")
	if cfg.Infura.Secret == "" {
		return nil, errors.New("INFURA_SECRET is not set")
	}

	if !cfg.Api.DevTesting {
		/*	OBLIO ENV VARIABLES	*/
		cfg.Oblio.ClientSecret = os.Getenv("OBLIO_CLIENT_SECRET")
		if cfg.Oblio.ClientSecret == "" {
			return nil, errors.New("OBLIO_CLIENT_SECRET is not set")
		}

		/*	MAILERLITE ENV VARIABLES */
		cfg.MailerLite.ApiKey = os.Getenv("MAILERLITE_API_KEY")
		if cfg.MailerLite.ApiKey == "" {
			return nil, errors.New("MAILERLITE_API_KEY is not set")
		}
		cfg.MailerLite.GroupId = os.Getenv("MAILERLITE_GROUP_ID")
		if cfg.MailerLite.GroupId == "" {
			return nil, errors.New("MAILERLITE_GROUP_ID is not set")
		}
		adminAddressesString := os.Getenv("ADMIN_ADDRESSES")
		if adminAddressesString == "" {
			return nil, errors.New("ADMIN_ADDRESSES is not set")
		}
		cfg.AdminAddresses = strings.Split(adminAddressesString, ",")

		/* VIES ENV VARIABLES	*/
		cfg.ViesApi.User = os.Getenv("VIES_USER")
		if cfg.ViesApi.User == "" {
			return nil, errors.New("VIES_USER is not set")
		}
		cfg.ViesApi.BaseUrl = os.Getenv("VIES_BASE_URL")
		if cfg.ViesApi.BaseUrl == "" {
			return nil, errors.New("VIES_BASE_URL is not set")
		}
		cfg.ViesApi.Password = os.Getenv("VIES_PASSWORD")
		if cfg.ViesApi.Password == "" {
			return nil, errors.New("VIES_PASSWORD is not set")
		}

		/*FREE CURRENCY API VARIABLES*/
		cfg.FreeCurrencyApiKey = os.Getenv("FREE_CURRENCY_API_KEY")
		if cfg.FreeCurrencyApiKey == "" {
			return nil, errors.New("FREE_CURRENCY_API_KEY is not set")
		}
	}

	/* GENERAL ENV VARIABLES */
	cfg.EmailTemplatesPath = os.Getenv("EMAIL_TEMPLATES_PATH")
	if cfg.EmailTemplatesPath == "" {
		return nil, errors.New("EMAIL_TEMPLATES_PATH is not set")
	}

	cfg.InvoiceMessageEmail = "corina.erhan@ratio1.ai"

	r1fsClient, err := r1fs.NewFromEnv()
	if err != nil {
		return nil, errors.New("error while connecting to r1fs: " + err.Error())
	}

	cfg.R1fsClient = r1fsClient

	return cfg, nil
}

func overrideDiditConfigFromEnvironment(cfg *DiditConfig) error {
	setStringFromEnv(&cfg.Environment, "DIDIT_ENVIRONMENT")
	setStringFromEnv(&cfg.ApplicationId, "DIDIT_APPLICATION_ID")
	setStringFromEnv(&cfg.CallbackUrl, "DIDIT_CALLBACK_URL")
	if err := setPositiveIntFromEnv(&cfg.KycWorkflowVersion, "DIDIT_KYC_WORKFLOW_VERSION"); err != nil {
		return err
	}
	if err := setPositiveIntFromEnv(&cfg.KybWorkflowVersion, "DIDIT_KYB_WORKFLOW_VERSION"); err != nil {
		return err
	}

	if err := overrideDiditQuestionnaireFromEnvironment(
		&cfg.KycQuestionnaire,
		"DIDIT_KYC",
	); err != nil {
		return err
	}
	if err := overrideDiditQuestionnaireFromEnvironment(
		&cfg.KybQuestionnaire,
		"DIDIT_KYB",
	); err != nil {
		return err
	}
	return nil
}

func overrideDiditQuestionnaireFromEnvironment(
	cfg *DiditQuestionnaireConfig,
	prefix string,
) error {
	setStringFromEnv(&cfg.QuestionnaireId, prefix+"_QUESTIONNAIRE_ID")
	if err := setPositiveIntFromEnv(
		&cfg.QuestionnaireVersion,
		prefix+"_QUESTIONNAIRE_VERSION",
	); err != nil {
		return err
	}
	setStringFromEnv(&cfg.FirstNameQuestionId, prefix+"_FIRST_NAME_QUESTION_ID")
	setStringFromEnv(&cfg.LastNameQuestionId, prefix+"_LAST_NAME_QUESTION_ID")
	setStringFromEnv(&cfg.CompanyNameQuestionId, prefix+"_COMPANY_NAME_QUESTION_ID")
	setStringFromEnv(&cfg.TaxIdQuestionId, prefix+"_TAX_ID_QUESTION_ID")
	setStringFromEnv(&cfg.VatNumberQuestionId, prefix+"_VAT_NUMBER_QUESTION_ID")
	setStringFromEnv(&cfg.AddressQuestionId, prefix+"_ADDRESS_QUESTION_ID")
	setStringFromEnv(&cfg.CityQuestionId, prefix+"_CITY_QUESTION_ID")
	setStringFromEnv(&cfg.PostalCodeQuestionId, prefix+"_POSTAL_CODE_QUESTION_ID")
	setStringFromEnv(&cfg.StateQuestionId, prefix+"_STATE_QUESTION_ID")
	setStringFromEnv(&cfg.CountryQuestionId, prefix+"_COUNTRY_QUESTION_ID")
	if value := strings.TrimSpace(os.Getenv(prefix + "_ADDITIONAL_REQUIRED_QUESTION_IDS")); value != "" {
		if questionIds := splitNonEmpty(value); len(questionIds) > 0 {
			cfg.AdditionalRequiredQuestionIds = questionIds
		}
	}
	return nil
}

func setStringFromEnv(destination *string, name string) {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		*destination = value
	}
}

func setPositiveIntFromEnv(destination *int, name string) error {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}
	*destination = parsed
	return nil
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.TrimSpace(part); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func (c *GeneralConfig) GetBuyLicenseInvoiceCronJobTiming(nodeAddress string) (string, bool) {
	nodeTiming, found := c.BuyLicenseInvoiceCronJobTiming[nodeAddress]
	return nodeTiming, found
}

func (c *GeneralConfig) GetDailyCronJobTiming(nodeAddress string) (string, bool) {
	nodeTiming, found := c.DailyCronJobTiming[nodeAddress]
	return nodeTiming, found
}

func (c *GeneralConfig) GetMonthlyCronJobTiming(nodeAddress string) (string, bool) {
	nodeTiming, found := c.MonthlyCronJobTiming[nodeAddress]
	return nodeTiming, found
}

func (c *GeneralConfig) GetOfflineNodesCronJobTiming(nodeAddress string) (string, bool) {
	nodeTiming, found := c.OfflineNodesCronJobTiming[nodeAddress]
	return nodeTiming, found
}
