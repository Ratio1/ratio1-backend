package storage

import (
	"gorm.io/gorm"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
)

func GetKycByEmail(email string) (*model.Kyc, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var acc model.Kyc
	txRead := db.Find(&acc, "email = ?", email)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &acc, true, nil
}

func GetKycByApplicantID(applicantId string) (*model.Kyc, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var acc model.Kyc
	txRead := db.Find(&acc, "applicant_id = ?", applicantId)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &acc, true, nil
}

func GetKycByUuid(uuid uuid.UUID) (*model.Kyc, bool, error) {
	db, err := GetDB()
	if err != nil {
		return nil, false, err
	}

	var acc model.Kyc
	txRead := db.Find(&acc, "uuid = ?", uuid)
	if txRead.Error != nil {
		return nil, false, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, false, nil
	}

	return &acc, true, nil
}

func CreateOrUpdateKyc(kyc *model.Kyc) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	var existingKyc model.Kyc
	err = db.Where("email = ?", kyc.Email).First(&existingKyc).Error
	if err == gorm.ErrRecordNotFound {
		err = db.Create(kyc).Error
		if err != nil {
			return err
		}
	} else if err == nil {
		err = db.Model(&existingKyc).Where("email = ?", existingKyc.Email).Updates(kyc).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func GetAllUsersEmails() ([]string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var kycs []model.Kyc
	txRead := db.Find(&kycs, "email != ''")
	if txRead.Error != nil {
		return nil, txRead.Error
	}
	if txRead.RowsAffected == 0 {
		return nil, nil
	}

	var emails []string
	for _, kyc := range kycs {
		if kyc.Email != "" {
			emails = append(emails, kyc.Email)
		}
	}

	return emails, nil
}
