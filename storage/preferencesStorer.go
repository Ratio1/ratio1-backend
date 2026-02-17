package storage

import (
	"errors"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetPreferenceByAddress(tx *gorm.DB, userAddress string) (*model.Preference, error) {
	return getPreferenceByAddress(tx, userAddress, false)
}

func GetPreferenceByAddressForUpdate(tx *gorm.DB, userAddress string) (*model.Preference, error) {
	return getPreferenceByAddress(tx, userAddress, true)
}

func getPreferenceByAddress(tx *gorm.DB, userAddress string, forUpdate bool) (*model.Preference, error) {
	exec, err := getExecutor(tx)
	if err != nil {
		return nil, err
	}

	query := exec.Where("user_address = ?", userAddress)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var pref model.Preference
	txRead := query.Take(&pref)
	if txRead.Error != nil {
		if errors.Is(txRead.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, txRead.Error
	}

	return &pref, nil
}

func CreatePreference(tx *gorm.DB, pref *model.Preference) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txCreate := exec.Create(pref)
	if txCreate.Error != nil {
		return txCreate.Error
	}
	if txCreate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func UpdatePreference(tx *gorm.DB, pref *model.Preference) error {
	exec, err := getExecutor(tx)
	if err != nil {
		return err
	}

	txUpdate := exec.Save(pref)
	if txUpdate.Error != nil {
		return txUpdate.Error
	}
	if txUpdate.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
