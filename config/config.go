package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ElrondNetwork/elrond-go-core/core"
)

var (
	Config         GeneralConfig
	BackendVersion = "1.0.0"
	_              = Config
)

type GeneralConfig struct {
	Api                ApiConfig
	Database           DatabaseConfig
	Jwt                JwtConfig
	Mail               MailConfig
	Sumsub             SumsubConfig
	MailerLite         MailerLiteConfig
	AcceptedDomains    AcceptedDomains
	ChainID            int
	Oblio              Oblio
	Infura             Infura
	NDContractAddress  string
	CronJobTiming      map[string]string
	AdminAddresses     []string
	EmailTemplatesPath string
	BuyLimitUSD        BuyLimitUSDConfig
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
	AuthUrl        string
	InvoiceUrl     string
	ClientSecret   string
	EventSignature string
	Abi            string
}

type Infura struct {
	ApiUrl string
	Secret string
}

type BuyLimitUSDConfig struct {
	Individual int
	Company    int
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
	cfg.Oblio.Abi = `[{
		"anonymous": false,
		"inputs": [
		  {
			"indexed": true,
			"internalType": "address",
			"name": "to",
			"type": "address"
		  },
		  {
			"indexed": true,
			"internalType": "bytes32",
			"name": "invoiceUuid",
			"type": "bytes32"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "tokenCount",
			"type": "uint256"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "unitUsdPrice",
			"type": "uint256"
		  },
		  {
			"indexed": false,
			"internalType": "uint256",
			"name": "totalR1Spent",
			"type": "uint256"
		  }
		],
		"name": "LicensesCreated",
		"type": "event"
  }]`

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
	cfg.Sumsub.SumsubAppToken = os.Getenv("SUMSUB_APP_TOKEN")
	if cfg.Sumsub.SumsubAppToken == "" {
		return nil, errors.New("SUMSUB_APP_TOKEN is not set")
	}
	cfg.Sumsub.SumsubSecretKey = os.Getenv("SUMSUB_SECRET_KEY")
	if cfg.Sumsub.SumsubSecretKey == "" {
		return nil, errors.New("SUMSUB_SECRET_KEY is not set")
	}
	cfg.Sumsub.SumsubJwtSecretKey = os.Getenv("SUMSUB_JWT_SECRET_KEY")
	if cfg.Sumsub.SumsubJwtSecretKey == "" {
		return nil, errors.New("SUMSUB_JWT_SECRET_KEY is not set")
	}

	if !cfg.Api.DevTesting {
		/*	OBLIO ENV VARIABLES	*/
		cfg.Oblio.ClientSecret = os.Getenv("OBLIO_CLIENT_SECRET")
		if cfg.Oblio.ClientSecret == "" {
			return nil, errors.New("OBLIO_CLIENT_SECRET is not set")
		}
		cfg.Oblio.EventSignature = os.Getenv("OBLIO_EVENT_SIGNATURE")
		if cfg.Oblio.EventSignature == "" {
			return nil, errors.New("OBLIO_EVENT_SIGNATURE is not set")
		}

		/*	INFURA ENV VARIABLES */
		cfg.Infura.Secret = os.Getenv("INFURA_SECRET")
		if cfg.Infura.Secret == "" {
			return nil, errors.New("INFURA_SECRET is not set")
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
	}

	/* GENERAL ENV VARIABLES */
	cfg.EmailTemplatesPath = os.Getenv("EMAIL_TEMPLATES_PATH")
	if cfg.EmailTemplatesPath == "" {
		return nil, errors.New("EMAIL_TEMPLATES_PATH is not set")
	}

	return cfg, nil
}

func (c *GeneralConfig) GetCronJobTiming(nodeAddress string) (string, bool) {
	nodeTiming, found := c.CronJobTiming[nodeAddress]
	return nodeTiming, found
}
