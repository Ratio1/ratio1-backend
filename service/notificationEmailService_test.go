package service

import (
	"testing"

	"github.com/NaeuralEdgeProtocol/ratio1-backend/model"
	"github.com/stretchr/testify/require"
)

func TestNotificationEmailsForAccount_DeduplicatesConfirmedRecipients(t *testing.T) {
	primaryEmail := " owner@example.com "
	additionalEmail := "OWNER@example.com"

	account := &model.Account{
		Email:          &primaryEmail,
		EmailConfirmed: true,
	}
	notificationEmail := &model.AccountNotificationEmail{
		Email:          &additionalEmail,
		EmailConfirmed: true,
	}

	require.Equal(
		t,
		[]string{"owner@example.com"},
		notificationEmailsForAccount(account, notificationEmail),
	)
}

func TestNotificationEmailsForAccount_SkipsUnconfirmedAdditionalEmail(t *testing.T) {
	primaryEmail := "owner@example.com"
	additionalEmail := "ops@example.com"

	account := &model.Account{
		Email:          &primaryEmail,
		EmailConfirmed: true,
	}
	notificationEmail := &model.AccountNotificationEmail{
		Email:          &additionalEmail,
		EmailConfirmed: false,
	}

	require.Equal(
		t,
		[]string{"owner@example.com"},
		notificationEmailsForAccount(account, notificationEmail),
	)
}

func TestConfirmPendingNotificationEmail_ReplacesActiveEmailOnlyWhenPendingMatches(t *testing.T) {
	activeEmail := "old-ops@example.com"
	notificationEmail := &model.AccountNotificationEmail{
		Email:          &activeEmail,
		EmailConfirmed: true,
		PendingEmail:   "new-ops@example.com",
	}

	confirmed, err := confirmPendingNotificationEmail(notificationEmail, "new-ops@example.com")

	require.NoError(t, err)
	require.True(t, confirmed)
	require.NotNil(t, notificationEmail.Email)
	require.Equal(t, "new-ops@example.com", *notificationEmail.Email)
	require.True(t, notificationEmail.EmailConfirmed)
	require.Empty(t, notificationEmail.PendingEmail)
}

func TestConfirmPendingNotificationEmail_LeavesActiveEmailWhenTokenDoesNotMatch(t *testing.T) {
	activeEmail := "old-ops@example.com"
	notificationEmail := &model.AccountNotificationEmail{
		Email:          &activeEmail,
		EmailConfirmed: true,
		PendingEmail:   "new-ops@example.com",
	}

	confirmed, err := confirmPendingNotificationEmail(notificationEmail, "other@example.com")

	require.NoError(t, err)
	require.False(t, confirmed)
	require.NotNil(t, notificationEmail.Email)
	require.Equal(t, "old-ops@example.com", *notificationEmail.Email)
	require.True(t, notificationEmail.EmailConfirmed)
	require.Equal(t, "new-ops@example.com", notificationEmail.PendingEmail)
}
