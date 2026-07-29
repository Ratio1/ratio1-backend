package storage

import (
	"errors"
	"strings"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrKycIdentityConflict = errors.New("kyc email belongs to a different uuid")

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

	return createOrUpdateKyc(db, kyc)
}

func createOrUpdateKyc(db *gorm.DB, kyc *model.Kyc) error {
	if kyc == nil {
		return errors.New("kyc is nil")
	}
	if kyc.Uuid == uuid.Nil {
		return errors.New("kyc uuid is required")
	}
	if strings.TrimSpace(kyc.Email) == "" || strings.TrimSpace(kyc.Email) != kyc.Email {
		return errors.New("kyc email must be non-empty and must not contain surrounding whitespace")
	}
	if kyc.KycStatus == "" {
		return errors.New("kyc status is required")
	}
	if kyc.ReceiveUpdates == nil {
		return errors.New("kyc receive-updates preference is required")
	}

	txUpdate := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.Assignments(kycUpdateAssignments(kyc)),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "kycs.uuid = EXCLUDED.uuid"},
		}},
	}).Create(kyc)
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		return ErrKycIdentityConflict
	}

	return nil
}

func kycUpdateAssignments(kyc *model.Kyc) map[string]any {
	return map[string]any{
		"applicant_id":          kyc.ApplicantId,
		"applicant_type":        kyc.ApplicantType,
		"verification_provider": kyc.VerificationProvider,
		"kyc_status":            kyc.KycStatus,
		"last_updated":          kyc.LastUpdated,
		"is_active":             kyc.IsActive,
		"has_been_deleted":      kyc.HasBeenDeleted,
		"receive_updates":       kyc.ReceiveUpdates,
		"country":               kyc.Country,
		"vies_registered":       kyc.ViesRegistered,
	}
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
