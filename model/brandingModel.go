package model

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/config"
	"github.com/Ratio1/ratio1_sdk_go/pkg/r1fs"
)

type Branding struct {
	UserAddress string  `gorm:"type:varchar(66);primaryKey" json:"userAddress"`
	Name        string  `gorm:"type:text"  json:"name"`
	Description string  `gorm:"type:text"  json:"description"`
	Links       string  `gorm:"type:jsonb;default:'{}'"  json:"links"`
	CidLogo     *string `gorm:"type:text"  json:"cidLogo"`
}

func (b *Branding) GetLogoBase64() ([]byte, error) {
	if b.CidLogo == nil {
		return nil, errors.New("no cid found")
	}
	data, _, err := config.Config.R1fsClient.GetFileBase64(context.Background(), *b.CidLogo, "")
	if err != nil {
		return nil, errors.New("error while retrieving file from r1fs: " + err.Error())
	}
	return data, nil
}

func (b *Branding) SetLogoBase64(logoReader io.Reader, filename string) error {
	cid, err := config.Config.R1fsClient.AddFileBase64(context.Background(), logoReader, &r1fs.DataOptions{Filename: filename})
	if err != nil {
		return errors.New("error while uploading file to r1fs: " + err.Error())
	}
	b.CidLogo = &cid
	return nil
}

func (b *Branding) GetLinks() (map[string]string, error) {
	platformsLinks := make(map[Platform]string)
	err := json.Unmarshal([]byte(b.Links), &platformsLinks)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for k, v := range platformsLinks {
		result[k.String()] = v
	}
	return result, nil
}

func (b *Branding) SetLinks(links string) error {
	platformLinks := make(map[string]string)
	err := json.Unmarshal([]byte(links), &platformLinks)
	if err != nil {
		return err
	}

	result := make(map[Platform]string)
	for k, v := range platformLinks {
		if !parsePlatform(k).isValid() {
			return errors.New("unknown platform")
		}
		result[parsePlatform(k)] = v
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	b.Links = string(resultBytes)
	return nil
}

const (
	X        = "X"
	Linkedin = "Linkedin"
	Website  = "Website"
)

type Platform int

const (
	PlatformUnknown Platform = iota
	PlatformX
	PlatformLinkedin
	PlatformWebsite
)

var platformNames = map[Platform]string{
	PlatformX:        X,
	PlatformLinkedin: Linkedin,
	PlatformWebsite:  Website,
}

var platformValues = map[string]Platform{
	X:        PlatformX,
	Linkedin: PlatformLinkedin,
	Website:  PlatformWebsite,
}

func (p Platform) String() string {
	if name, ok := platformNames[p]; ok {
		return name
	}
	return "Unknown"
}

func parsePlatform(s string) Platform {
	if p, ok := platformValues[strings.ToLower(s)]; ok {
		return p
	}
	return PlatformUnknown
}

func (p Platform) isValid() bool {
	_, ok := platformNames[p]
	return ok
}

func (p Platform) GetPlatforms() []string {
	var result []string
	for _, v := range platformNames {
		result = append(result, v)
	}
	return result
}
