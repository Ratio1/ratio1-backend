package model

import (
	"encoding/json"
	"errors"
	"strings"
)

type Branding struct {
	UserAddress string `gorm:"type:varchar(66);primaryKey" json:"userAddress"`
	Name        string `gorm:"type:text"  json:"name"`
	Description string `gorm:"type:text"  json:"description"`
	Links       string `gorm:"type:jsonb;default:'{}'"  json:"links"`
	//TODO add Logo
}

func (b *Branding) GetLinks() (map[string]string, error) { //type
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

type Platform int

const (
	PlatformUnknown Platform = iota
	PlatformX
	PlatformLinkedin
	PlatformWebsite
)

var platformNames = map[Platform]string{
	PlatformX:        "X",
	PlatformLinkedin: "Linkedin",
	PlatformWebsite:  "Website",
}

var platformValues = map[string]Platform{
	"x":        PlatformX,
	"linkedin": PlatformLinkedin,
	"website":  PlatformWebsite,
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
